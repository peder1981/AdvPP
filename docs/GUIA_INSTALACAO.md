# Guia de Instalação - AdvPP

## Introdução

Este guia fornece instruções detalhadas para instalar o AdvPP em sistemas Linux. O AdvPP é uma suite completa de ferramentas para desenvolvimento AdvPL/TLPP, incluindo IDE, configurador de tabelas, editor de banco de dados e compilador.

## Requisitos do Sistema

### Requisitos Mínimos

- **Sistema Operacional**: Linux (Ubuntu 20.04+, Debian 11+, Fedora 35+)
- **Arquitetura**: x86_64 (amd64)
- **Memória RAM**: 4GB
- **Espaço em Disco**: 500MB
- **Processador**: Dual-core 1.5GHz

### Requisitos Recomendados

- **Sistema Operacional**: Ubuntu 22.04+, Debian 12+, Fedora 38+
- **Arquitetura**: x86_64 (amd64)
- **Memória RAM**: 8GB
- **Espaço em Disco**: 1GB
- **Processador**: Quad-core 2.0GHz

### Dependências do Sistema

- **libc6**: Biblioteca C padrão
- **libgcc-s1**: Biblioteca GCC runtime
- **libstdc++6**: Biblioteca C++ padrão

## Métodos de Instalação

### Método 1: Pacote Debian/Ubuntu (Recomendado)

Este é o método mais simples e recomendado para usuários de Debian, Ubuntu e distribuições derivadas.

#### Passo 1: Baixar o Pacote

Acesse https://github.com/peder1981/AdvPP/releases e baixe o
`advpp_<versão>_amd64.deb` mais recente, ou via linha de comando:

```bash
# Baixa o .deb da última release automaticamente
curl -sL https://api.github.com/repos/peder1981/AdvPP/releases/latest \
  | grep browser_download_url | grep amd64.deb | cut -d'"' -f4 | xargs wget
```

#### Passo 2: Instalar o Pacote

```bash
# Instalar o pacote
sudo dpkg -i advpp_*_amd64.deb
```

#### Passo 3: Resolver Dependências

Se houver dependências não satisfeitas, execute:

```bash
sudo apt-get install -f
```

#### Passo 4: Verificar Instalação

```bash
# Verificar se os comandos estão disponíveis
which advpp-ide
which advcfg
which adveditor
which advplc

# Testar versão
advpp-ide --version
```

#### Passo 5: Limpeza

```bash
# Remover o pacote baixado
rm advpp_1.0.0_amd64.deb
```

### Método 2: Compilação a Partir do Código Fonte

Este método é recomendado para desenvolvedores que desejam modificar o código ou necessitam de uma versão específica.

#### Passo 1: Instalar Dependências de Compilação

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install -y golang git build-essential
```

**Fedora:**
```bash
sudo dnf install -y golang git gcc make
```

#### Passo 2: Clonar o Repositório

```bash
# Clonar o repositório
git clone https://github.com/peder1981/AdvPP.git
cd AdvPP
```

#### Passo 3: Compilar as Ferramentas

```bash
# Compilar todas as ferramentas
make build

# (ou individualmente)
go build -o advplc ./cmd/advplc
```

#### Passo 4: Instalar as Ferramentas

```bash
# Instalar globalmente
sudo cp advpp-ide /usr/local/bin/
sudo cp advcfg /usr/local/bin/
sudo cp adveditor /usr/local/bin/
sudo cp advplc /usr/local/bin/

# Tornar executáveis
sudo chmod +x /usr/local/bin/advpp-ide
sudo chmod +x /usr/local/bin/advcfg
sudo chmod +x /usr/local/bin/adveditor
sudo chmod +x /usr/local/bin/advplc
```

#### Passo 5: Verificar Instalação

```bash
# Verificar se os comandos estão disponíveis
which advpp-ide
which advcfg
which adveditor
which advplc
```

### Método 3: Instalação Manual

Este método é útil quando você não tem permissões de administrador ou prefere instalar em um diretório específico.

#### Passo 1: Baixar os Binários

```bash
# Criar diretório de instalação
mkdir -p ~/advpp/bin
cd ~/advpp/bin

# Baixar o pacote da sua plataforma em:
#   https://github.com/peder1981/AdvPP/releases
# Linux:   advpp-<versão>-linux-amd64.tar.gz   (suite completa)
# Windows: advpp-<versão>-windows-amd64.zip
# macOS:   advpp-<versão>-darwin-arm64.tar.gz (Apple Silicon)
#          advpp-<versão>-darwin-amd64.tar.gz (Intel)
# Somente CLI (todas as plataformas): advpp-cli-<versão>-*.tar.gz/.zip
tar xzf advpp-*.tar.gz
```

#### Passo 2: Tornar Executáveis

```bash
chmod +x advpp-ide
chmod +x advcfg
chmod +x adveditor
chmod +x advplc
```

#### Passo 3: Adicionar ao PATH

```bash
# Adicionar ao ~/.bashrc
echo 'export PATH="$HOME/advpp/bin:$PATH"' >> ~/.bashrc

