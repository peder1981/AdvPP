package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config representa a configuração compartilhada entre as ferramentas
// (~/.advpp/advpp_config.json — editável à mão; ver ResolveDatabasePath)
type Config struct {
	DefaultDatabase string `json:"default_database"`
	WebUIPort       string `json:"webui_port,omitempty"` // porta do advplc serve (padrão 8080)
}

// ResolveWebUIPort resolve a porta do modo web: explícita (--port) →
// config ~/.advpp → padrão 8080.
func ResolveWebUIPort(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if config, err := LoadConfig(); err == nil && config.WebUIPort != "" {
		return config.WebUIPort
	}
	return "8080"
}

const configFileName = "advpp_config.json"

// GetConfigPath retorna o caminho do arquivo de configuração
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("erro ao obter diretório home: %w", err)
	}

	configDir := filepath.Join(homeDir, ".advpp")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("erro ao criar diretório de configuração: %w", err)
	}

	return filepath.Join(configDir, configFileName), nil
}

// LoadConfig carrega a configuração do arquivo
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Se o arquivo não existir, retorna configuração padrão
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{
			DefaultDatabase: DefaultDatabasePath(),
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler arquivo de configuração: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("erro ao decodificar configuração: %w", err)
	}

	// Se o banco padrão não estiver definido, usa o padrão
	if config.DefaultDatabase == "" {
		config.DefaultDatabase = DefaultDatabasePath()
	}

	return &config, nil
}

// DefaultDatabasePath retorna o caminho absoluto padrão do banco (~/.advpp/ADVPP.db)
func DefaultDatabasePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "ADVPP.db"
	}
	return filepath.Join(homeDir, ".advpp", "ADVPP.db")
}

// LocalDatabaseName é o nome do banco SQLite local de um diretório de
// trabalho (o "./advpp.db" que ResolveDatabasePath cria/procura quando
// nada foi configurado globalmente) — cada diretório onde advplc
// check/run/compile/serve (ou qualquer outra ferramenta AdvPP) roda ganha
// seu próprio banco por padrão, sem exigir nenhuma configuração prévia.
const LocalDatabaseName = "advpp.db"

// dirGravavel diz se e possivel criar arquivo em dir. Um os.Stat do
// diretorio nao serve: no Windows a permissao efetiva depende da ACL, e
// Program Files aparece como diretorio normal ate a hora de escrever.
func dirGravavel(dir string) bool {
	f, err := os.CreateTemp(dir, ".advpp-w-*")
	if err != nil {
		return false
	}
	nome := f.Name()
	f.Close()
	os.Remove(nome)
	return true
}

// ResolveStandaloneDatabasePath escolhe o banco de um executavel gerado por
// `advplc build`.
//
// Regra diferente da do ResolveDatabasePath, e de proposito. As ferramentas
// (advplc, adveditor, advpp-ide) rodam dentro de um diretorio de projeto e
// devem compartilhar o "./advpp.db" dali -- e o que faz o editor enxergar o
// banco que o compilador acabou de usar. Um app distribuido nao tem
// diretorio de projeto: e lancado do menu Iniciar, da Area de Trabalho, de
// Documentos, de um pendrive. Se o banco seguisse o diretorio de trabalho, o
// mesmo app lancado de duas pastas usaria dois bancos, e o dado do usuario
// ficaria partido sem nenhum aviso -- alem de colidir com o banco das
// ferramentas quando por acaso caissem na mesma pasta.
//
// Por isso: ADVPP_DB, se definida, e o resto sempre na pasta de dados do
// usuario, numa subpasta com o nome do app. Um caminho so, independente de
// onde o atalho aponta. Quem quiser banco por diretorio define ADVPP_DB.
func ResolveStandaloneDatabasePath(appName string) string {
	if env := os.Getenv("ADVPP_DB"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	if caminho := bancoDoArquivoDeConfig(); caminho != "" {
		return caminho
	}
	return pastaDeDados(sanitizaNomeApp(appName))
}

// bancoConfigNome e o arquivo, ao lado do executavel, com o caminho do banco
// a usar -- uma linha, texto puro.
const bancoConfigNome = "advpp-db.txt"

// bancoDoArquivoDeConfig le esse arquivo, ou devolve "".
//
// Existe para o caso de um instalador precisar apontar o banco para um lugar
// escolhido por ele -- tipicamente uma pasta compartilhada entre as contas do
// Windows, quando o dado e da organizacao e nao de quem abriu o programa.
//
// A alternativa que isso substitui era um executavel lancador que definia
// ADVPP_DB e chamava o programa. Custava um processo intermediario, herança de
// handles e uma classe inteira de falhas novas, para transportar uma string.
// Ler um arquivo nao tem como falhar pela metade.
func bancoDoArquivoDeConfig() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dados, err := os.ReadFile(filepath.Join(filepath.Dir(exe), bancoConfigNome))
	if err != nil {
		return ""
	}
	caminho := strings.TrimSpace(string(dados))
	if caminho == "" {
		return ""
	}
	// A pasta pode nao existir ainda na primeira execucao; criar aqui evita
	// que o instalador precise adivinhar quando o usuario mudou o destino.
	_ = os.MkdirAll(filepath.Dir(caminho), 0o777)
	if abs, err := filepath.Abs(caminho); err == nil {
		return abs
	}
	return caminho
}

