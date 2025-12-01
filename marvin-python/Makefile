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

.PHONY: run
run:
	BUILD_VERSION=${GITCOMMIT} uv run uvicorn --host 127.0.0.1 --port 3000 --app-dir app --factory --reload main:make_app
