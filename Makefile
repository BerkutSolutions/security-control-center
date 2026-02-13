GO_PACKAGES := ./...
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: help fmt fmt-check vet test lint ci migrate

help:
	@echo "Targets:"
	@echo "  fmt        - format Go files in-place"
	@echo "  fmt-check  - fail if any Go file is not gofmt-formatted"
	@echo "  vet        - run go vet"
	@echo "  test       - run go test"
	@echo "  lint       - run fmt-check and vet"
	@echo "  migrate    - apply database migrations"
	@echo "  ci         - run lint and test"

fmt:
	@gofmt -w $(GO_FILES)

fmt-check:
	@out=$$(gofmt -l $(GO_FILES)); \
	if [ -n "$$out" ]; then \
		echo "Unformatted Go files:"; \
		echo "$$out"; \
		exit 1; \
	fi

vet:
	@go vet $(GO_PACKAGES)

test:
	@go test -count=1 $(GO_PACKAGES)

lint: fmt-check vet

migrate:
	@go run ./cmd/migrate

ci: lint test
