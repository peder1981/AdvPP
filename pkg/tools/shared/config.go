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
			DefaultDatabase: "./data/advpl_dictionary.db",
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
		config.DefaultDatabase = "./data/advpl_dictionary.db"
	}
	
	return &config, nil
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
