// Package db abre o pool do Postgres, aplica migrations e oferece transações.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open abre o pool e confirma que o banco responde.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: DATABASE_URL inválida: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: banco inacessível: %w", err)
	}
	return pool, nil
}

func withProvider(pool *pgxpool.Pool, fn func(*goose.Provider) error) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return fmt.Errorf("db: migrations: %w", err)
	}
	return fn(provider)
}

// Migrate aplica todas as migrations pendentes.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return withProvider(pool, func(p *goose.Provider) error {
		_, err := p.Up(ctx)
		return err
	})
}

// MigrateDown desfaz todas as migrations (usado nos testes de migração).
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	return withProvider(pool, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
}

// MigrateDownTo desfaz as migrations acima de version (usado nos testes de backfill).
func MigrateDownTo(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	return withProvider(pool, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, version)
		return err
	})
}

// WithTx executa fn numa transação: commit se fn devolver nil, rollback caso contrário.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, pool, fn)
}

// IsUniqueViolation informa se err é violação de unicidade (da constraint indicada, se não vazia).
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}

// IsInvalidInput informa se err é entrada com sintaxe inválida (ex.: UUID malformado vindo da URL).
func IsInvalidInput(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
