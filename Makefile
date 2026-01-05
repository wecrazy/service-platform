.PHONY: run-api run-wa run-scheduler run-grpc run-all build build-api build-wa build-scheduler build-grpc build-monitoring docs-install docs-grpc docs-serve swagger clean-dashboard config-dev config-prod monitoring-start monitoring-stop monitoring-restart monitoring-deep-restart monitoring-status monitoring-cleanup monitoring-ensure-running install-monitoring uninstall-monitoring build-migrate migrate-up migrate-down migrate-status migrate-reset help

run-api:
	go run cmd/api/main.go

run-wa:
	go run cmd/whatsapp/main.go

run-scheduler:
	go run cmd/scheduler/main.go

run-grpc:
	go run cmd/grpc/main.go

build-api:
	mkdir -p bin
	go build -o bin/api cmd/api/main.go

build-wa:
	mkdir -p bin
	go build -o bin/wa cmd/whatsapp/main.go

build-scheduler:
	mkdir -p bin
	go build -o bin/scheduler cmd/scheduler/main.go

build-grpc:
	mkdir -p bin
	go build -o bin/grpc cmd/grpc/main.go

build-monitoring:
	mkdir -p bin
	go build -o bin/monitoring cmd/monitoring/main.go

build-n8n:
	mkdir -p bin
	go build -o bin/n8n cmd/n8n/main.go

build: build-api build-wa build-grpc build-monitoring build-n8n

test:
	go test -v -cover ./tests/... ./internal/migrations/...

run-n8n:
	go run cmd/n8n/main.go

run-all: monitoring-ensure-running run-api run-grpc run-scheduler run-wa

# Database migrations
build-migrate:
	mkdir -p bin
	go build -o bin/migrate cmd/migrate/main.go

# Check if migrate binary exists, if not use go run
migrate-up:
	@echo "🚀 Running database migrations..."
	@if [ -f "./bin/migrate" ]; then \
		./bin/migrate -action up; \
	else \
		echo "📦 Binary not found, using go run..."; \
		go run cmd/migrate/main.go -action up; \
	fi

migrate-down:
	@echo "⬇️ Rolling back database migration..."
	@if [ -f "./bin/migrate" ]; then \
		./bin/migrate -action down -steps 1; \
	else \
		echo "📦 Binary not found, using go run..."; \
		go run cmd/migrate/main.go -action down -steps 1; \
	fi

migrate-status:
	@echo "📊 Checking migration status..."
	@if [ -f "./bin/migrate" ]; then \
		./bin/migrate -action status; \
	else \
		echo "📦 Binary not found, using go run..."; \
		go run cmd/migrate/main.go -action status; \
	fi

migrate-reset:
	@echo "⚠️ Resetting all migrations..."
	@if [ -f "./bin/migrate" ]; then \
		./bin/migrate -action reset; \
	else \
		echo "📦 Binary not found, using go run..."; \
		go run cmd/migrate/main.go -action reset; \
	fi

# clean:
# 	rm -rf bin

install-swagger:
	@echo "Ensuring swag v1.16.6 is installed"
	@GOBIN=$$(go env GOPATH)/bin; \
	if [ ! -x "$$GOBIN/swag" ] || [ "$$($$GOBIN/swag --version 2>/dev/null | awk '{print $$3}')" != "v1.16.6" ]; then \
		echo "Installing/updating swag to v1.16.6..."; \
		GOBIN="$$GOBIN" go install github.com/swaggo/swag/cmd/swag@v1.16.6; \
	else \
		echo "swag v1.16.6 already installed"; \
	fi

swagger: install-swagger
	@GOBIN=$$(go env GOPATH)/bin; "$$GOBIN/swag" init -g cmd/api/main.go

# Documentation
docs-install:
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
	go install golang.org/x/tools/cmd/godoc@latest

docs-grpc:
	mkdir -p docs/grpc/html
	protoc --plugin=protoc-gen-doc=$(shell go env GOPATH)/bin/protoc-gen-doc --doc_out=./docs/grpc/html --doc_opt=html,whatsapp.html proto/whatsapp.proto
	protoc --plugin=protoc-gen-doc=$(shell go env GOPATH)/bin/protoc-gen-doc --doc_out=./docs/grpc/html --doc_opt=html,auth.html proto/auth.proto
	@echo "gRPC HTML docs generated in docs/grpc/html/"

docs-serve:
	go mod tidy
	@echo "Starting Go documentation server at http://localhost:6006/pkg/service-platform/?m=all"
	$(shell go env GOPATH)/bin/godoc -http=:6006

clean-dashboard:
	./scripts/remove_table_for_renew_dashboard.sh

# Configuration
config-dev:
	@./scripts/switch_config.sh dev

config-prod:
	@./scripts/switch_config.sh prod

