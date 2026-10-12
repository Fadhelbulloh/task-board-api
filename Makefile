.PHONY: run test test-race vet up down migrate-up migrate-down migrate-new

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

up:
	docker compose up --build

down:
	docker compose down -v

# Migrations also run automatically when the app starts. These targets are
# for running them by hand with the golang-migrate CLI.
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

# make migrate-new name=add_something
migrate-new:
	migrate create -ext sql -dir migrations -seq $(name)
