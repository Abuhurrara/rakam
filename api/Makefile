-include .env
export

# Keep Go commands usable in restricted build and agent environments where the
# default user cache is readable but cannot be trimmed. An explicit GOCACHE from
# the caller still takes precedence.
GOCACHE ?= /tmp/rakam-go-build
export GOCACHE

.PHONY: run test vet check migrate-up migrate-down seed

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
