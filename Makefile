.PHONY: help install build test validate web-install web-build web-dev clean

help:
	@echo "Available targets:"
	@echo "  install      Install 'taite' CLI binary to \$$GOBIN"
	@echo "  build        Build 'taite' CLI binary to bin/taite"
	@echo "  test         Run Go tests"
	@echo "  validate     Validate data/generated/schedule.json schema"
	@echo "  web-install  Install npm dependencies for apps/web"
	@echo "  web-build    Build modern TypeScript web frontend"
	@echo "  web-dev      Start Vite dev server for frontend"
	@echo "  clean        Remove build artifacts"

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
