package note

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres" // registers the postgres dialect
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var notesTable = goqu.T("notes")

var noteCols = []any{
	goqu.C("id"),
	goqu.C("user_id"),
	goqu.C("version"),
	goqu.C("nonce"),
	goqu.C("meta"),
	goqu.C("ciphertext"),
	goqu.C("ciphertext_sha256"),
	goqu.C("deleted_at"),
	goqu.C("created_at"),
	goqu.C("updated_at"),
}

// NotesRepo persists notes in Postgres.
type NotesRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

// NewRepository builds a Postgres notes repository.
func NewRepository(db *pgxpool.Pool) *NotesRepo {
	return &NotesRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

// Create inserts a note. Duplicate id is ErrConflict.
func (r *NotesRepo) Create(ctx context.Context, n Note) (Note, error) {
	query, args, err := r.builder.Insert(notesTable).
		Prepared(true).
		Rows(goqu.Record{
			"id":                n.ID,
			"user_id":           n.UserID,
			"version":           n.Version,
			"nonce":             n.Nonce,
			"meta":              n.Meta,
			"ciphertext":        n.Ciphertext,
			"ciphertext_sha256": n.CiphertextSHA256,
		}).
		Returning(noteCols...).
		ToSQL()
	if err != nil {
		return Note{}, fmt.Errorf("build insert note: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Note{}, wrapWriteErr(err, "create note")
	}
	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Note])
	if err != nil {
		return Note{}, wrapWriteErr(err, "create note")
	}
	return created, nil
}

// Update applies a new blob when version matches and the row is live.
func (r *NotesRepo) Update(ctx context.Context, n Note) (Note, error) {
	query, args, err := r.builder.Update(notesTable).
		Prepared(true).
		Set(goqu.Record{
			"version":           goqu.L("version + 1"),
			"nonce":             n.Nonce,
			"meta":              n.Meta,
			"ciphertext":        n.Ciphertext,
			"ciphertext_sha256": n.CiphertextSHA256,
			"updated_at":        goqu.L("now()"),
		}).
		Where(goqu.Ex{
			"id":      n.ID,
			"user_id": n.UserID,
			"version": n.Version,
		}).
		Where(goqu.C("deleted_at").IsNull()).
		Returning(noteCols...).
		ToSQL()
	if err != nil {
		return Note{}, fmt.Errorf("build update note: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	updated, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Note])
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	return Note{}, r.missOrConflict(ctx, n.UserID, n.ID)
}

// SoftDelete sets deleted_at. A second call returns the existing tombstone.
func (r *NotesRepo) SoftDelete(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	query, args, err := r.builder.Update(notesTable).
		Prepared(true).
		Set(goqu.Record{
			"deleted_at": goqu.L("now()"),
			"updated_at": goqu.L("now()"),
			"version":    goqu.L("version + 1"),
		}).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		Where(goqu.C("deleted_at").IsNull()).
		Returning(noteCols...).
		ToSQL()
	if err != nil {
		return Note{}, fmt.Errorf("build delete note: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Note{}, fmt.Errorf("delete note: %w", err)
	}
	deleted, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Note])
	if err == nil {
		return deleted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Note{}, fmt.Errorf("delete note: %w", err)
	}

	existing, getErr := r.Get(ctx, userID, id)
	if getErr != nil {
		return Note{}, getErr
	}
	return existing, nil
}

// Get loads a note owned by userID.
func (r *NotesRepo) Get(ctx context.Context, userID, id uuid.UUID) (Note, error) {
	query, args, err := r.builder.From(notesTable).
		Select(noteCols...).
		Prepared(true).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		ToSQL()
	if err != nil {
		return Note{}, fmt.Errorf("build get note: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Note{}, fmt.Errorf("query get note: %w", err)
	}
	n, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Note])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Note{}, ErrNotFound
		}
		return Note{}, fmt.Errorf("get note: %w", err)
	}
	return n, nil
}

// List returns the owner's notes. A nil since means live rows only.
func (r *NotesRepo) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Note, error) {
	q := r.builder.From(notesTable).
		Select(noteCols...).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID})
	if since != nil {
		q = q.Where(goqu.C("updated_at").Gte(*since))
	} else {
		q = q.Where(goqu.C("deleted_at").IsNull())
	}
	query, args, err := q.Order(goqu.C("updated_at").Asc()).ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build list notes: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list notes: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Note])
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	return items, nil
}

func (r *NotesRepo) missOrConflict(ctx context.Context, userID, id uuid.UUID) error {
	existing, err := r.Get(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing.DeletedAt != nil {
		return ErrNotFound
	}
	return ErrConflict
}

func wrapWriteErr(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return fmt.Errorf("%w: %w", ErrConflict, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}