# Recarregar o arquivo
source ~/.bashrc
```

#### Passo 4: Verificar Instalação

```bash
# Verificar se os comandos estão disponíveis
which advpp-ide
which advcfg
which adveditor
which advplc
```

## Configuração Pós-Instalação

### Configurar Diretórios

#### Criar Diretório de Dados

```bash
# Criar diretório para dados
mkdir -p ~/advpp/data

# Criar diretório para projetos
mkdir -p ~/advpp/projects

# Criar diretório para logs
mkdir -p ~/.advpp/logs
```

#### Configurar Variáveis de Ambiente

```bash
# Adicionar ao ~/.bashrc
echo 'export ADVP_DATA_DIR="$HOME/advpp/data"' >> ~/.bashrc
echo 'export ADVP_PROJECT_DIR="$HOME/advpp/projects"' >> ~/.bashrc
echo 'export ADVP_LOG_DIR="$HOME/.advpp/logs"' >> ~/.bashrc

# Recarregar o arquivo
source ~/.bashrc
```

### Configurar Dicionário Padrão

#### Criar Dicionário Inicial

```bash
# O dicionário será criado automaticamente na primeira execução
# Mas você pode criar manualmente se desejar
advcfg
```

O AdvCfg criará automaticamente o arquivo `~/.advpp/ADVPP.db` com as tabelas do dicionário.

#### Configurar Caminho do Dicionário

```bash
# Criar arquivo de configuração
mkdir -p ~/.advpp
cat > ~/.advpp/advpp_config.json << EOF
{
  "default_database": "~/.advpp/ADVPP.db",
  "recent_files": [],
  "editor_settings": {
    "theme": "dark",
    "font_size": 12,
    "tab_size": 4
  }
}
EOF
```

### Configurar Integração com Menu do Sistema

#### Criar Arquivos .desktop

**AdvPP IDE:**
```bash
cat > ~/.local/share/applications/advpp-ide.desktop << EOF
[Desktop Entry]
Name=AdvPP IDE
Comment=AdvPL/TLPP Integrated Development Environment
Exec=advpp-ide
Icon=advpp-ide
Type=Application
Categories=Development;IDE;
Terminal=false
EOF
```

**AdvCfg:**
```bash
cat > ~/.local/share/applications/advcfg.desktop << EOF
[Desktop Entry]
Name=AdvCfg
Comment=AdvPL/TLPP Table Configuration Tool
Exec=advcfg
Icon=advcfg
Type=Application
Categories=Development;Database;
Terminal=false
EOF
```

**AdvEditor:**
```bash
cat > ~/.local/share/applications/adveditor.desktop << EOF
[Desktop Entry]
Name=AdvEditor
Comment=AdvPL/TLPP Database Editor
Exec=adveditor
Icon=adveditor
Type=Application
Categories=Development;Database;
Terminal=false
EOF
```

#### Atualizar Banco de Dados de Aplicativos

```bash
update-desktop-database ~/.local/share/applications/
```

## Verificação da Instalação

### Testar Todas as Ferramentas

```bash
# Testar AdvPP IDE
advpp-ide --version

# Testar AdvCfg
advcfg --version

# Testar AdvEditor
adveditor --version

# Testar AdvPlc
advplc --version
```

### Testar Funcionalidade Básica

#### Testar Compilador

```bash
# Criar arquivo de teste
cat > ~/advpp/test.prw << EOF
#include "totvs.ch"

function Hello()
    Alert("Hello, World!")
return
EOF

# Compilar
cd ~/advpp
advplc test.prw

# Verificar se o bytecode foi criado
ls -la test.bytecode
```

#### Testar AdvCfg

```bash
# Iniciar AdvCfg
advcfg

# Verificar se o dicionário foi criado
ls -la ~/.advpp/ADVPP.db
```

#### Testar AdvEditor

```bash
# Iniciar AdvEditor
adveditor

# Abrir o dicionário
# Arquivo → Abrir → ~/.advpp/ADVPP.db
```

## Desinstalação

### Desinstalar Pacote Debian/Ubuntu

```bash
# Remover o pacote
sudo dpkg -r advpp

# Remover arquivos de configuração
sudo dpkg -P advpp

# Remover diretórios de dados (opcional)
rm -rf ~/advpp
rm -rf ~/.advpp
```

### Desinstalar Compilação Manual

```bash
# Remover binários
sudo rm /usr/local/bin/advpp-ide
sudo rm /usr/local/bin/advcfg
sudo rm /usr/local/bin/adveditor
sudo rm /usr/local/bin/advplc

# Remover diretórios de dados (opcional)
rm -rf ~/advpp
rm -rf ~/.advpp

# Remover arquivos .desktop
rm ~/.local/share/applications/advpp-ide.desktop
rm ~/.local/share/applications/advcfg.desktop
rm ~/.local/share/applications/adveditor.desktop

