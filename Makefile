.PHONY: help migrate-create clean-sqlc sqlc fmt lint test t clean coverage coverage-view coverage-func

COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html
COVER_EXCLUDE := /internal/db/sqlc

.DEFAULT_GOAL := help

help:
	@echo "Available targets:"
	@echo "  make test            - Run all tests"
	@echo "  make coverage         - Generate coverage report"
	@echo "  make coverage-view    - Open coverage in browser"
	@echo "  make clean            - Remove coverage files"

migrate-create:
	@migrate create -ext sql -dir db/migrations $(name)

clean-sqlc:
	@rm -rf internal/db/sqlc

sqlc: clean-sqlc
	@sqlc generate

fmt:
	@gofmt -s -w .

lint:
	@golangci-lint run

test t:
	@go test ./... -v

clean:
	@rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)

coverage: clean
	@echo "Running tests with coverage..."
	@PKGS=$$(go list ./... | grep -v '$(COVER_EXCLUDE)'); \
	go test $$PKGS \
		-coverprofile=$(COVERAGE_OUT) \
		-covermode=atomic \
		-coverpkg=$$(echo $$PKGS | tr ' ' ',')
	@go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo ""
	@echo "Coverage Summary:"
	@go tool cover -func=$(COVERAGE_OUT) | tail -n 1
	@echo ""
	@echo "Detailed report: $(COVERAGE_HTML)"

coverage-func: clean
	@echo "Running tests with coverage..."
	@PKGS=$$(go list ./... | grep -v '$(COVER_EXCLUDE)'); \
	go test $$PKGS \
		-coverprofile=$(COVERAGE_OUT) \
		-covermode=atomic \
		-coverpkg=$$(echo $$PKGS | tr ' ' ',')
	@go tool cover -func=$(COVERAGE_OUT)
