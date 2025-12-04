GITCOMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)

.PHONY: all
all: sync format

.PHONY: sync
sync:
	uv sync

.PHONY: format
format:
	uv run isort .
	uv run ruff format .
	uv run ruff check --fix .

.PHONY: dev
dev:
	@echo ">>> Running in development mode with application and template reloading."
	ENV=development BUILD_VERSION=local uv run uvicorn --host 127.0.0.1 --port 3000 --app-dir app --factory --reload --reload-dir app main:make_app

.PHONY: run
run:
	@echo ">>> Running in production mode without application or template reloading."
	ENV=production BUILD_VERSION=${GITCOMMIT} uv run uvicorn --host 127.0.0.1 --port 3000 --app-dir app --factory main:make_app
