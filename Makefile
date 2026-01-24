.PHONY: run-api run-wa run-telegram run-scheduler run-grpc run-all build build-api build-wa build-telegram build-scheduler build-grpc build-monitoring docs-install docs-grpc docs-serve swagger clean-dashboard config-dev config-prod monitoring-start monitoring-stop monitoring-restart monitoring-deep-restart monitoring-status monitoring-cleanup monitoring-ensure-running monitoring-cleanup-data monitoring-deep-restart-alt monitoring-start-alt monitoring-stop-alt tempo-test observability-verify install-monitoring uninstall-monitoring build-migrate migrate-up migrate-down migrate-status migrate-reset k6-health-check k6-smoke-test k6-login-test k6-stress-test k6-run-script k6-status k6-stop k6-results health-check-all mongo-up mongo-down mongo-logs mongo-status test test-mongo help

run-api:
	go run cmd/api/main.go

run-wa:
	go run cmd/whatsapp/main.go

run-telegram:
	go run cmd/telegram/main.go

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

build-telegram:
	mkdir -p bin
	go build -o bin/telegram cmd/telegram/main.go

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

test-mongo:
	@echo "🧪 Running MongoDB tests..."
	@go test -v ./tests/unit/mongodb_test.go

n8n-start:
	go run cmd/n8n/main.go

n8n-stop:
	go run cmd/n8n/main.go --stop

n8n-import:
	@echo "📥 Importing workflows..."
	@podman run --rm \
		--name n8n-import-temp \
		--network n8n-net \
		-u node \
		-e N8N_ENFORCE_SETTINGS_FILE_PERMISSIONS=true \
		-e DB_TYPE=postgresdb \
		-e DB_POSTGRESDB_HOST=n8n-postgres \
		-e DB_POSTGRESDB_PORT=5432 \
		-e DB_POSTGRESDB_DATABASE=n8n \
		-e DB_POSTGRESDB_USER=n8n \
		-e DB_POSTGRESDB_PASSWORD=n8n \
		-v n8n_data:/home/node/.n8n \
		-v $(shell pwd)/internal/n8n/workflows:/home/node/workflows \
		n8nio/n8n:latest import:workflow --separate --input=/home/node/workflows 2>&1 | grep -vE "Could not find workflow|ActiveWorkflowManager|processTicksAndRejections|ImportService|ImportWorkflowsCommand|CommandRegistry|remove webhooks|Active version not found|at /usr/local" | grep "\S" || true

n8n-export:
	@echo "📤 Exporting workflows to internal/n8n/workflows..."
	podman exec -u node -it service-platform-n8n n8n export:workflow --backup --output=/home/node/workflows
n8n-clear:
	@echo "🧹 Clearing all workflows from N8N instance..."
	@bash scripts/n8n-clear-workflows.sh

n8n-clear-force:
	@echo "🧹 Force clearing all workflows from N8N instance (no confirmation)..."
	@bash scripts/n8n-clear-workflows.sh --force
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
	@echo "Ensuring swag latest is installed"
	@GOBIN=$$(go env GOPATH)/bin; \
	if [ ! -x "$$GOBIN/swag" ] || [ "$$($$GOBIN/swag --version 2>/dev/null | awk '{print $$3}')" != "latest" ]; then \
		echo "Installing/updating swag to latest..."; \
		GOBIN="$$GOBIN" go get -u github.com/swaggo/swag/cmd/swag@latest; \
	else \
		echo "swag latest already installed"; \
	fi

swagger: install-swagger
	@GOBIN=$$(go env GOPATH)/bin; "$$GOBIN/swag" init -g cmd/api/main.go

# gRPC Code Generation
install-protoc-gen:
	@echo "📦 Installing protoc-gen-go and protoc-gen-go-grpc..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ protoc plugins installed"

