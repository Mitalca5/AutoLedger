# ==============================================================================
# TeslaCost Makefile
# ==============================================================================

.PHONY: help dev build test clean docker-up docker-down

help:
	@echo "Commandes disponibles :"
	@echo "  make dev          - Lance le serveur Go en local"
	@echo "  make test         - Exécute l'ensemble des tests unitaires"
	@echo "  make build        - Compile le binaire Go"
	@echo "  make docker-up    - Démarre l'environnement Docker Compose (Postgres + App)"
	@echo "  make docker-down  - Stoppe l'environnement Docker Compose"

dev:
	go run ./cmd/server

test:
	go test -v -race ./...

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/teslacost ./cmd/server

docker-up:
	docker compose up -d

docker-down:
	docker compose down
