.PHONY: proto
proto:
	@echo "Generating protobuf files..."
	@bash scripts/protogen.sh

.PHONY: docs
docs: proto
	@echo "Merging OpenAPI specifications..."
	@bash scripts/merge-openapi.sh

.PHONY: build
build: proto
	@echo "Building cegw..."
	@go build -o bin/cegw ./cmd/cegw

.PHONY: test
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run

.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf gen/ bin/ coverage.out

.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

.PHONY: run
run: build
	@echo "Running cegw..."
	@./bin/cegw

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  proto  - Generate protobuf files"
	@echo "  docs   - Merge OpenAPI specifications"
	@echo "  build  - Build the binary"
	@echo "  test   - Run tests"
	@echo "  lint   - Run linters"
	@echo "  clean  - Clean generated files"
	@echo "  deps   - Install dependencies"
	@echo "  run    - Build and run"
	@echo "  help   - Show this help"
