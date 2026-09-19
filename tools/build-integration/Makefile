.PHONY: hook decrypt clean help

HOOK_DIR := ../rpo-live-inspect/rpo_key_hook
HOOK_SO  := $(HOOK_DIR)/rpo_key_hook.so
KEYS     := /tmp/rpo_keys_export.json

help:
	@echo "RPO capture & decrypt workflow (ver docs/BUILD-INTEGRATION.md)"
	@echo ""
	@echo "  make hook                                   - compila rpo_key_hook.so"
	@echo "  make decrypt RPO=custom.rpo                 - decodifica RPO com a ultima captura"
	@echo "  make clean                                  - remove arquivos gerados"
	@echo ""
	@echo "Pra capturar, rode o appserver sob o hook diretamente:"
	@echo "  LD_PRELOAD=$(HOOK_SO) ./appsrvlinux -compile -env=P12 -files=f.prw -includes=/protheus12/apo"
	@echo "(precisa LD_LIBRARY_PATH=. e -env=/-includes= corretos, ver troubleshooting no doc)"

hook: $(HOOK_SO)

$(HOOK_SO):
	$(MAKE) -C $(HOOK_DIR)

decrypt: hook
	@if [ -z "$(RPO)" ]; then \
		echo "Uso: make decrypt RPO=custom.rpo  (usa a captura mais recente em $(KEYS))"; \
		exit 1; \
	fi
	@if [ ! -f "$(KEYS)" ]; then \
		echo "Nenhuma captura em $(KEYS) — rode o appserver sob LD_PRELOAD=$(HOOK_SO) primeiro."; \
		exit 1; \
	fi
	go run ../../cmd/advplc rpo decrypt "$(RPO)" "$(KEYS)"

clean:
	rm -f $(HOOK_SO) $(KEYS)
	@echo "[build-integration] limpo"
