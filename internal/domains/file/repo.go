package file

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

var filesTable = goqu.T("files")

var fileCols = []any{
	goqu.C("id"),
	goqu.C("user_id"),
	goqu.C("version"),
	goqu.C("nonce"),
	goqu.C("meta"),
	goqu.C("name"),
	goqu.C("chunk_count"),
	goqu.C("byte_size"),
	goqu.C("ciphertext_sha256"),
	goqu.C("deleted_at"),
	goqu.C("created_at"),
	goqu.C("updated_at"),
}

// FilesRepo persists file metadata in Postgres.
type FilesRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

// NewRepository builds a Postgres files repository.
func NewRepository(db *pgxpool.Pool) *FilesRepo {
	return &FilesRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

// Create inserts file metadata. Duplicate id is ErrConflict.
func (r *FilesRepo) Create(ctx context.Context, n File) (File, error) {
	query, args, err := r.builder.Insert(filesTable).
		Prepared(true).
		Rows(goqu.Record{
			"id":                n.ID,
			"user_id":           n.UserID,
			"version":           n.Version,
			"nonce":             n.Nonce,
			"meta":              n.Meta,
			"name":              n.Name,
			"chunk_count":       n.ChunkCount,
			"byte_size":         n.ByteSize,
			"ciphertext_sha256": n.CiphertextSHA256,
		}).
		Returning(fileCols...).
		ToSQL()
	if err != nil {
		return File{}, fmt.Errorf("build insert file: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return File{}, wrapWriteErr(err, "create file")
	}
	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if err != nil {
		return File{}, wrapWriteErr(err, "create file")
	}
	return created, nil
}

// Update applies a new blob when version matches and the row is live.
func (r *FilesRepo) Update(ctx context.Context, n File) (File, error) {
	query, args, err := r.builder.Update(filesTable).
		Prepared(true).
		Set(goqu.Record{
			"version":           goqu.L("version + 1"),
			"nonce":             n.Nonce,
			"meta":              n.Meta,
			"name":              n.Name,
			"chunk_count":       n.ChunkCount,
			"byte_size":         n.ByteSize,
			"ciphertext_sha256": n.CiphertextSHA256,
			"updated_at":        goqu.L("now()"),
		}).
		Where(goqu.Ex{
			"id":      n.ID,
			"user_id": n.UserID,
			"version": n.Version,
		}).
		Where(goqu.C("deleted_at").IsNull()).
		Returning(fileCols...).
		ToSQL()
	if err != nil {
		return File{}, fmt.Errorf("build update file: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return File{}, fmt.Errorf("update file: %w", err)
	}
	updated, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return File{}, fmt.Errorf("update file: %w", err)
	}
	return File{}, r.missOrConflict(ctx, n.UserID, n.ID)
}

// SoftDelete sets deleted_at. A second call returns the existing tombstone.
func (r *FilesRepo) SoftDelete(ctx context.Context, userID, id uuid.UUID) (File, error) {
	query, args, err := r.builder.Update(filesTable).
		Prepared(true).
		Set(goqu.Record{
			"deleted_at": goqu.L("now()"),
			"updated_at": goqu.L("now()"),
			"version":    goqu.L("version + 1"),
		}).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		Where(goqu.C("deleted_at").IsNull()).
		Returning(fileCols...).
		ToSQL()
	if err != nil {
		return File{}, fmt.Errorf("build delete file: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return File{}, fmt.Errorf("delete file: %w", err)
	}
	deleted, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if err == nil {
		return deleted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return File{}, fmt.Errorf("delete file: %w", err)
	}

	existing, getErr := r.Get(ctx, userID, id)
	if getErr != nil {
		return File{}, getErr
	}
	return existing, nil
}

// Get loads file metadata owned by userID.
func (r *FilesRepo) Get(ctx context.Context, userID, id uuid.UUID) (File, error) {
	query, args, err := r.builder.From(filesTable).
		Select(fileCols...).
		Prepared(true).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		ToSQL()
	if err != nil {
		return File{}, fmt.Errorf("build get file: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return File{}, fmt.Errorf("query get file: %w", err)
	}
	n, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[File])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return File{}, ErrNotFound
		}
		return File{}, fmt.Errorf("get file: %w", err)
	}
	return n, nil
}

// List returns the owner's files. A nil since means live rows only.
func (r *FilesRepo) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]File, error) {
	q := r.builder.From(filesTable).
		Select(fileCols...).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID})
	if since != nil {
		q = q.Where(goqu.C("updated_at").Gte(*since))
	} else {
		q = q.Where(goqu.C("deleted_at").IsNull())
	}
	query, args, err := q.Order(goqu.C("updated_at").Asc()).ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build list files: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list files: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[File])
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return items, nil
}

func (r *FilesRepo) missOrConflict(ctx context.Context, userID, id uuid.UUID) error {
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
