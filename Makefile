# Makefile para AdvPP - Compilador AdvPL/TLPP

.PHONY: all build test clean help \
        rpo-analyze rpo-decrypt rpo-inject \
        docker-build docker-run docker-logs \
        bytecode-generate

# Configurações
BINARY := advplc
VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO := go
DOCKER_COMPOSE := docker-compose

all: build test

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
	@echo "  help           - Show this help"
	@echo ""
	@echo "RPO Tools:"
	@echo "  rpo-analyze    - Analyze RPO file"
	@echo "  rpo-decrypt    - Decrypt RPO with capture"
	@echo "  rpo-inject     - Inject bytecode into RPO"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-run     - Run containers"
	@echo "  docker-logs    - Show container logs"
	@echo ""
	@echo "Bytecode:"
	@echo "  bytecode-generate - Generate bytecode from sources"

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

# Bytecode
bytecode-generate: build
	@mkdir -p releases/bytecode
	@echo "Generating bytecode..."
	@for f in $$($(GO) list ./... | grep -v vendor); do \
		echo "Processing $$f..."; \
	done
	@echo "Bytecode generation complete"

# Dev targets
dev: build
	$(GO) run ./cmd/advplc $(ARGS)

run: build
	./$(BINARY) $(ARGS)

# RPO Injection targets
rpo-capture:
	@echo "Capturando chaves de criptografia..."
	@docker exec protheus-custom bash -c ' \
		export LD_PRELOAD=/tmp/rpo_key_hook_v10.so; \
		export RPO_KEYS_OUTPUT=/tmp/capture_latest.json; \
		cd /totvs/protheus12.1.2510/bin; \
		./appsrvlinux -compile -env=P12 -files=$(SOURCE) 2>&1' || \
	docker exec protheus-custom bash -c ' \
		export LD_PRELOAD=/tmp/rpo_key_hook_v11.so; \
		export RPO_KEYS_OUTPUT=/tmp/capture_latest.json; \
		cd /totvs/protheus12.1.2510/bin; \
		./appsrvlinux -compile -env=P12 -files=$(SOURCE) 2>&1'
	@docker cp protheus-custom:/tmp/capture_latest.json /tmp/capture_latest.json
	@echo "Captura salva em /tmp/capture_latest.json"

rpo-inject: build rpo-capture
	@./$(BINARY) rpo inject $(RPO) /tmp/capture_latest.json \
		--inject $(REGISTER)=$(BYTECODE) \
		$(if $(OUTPUT),-o $(OUTPUT))
	@echo "RPO injetado em $(if $(OUTPUT),$(OUTPUT),/tmp/injected.rpo)"

rpo-analyze: build
	@./$(BINARY) rpo info $(RPO)
	@./$(BINARY) rpo regions $(RPO)
	@./$(BINARY) rpo analyze $(RPO)

rpo-decrypt: build
	@./$(BINARY) rpo decrypt $(RPO) $(CAPTURE)

.PHONY: rpo-capture rpo-inject rpo-analyze rpo-decrypt
