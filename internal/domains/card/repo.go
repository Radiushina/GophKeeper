package card

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

var cardsTable = goqu.T("cards")

var cardCols = []any{
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

type CardsRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

func NewRepository(db *pgxpool.Pool) *CardsRepo {
	return &CardsRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

func (r *CardsRepo) Create(ctx context.Context, n Card) (Card, error) {
	query, args, err := r.builder.Insert(cardsTable).
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
		Returning(cardCols...).
		ToSQL()
	if err != nil {
		return Card{}, fmt.Errorf("build insert card: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Card{}, wrapWriteErr(err, "create card")
	}
	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Card])
	if err != nil {
		return Card{}, wrapWriteErr(err, "create card")
	}
	return created, nil
}

func (r *CardsRepo) Update(ctx context.Context, n Card) (Card, error) {
	query, args, err := r.builder.Update(cardsTable).
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
		Returning(cardCols...).
		ToSQL()
	if err != nil {
		return Card{}, fmt.Errorf("build update card: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Card{}, fmt.Errorf("update card: %w", err)
	}
	updated, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Card])
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Card{}, fmt.Errorf("update card: %w", err)
	}
	return Card{}, r.missOrConflict(ctx, n.UserID, n.ID)
}

func (r *CardsRepo) SoftDelete(ctx context.Context, userID, id uuid.UUID) (Card, error) {
	query, args, err := r.builder.Update(cardsTable).
		Prepared(true).
		Set(goqu.Record{
			"deleted_at": goqu.L("now()"),
			"updated_at": goqu.L("now()"),
			"version":    goqu.L("version + 1"),
		}).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		Where(goqu.C("deleted_at").IsNull()).
		Returning(cardCols...).
		ToSQL()
	if err != nil {
		return Card{}, fmt.Errorf("build delete card: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Card{}, fmt.Errorf("delete card: %w", err)
	}
	deleted, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Card])
	if err == nil {
		return deleted, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Card{}, fmt.Errorf("delete card: %w", err)
	}

	existing, getErr := r.Get(ctx, userID, id)
	if getErr != nil {
		return Card{}, getErr
	}
	return existing, nil
}

func (r *CardsRepo) Get(ctx context.Context, userID, id uuid.UUID) (Card, error) {
	query, args, err := r.builder.From(cardsTable).
		Select(cardCols...).
		Prepared(true).
		Where(goqu.Ex{"id": id, "user_id": userID}).
		ToSQL()
	if err != nil {
		return Card{}, fmt.Errorf("build get card: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return Card{}, fmt.Errorf("query get card: %w", err)
	}
	n, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Card])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, fmt.Errorf("get card: %w", err)
	}
	return n, nil
}

func (r *CardsRepo) List(ctx context.Context, userID uuid.UUID, since *time.Time) ([]Card, error) {
	q := r.builder.From(cardsTable).
		Select(cardCols...).
		Prepared(true).
		Where(goqu.Ex{"user_id": userID})
	if since != nil {
		q = q.Where(goqu.C("updated_at").Gte(*since))
	} else {
		q = q.Where(goqu.C("deleted_at").IsNull())
	}
	query, args, err := q.Order(goqu.C("updated_at").Asc()).ToSQL()
	if err != nil {
		return nil, fmt.Errorf("build list cards: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list cards: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Card])
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	return items, nil
}

func (r *CardsRepo) missOrConflict(ctx context.Context, userID, id uuid.UUID) error {
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
