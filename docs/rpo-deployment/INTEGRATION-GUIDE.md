# Guia de Integração - RPO Bytecode Injector

## Visão Geral

Este guia demonstra como integrar o RPO Bytecode Injector em workflows de desenvolvimento.

## Requisitos

- Go 1.27+
- Docker (para containers Protheus)
- Access ao source code do Protheus 12.1.2510

## Instalação

```bash
# Clonar repositório
git clone <repo-url>
cd AdvPP-unstable

# Build
make build

# Testes
make test
```

## Workflow Básico

### Passo 1: Preparar Ambiente Docker

```bash
# Iniciar container
docker-compose up -d protheus-custom

# Verificar
docker ps | grep protheus-custom
```

### Passo 2: Compilar com Hook

```bash
# Criar fonte de teste
cat > /tmp/teste.prw << 'EOF'
User Function Teste()
    Local cVar := "Hello"
Return cVar