# Monitoring
monitoring-start:
	@echo "🚀 Starting Service Platform Monitoring..."
	@./scripts/start-monitoring.sh

monitoring-stop:
	@echo "🛑 Stopping Service Platform Monitoring..."
	@./scripts/stop-monitoring.sh

monitoring-restart: monitoring-stop
	@echo "🔄 Restarting Service Platform Monitoring..."
	@sleep 2
	@./scripts/start-monitoring.sh

monitoring-deep-restart:
	@echo "🔄 Deep restarting Service Platform Monitoring (clearing Grafana cache)..."
	@./scripts/deep-restart-monitoring.sh

monitoring-status:
	@echo "📊 Checking monitoring services status..."
	@podman ps --filter "label=io.podman.compose.project=service-platform" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || docker ps --filter "label=com.docker.compose.project=service-platform" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "No monitoring services running or container runtime not available"

monitoring-cleanup:
	@echo "🧹 Cleaning up monitoring data and logs..."
	@./scripts/cleanup-monitoring.sh

monitoring-ensure-running:
	@echo "🔍 Checking monitoring status..."
	@if ! podman ps --filter "label=io.podman.compose.project=service-platform" --format "{{.Names}}" | grep -q . 2>/dev/null && ! docker ps --filter "label=com.docker.compose.project=service-platform" --format "{{.Names}}" | grep -q . 2>/dev/null; then \
		echo "📴 Monitoring stopped, cleaning up and starting..."; \
		./scripts/cleanup-monitoring.sh; \
		echo "✅ Cleanup finished, starting monitoring..."; \
		./scripts/start-monitoring.sh; \
	else \
		echo "✅ Monitoring already running."; \
	fi

install-monitoring:
	@echo "🔧 Installing monitoring service..."
	@if [ -f "./bin/monitoring" ]; then \
		sudo ./bin/monitoring --install; \
	else \
		echo "📦 Binary not found, using go run..."; \
		sudo go run cmd/monitoring/main.go --install; \
	fi

uninstall-monitoring:
	@echo "🗑️  Uninstalling monitoring service..."
	@if [ -f "./bin/monitoring" ]; then \
		sudo ./bin/monitoring --uninstall; \
	else \
		echo "📦 Binary not found, using go run..."; \
		sudo go run cmd/monitoring/main.go --uninstall; \
	fi

help:
	@echo "🚀 Service Platform - Available Commands:"
	@echo ""
	@echo "📦 Build Commands:"
	@echo "  make build-api          - Build API service"
	@echo "  make build-grpc         - Build gRPC service"
	@echo "  make build-scheduler    - Build scheduler service"
	@echo "  make build-wa           - Build WhatsApp service"
	@echo "  make build-monitoring   - Build monitoring service"
	@echo "  make build              - Build all services"
	@echo ""
	@echo "🏃 Run Commands:"
	@echo "  make run-api            - Run API service"
	@echo "  make run-grpc           - Run gRPC service"
	@echo "  make run-scheduler      - Run scheduler service"
	@echo "  make run-wa             - Run WhatsApp service"
	@echo "  make run-all            - Run all services"
	@echo ""
	@echo "🗃️  Database/Migration Commands:"
	@echo "  make migrate-up         - Run all pending database migrations (auto-detects binary)"
	@echo "  make migrate-down       - Rollback last database migration (auto-detects binary)"
	@echo "  make migrate-status     - Check migration status (auto-detects binary)"
	@echo "  make migrate-reset      - Reset all migrations (⚠️  destructive, auto-detects binary)"
	@echo "  make build-migrate      - Build migration CLI tool"
	@echo ""
	@echo "📊 Monitoring Commands:"
	@echo "  make monitoring-start   			- Start Prometheus + Grafana"
	@echo "  make monitoring-stop    			- Stop monitoring services"
	@echo "  make monitoring-restart 			- Restart monitoring services"
	@echo "  make monitoring-deep-restart 			- Deep restart with Grafana cache cleanup"
	@echo "  make monitoring-status  			- Check monitoring status"
	@echo "  make monitoring-cleanup 			- Clean old monitoring data"
	@echo "  make monitoring-ensure-running  		- Ensure monitoring is running (cleanup if stopped)"
	@echo "  make install-monitoring 			- Install monitoring as a system service"
	@echo "  make uninstall-monitoring 			- Uninstall monitoring system service"
	@echo ""
	@echo "🛠️  Development Commands:"
	@echo "  make config-dev         - Setup dev configuration"
	@echo "  make config-prod        - Setup prod configuration"
	@echo "  make docs-install       - Install API documentation tools"
	@echo "  make docs-serve         - Serve API documentation"
	@echo "  make swagger            - Generate Swagger docs"
	@echo "  make clean-dashboard    - Clean dashboard files"
