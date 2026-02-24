.PHONY: up down test test-integration test-all run-products run-notifications lint swagger migrate-create migrate-up migrate-down migrate-status

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/products?sslmode=disable
MIGRATIONS_DIR = migrations/products

up:
	docker compose up --build

down:
	docker compose down -v

test:
	CGO_ENABLED=0 go test ./...

test-integration:
	go test -tags=integration -v -count=1 -timeout=300s ./internal/products/repository/

test-all: test test-integration

run-products:
	go run ./cmd/products

run-notifications:
	go run ./cmd/notifications

lint:
	golangci-lint run --build-tags=integration ./...

swagger:
	swag init -g cmd/products/main.go -o docs

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=add_description_column"; exit 1; fi
	@NEXT=$$(ls -1 $(MIGRATIONS_DIR)/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	NEXT=$$((NEXT + 1)); \
	SEQ=$$(printf "%06d" $$NEXT); \
	touch "$(MIGRATIONS_DIR)/$${SEQ}_$(NAME).up.sql"; \
	touch "$(MIGRATIONS_DIR)/$${SEQ}_$(NAME).down.sql"; \
	echo "Created: $(MIGRATIONS_DIR)/$${SEQ}_$(NAME).up.sql"; \
	echo "Created: $(MIGRATIONS_DIR)/$${SEQ}_$(NAME).down.sql"

migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-status:
	migrate -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version
