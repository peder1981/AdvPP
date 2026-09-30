# TIR Testing Guide — AdvPP

## Status Atual

**NÃO É POSSÍVEL executar TIR tests no momento** devido à ausência de um ambiente
Protheus Webapp/SmartClient rodando.

### Problemas Identificados

1. **Container Protheus não inicia aplicação**
   - A imagem `protheus:12.1.2510` tem CMD `sleep infinity`
   - Precisa de configuração adicional para iniciar o application server

2. **Banco PostgreSQL vazio**
   - O banco `p1212510` existe mas não contém tabelas do Protheus (SA1, SC5, etc.)
   - Precisa ser populado com dados de teste

3. **TIR instalado corretamente**
   - Pacote `tir_framework-2.14.10` instalado
   - Import `from tir import Webapp` funciona

## Pré-requisitos para TIR

### 1. Ambiente Protheus Rodando

Opção A — Usar container com compose:
```bash
docker-compose up -d protheus
```

Opção B — Iniciar manualmente:
```bash
docker run -d \
  --name protheus-tir \
  --network microsiga_protheus \
  -e DBSERVER=postgresql \
  -e DBUSER=protheus \
  -e DBPASSWORD=protheus \
  -e DBNAME=p1212510 \
  -e COMPANY=01 \
  -e MODULE=TM1 \
  -p 8080:8080 \
  protheus:12.1.2510
```

**Nota:** O container precisa de script de inicialização que start o application server.

### 2. Dados de Teste

Popular banco com tabelas mínimas:
```sql
CREATE TABLE SA1 (
  A1_COD VARCHAR(6) NOT NULL,
  A1_NOME VARCHAR(40) NOT NULL,
  A1_END VARCHAR(30),
  A1_CIDADE VARCHAR(20),
  D_E_L_E_T_ CHAR(1) DEFAULT ' ',
  PRIMARY KEY (A1_COD)
);

INSERT INTO SA1 VALUES ('01', 'CLIENTE TESTE', 'RUA TESTE', 'CIDADE TESTE', ' ');
```

### 3. Configuração TIR

Arquivo `tests/tir/config.json`:
```json
{
  "Url": "http://localhost:8080/",
  "Browser": "Chrome",
  "Environment": "TM1",
  "User": "admin",
  "Password": "admin",
  "Language": "pt-br"
}
```

## Tests Criados

### SA1 — Cadastro de Clientes
- **Arquivos:** `SA1TESTCASE.py`, `SA1TESTSUITE.py`
- **Casos de teste:**
  - CT001: Incluir cliente
  - CT002: Alterar cliente
  - CT003: Validação campo obrigatório
  - CT004: Excluir cliente

### SC5 — Notas Fiscais
- **Arquivos:** `SC5TESTCASE.py`, `SC5TESTSUITE.py`
- **Casos de teste:**
  - CT001: Visualizar NF
  - CT002: Imprimir relatório

## Como Executar (quando ambiente estiver pronto)

```bash
# Navegar para diretório dos testes
cd tests/tir

# Executar suite completa
python3 SA1TESTSUITE.py

# Executar caso específico
python3 -m unittest SA1TESTCASE.SA1.test_SA1_CT001

# Com verbosidade
python3 SA1TESTSUITE.py -v
```

## Alternativas Imediatas

### 1. Testes Unitários AdvPL (ProBat)
Executar testes existentes:
```bash
./advplc check tests/dictionary_update_test.prw
```

### 2. Testes de Integração via SQL
Verificar dados diretamente no PostgreSQL:
```bash
docker exec postgresql psql -U protheus -d p1212510 -c "SELECT 1+1;"
```

### 3. Mock Tests
Criar tests que não dependem de ambiente Protheus real.

## Próximos Passos

1. [ ] Configurar container Protheus com application server
2. [ ] Poplar banco com dados de teste
3. [ ] Validar conexão TIR
4. [ ] Executar suite SA1
5. [ ] Executar suite SC5
6. [ ] Adicionar mais casos de teste

## Referências

- [TIR Documentation](https://github.com/totvs/tir)
- [TOTVS TIR Framework](https://github.com/totvs/tir)
- `tests/tir/SA1TESTCASE.py` — Exemplo de implementação
