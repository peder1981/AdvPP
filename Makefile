# AdvPP — build, cross-compile, empacotamento e release
#
# Uso rápido:
#   make build                 # compila as 4 ferramentas para a máquina local
#   make test                  # build + verifica todos os fixtures de tests/
#   make cross                 # cross-compila o advplc (CLI) p/ Linux/Win64/macOS em dist/
#   make package VERSION=1.1.0 # gera os pacotes .tar.gz/.zip em dist/
#   make release VERSION=1.1.0 # cria e publica a tag vVERSION — o GitHub Actions
#                              # compila nativamente nas 3 plataformas (incl. GUI Fyne)
#                              # e anexa todos os pacotes à Release

VERSION ?= dev
LDFLAGS  = -s -w -X main.version=v$(VERSION)
GOFLAGS  = -trimpath -ldflags '$(LDFLAGS)'
TOOLS    = advplc advcfg adveditor advpp-ide
# Alvos do CLI (puro Go, CGO_ENABLED=0). GUIs Fyne exigem build nativo (CI).
CLI_TARGETS = linux/amd64 linux/arm64 windows/amd64 darwin/arm64

.PHONY: build test cross package release clean web

# Recompila o frontend PO-UI (advplc serve) e embute em pkg/webui/dist.
# Requer Node 20+; o dist é versionado, então `go build` funciona sem Node.
web:
	cd web && npx ng build
	rm -rf pkg/webui/dist
	cp -r web/dist/advpp-web/browser pkg/webui/dist

build:
	@for t in $(TOOLS); do \
		echo "building $$t"; \
		go build $(GOFLAGS) -o $$t ./cmd/$$t || exit 1; \
	done

test: build
	@go vet ./...
	@pass=0; fail=0; \
	for f in tests/*.prw tests/*.tlpp; do \
		if ./advplc check $$f >/dev/null 2>&1; then pass=$$((pass+1)); \
		else fail=$$((fail+1)); echo "FAIL: $$f"; fi; \
	done; \
	echo "fixtures: $$pass pass, $$fail fail"
	@# real_protheus_test.prw é falha conhecida pré-existente (parser)

cross:
	@mkdir -p dist
	@for target in $(CLI_TARGETS); do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		ext=""; [ $$goos = windows ] && ext=".exe"; \
		out=dist/advplc-$$goos-$$goarch$$ext; \
		echo "building $$out"; \
		GOOS=$$goos GOARCH=$$goarch CGO_ENABLED=0 \
			go build $(GOFLAGS) -o $$out ./cmd/advplc || exit 1; \
	done

package: cross
	@cd dist && for target in $(CLI_TARGETS); do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		name=advpp-cli-$(VERSION)-$$goos-$$goarch; \
		if [ $$goos = windows ]; then \
			cp advplc-$$goos-$$goarch.exe advplc.exe && \
			zip -q $$name.zip advplc.exe && rm advplc.exe; \
		else \
			cp advplc-$$goos-$$goarch advplc && \
			tar czf $$name.tar.gz advplc && rm advplc; \
		fi; \
		echo "packaged dist/$$name"; \
	done

release:
	@[ "$(VERSION)" != "dev" ] || { echo "uso: make release VERSION=1.1.0"; exit 1; }
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)
	@echo "Tag v$(VERSION) publicada — acompanhe o build em:"
	@echo "  https://github.com/peder1981/AdvPP/actions"

clean:
	rm -rf dist $(TOOLS) *.exe
