package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type Config struct {
	AgentName       string `yaml:"agent_name"`
	AgentID         string `yaml:"agent_id"`
	IntervalSeconds int    `yaml:"interval_seconds"`
	TargetURL       string `yaml:"target_url"` // Cambiado a TargetURL para consistencia Go
	LogLevel        string `yaml:"log_level"`  // <-- Reintroducido
}

// LoadConfig carga la configuración desde un archivo YAML
func LoadConfig(filePath string) (*Config, error) {
	cfg := &Config{}
	var configModified bool // Bandera para saber si necesitamos guardar el archivo

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Si el archivo no existe, creamos una configuración por defecto
			fmt.Printf("Archivo de configuración %s no encontrado, creando uno nuevo con valores por defecto.\n", filePath)
			cfg.AgentName = "default-agent"
			cfg.IntervalSeconds = 5
			cfg.TargetURL = "http://localhost:4003/metrics" // Usamos el puerto de tu ejemplo de main.go
			cfg.LogLevel = "info"
			cfg.AgentID = uuid.New().String() // Generar nuevo UUID
			configModified = true             // Necesitamos guardar esta nueva config
		} else {
			return nil, fmt.Errorf("error al leer el archivo de configuración %s: %w", filePath, err)
		}
	} else {
		// Si el archivo existe, lo parseamos
		err = yaml.Unmarshal(data, cfg)
		if err != nil {
			return nil, fmt.Errorf("error al parsear el archivo de configuración %s: %w", filePath, err)
		}

		// Validar y generar AgentID si está vacío después de parsear
		if cfg.AgentID == "" {
			cfg.AgentID = uuid.New().String()
			fmt.Printf("AgentID vacío en la configuración, generando uno nuevo: %s\n", cfg.AgentID)
			configModified = true // Necesitamos guardar el nuevo ID
		}
		// Validar LogLevel si está vacío (opcional, Logrus maneja bien un string vacío)
		if cfg.LogLevel == "" {
			cfg.LogLevel = "info"
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

	// Si la configuración fue modificada (nuevo archivo o nuevo AgentID), la guardamos
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
