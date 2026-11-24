.PHONY: up down migrate-up migrate-down sqlc test run build

DATABASE_URL ?= postgres://hotel:hotel_dev_pass@localhost:5432/hotel_management?sslmode=disable

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migration -database "$(DATABASE_URL)" up; \
	else \
		docker run --rm --network host -v "$(CURDIR)/db/migration:/migrations" migrate/migrate:v4.17.0 -path=/migrations -database "$(DATABASE_URL)" up; \
	fi

migrate-down:
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path db/migration -database "$(DATABASE_URL)" down 1; \
	else \
		docker run --rm --network host -v "$(CURDIR)/db/migration:/migrations" migrate/migrate:v4.17.0 -path=/migrations -database "$(DATABASE_URL)" down 1; \
	fi

sqlc:
	sqlc generate

test:
	go test -race ./...

build:
	go build ./cmd/api ./cmd/bootstrap

run:
	go run ./cmd/api
