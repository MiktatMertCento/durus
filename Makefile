.PHONY: tidy deps frontend-install build frontend-build backend-build run docker-up docker-down db-up check clean

ADDR ?= :8080
PORT ?= 8080
POSTGRES_USER ?= durus
POSTGRES_PASSWORD ?= durus
POSTGRES_DB ?= durus
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:5433/$(POSTGRES_DB)?sslmode=disable

tidy:
	cd backend && go mod tidy

deps: tidy frontend-install

frontend-install:
	cd frontend && npm ci

frontend-build:
	cd frontend && npm run build

backend-build:
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server .

build: frontend-build
	mkdir -p backend/static
	rm -rf backend/static/*
	cp -r frontend/dist/. backend/static/
	$(MAKE) backend-build

db-up:
	docker compose up -d db

run: build db-up
	@echo "waiting for postgres..."
	@until docker compose exec -T db pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) >/dev/null 2>&1; do sleep 1; done
	cd backend && DATABASE_URL='$(DATABASE_URL)' STATIC_DIR=./static ADDR=$(ADDR) ./bin/server

dev-backend: db-up
	@until docker compose exec -T db pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) >/dev/null 2>&1; do sleep 1; done
	cd backend && DATABASE_URL='$(DATABASE_URL)' ADDR=$(ADDR) go run .

dev-frontend:
	cd frontend && npm run dev

docker-up:
	docker compose up --build -d --remove-orphans

docker-down:
	docker compose down --remove-orphans --timeout 10
	@if ss -tln '( sport = :$(PORT) )' 2>/dev/null | grep -q ':$(PORT)'; then \
	  echo "warning: :$(PORT) still in use:"; ss -tlnp '( sport = :$(PORT) )' || true; exit 1; \
	else \
	  echo "port :$(PORT) is free"; \
	fi

check: tidy db-up
	@until docker compose exec -T db pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) >/dev/null 2>&1; do sleep 1; done
	cd backend && go vet ./...
	cd backend && DATABASE_URL='$(DATABASE_URL)' go test ./... -count=1
	cd frontend && npm run build
	cd frontend && npm run lint

clean:
	rm -rf frontend/dist backend/bin backend/static backend/data
