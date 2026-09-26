-include .env
export

# Keep Go commands usable in restricted build and agent environments where the
# default user cache is readable but cannot be trimmed. An explicit GOCACHE from
# the caller still takes precedence.
GOCACHE ?= /tmp/rakam-go-build
export GOCACHE

.PHONY: run test vet check migrate-up migrate-down seed user-create user-reset-password user-seed-categories

run:
	go run ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

check: test vet

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

seed:
	go run ./cmd/seed

user-create:
	go run ./cmd/user create --email "$(EMAIL)" --name "$(NAME)"

user-reset-password:
	go run ./cmd/user reset-password --email "$(EMAIL)"

user-seed-categories:
	go run ./cmd/user seed-categories --email "$(EMAIL)"
