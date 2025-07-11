package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type MySQLConfig struct {
	Enabled                   bool   `yaml:"enabled"`
	DSN                       string `yaml:"dsn"`
	CollectionIntervalSeconds int    `yaml:"collection_interval_seconds"`
}

type NginxConfig struct {
	Enabled                   bool   `yaml:"enabled"`
	StubStatusURL             string `yaml:"stub_status_url"`
	CollectionIntervalSeconds int    `yaml:"collection_interval_seconds"`
}

type ProcessConfig struct {
	Enabled                   bool     `yaml:"enabled"`
	ProcessNames              []string `yaml:"process_names"`
	CollectionIntervalSeconds int      `yaml:"collection_interval_seconds"`
}

type Config struct {
	AgentName       string         `yaml:"agent_name"`
	AgentID         string         `yaml:"agent_id"`
	IntervalSeconds int            `yaml:"interval_seconds"`
	TargetURL       string         `yaml:"target_url"`
	WebSocketLogURL string         `yaml:"websocket_log_url"`
	LogLevel        string         `yaml:"log_level"`
	MySQL           *MySQLConfig   `yaml:"mysql,omitempty"`
	Nginx           *NginxConfig   `yaml:"nginx,omitempty"`
	Process         *ProcessConfig `yaml:"process,omitempty"`
}

func LoadConfig(filePath string) (*Config, error) {
	cfg := &Config{}
	var configModified bool

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Archivo de configuración %s no encontrado, creando uno nuevo con valores por defecto.\n", filePath)
			cfg.AgentName = "default-agent"
			cfg.IntervalSeconds = 5 // Intervalo por defecto para sistema
			cfg.TargetURL = "http://localhost:4003/metrics"
			cfg.WebSocketLogURL = "ws://localhost:4003/ws/logs"
			cfg.LogLevel = "info"
			cfg.AgentID = uuid.New().String()
			configModified = true

			cfg.MySQL = &MySQLConfig{
				Enabled:                   false,
				DSN:                       "user:password@tcp(127.0.0.1:3306)/mysql?charset=utf8",
				CollectionIntervalSeconds: 10,
			}
			cfg.Nginx = &NginxConfig{
				Enabled:                   false,
				StubStatusURL:             "http://localhost/nginx_status",
				CollectionIntervalSeconds: 10,
			}

		} else {
			return nil, fmt.Errorf("error al leer el archivo de configuración %s: %w", filePath, err)
		}
	} else {
		err = yaml.Unmarshal(data, cfg)
		if err != nil {
			return nil, fmt.Errorf("error al parsear el archivo de configuración %s: %w", filePath, err)
		}

		if cfg.AgentID == "" {
			cfg.AgentID = uuid.New().String()
			fmt.Printf("AgentID vacío en la configuración, generando uno nuevo: %s\n", cfg.AgentID)
			configModified = true
		}
		if cfg.LogLevel == "" {
			cfg.LogLevel = "info"
			configModified = true
		}

		if cfg.MySQL == nil {
			cfg.MySQL = &MySQLConfig{
				Enabled:                   false,
				DSN:                       "user:password@tcp(127.0.0.1:3306)/mysql?charset=utf8",
				CollectionIntervalSeconds: 10,
			}
		} else if cfg.MySQL.Enabled && cfg.MySQL.DSN == "" {
			return nil, fmt.Errorf("MySQL plugin enabled but DSN is empty")
		}
		if cfg.MySQL.Enabled && cfg.MySQL.CollectionIntervalSeconds <= 0 {
			cfg.MySQL.CollectionIntervalSeconds = 10
			configModified = true
		}

		if cfg.Nginx == nil {
			cfg.Nginx = &NginxConfig{
				Enabled:                   false,
				StubStatusURL:             "http://localhost/nginx_status",
				CollectionIntervalSeconds: 10,
			}
		} else if cfg.Nginx.Enabled && cfg.Nginx.StubStatusURL == "" {
			return nil, fmt.Errorf("nginx plugin enabled but StubStatusURL is empty")
		}
		if cfg.Nginx.Enabled && cfg.Nginx.CollectionIntervalSeconds <= 0 {
			cfg.Nginx.CollectionIntervalSeconds = 10
			configModified = true
		}

		if cfg.Process == nil {
			cfg.Process = &ProcessConfig{
				Enabled:                   false,
				ProcessNames:              []string{},
				CollectionIntervalSeconds: 15,
			}
		} else if cfg.Process.Enabled && len(cfg.Process.ProcessNames) == 0 {
			return nil, fmt.Errorf("process plugin enabled but ProcessNames is empty")
		}
		if cfg.Process.Enabled && cfg.Process.CollectionIntervalSeconds <= 0 {
			cfg.Process.CollectionIntervalSeconds = 15
			configModified = true
		}
	}

	if cfg.AgentName == "" {
		return nil, fmt.Errorf("agent_name es requerido y no puede estar vacío")
	}
	if cfg.IntervalSeconds <= 0 {
		return nil, fmt.Errorf("interval_seconds debe ser un número positivo")
	}
	if cfg.TargetURL == "" {
		return nil, fmt.Errorf("target_url no puede estar vacío")
	}

	if configModified {
		if saveErr := SaveConfig(cfg, filePath); saveErr != nil {
			return nil, fmt.Errorf("error al guardar la configuración actualizada: %w", saveErr)
		}
		fmt.Printf("Archivo de configuración %s actualizado y guardado.\n", filePath)
	}

	return cfg, nil
}

func SaveConfig(cfg *Config, filePath string) error {
	if cfg.AgentID == "" {
		cfg.AgentID = uuid.New().String()
		fmt.Printf("Generando AgentID durante SaveConfig: %s\n", cfg.AgentID)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error al serializar la configuración a YAML: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error al escribir el archivo de configuración: %w", err)
	}

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error al leer el archivo de configuración para añadir comentario: %w", err)
	}
	lines := strings.Split(string(fileContent), "\n")
	if len(lines) > 1 && strings.HasPrefix(lines[1], "agent_id:") {
		lines[1] = lines[1] + " # Agent ID generado por el agente, no modificar ni eliminar esta línea"
		data = []byte(strings.Join(lines, "\n"))
		err = os.WriteFile(filePath, data, 0644)
		if err != nil {
			return fmt.Errorf("error al reescribir el archivo de configuración con comentario: %w", err)
		}
	}
	return nil
}
