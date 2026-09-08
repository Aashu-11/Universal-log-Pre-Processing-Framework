.PHONY: up down logs ps test lint clean demo verify bench airgap certs enrichment build

COMPOSE ?= docker compose

up:
	$(COMPOSE) up -d --build
	@echo "Waiting for services to become healthy..."
	@for i in $$(seq 1 60); do \
		unhealthy=$$($(COMPOSE) ps --format '{{.Name}} {{.Health}}' | grep -v 'healthy\|running (healthy)\|^$$' | grep -v 'NAME' || true); \
		if [ -z "$$unhealthy" ]; then break; fi; \
		sleep 5; \
	done
	@echo ""
	@echo "Service URLs:"
	@echo "  Presto UI     http://localhost:8080"
	@echo "  MinIO console http://localhost:9001  (ulpfadmin / ulpf_dev_only)"
	@echo "  Grafana       http://localhost:3000"
	@echo "  Prometheus    http://localhost:9090"
	@echo "  Control API   http://localhost:8000/docs"
	@echo "  Console       http://localhost:5173"
	@$(COMPOSE) ps

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

build:
	go build ./...

test:
	go test ./... -race -count=1
	cd services/control-plane && python -m pytest -q
	cd services/onboarding && python -m pytest -q
	cd services/console && npm run build

lint:
	gofmt -l . | (! grep .)
	go vet ./...
	cd services/control-plane && ruff check . && black --check .
	cd services/console && npm run lint

certs:
	mkdir -p deploy/certs
	openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
		-keyout deploy/certs/server.key -out deploy/certs/server.crt \
		-subj "/CN=ulpf-collector-demo"

enrichment:
	go run ./tools/gen-enrichment

clean:
	$(COMPOSE) down -v
	rm -rf bin/ dist/

demo:
	bash scripts/demo.sh

verify:
	bash scripts/verify.sh

bench:
	bash bench/run.sh

airgap:
	bash deploy/airgap/bundle.sh --version 1.0.0
	bash deploy/airgap/verify-offline.sh
