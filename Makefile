.PHONY: build demo test selftest demo-github release-check

COMPOSE := docker compose -f examples/mcp-postgres-gateway/docker-compose.yml
BOUNDARY_DEMO_PORT ?= 18080

build:
	go build -o bin/boundary ./cmd/boundary

demo:
	@set -e; \
	export BOUNDARY_DEMO_PORT=$(BOUNDARY_DEMO_PORT); \
	cleanup() { $(COMPOSE) down -v; }; \
	trap cleanup EXIT; \
	$(COMPOSE) up -d --build; \
	$(COMPOSE) exec -T demo-agent boundary demo postgres --gateway http://gateway:8080/mcp --bypass-host postgres --bypass-port 5432; \
	$(COMPOSE) logs gateway

test:
	env -u GOROOT go test ./...

selftest:
	./scripts/selftest.sh

demo-github:
	./scripts/demo-github.sh

release-check:
	./scripts/release-check.sh