// sanitizaNomeApp reduz o titulo do app a algo seguro como nome de pasta.
func sanitizaNomeApp(nome string) string {
	limpo := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		}
		return -1
	}, nome)
	if limpo == "" {
		return "app"
	}
	return limpo
}

// ResolveDatabasePath resolve o caminho do banco compartilhado entre TODAS as
// ferramentas, nesta ordem de precedência:
//  1. caminho explícito (flag de linha de comando / seleção do usuário)
//  2. variável de ambiente ADVPP_DB
//  3. banco configurado em ~/.advpp/advpp_config.json — só se esse arquivo
//     de config REALMENTE existir em disco (não o valor sintético que
//     LoadConfig devolve quando não há config nenhuma, que apontaria
//     silenciosamente para o banco global mesmo sem o usuário ter
//     configurado nada)
//  4. ./advpp.db — banco local do diretório de trabalho atual. Nada
//     configurado e nada encontrado: cria (OpenSQLite materializa o
//     arquivo no primeiro open) e usa um banco local aqui em vez do
//     global ~/.advpp/ADVPP.db, para que `advplc run/check/compile/serve`
//     sempre tenham um banco ali mesmo, e as demais ferramentas
//     (adveditor/advpp-ide) rodadas no MESMO diretório enxerguem o mesmo
//     arquivo automaticamente (mesmo resolver). O banco global só volta a
//     valer depois que o usuário configura explicitamente
//     ~/.advpp/advpp_config.json (passo 3).
//
// O resultado é sempre um caminho absoluto. A criação física do arquivo
// (se ainda não existir) é feita por OpenSQLite no primeiro open, não
// aqui — esta função só decide QUAL caminho usar.
func ResolveDatabasePath(explicit string) string {
	candidate := explicit
	if candidate == "" {
		candidate = os.Getenv("ADVPP_DB")
	}
	if candidate == "" {
		if configPath, err := GetConfigPath(); err == nil {
			if _, statErr := os.Stat(configPath); statErr == nil {
				if config, err := LoadConfig(); err == nil && config.DefaultDatabase != "" {
					// Config legada pode conter caminho relativo que só existe no repo;
					// só a usa se o arquivo realmente existir ou se for absoluta.
					if filepath.IsAbs(config.DefaultDatabase) {
						candidate = config.DefaultDatabase
					} else if _, err := os.Stat(config.DefaultDatabase); err == nil {
						candidate = config.DefaultDatabase
					}
				}
			}
		}
	}
	if candidate == "" {
		// Passo 4 com uma ressalva: "./advpp.db" pressupoe diretorio de
		// trabalho gravavel. Vale para a CLI num diretorio de projeto e nao
		// vale para adveditor/advpp-ide abertos por um atalho que aponta
		// para dentro de Program Files -- ali o SQLite nao cria nada, quem
		// chamou recebe um engine nulo e o erro so aparece la na frente,
		// como falha de query ou como janela que fecha sem mensagem.
		return bancoLocalOuPastaDeDados("")
	}
	if abs, err := filepath.Abs(candidate); err == nil {
		return abs
	}
	return candidate
}

// bancoLocalOuPastaDeDados devolve ./advpp.db quando isso e viavel, e um
// caminho na pasta de dados do usuario quando nao e. subpasta separa o banco
// por aplicacao (vazio = banco comum das ferramentas AdvPP).
func bancoLocalOuPastaDeDados(subpasta string) string {
	// Banco que ja existe no diretorio manda, gravavel ou nao: mudar o
	// caminho por baixo de quem ja usa trocaria o banco em uso por um vazio.
	if _, err := os.Stat(LocalDatabaseName); err == nil {
		if abs, err := filepath.Abs(LocalDatabaseName); err == nil {
			return abs
		}
	}
	if dirGravavel(".") {
		if abs, err := filepath.Abs(LocalDatabaseName); err == nil {
			return abs
		}
	}

	return pastaDeDados(subpasta)
}

// pastaDeDados devolve o caminho do banco na pasta de dados do usuario.
// subpasta separa por aplicacao; vazio = banco comum das ferramentas AdvPP.
func pastaDeDados(subpasta string) string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	partes := []string{base, "advpp"}
	if subpasta != "" {
		partes = append(partes, subpasta)
	}
	dir := filepath.Join(partes...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		// Ultimo recurso: TempDir e gravavel em qualquer plataforma. Banco
		// volatil e ruim, mas some junto com o problema -- melhor que a
		// ferramenta nao abrir.
		return filepath.Join(os.TempDir(), LocalDatabaseName)
	}
	return filepath.Join(dir, LocalDatabaseName)
}

// SaveConfig salva a configuração no arquivo
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao codificar configuração: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("erro ao escrever arquivo de configuração: %w", err)
	}

	return nil
}

// SetDefaultDatabase define o banco de dados padrão
func SetDefaultDatabase(dbPath string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	config.DefaultDatabase = dbPath
	return SaveConfig(config)
}

// GetDefaultDatabase retorna o banco de dados padrão
func GetDefaultDatabase() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	return config.DefaultDatabase, nil
}
