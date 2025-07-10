package sender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPSender es una interfaz para enviar datos via HTTP
type HTTPSender struct {
	client *http.Client
	url    string
}

// NewHTTPSender crea una nueva instancia de HTTPSender
func NewHTTPSender(url string) *HTTPSender {
	return &HTTPSender{
		client: &http.Client{Timeout: 10 * time.Second}, // Timeout para evitar bloqueos
		url:    url,
	}
}

// Send envía los datos en formato JSON a la URL configurada
func (s *HTTPSender) Send(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error al serializar los datos a JSON: %w", err)
	}

	req, err := http.NewRequest("POST", s.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error al crear la solicitud HTTP: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error al enviar la solicitud HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil // Éxito
	} else {
		return fmt.Errorf("el servidor respondió con el estado %d: %s", resp.StatusCode, resp.Status)
	}
}
