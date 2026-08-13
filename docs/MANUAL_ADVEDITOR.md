# Manual do Usuário — AdvEditor (Editor de Banco de Dados)

## Introdução

O AdvEditor (`adveditor`) é um editor gráfico (Fyne) do banco SQLite local
compartilhado pelo AdvPP (`~/.advpp/ADVPP.db`, ou outro `.db` que você
abrir): visualizar/editar dados, gerenciar estrutura de tabelas e índices.
Só o driver **SQLite** lê/escreve dados de verdade hoje — DBF/TopConnect/
Ctree/BTrieve aparecem na tela de seleção de driver, mas os drivers
correspondentes (`pkg/tools/shared/database.go`) ainda são stubs que não
fazem I/O real (não leem nem escrevem nenhum arquivo/servidor de verdade).
Se você selecionar um deles, a abertura "funciona" sem erro mas não há
dado real por trás.

## Requisitos do Sistema

- **Sistema Operacional**: Linux, Windows ou macOS
- **Memória RAM**: Mínimo 2GB
- **Processador**: x86_64 (amd64) ou ARM64

## Instalação

### Via Pacote Debian/Ubuntu (Linux)

```bash
curl -sL https://api.github.com/repos/peder1981/AdvPP/releases/latest \
  | grep browser_download_url | grep amd64.deb | cut -d'"' -f4 | xargs wget
sudo dpkg -i advpp_*_amd64.deb
sudo apt-get install -f
```

### Via Compilação

```bash
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP
go build -o adveditor ./cmd/adveditor
sudo cp adveditor /usr/local/bin/   # opcional
```

## Iniciando

```bash
adveditor
```

O AdvEditor abre automaticamente `~/.advpp/ADVPP.db` ao iniciar, se ele
existir (criado sob demanda pelo `advplc`/outras ferramentas AdvPP).

## Interface

Árvore de tabelas à esquerda, grid de dados no centro, barra de status
embaixo, barra de menu no topo.

## Menu Arquivo

| Item | O que faz |
|---|---|
| Abrir (Ctrl+B) | Abre a tela de seleção de driver/arquivo (ver abaixo) |
| Trocar Banco de Dados | Fecha o atual e abre outro |
| Fechar | Fecha a tabela/banco atual |
| Sair | Fecha a janela |

### Seleção de driver

- **SQLite** (padrão, recomendado): abre `~/.advpp/ADVPP.db` diretamente
  ou o `.db`/`.sqlite`/`.sqlite3` escolhido — dados reais, leitura e
  escrita.
- **DBF / TopConnect / Ctree / BTrieve**: presentes na lista por
  compatibilidade de interface com o legado Protheus, mas **não fazem
  I/O real** no estado atual do projeto — não leem o `.dbf` do disco nem
  se conectam a um servidor de verdade. Não use para trabalho real ainda.

## Menu Tabela

| Item | O que faz |
|---|---|
| Nova Tabela | Cria tabela (define campos via diálogo) |
| Excluir Tabela | Remove a tabela selecionada |
| Estrutura | Mostra os campos da tabela (nome/tipo/tamanho/decimais) |
| Adicionar Campo | Adiciona coluna à tabela |
| Remover Campo | Remove coluna da tabela |

## Menu Editar

| Item | O que faz |
|---|---|
| Incluir | Abre formulário para novo registro |
| Alterar | Edita o registro selecionado no grid |
| Excluir | Remove o registro selecionado (com confirmação) |

## Menu Índice

| Item | O que faz |
|---|---|
| Criar | Cria índice sobre campo(s) da tabela |
| Excluir | Remove um índice existente |

## Menu Ajuda

**Sobre**: informações de versão.

## O que este editor NÃO tem (evite documentação/expectativa contrária)

- Sem importação/exportação (CSV/JSON/XML/Excel) — não existe no código.
- Sem SQL personalizado, filtro avançado por menu, operações em lote
  (atualizar/excluir/copiar em massa), transações explícitas
  (commit/rollback pela UI), geração de relatórios, backup automático ou
  auditoria configurável.
- Drivers DBF/TopConnect/Ctree/BTrieve são placeholders de interface, não
  implementações funcionais (ver acima).

## Solução de Problemas

**Erro ao abrir banco**: confirme que o caminho existe e você tem
permissão de leitura/escrita nele.

**Log de Erros**: mensagens de erro aparecem em diálogo na própria janela
(não há arquivo de log dedicado do AdvEditor hoje).

### Suporte

- **GitHub Issues**: https://github.com/peder1981/AdvPP/issues
- **Repositório**: https://github.com/peder1981/AdvPP