# Atualizar banco de dados de aplicativos
update-desktop-database ~/.local/share/applications/
```

## Atualização

### Atualizar Pacote Debian/Ubuntu

```bash
# Baixar nova versão
# Baixe o .deb mais recente em https://github.com/peder1981/AdvPP/releases
curl -sL https://api.github.com/repos/peder1981/AdvPP/releases/latest \
  | grep browser_download_url | grep amd64.deb | cut -d'"' -f4 | xargs wget

# Instalar nova versão
sudo dpkg -i advpp_*_amd64.deb

# Resolver dependências se necessário
sudo apt-get install -f
```

### Atualizar Compilação Manual

```bash
# Clonar ou atualizar repositório
cd AdvPP
git pull origin master

# Recompilar
go build -o advpp-ide ./cmd/advpp-ide
go build -o advcfg ./cmd/advcfg
go build -o adveditor ./cmd/adveditor
go build -o advplc ./cmd/advplc

# Reinstalar
sudo cp advpp-ide /usr/local/bin/
sudo cp advcfg /usr/local/bin/
sudo cp adveditor /usr/local/bin/
sudo cp advplc /usr/local/bin/
```

## Solução de Problemas

### Problema: Comando não encontrado

**Sintoma:**
```bash
$ advpp-ide
bash: advpp-ide: command not found
```

**Solução:**
```bash
# Verificar se o binário existe
which advpp-ide

# Se não existir, reinstale
sudo dpkg -i advpp_*_amd64.deb

# Se existir, adicione ao PATH
export PATH="/usr/local/bin:$PATH"
```

### Problema: Dependências não satisfeitas

**Sintoma:**
```bash
$ sudo dpkg -i advpp_*_amd64.deb
dpkg: dependency problems prevent configuration...
```

**Solução:**
```bash
# Resolver dependências
sudo apt-get install -f

# Se ainda falhar, instale manualmente
sudo apt-get install libc6 libgcc-s1 libstdc++6
```

### Problema: Permissão negada

**Sintoma:**
```bash
$ advpp-ide
bash: /usr/local/bin/advpp-ide: Permission denied
```

**Solução:**
```bash
# Tornar executável
sudo chmod +x /usr/local/bin/advpp-ide
sudo chmod +x /usr/local/bin/advcfg
sudo chmod +x /usr/local/bin/adveditor
sudo chmod +x /usr/local/bin/advplc
```

### Problema: Biblioteca não encontrada

**Sintoma:**
```bash
$ advpp-ide
error while loading shared libraries: libxxx.so: cannot open shared object file
```

**Solução:**
```bash
# Instalar biblioteca faltando
sudo apt-get install libxxx

# Ou configurar LD_LIBRARY_PATH
export LD_LIBRARY_PATH="/usr/local/lib:$LD_LIBRARY_PATH"
```

### Problema: Erro de segmentação

**Sintoma:**
```bash
$ advpp-ide
Segmentation fault (core dumped)
```

**Solução:**
```bash
# Verificar logs
cat ~/.advpp/logs/advpp-ide.log

# Reinstalar
sudo dpkg -r advpp
sudo dpkg -i advpp_*_amd64.deb

# Se persistir, reporte o bug
```

## Suporte

### Obter Ajuda

- **Documentação**: https://github.com/peder1981/AdvPP/wiki
- **Issues**: https://github.com/peder1981/AdvPP/issues
- **Discussions**: https://github.com/peder1981/AdvPP/discussions

### Reportar Bugs

Para reportar bugs, forneça:

- Versão do AdvPP
- Sistema operacional e versão
- Descrição detalhada do problema
- Passos para reproduzir
- Logs relevantes

### Logs

Os logs estão localizados em:

- **AdvPP IDE**: `~/.advpp/logs/advpp-ide.log`
- **AdvCfg**: `~/.advpp/logs/advcfg.log`
- **AdvEditor**: `~/.advpp/logs/adveditor.log`
- **AdvPlc**: `~/.advpp/logs/advplc.log`

## Próximos Passos

Após a instalação:

1. Leia o **Manual do Usuário** da ferramenta que deseja usar
2. Configure o **Dicionário de Dados** através do AdvCfg
3. Crie seu **Primeiro Projeto** no AdvPP IDE
4. Explore os **Exemplos** disponíveis

## Recursos Adicionais

- **Documentação Técnica**: docs/TECNICO.md
- **Manual do AdvPP IDE**: docs/MANUAL_IDE.md
- **Manual do AdvCfg**: docs/MANUAL_ADVCFG.md
- **Manual do AdvEditor**: docs/MANUAL_ADVEDITOR.md
- **Manual do AdvPlc**: docs/MANUAL_ADVPLC.md

## Conclusão

Com este guia, você deve ter o AdvPP instalado e configurado corretamente em seu sistema Linux. Se encontrar problemas, consulte a seção de solução de problemas ou entre em contato através dos canais de suporte.

Para mais informações, visite a documentação oficial em https://github.com/peder1981/AdvPP.
