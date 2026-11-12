GOBIN := $(shell go env GOPATH)/bin

# Variáveis locais (segredos OAuth etc.) ficam em .env, que não vai para o git.
ifneq (,$(wildcard .env))
include .env
export
endif

DATABASE_URL ?= postgres://erevia:erevia@localhost:5432/erevia?sslmode=disable
TEST_DATABASE_URL ?= $(DATABASE_URL)
DEV_LOGIN ?= true

.PHONY: help setup proto db-up db-down server client test build docker

help: ## Lista os comandos
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  make %-10s %s\n", $$1, $$2}'

setup: ## Instala dependências (Go, npm e plugin protoc-gen-go)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	cd server && go mod download
	cd client && npm install

proto: ## Gera código Go e TS a partir de proto/
	PATH="$(GOBIN):$$PATH" client/node_modules/.bin/buf generate

db-up: ## Sobe o Postgres (Docker)
	docker compose -f infra/docker-compose.yml up -d --wait

db-down: ## Para o Postgres
	docker compose -f infra/docker-compose.yml down

server: ## Roda o servidor (porta 8080) com login de desenvolvimento
	cd server && DATABASE_URL="$(DATABASE_URL)" DEV_LOGIN="$(DEV_LOGIN)" go run ./cmd/zone

client: ## Roda o cliente em modo dev (http://localhost:5173)
	cd client && npm run dev

test: ## Testes do servidor (com Postgres) + checagem de tipos do cliente
	cd server && go vet ./... && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./... -race
	cd client && npm run typecheck

build: ## Gera binário do servidor e build do cliente
	cd server && go build -o bin/zone ./cmd/zone
	cd client && npm run build

docker: ## Gera a imagem de produção localmente
	docker build -t erevia .
