//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"product-notifications/internal/products"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDBName = "test_products"
	testDBUser = "test"
	testDBPass = "test"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17-alpine"),
		postgres.WithDatabase(testDBName),
		postgres.WithUsername(testDBUser),
		postgres.WithPassword(testDBPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	migrationsPath := migrationsDir(t)
	m, err := migrate.New("file://"+migrationsPath, connStr)
	if err != nil {
		t.Fatalf("init migrate: %v", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("run migrations: %v", err)
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		t.Fatalf("close migrate source: %v", srcErr)
	}
	if dbErr != nil {
		t.Fatalf("close migrate db: %v", dbErr)
	}

	return db
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "migrations", "products")
}

func TestPostgresRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgres(db)
	ctx := context.Background()

	t.Run("creates product and returns it", func(t *testing.T) {
		p, err := repo.Create(ctx, "Laptop")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ID == 0 {
			t.Fatal("expected non-zero ID")
		}
		if p.Name != "Laptop" {
			t.Fatalf("want name Laptop, got %q", p.Name)
		}
		if p.CreatedAt.IsZero() {
			t.Fatal("expected non-zero created_at")
		}
	})

	t.Run("auto-increments IDs", func(t *testing.T) {
		p1, _ := repo.Create(ctx, "A")
		p2, _ := repo.Create(ctx, "B")
		if p2.ID <= p1.ID {
			t.Fatalf("expected p2.ID > p1.ID, got %d <= %d", p2.ID, p1.ID)
		}
	})
}

func TestPostgresRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgres(db)
	ctx := context.Background()

	t.Run("deletes existing product", func(t *testing.T) {
		p, _ := repo.Create(ctx, "ToDelete")
		if err := repo.Delete(ctx, p.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		count, _ := repo.Count(ctx)
		list, _ := repo.List(ctx, 100, 0)
		for _, item := range list {
			if item.ID == p.ID {
				t.Fatalf("product %d should have been deleted, but still in list (count=%d)", p.ID, count)
			}
		}
	})

	t.Run("returns ErrNotFound for non-existent ID", func(t *testing.T) {
		err := repo.Delete(ctx, 999999)
		if !errors.Is(err, products.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("delete is idempotent — second call returns ErrNotFound", func(t *testing.T) {
		p, _ := repo.Create(ctx, "DeleteTwice")
		_ = repo.Delete(ctx, p.ID)
		err := repo.Delete(ctx, p.ID)
		if !errors.Is(err, products.ErrNotFound) {
			t.Fatalf("want ErrNotFound on second delete, got %v", err)
		}
	})
}

func TestPostgresRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgres(db)
	ctx := context.Background()

	names := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	for _, name := range names {
		if _, err := repo.Create(ctx, name); err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
	}

	t.Run("returns all with large limit", func(t *testing.T) {
		list, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != len(names) {
			t.Fatalf("want %d items, got %d", len(names), len(list))
		}
	})

	t.Run("ordered by id DESC", func(t *testing.T) {
		list, _ := repo.List(ctx, 100, 0)
		for i := 1; i < len(list); i++ {
			if list[i].ID >= list[i-1].ID {
				t.Fatalf("expected descending order, got id %d after %d", list[i].ID, list[i-1].ID)
			}
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		list, _ := repo.List(ctx, 2, 0)
		if len(list) != 2 {
			t.Fatalf("want 2 items, got %d", len(list))
		}
	})

	t.Run("respects offset", func(t *testing.T) {
		all, _ := repo.List(ctx, 100, 0)
		page2, _ := repo.List(ctx, 2, 2)
		if len(page2) != 2 {
			t.Fatalf("want 2 items, got %d", len(page2))
		}
		if page2[0].ID != all[2].ID {
			t.Fatalf("offset mismatch: want id %d, got %d", all[2].ID, page2[0].ID)
		}
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		list, _ := repo.List(ctx, 10, 1000)
		if list == nil {
			t.Fatal("expected non-nil empty slice")
		}
		if len(list) != 0 {
			t.Fatalf("want 0 items, got %d", len(list))
		}
	})
}

func TestPostgresRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgres(db)
	ctx := context.Background()

	t.Run("empty table returns zero", func(t *testing.T) {
		count, err := repo.Count(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Fatalf("want 0, got %d", count)
		}
	})

	t.Run("count reflects inserts and deletes", func(t *testing.T) {
		p1, _ := repo.Create(ctx, "X")
		_, _ = repo.Create(ctx, "Y")

		count, _ := repo.Count(ctx)
		if count != 2 {
			t.Fatalf("want 2 after inserts, got %d", count)
		}

		_ = repo.Delete(ctx, p1.ID)
		count, _ = repo.Count(ctx)
		if count != 1 {
			t.Fatalf("want 1 after delete, got %d", count)
		}
	})
}

func TestPostgresRepository_Health(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgres(db)

	if err := repo.Health(); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}
