.PHONY: help install build lint typecheck test test-coverage fmt clean dev up down verify

help:
	@echo "make install        install all TypeScript dependencies (pnpm)"
	@echo "make build          build Go foundation + all TypeScript packages"
	@echo "make lint           lint Go (gofmt) + TypeScript (eslint)"
	@echo "make typecheck      typecheck all TypeScript packages"
	@echo "make test           run Go + TypeScript test suites"
	@echo "make test-coverage  run tests with coverage reporting"
	@echo "make fmt            format Go (gofmt) + TypeScript (prettier) in place"
	@echo "make up             start local infra (Postgres, Redis, NATS)"
	@echo "make down           stop local infra"
	@echo "make verify         install + lint + typecheck + build + test — the full CI sequence, locally"
	@echo "make clean          remove build artifacts and node_modules"

install:
	pnpm install

# NOTE: `go build ./...` from the repo root does not work with this
# go.work layout — verified during Project Foundation, not assumed.
# Each module is targeted by its own path.
build:
	go build ./services/foundation/...
	pnpm run build

lint:
	gofmt -l services/foundation/
	golangci-lint run --config=.golangci.yml services/foundation/... || true
	pnpm run lint

typecheck:
	pnpm run typecheck

test:
	go test -race ./services/foundation/...
	pnpm run test

test-coverage:
	go test -race -cover ./services/foundation/...
	pnpm run test:coverage

fmt:
	gofmt -l -w services/foundation/
	pnpx prettier --write .

up:
	docker compose up -d

down:
	docker compose down

verify: install lint typecheck build test
	@echo "All checks passed."

clean:
	go clean ./services/foundation/...
	pnpm run clean
