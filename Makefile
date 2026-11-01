.PHONY: help install build test validate sync verify ship release version web-install web-build web-dev clean

help:
	@echo "Available targets:"
	@echo "  verify       Run full mechanical verification pipeline (sync, validate, test, typecheck, build)"
	@echo "  sync         Sync data/generated/schedule.json to apps/web/public/"
	@echo "  version      Print current application version"
	@echo "  release      Release new version via scripts/release.sh (usage: make release TYPE=patch)"
	@echo "  install      Install 'taite' CLI binary to \$$GOBIN"
	@echo "  build        Build 'taite' CLI binary to bin/taite"
	@echo "  test         Run Go tests"
	@echo "  validate     Validate data/generated/schedule.json schema"
	@echo "  ship         Ship current changes via scripts/ship.sh"
	@echo "  web-install  Install npm dependencies for apps/web"
	@echo "  web-build    Build modern TypeScript web frontend"
	@echo "  web-dev      Start Vite dev server for frontend"
	@echo "  clean        Remove build artifacts"

sync:
	./scripts/sync-schedule.sh

verify:
	./scripts/verify.sh

ship:
	./scripts/ship.sh $(TITLE)

version:
	@go run ./cmd/taite version

release:
	./scripts/release.sh $(TYPE)

install:
	go install ./cmd/taite

build:
	go build -o bin/taite ./cmd/taite

test:
	go test ./...

validate:
	go run ./cmd/taite validate --file data/generated/schedule.json

web-install:
	cd apps/web && npm ci

web-build:
	cd apps/web && npm run build

web-dev:
	cd apps/web && npm run dev

clean:
	rm -rf bin/ apps/web/dist
