.PHONY: migrate-create, clean-sqlc, sqlc, fmt, lint, test

migrate-create:
	migrate create -ext sql -dir db/migrations $(name)  # Create migration file with make migrate-create name=<name>

clean-sqlc:
	rm -rf internal/db/sqlc

sqlc: clean-sqlc
	sqlc generate

fmt:
	gofmt -s -w .

lint:
	golangci-lint run

test:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out -v ./...
	go tool cover -html=coverage.out
