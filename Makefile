.PHONY: run-api run-wa run-scheduler run-grpc build build-api build-wa build-scheduler build-grpc docs-install docs-grpc docs-serve swagger clean-dashboard config-dev config-prod

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

build: build-api build-wa build-grpc

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
	@echo "Starting Go documentation server at http://localhost:6006/pkg/service_platform/?m=all"
	$(shell go env GOPATH)/bin/godoc -http=:6006

clean-dashboard:
	./scripts/remove_table_for_renew_dashboard.sh

# Configuration
config-dev:
	@./scripts/switch_config.sh dev

config-prod:
	@./scripts/switch_config.sh prod
