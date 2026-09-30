# Makefile para AdvPP - Compilador AdvPL/TLPP

.PHONY: all build test clean help cross \
        rpo-analyze rpo-decrypt rpo-inject \
        docker-build docker-run docker-logs

# Configurações
BINARY := advplc
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO := go
DOCKER_COMPOSE := docker-compose

all: build test

cross:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o advplc-linux-amd64 ./cmd/advplc
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o advplc-windows-amd64.exe ./cmd/advplc
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o advplc-darwin-arm64 ./cmd/advplc
	@echo "Cross-compile concluído:"
	@ls -lh advplc-*

# Build
build:
	$(GO) build -o $(BINARY) ./cmd/advplc
	@echo "Build concluído: $(BINARY) (version $(VERSION))"

# Testes
test:
	$(GO) test ./... -v -count=1

test-short:
	$(GO) test ./... -short

# Limpeza
clean:
	rm -f $(BINARY)
	rm -f advplc-*
	rm -rf releases/bytecode/*.bytecode
	find . -name "*.rpo" -path "*/tmp/*" -delete 2>/dev/null || true

# Help
help:
	@echo "AdvPP Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build          - Build the compiler"
	@echo "  test           - Run all tests"
	@echo "  test-short     - Run short tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  cross          - Cross-compile for all platforms"
	@echo "  help           - Show this help"
	@echo ""
	@echo "RPO Tools:"
	@echo "  rpo-analyze    - Analyze RPO file"
	@echo "  rpo-decrypt    - Decrypt RPO with capture"
	@echo "  rpo-inject     - Inject bytecode into RPO"

# RPO Analysis
rpo-analyze: build
	./$(BINARY) rpo info $(FILE)

rpo-decrypt: build
	./$(BINARY) rpo decrypt $(FILE) $(CAPTURE)

rpo-inject: build
	./$(BINARY) rpo inject $(FILE) $(CAPTURE) --inject $(REGISTER)=$(BYTECODE) -o $(OUTPUT)

# Docker
docker-build:
	$(DOCKER_COMPOSE) build

docker-run:
	$(DOCKER_COMPOSE) up -d

docker-logs:
	$(DOCKER_COMPOSE) logs -f

# Dev targets
dev: build
	$(GO) run ./cmd/advplc $(ARGS)

run: build
	./$(BINARY) $(ARGS)
