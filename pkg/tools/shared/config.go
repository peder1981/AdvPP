package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		candidate = LocalDatabaseName
	}
	if abs, err := filepath.Abs(candidate); err == nil {
		return abs
	}
	return candidate
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
