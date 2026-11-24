include .env
export

DB_URL=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/updog?sslmode=disable
MIGRATIONS_DIR=db/migrations

.PHONY: migrate-create migrate-up migrate-down sqlc

# Create a new migration file: make migrate-create NAME=add_users
migrate-create:
	goose -dir $(MIGRATIONS_DIR) create $(NAME) sql

# Apply migrations
migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" up

# Rollback last migration
migrate-down:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" down

# Check migration status
migrate-status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DB_URL)" status

# Generate Go code via SQLC
sqlc:
	sqlc generate