proto-gen: install-protoc-gen
	@echo "🔧 Generating gRPC code from .proto files..."
	@echo "⛓️  Generating code for auth.proto"
	@GOBIN=$$(go env GOPATH)/bin; \
	protoc --plugin=protoc-gen-go="$$GOBIN/protoc-gen-go" \
		--plugin=protoc-gen-go-grpc="$$GOBIN/protoc-gen-go-grpc" \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth.proto
	@echo "⛓️  Generating code for scheduler.proto"
	@GOBIN=$$(go env GOPATH)/bin; \
	protoc --plugin=protoc-gen-go="$$GOBIN/protoc-gen-go" \
		--plugin=protoc-gen-go-grpc="$$GOBIN/protoc-gen-go-grpc" \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/scheduler.proto
	@echo "⛓️  Generating code for whatsapp.proto"
	@GOBIN=$$(go env GOPATH)/bin; \
	protoc --plugin=protoc-gen-go="$$GOBIN/protoc-gen-go" \
		--plugin=protoc-gen-go-grpc="$$GOBIN/protoc-gen-go-grpc" \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/whatsapp.proto
	@echo "⛓️  Generating code for telegram.proto"
	@GOBIN=$$(go env GOPATH)/bin; \
	protoc --plugin=protoc-gen-go="$$GOBIN/protoc-gen-go" \
		--plugin=protoc-gen-go-grpc="$$GOBIN/protoc-gen-go-grpc" \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/telegram.proto
	@echo "✅ gRPC code generation complete"

# Documentation
docs-install:
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
	go install golang.org/x/tools/cmd/godoc@latest

docs-grpc:
	mkdir -p docs/grpc/html
	protoc --plugin=protoc-gen-doc=$(shell go env GOPATH)/bin/protoc-gen-doc --doc_out=./docs/grpc/html --doc_opt=html,whatsapp.html proto/whatsapp.proto
	protoc --plugin=protoc-gen-doc=$(shell go env GOPATH)/bin/protoc-gen-doc --doc_out=./docs/grpc/html --doc_opt=html,auth.html proto/auth.proto
	protoc --plugin=protoc-gen-doc=$(shell go env GOPATH)/bin/protoc-gen-doc --doc_out=./docs/grpc/html --doc_opt=html,telegram.html proto/telegram.proto
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
	@bash scripts/monitoring-quickstart.sh start

monitoring-stop:
	@bash scripts/monitoring-quickstart.sh stop

monitoring-restart:
	@bash scripts/monitoring-quickstart.sh restart

monitoring-deep-restart:
	@echo "🔄 Deep restarting Service Platform Monitoring (clearing Grafana cache)..."
	@bash scripts/monitoring-quickstart.sh restart

monitoring-status:
	@echo "📊 Checking monitoring services status..."
	@bash scripts/monitoring-quickstart.sh status

monitoring-cleanup:
	@echo "🧹 Cleaning up monitoring data and logs..."
	@bash scripts/monitoring-quickstart.sh clean

monitoring-ensure-running:
	@echo "🔍 Checking monitoring status..."
	@bash scripts/monitoring-quickstart.sh start

# Clean old monitoring data (Prometheus + Grafana logs)
monitoring-cleanup-data:
	@echo "🧹 Cleaning old monitoring data..."
	@bash scripts/cleanup-monitoring.sh

# Deep restart monitoring with cache clearing
monitoring-deep-restart-alt:
	@echo "🔄 Deep restarting monitoring (with cache clearing)..."
	@bash scripts/deep-restart-monitoring.sh

# Alternative monitoring start (using config-based script)
monitoring-start-alt:
	@echo "🚀 Starting monitoring services (alternative)..."
	@./scripts/start-monitoring.sh

# Alternative monitoring stop (using config-based script)
monitoring-stop-alt:
	@echo "🛑 Stopping monitoring services (alternative)..."
	@./scripts/stop-monitoring.sh

# Test Tempo tracing setup
tempo-test:
	@echo "🧪 Testing Tempo tracing..."
	@./scripts/test-tempo.sh

# Verify observability stack (logs, Loki, Tempo, Grafana)
observability-verify:
	@echo "🔍 Verifying observability stack..."
	@./scripts/verify-logging.sh

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

# k6 Load Testing
k6-health-check:
	@echo "🧪 Running k6 health check load test..."
	@./scripts/run-k6-test.sh health-check.js

k6-smoke-test:
	@echo "🧪 Running k6 smoke test..."
	@./scripts/run-k6-test.sh api-smoke-test.js

k6-login-test:
	@echo "🧪 Running k6 login flow test..."
	@./scripts/run-k6-test.sh login-flow.js

k6-stress-test:
	@echo "🧪 Running k6 stress test..."
	@./scripts/run-k6-test.sh stress-test.js

