package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config representa a configuração compartilhada entre as ferramentas
type Config struct {
	DefaultDatabase string `json:"default_database"`
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

// ResolveDatabasePath resolve o caminho do banco compartilhado entre TODAS as
// ferramentas, nesta ordem de precedência:
//  1. caminho explícito (flag de linha de comando / seleção do usuário)
//  2. variável de ambiente ADVPP_DB
//  3. banco padrão configurado em ~/.advpp/advpp_config.json
//  4. banco legado ./data/advpl_dictionary.db (se existir no diretório atual)
//  5. padrão absoluto ~/.advpp/advpl_dictionary.db
//
// O resultado é sempre um caminho absoluto.
func ResolveDatabasePath(explicit string) string {
	candidate := explicit
	if candidate == "" {
		candidate = os.Getenv("ADVPP_DB")
	}
	if candidate == "" {
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
	if candidate == "" {
		for _, legacy := range []string{
			filepath.Join("data", "ADVPP.db"),
			filepath.Join("data", "advpl_dictionary.db"), // nome antigo
		} {
			if _, err := os.Stat(legacy); err == nil {
				candidate = legacy
				break
			}
		}
	}
	if candidate == "" {
		candidate = DefaultDatabasePath()
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
