# ShopKart Management Makefile
.PHONY: help init up down restart reset test build load-baseline load-spike load-cache load-queue load-mixed fault-crash revert-crash fault-cache revert-cache fault-db revert-db fault-slow-query revert-slow-query fault-memory revert-memory fault-netem revert-netem revert-all

SHELL := /usr/bin/env bash

help:
	@echo "Available commands:"
	@echo "  make init               - Copy .env.example to .env if not exists"
	@echo "  make up                 - Build and start all services and infra"
	@echo "  make down               - Stop all containers"
	@echo "  make restart            - Restart all containers"
	@echo "  make reset              - Clean and reset DB volumes & containers"
	@echo "  make test               - Run Go unit tests for backend services"
	@echo "  make build              - Build all service docker images"
	@echo "  make load-baseline      - Run baseline k6 load test"
	@echo "  make load-spike         - Run checkout spike k6 load test"
	@echo "  make load-cache         - Run flash sale cache stampede k6 test"
	@echo "  make load-queue         - Run queue flood k6 test"
	@echo "  make load-mixed         - Run realistic mixed k6 load test"
	@echo "  make fault-crash        - Inject container crash on inventory"
	@echo "  make revert-crash       - Recover crashed container"
	@echo "  make fault-cache        - Stop Redis cache tier"
	@echo "  make revert-cache       - Restore Redis cache tier"
	@echo "  make fault-db           - Inject Postgres row-level lock"
	@echo "  make revert-db          - Release Postgres row lock"
	@echo "  make fault-slow-query   - Saturate DB connection pool with slow queries"
	@echo "  make revert-slow-query  - Terminate slow queries and free connection pool"
	@echo "  make fault-memory       - Enable and trigger memory leak in payments"
	@echo "  make revert-memory      - Clear memory leak in payments"
	@echo "  make fault-netem        - Inject network latency into bank mock"
	@echo "  make revert-netem       - Revert network latency"
	@echo "  make revert-all         - Revert all active faults to healthy baseline"

init:
	@if [ ! -f .env ]; then cp .env.example .env && echo "[OK] Created .env from .env.example"; else echo "[INFO] .env already exists"; fi

build: init
	docker compose build

up: init
	docker compose up -d

down:
	docker compose down

restart:
	docker compose restart

reset:
	docker compose down -v --remove-orphans
	docker compose up -d

test:
	cd services && go test -v ./...

load-baseline:
	k6 run load/baseline.js

load-spike:
	k6 run load/spike_checkout.js

load-cache:
	k6 run load/flash_sale_catalog.js

load-queue:
	k6 run load/queue_flood.js

load-mixed:
	k6 run load/mixed_realistic.js

fault-crash:
	./scripts/fault_crash.sh shopkart-inventory

revert-crash:
	./scripts/revert_crash.sh shopkart-inventory

fault-cache:
	./scripts/fault_cache_down.sh

revert-cache:
	./scripts/revert_cache_down.sh

fault-db:
	./scripts/fault_db_lock.sh

revert-db:
	./scripts/revert_db_lock.sh

fault-slow-query:
	./scripts/fault_slow_query.sh

revert-slow-query:
	./scripts/revert_slow_query.sh

fault-memory:
	./scripts/fault_memory_pressure.sh

revert-memory:
	./scripts/revert_memory_pressure.sh

fault-netem:
	./scripts/fault_netem.sh

revert-netem:
	./scripts/revert_netem.sh

revert-all:
	./scripts/revert_all.sh
