# Makefile para AdvPP - Compilador AdvPL/TLPP

.PHONY: all build test clean help cross web \
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

# Frontend PO-UI do `advplc serve`: compila web/ (Angular) e atualiza
# pkg/webui/dist, que vai embutido no binário. Requer Node 20+; só roda
# `npm ci` se web/node_modules ainda não existir.
web:
	cd web && (test -d node_modules || npm ci) && npx ng build
	rm -rf pkg/webui/dist && mkdir -p pkg/webui/dist
	cp -r web/dist/advpp-web/browser/. pkg/webui/dist/

# App PO-UI de teste (plano onda 2, Task 2.7): compila
# tests/poui/fixture-app-src (Angular + @totvs/protheus-lib-core 21.1.4)
# e achata o dist/browser em tests/poui/apps/fixture-app — onde o
# FwCallApp("fixture-app") procura o app. Requer Node 20+.
fixture-app:
	cd tests/poui/fixture-app-src && (test -d node_modules || npm ci) && npx ng build
	cd tests/poui/apps/fixture-app && mv browser/* . && rmdir browser

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

# Package (usado pelo workflow de release): só o CLI, nomes advpp-cli-* —
# os pacotes advpp-<versão>-<plataforma> (suíte completa com as GUIs) vêm
# do job `native`. Mesmo nome nos dois jobs fazia o merge dos artifacts
# misturar os bytes dos dois arquivos (tar.gz corrompido até a v4.7.0).
# Build em diretório próprio: não toca o ./advplc da raiz do repositório.
# Nome de arquivo fixo `advplc`/`advplc.exe` na raiz do pacote — é o que o
# install.sh extrai.
CLI_TARGETS = linux/amd64 linux/arm64 darwin/arm64 windows/amd64
package:
	@echo "Packaging CLI for release $(VERSION)..."
	mkdir -p dist
	@set -e; for t in $(CLI_TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; dir=dist/cli-$$os-$$arch; ext=""; \
		[ "$$os" = windows ] && ext=".exe"; \
		mkdir -p $$dir; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=v$(VERSION)" -o $$dir/advplc$$ext ./cmd/advplc; \
		if [ "$$os" = windows ]; then \
			(cd $$dir && zip -q "../advpp-cli-$(VERSION)-$$os-$$arch.zip" advplc.exe); \
		else \
			tar czf "dist/advpp-cli-$(VERSION)-$$os-$$arch.tar.gz" -C $$dir advplc; \
		fi; \
		rm -rf $$dir; \
	done
	@ls -lh dist/
