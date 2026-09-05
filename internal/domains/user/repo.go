package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	usersTable = goqu.T("users")
)

type UsersRepo struct {
	db      *pgxpool.Pool
	builder goqu.DialectWrapper
}

func NewRepository(db *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{
		db:      db,
		builder: goqu.Dialect("postgres"),
	}
}

func (r *UsersRepo) CreateUser(ctx context.Context, u User) (User, error) {
	query, args, err := r.builder.Insert(usersTable).
		Prepared(true).
		Rows(goqu.Record{
			"login":           u.Login,
			"password":        u.Password,
			"kdf_salt":        u.KdfSalt,
			"kdf_memory":      u.KdfMemory,
			"kdf_iterations":  u.KdfIterations,
			"kdf_parallelism": u.KdfParallelism,
			"kdf_version":     u.KdfVersion,
			"protected_key":   u.ProtectedKey,
			"key_hash":        u.KeyHash,
		}).
		Returning(
			goqu.C("id"),
			goqu.C("login"),
			goqu.C("password"),
			goqu.C("kdf_salt"),
			goqu.C("kdf_memory"),
			goqu.C("kdf_iterations"),
			goqu.C("kdf_parallelism"),
			goqu.C("kdf_version"),
			goqu.C("protected_key"),
			goqu.C("key_hash"),
		).
		ToSQL()
	if err != nil {
		return User{}, fmt.Errorf("failed to build insert user query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return User{}, wrapUserInsertErr(err)
	}

	created, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		return User{}, wrapUserInsertErr(err)
	}
	return created, nil
}

func (r *UsersRepo) GetByLogin(ctx context.Context, login string) (User, error) {
	query, args, err := r.builder.From(usersTable).
		Select(
			goqu.C("id"),
			goqu.C("login"),
			goqu.C("password"),
			goqu.C("kdf_salt"),
			goqu.C("kdf_memory"),
			goqu.C("kdf_iterations"),
			goqu.C("kdf_parallelism"),
			goqu.C("kdf_version"),
			goqu.C("protected_key"),
			goqu.C("key_hash"),
		).
		Prepared(true).
		Where(goqu.Ex{"login": login}).
		ToSQL()
	if err != nil {
		return User{}, fmt.Errorf("failed to build get user query: %w", err)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return User{}, fmt.Errorf("failed to query get user: %w", err)
	}

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("%w: %w", ErrUserNotFound, err)
		}
		return User{}, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

func wrapUserInsertErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return fmt.Errorf("%w: %w", ErrUserAlreadyExists, err)
	}
	return fmt.Errorf("failed to create user: %w", err)
}
