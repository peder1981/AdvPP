# Makefile — rpo_key_hook.so
# Compila para x86_64 Linux (Oracle Linux 9, Debian 12, etc.)
# Requer: g++ 7+ (C++17), binutils (nm, objdump)
#
# Uso:
#   make             — compila
#   make clean       — limpa
#   make info        — mostra symbols esperados do binário Protheus

CXX      := g++
CXXFLAGS := -Wall -Wextra -O2 -fPIC -std=c++17
LDFLAGS  := -shared -ldl
TARGET   := rpo_key_hook.so
SRC      := rpo_key_hook.cpp

.PHONY: all clean info

all: $(TARGET)

$(TARGET): $(SRC)
	$(CXX) $(CXXFLAGS) -o $@ $< $(LDFLAGS)
	@echo ">>> Compiled $(TARGET)"
	@size $(TARGET) 2>/dev/null || true
	@nm -D $(TARGET) 2>/dev/null | grep -i setkey || true

clean:
	rm -f $(TARGET) *.o *.tmp.*
	@echo ">>> Cleaned"

# Ajuda: lista symbols mangled relacionados a tCryptoEVP no binário target
# Uso: make info APPSRV=/protheus12/bin/appserver/appsrvlinux
info:
	@if [ -z "$(APPSRV)" ]; then \
		echo "Usage: make info APPSRV=/caminho/para/appsrvlinux"; \
		exit 1; \
	fi
	@echo ">>> Symbols tCryptoEVP em $(APPSRV):"
	@nm -D "$(APPSRV)" 2>/dev/null | grep -i 'crypto.*setkey\|setkey.*crypto' | head -20
	@echo "---"
	@objdump -T "$(APPSRV)" 2>/dev/null | grep -i 'crypto.*setkey\|setkey.*crypto' | head -20
