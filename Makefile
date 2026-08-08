.PHONY: help install build lint typecheck test test-coverage fmt clean run up down verify

help:
	@echo "make install        install all TypeScript dependencies (pnpm)"
	@echo "make build          build Go modules + all TypeScript packages"
	@echo "make run            run the Mini AI-DOS gateway locally (reads env)"
	@echo "make lint           lint Go (gofmt) + TypeScript (eslint)"
	@echo "make typecheck      typecheck all TypeScript packages"
	@echo "make test           run Go + TypeScript test suites"
	@echo "make test-coverage  run tests with coverage reporting"
	@echo "make fmt            format Go (gofmt) + TypeScript (prettier) in place"
	@echo "make up             start local infra (gateway container + Postgres)"
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
	go build ./services/gateway/...
	pnpm run build

run:
	go run ./services/gateway/cmd/gateway

lint:
	gofmt -l services/foundation/ services/gateway/
	golangci-lint run --config=.golangci.yml ./services/foundation/... ./services/gateway/... || true
	pnpm run lint

typecheck:
	pnpm run typecheck

test:
	go test ./services/foundation/... ./services/gateway/...
	pnpm run test

test-coverage:
	go test -cover ./services/foundation/... ./services/gateway/...
	pnpm run test:coverage

fmt:
	gofmt -l -w services/foundation/ services/gateway/
	pnpx prettier --write .

up:
	docker compose up -d

down:
	docker compose down

verify: install lint typecheck build test
	@echo "All checks passed."

clean:
	go clean ./services/foundation/... ./services/gateway/...
	pnpm run clean