k6-run-script:
	@if [ -z "$(SCRIPT)" ]; then \
		echo "❌ Error: SCRIPT variable is required"; \
		echo "Usage: make k6-run-script SCRIPT=your-test.js"; \
		exit 1; \
	fi
	@echo "🧪 Running k6 custom script: $(SCRIPT)..."
	@./scripts/run-k6-test.sh $(SCRIPT)

k6-status:
	@echo "📊 k6 Container Status:"
	@podman ps -a --filter "name=service-platform-k6" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || docker ps -a --filter "name=service-platform-k6" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "k6 container not found"
	@echo ""
	@echo "📊 k6 Web Dashboard: http://localhost:6680"
	@echo "📊 k6 Prometheus Endpoint: http://localhost:5665/metrics"

k6-stop:
	@echo "🛑 Stopping k6 tests..."
	@podman stop service-platform-k6 2>/dev/null || docker stop service-platform-k6 2>/dev/null || echo "k6 container not running"

k6-results:
	@echo "📊 k6 Test Results:"
	@if [ -d "./tests/k6/results" ]; then \
		ls -lh ./tests/k6/results; \
	else \
		echo "No results directory found. Results are stored in the k6 container volume."; \
		echo "To access results, check: podman volume inspect service-platform_k6_results"; \
	fi

health-check-all:
	@echo "🩺 Running comprehensive health check..."
	@./scripts/health-check-all.sh

mongo-up:
	@echo "🐳 Starting MongoDB container..."
	@podman-compose -f docker/docker-compose.mongodb.yml up -d

mongo-down:
	@echo "🐳 Stopping MongoDB container..."
	@podman-compose -f docker/docker-compose.mongodb.yml down

mongo-logs:
	@echo "📋 Viewing MongoDB logs..."
	@podman-compose -f docker/docker-compose.mongodb.yml logs -f

mongo-status:
	@echo "📊 Checking MongoDB container status..."
	@podman ps | grep mongodb || echo "MongoDB container not running"

mongo-check:
	@echo "🔍 Checking MongoDB and MongoExpress accessibility..."
	@echo ""
	@echo "📦 Container Status:"
	@podman ps --filter "name=service-platform-mongo" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "  ❌ MongoDB containers not running"
	@echo ""
	@echo "🌐 MongoExpress Web UI:"
	@if podman ps --filter "name=service-platform-mongoexpress" --filter "status=running" -q 2>/dev/null | grep -q .; then \
		HTTP_CODE=$$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8081 2>/dev/null); \
		if [ "$$HTTP_CODE" = "200" ] || [ "$$HTTP_CODE" = "401" ]; then \
			echo "  ✅ MongoExpress is accessible at http://localhost:8081"; \
			echo "     Username: admin"; \
			echo "     Password: pass"; \
		else \
			echo "  ⏳ MongoExpress container is running but still initializing (HTTP $$HTTP_CODE)..."; \
			echo "     Wait a few seconds and check: http://localhost:8081"; \
		fi; \
	else \
		echo "  ❌ MongoExpress container is not running"; \
		echo "     Run 'make mongo-up' to start the services"; \
	fi
	@echo ""
	@echo "🗄️  MongoDB Database:"
	@if timeout 2 bash -c 'until nc -z localhost 27007 2>/dev/null; do sleep 0.5; done' 2>/dev/null; then \
		echo "  ✅ MongoDB is accessible at localhost:27007"; \
		echo "     Connection: mongodb://mongo_admin:password_admin_mongo@localhost:27007/service_platform_mongo_test?authSource=admin"; \
	else \
		echo "  ❌ MongoDB is NOT accessible at localhost:27007"; \
	fi
	@echo ""
	@echo "💡 Run 'make test-mongo' to execute MongoDB tests"

