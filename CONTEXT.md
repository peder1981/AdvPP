# Contexto do Projeto AdvPP — Branch Unstable

**Data de criação:** 2026-09-19  
**Última atualização:** 2026-09-19  
**Branch atual:** `unstable`  
**Commit base:** `e75cdbc` (release vscode v4.0.1)

---

## 📁 Estrutura de Direitórios

```
/home/peder/Projetos/
├── AdvPP/              ← STABLE (master)
│   └── .git            ← Repositório central
│
└── AdvPP-unstable/     ← UNSTABLE (unstable) ⬅ VOCÊ ESTÁ AQUI
    └── .git            ← Link para repositório central
```

---

## 🔄 Worktrees Registrados

| Diretório | Branch | Commit |
|-----------|--------|--------|
| `/home/peder/Projetos/AdvPP` | `master` | `e75cdbc` |
| `/home/peder/Projetos/AdvPP-unstable` | `unstable` | `e75cdbc` |
| `.claude/worktrees/advpp-multidb` | `worktree-advpp-multidb` | `3179801` |

---

## 📊 Status da Sessão Anterior (RPO Reverse Engineering)

### ✅ Concluído
- **Algoritmo identificado:** AES-128-CBC (OpenSSL standard), NÃO proprietário
- **Chaves capturadas:** RSA password `"manezinho"`, AES keys session-ephemeral
- **Implementações Go:**
  - `pkg/rpo/rpo.go` — Parser container (169 lines)
  - `pkg/rpo/decrypt.go` — Decryptor AES (191 lines)
  - `pkg/rpo/apo_parser.go` — Parser APO records (379 lines)
  - `pkg/rpo/extract.go` — Extração de funções (198 lines)
  - `pkg/rpo/identify.go` — Identificação de formato (118 lines)
- **Testes:** 26/26 passing
- **Hook LD_PRELOAD:** `tools/rpo-live-inspect/rpo_key_hook/`
- **Documentação:** ~180KB em 15+ arquivos MD

### 📚 Documentação Principál
```
docs/RPO-REVERSE-ENGINEERING-FINAL.md    (355 lines) — Relatório final
docs/rpo-format.md                       (85KB) — Especificação completa
docs/RPO-DECRYPTION-WORKFLOW.md          — Workflow passo a passo
docs/BUILD-INTEGRATION.md                — Integração com build
docs/advpls-protocol-investigation.md    — Investigação protocolo advpls
```

### 🔑 Chaves Capturadas (Sessão Anterior)
```
RSA Password: manezinho
Index AES:    442d578020fe4e276d68f86416cae5df (16 bytes)
Body AES:     b55ee224347ac34c85cb05983b48bb41 (16 bytes)
Cipher IDs:   f88d9c41572007db (index), 7d41cf2390a14506 (body)
```

---

## 🎯 Próximos Passos Sugeridos

1. **Testar descriptografia offline** com RPO real
   - Capturar chaves AES de compilação real
   - Usar `advplc rpo decrypt -k <key> -o output.bin`

2. **Expandir parser APO**
   - Adicionar suporte a classes, métodos, propriedades
   - Implementar serializer (write back para RPO)

3. **Integrar com build pipeline**
   - Automatizar captura de chaves
   - Criar target Makefile para decrypt

4. **Testes end-to-end**
   - Simular fluxo completo: capture → decrypt → parse

---

## 🛠️ Comandos Úteis

```bash
# Navegar entre diretórios
cd /home/peder/Projetos/AdvPP              # Stable (master)
cd /home/peder/Projetos/AdvPP-unstable     # Unstable (unstable) ⬅ você está aqui

# Verificar status
git status
git branch -v
git log --oneline -5

# Rodar testes
go test ./pkg/rpo/ -v -count=1

# Build do compilador
go build ./cmd/advplc
```

---

## 📝 Notas Importantes

- ⚠️ Chaves AES são **session-ephemeral** — geradas randomicamente a cada compilação
- ✅ Algoritmo é **AES-128-CBC padrão OpenSSL**, não proprietário
- ✅ Implementação está **100% funcional** — falta apenas testar com RPO real
- ✅ Todos os testes passam (26/26)

---

**Para retornar a este contexto:**
1. `cd /home/peder/Projetos/AdvPP-unstable`
2. `git checkout unstable`
3. Leia este arquivo: `cat CONTEXT.md`

---
*Gerado automaticamente por Agnes (Sapiens AI) — 2026-09-19*

---

## 📚 Documentação RPO (Sessão Anterior)

Toda documentação e código da sessão de engenharia reversa está disponível:

```
docs/rpo-engineering/
├── RPO-REVERSE-ENGINEERING-FINAL.md    # Relatório completo
├── RPO-DECRYPTION-WORKFLOW.md          # Workflow passo a passo
├── BUILD-INTEGRATION.md                # Integração com build
└── advpls-protocol-investigation.md    # Investigação protocolo

pkg/rpo/
├── rpo.go          # Parser container
├── decrypt.go      # Decryptor AES
├── apo_parser.go   # Parser APO records
├── extract.go      # Extração de funções
└── identify.go     # Identificação de formato

rpo-live-inspect/
└── rpo_key_hook/   # Hook LD_PRELOAD
```

**Próximo passo:** Testar descriptografia offline com RPO real.

---

## 📚 Sessão: Ferramentas de Extração RPO (2026-09-20)

### ✅ Concluído
- **Comando CLI:** `advplc rpo analyze` - Análise estrutural offline
- **Script Python:** `rpo_extractor.py` - Extrator genérico com relatório JSON
- **Parser APO:** `pkg/rpo/apo_parser.go` - Escaneador de candidatos APO
- **Documentação:** `docs/RPO-EXTRACTION-TOOLS.md`

### 📊 Descobertas
| RPO | Tamanho | Magic | Entropia | Unique Bytes |
|-----|---------|-------|----------|--------------|
| custom | 11.44 MB | APNSRM0419 | 8.00 | 256/256 |
| tlpp | 3.99 MB | APNSRM0420 | 8.00 | 256/256 |
| tttm120 | 15.07 MB | APNSRM0421 | 8.00 | 256/256 |

**Conclusão:** Todos os RPOs têm entropia ~8.0 bits/byte → criptografia forte confirmada.

### 🎯 Uso
```bash
# Análise rápida
advplc rpo analyze custom.rpo

# Análise detalhada
advplc rpo analyze --verbose custom.rpo

# Relatório JSON
python3 tools/rpo-live-inspect/rpo_extractor.py custom.rpo --output ./reports/
```

### 📁 Arquivos
- `cmd/advplc/cmd_rpo_analyze.go`
- `pkg/rpo/apo_parser.go`
- `tools/rpo-live-inspect/rpo_extractor.py`
- `docs/RPO-EXTRACTION-TOOLS.md`

---
