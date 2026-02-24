package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"product-notifications/internal/products"
)

const healthCheckTimeout = 2 * time.Second

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgres(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, name string) (products.Product, error) {
	query := `
		INSERT INTO products (name)
		VALUES ($1)
		RETURNING id, name, created_at
	`

	var p products.Product
	if err := r.db.QueryRowContext(ctx, query, name).Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
		return products.Product{}, fmt.Errorf("insert product: %w", err)
	}
	return p, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM products WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete product %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return products.ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]products.Product, error) {
	query := `
		SELECT id, name, created_at
		FROM products
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	list := make([]products.Product, 0)
	for rows.Next() {
		var p products.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		list = append(list, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}

	return list, nil
}

func (r *PostgresRepository) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count products: %w", err)
	}
	return total, nil
}

func (r *PostgresRepository) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	return r.db.PingContext(ctx)
}
