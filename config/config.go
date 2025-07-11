package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// MySQLConfig define la configuración para el colector de MySQL
type MySQLConfig struct {
	Enabled                   bool   `yaml:"enabled"`
	DSN                       string `yaml:"dsn"`                         // Data Source Name, ej: user:password@tcp(127.0.0.1:3306)/dbname
	CollectionIntervalSeconds int    `yaml:"collection_interval_seconds"` // Intervalo específico para MySQL
}

// NginxConfig define la configuración para el colector de Nginx
type NginxConfig struct {
	Enabled                   bool   `yaml:"enabled"`
	StubStatusURL             string `yaml:"stub_status_url"`             // URL del endpoint ngx_http_stub_status_module
	CollectionIntervalSeconds int    `yaml:"collection_interval_seconds"` // Intervalo específico para Nginx
}

// Config define la estructura de nuestra configuración principal
type Config struct {
	AgentName       string       `yaml:"agent_name"`
	AgentID         string       `yaml:"agent_id"`
	IntervalSeconds int          `yaml:"interval_seconds"` // Intervalo global para métricas de sistema
	TargetURL       string       `yaml:"target_url"`
	LogLevel        string       `yaml:"log_level"`
	MySQL           *MySQLConfig `yaml:"mysql,omitempty"` // Usamos omitempty para no escribirlo si es nil
	Nginx           *NginxConfig `yaml:"nginx,omitempty"` // Usamos omitempty para no escribirlo si es nil
}

// LoadConfig carga la configuración desde un archivo YAML
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
			cfg.LogLevel = "info"
			cfg.AgentID = uuid.New().String()
			configModified = true

			// Inicializar configuraciones de plugins con valores por defecto (deshabilitados)
			// Esto evita panics si no hay sección en el YAML y el código intenta acceder a cfg.MySQL.Enabled
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

		// Si el AgentID está vacío después de parsear, generarlo
		if cfg.AgentID == "" {
			cfg.AgentID = uuid.New().String()
			fmt.Printf("AgentID vacío en la configuración, generando uno nuevo: %s\n", cfg.AgentID)
			configModified = true
		}
		// Si el LogLevel está vacío, establecer un valor por defecto
		if cfg.LogLevel == "" {
			cfg.LogLevel = "info"
			configModified = true
		}

		// Asegurarse de que las sub-configuraciones de los plugins no sean nil si están habilitadas
		// y rellenar con valores por defecto o lanzar error si faltan campos críticos.
		if cfg.MySQL == nil { // Si no hay sección 'mysql' en el YAML, inicializar como deshabilitado
			cfg.MySQL = &MySQLConfig{
				Enabled:                   false,
				DSN:                       "user:password@tcp(127.0.0.1:3306)/mysql?charset=utf8", // Valor por defecto
				CollectionIntervalSeconds: 10,
			}
		} else if cfg.MySQL.Enabled && cfg.MySQL.DSN == "" {
			return nil, fmt.Errorf("MySQL plugin enabled but DSN is empty")
		}
		// Si está habilitado pero no tiene intervalo específico, usar el global o un default
		if cfg.MySQL.Enabled && cfg.MySQL.CollectionIntervalSeconds <= 0 {
			cfg.MySQL.CollectionIntervalSeconds = 10 // Default si no se especifica
			configModified = true
		}

		if cfg.Nginx == nil { // Si no hay sección 'nginx' en el YAML, inicializar como deshabilitado
			cfg.Nginx = &NginxConfig{
				Enabled:                   false,
				StubStatusURL:             "http://localhost/nginx_status", // Valor por defecto
				CollectionIntervalSeconds: 10,
			}
		} else if cfg.Nginx.Enabled && cfg.Nginx.StubStatusURL == "" {
			return nil, fmt.Errorf("Nginx plugin enabled but StubStatusURL is empty")
		}
		// Si está habilitado pero no tiene intervalo específico, usar el global o un default
		if cfg.Nginx.Enabled && cfg.Nginx.CollectionIntervalSeconds <= 0 {
			cfg.Nginx.CollectionIntervalSeconds = 10 // Default si no se especifica
			configModified = true
		}
	}

	// Realizar validaciones básicas después de cargar/generar
	if cfg.AgentName == "" {
		return nil, fmt.Errorf("agent_name es requerido y no puede estar vacío")
	}
	if cfg.IntervalSeconds <= 0 {
		return nil, fmt.Errorf("interval_seconds debe ser un número positivo")
	}
	if cfg.TargetURL == "" {
		return nil, fmt.Errorf("target_url no puede estar vacío")
	}

	// Si la configuración fue modificada (nuevo archivo o nuevo AgentID/valores por defecto), la guardamos
	if configModified {
		if saveErr := SaveConfig(cfg, filePath); saveErr != nil {
			return nil, fmt.Errorf("error al guardar la configuración actualizada: %w", saveErr)
		}
		fmt.Printf("Archivo de configuración %s actualizado y guardado.\n", filePath)
	}

	return cfg, nil
}

// SaveConfig guarda la configuración actual en un archivo YAML
func SaveConfig(cfg *Config, filePath string) error {
	// Asegúrate de que el AgentID se genere si SaveConfig es llamado directamente y está vacío
	if cfg.AgentID == "" {
		cfg.AgentID = uuid.New().String()
		fmt.Printf("Generando AgentID durante SaveConfig: %s\n", cfg.AgentID)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error al serializar la configuración a YAML: %w", err)
	}

	// Escribir el archivo inicialmente
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("error al escribir el archivo de configuración: %w", err)
	}

	// Leer y modificar la segunda línea para añadir el comentario (frágil, como antes)
	// Esta parte sigue siendo un poco frágil si el orden de las claves cambia.
	// Una forma más robusta sería usar un marshal/unmarshal a un map[string]interface{}
	// para insertar el comentario antes de volver a marshalear.
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error al leer el archivo de configuración para añadir comentario: %w", err)
	}
	lines := strings.Split(string(fileContent), "\n")
	// Asumiendo que agent_id siempre será la segunda línea después de agent_name
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