help:
	@echo "🚀 Service Platform - Available Commands:"
	@echo ""
	@echo "📦 Build Commands:"
	@echo "  make build-api            			- Build API service"
	@echo "  make build-grpc         			- Build gRPC service"
	@echo "  make build-scheduler   			- Build scheduler service"
	@echo "  make build-wa           			- Build WhatsApp service"
	@echo "  make build-telegram      			- Build Telegram service"
	@echo "  make build-monitoring   			- Build monitoring service"
	@echo "  make build             			- Build all services"
	@echo ""
	@echo "🏃 Run Commands:"
	@echo "  make run-api            			- Run API service"
	@echo "  make run-grpc           			- Run gRPC service"
	@echo "  make run-scheduler      			- Run scheduler service"
	@echo "  make run-wa             			- Run WhatsApp service"
	@echo "  make run-telegram         			- Run Telegram service"
	@echo "  make run-all            			- Run all services"
	@echo ""
	@echo "🤖 n8n Workflow Automation Commands:"
	@echo "  make n8n-start          			- Start n8n workflow automation service"
	@echo "  make n8n-stop           			- Stop n8n service"
	@echo "  make n8n-import         			- Import workflows from internal/n8n/workflows into n8n"
	@echo "  make n8n-export         			- Export workflows from n8n to internal/n8n/workflows"
	@echo "  make n8n-clear          			- Clear all workflows from N8N instance (with confirmation)"
	@echo "  make n8n-clear-force    			- Force clear all workflows from N8N (no confirmation)"
	@echo ""
	@echo "🗃️  Database/Migration Commands:"
	@echo "  make migrate-up         			- Run all pending database migrations (auto-detects binary)"
	@echo "  make migrate-down       			- Rollback last database migration (auto-detects binary)"
	@echo "  make migrate-status     			- Check migration status (auto-detects binary)"
	@echo "  make migrate-reset      			- Reset all migrations (⚠️  destructive, auto-detects binary)"
	@echo "  make build-migrate      			- Build migration CLI tool"
	@echo ""
	@echo "📊 Monitoring Commands:"
	@echo "  make monitoring-start   			- Start Prometheus + Grafana"
	@echo "  make monitoring-stop    			- Stop monitoring services"
	@echo "  make monitoring-restart 			- Restart monitoring services"
	@echo "  make monitoring-deep-restart 			- Deep restart with Grafana cache cleanup"
	@echo "  make monitoring-status  			- Check monitoring status"
	@echo "  make monitoring-cleanup 			- Clean old monitoring data"
	@echo "  make monitoring-ensure-running  		- Ensure monitoring is running (cleanup if stopped)"
	@echo "  make monitoring-cleanup-data 			- Clean old Prometheus/Grafana data (detailed)"
	@echo "  make monitoring-deep-restart-alt 		- Deep restart with volume removal (alternative)"
	@echo "  make monitoring-start-alt 			- Start monitoring (config-based alternative)"
	@echo "  make monitoring-stop-alt 			- Stop monitoring (config-based alternative)"
	@echo "  make install-monitoring 			- Install monitoring as a system service"
	@echo "  make uninstall-monitoring 			- Uninstall monitoring system service"
	@echo ""
	@echo "🧪 k6 Load Testing Commands:"
	@echo "  make k6-health-check    			- Run health check load test"
	@echo "  make k6-smoke-test      			- Run smoke test (quick validation)"
	@echo "  make k6-login-test      			- Run login flow load test"
	@echo "  make k6-stress-test     			- Run stress test (progressive load increase)"
	@echo "  make k6-run-script      			- Run custom k6 script (usage: make k6-run-script SCRIPT=test.js)"
	@echo "  make k6-status          			- Check k6 container status and endpoints"
	@echo "  make k6-stop            			- Stop running k6 tests"
	@echo "  make k6-results         			- View k6 test results"
	@echo ""
	@echo "🛠️  Development Commands:"
	@echo "  make config-dev         			- Setup dev configuration"
	@echo "  make config-prod        			- Setup prod configuration"
	@echo "  make install-protoc-gen 			- Install protoc Go plugins"
	@echo "  make proto-gen          			- Generate gRPC code from .proto files"
	@echo "  make docs-install       			- Install API documentation tools"
	@echo "  make docs-serve         			- Serve API documentation"
	@echo "  make swagger            			- Generate Swagger docs"
	@echo "  make clean-dashboard    			- Clean dashboard files"
	@echo "  make tempo-test         			- Test Tempo tracing setup"
	@echo "  make observability-verify 			- Verify observability stack (logs, tracing, metrics)"
	@echo "  make health-check-all   			- Run comprehensive health check for all services"
	@echo "  make mongo-up            			- Start MongoDB container"
	@echo "  make mongo-down          			- Stop MongoDB container"
	@echo "  make mongo-logs          			- View MongoDB logs"
	@echo "  make mongo-status        			- Check MongoDB container status"
	@echo "  make mongo-check         			- Check MongoDB and MongoExpress accessibility"
	@echo "  make test-mongo          			- Run MongoDB-specific tests"
