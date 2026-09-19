# Integração com Pipeline de Build

**Data:** 2026-09-19 (reescrito — versão anterior descrevia uma
interface de CLI que nunca existiu e um mecanismo de cifra errado)

---

## Visão Geral

Este módulo automatiza, via `LD_PRELOAD`, a captura ao vivo de
chave/IV/cifra usados por `tCryptoEVP::Encrypt` durante uma compilação
Protheus real — a mesma técnica manual documentada em
`docs/rpo-format-sonnet.md` (via `gdb`), mas sem precisar de `gdb`.

**Importante**: o RPO não usa um único algoritmo fixo. `tCryptoEVP::Encrypt`
escolhe, por chamada, uma cifra de uma tabela rotativa de ~12 algoritmos
legados do OpenSSL (DES, 3DES, RC4, RC5, CAST5, Blowfish, RC2 — nunca
AES), com chave/IV efêmeros por sessão de compilação. Não há
descriptografia offline possível sem uma captura feita durante a MESMA
compilação que gerou o RPO. Ver `docs/rpo-format-sonnet.md` para a
investigação completa e `pkg/rpo/cipher_dispatch.go` para a
implementação.

```
┌─────────────────┐     ┌──────────────────────┐     ┌────────────────────┐
│  appsrvlinux     │────▶│  LD_PRELOAD          │────▶│  advplc rpo decrypt│
│  -compile        │     │  rpo_key_hook.so     │     │  (offline)         │
└─────────────────┘     └──────────────────────┘     └────────────────────┘
                              ↓
                    /tmp/rpo_keys_export.json
                    (lista de eventos: setkey/evpinit/encrypt)
```

## Componentes

| Arquivo | Função |
|---------|--------|
| `tools/rpo-live-inspect/rpo_key_hook/rpo_key_hook.cpp` | Biblioteca LD_PRELOAD (hooka `tCryptoEVP::SetKey`, `EVP_EncryptInit_ex`, `EVP_EncryptUpdate`) |
| `tools/build-integration/capture-keys.sh` | Prepara/compila o hook, helpers `wait_for_keys`/`get_key` |
| `tools/build-integration/advpl-build-wrapper.sh` | Wrapper: roda um comando sob o hook e mostra o resultado |
| `pkg/rpo/cipher_dispatch.go` | `EncryptSegment`/`DecryptSegment` — as ~12 cifras reais, testadas |
| `cmd/advplc/cmd_rpo_decrypt.go` | `advplc rpo decrypt` — usa a captura pra decodificar o RPO |

## Uso

### 1. Capturar durante a compilação

```bash
# Dentro do container/ambiente com o appserver:
cd tools/rpo-live-inspect/rpo_key_hook && make   # gera rpo_key_hook.so (não comitado)

cd /protheus12/bin/appserver
export LD_LIBRARY_PATH=.
LD_PRELOAD=/caminho/para/rpo_key_hook.so \
  ./appsrvlinux -compile -env=P12 -files=fonte.prw -includes=/protheus12/apo

cat /tmp/rpo_keys_export.json   # lista de eventos capturados
```

Ou via o wrapper (mesma coisa, com relatório de sucesso/falha):

```bash
./tools/build-integration/advpl-build-wrapper.sh \
  /protheus12/bin/appserver/appsrvlinux \
  -compile -env=P12 -files=fonte.prw -includes=/protheus12/apo
```

Alternativa sem compilar o hook (mais lenta, mas sem precisar de
`g++`): `advplc rpo extract <rpo> --auto`, que roda um script `gdb`
equivalente — ver `docs/MANUAL_ADVPLC.md`.

### 2. Decodificar o RPO com a captura

```bash
advplc rpo decrypt /protheus12/apo/custom.rpo /tmp/rpo_keys_export.json
```

Saída real (exemplo, `pkg/rpo/testdata/live_capture.{rpo,json}` —
fixture comitada no repositório):

```
RPO: pkg/rpo/testdata/live_capture.rpo (admin=336B body=20710B)
Captura: pkg/rpo/testdata/live_capture.json (15 segmentos)

#1 des_ede_ecb_cipher (158 bytes): admin_section offset 176
    zlib inflate OK (188 bytes): "...RPORC5_TRIGGER.PRW..."
...
#15 cast5_cbc_cipher (262 bytes): body.bin offset 0
    zlib inflate OK (818 bytes): "...SIGA.MAP...SIGAANNOT.MAP..."

10/15 segmentos decodificados e confirmados contra o RPO real.
```

(As 5 lacunas são cifras `idea_*` — ver aviso em `pkg/rpo/idea.go`:
implementação de IDEA ainda não passa em validação, retorna erro
explícito em vez de decodificar errado silenciosamente.)

## Configuração

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `RPO_KEYS_OUTPUT` | Caminho do arquivo de saída do hook | `/tmp/rpo_keys_export.json` |

## Troubleshooting

**"Hook não encontrado"**: `cd tools/rpo-live-inspect/rpo_key_hook && make clean && make` (o `.so` nunca é comitado — ver `.gitignore`).

**Nenhum evento capturado**: confira se o compile usou `-env=`/`-includes=` corretos (sem isso o appserver falha com "Invalid Environment" antes de tocar em qualquer cifra) e se rodou com `LD_LIBRARY_PATH=.` a partir do diretório do `appsrvlinux` (ele precisa achar suas próprias `.so`).

**`advplc rpo decrypt` diz "SEM MATCH" pra todo segmento**: a captura não é da MESMA compilação que gerou esse `.rpo` específico — chave/IV são efêmeros por sessão, uma captura de uma compilação não decodifica o RPO de outra.

## Testes

```bash
# Hook (end-to-end, contra appserver real — requer container)
cd tools/rpo-live-inspect/rpo_key_hook
make && ./test_hook.sh /protheus12/bin/appserver/appsrvlinux

# Dispatcher de cifras (unitário, usa fixture comitada, não precisa de container)
go test ./pkg/rpo/... -run TestCipherDispatch -v
```
