package nginx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/atrox39/logtick/collector" // Importa el paquete collector para la interfaz
	"github.com/atrox39/logtick/config"
)

// NginxMetrics contiene las métricas específicas de Nginx
type NginxMetrics struct {
	ActiveConnections uint64 `json:"active_connections"`
	Accepts           uint64 `json:"total_accepts"`
	Handled           uint64 `json:"total_handled"`
	Requests          uint64 `json:"total_requests"`
	Reading           uint64 `json:"reading_connections"`
	Writing           uint64 `json:"writing_connections"`
	Waiting           uint64 `json:"waiting_connections"`
}

// NginxCollector implementa la interfaz Collector para métricas de Nginx
type NginxCollector struct {
	client        *http.Client
	stubStatusURL string
	interval      time.Duration
	log           *logrus.Entry // Logger para este colector
}

// NewNginxCollector crea una nueva instancia de NginxCollector
func NewNginxCollector(cfg *config.NginxConfig) (*NginxCollector, error) {
	if cfg.StubStatusURL == "" {
		return nil, fmt.Errorf("URL de stub_status de Nginx no puede estar vacía")
	}
	return &NginxCollector{
		client:        &http.Client{Timeout: 5 * time.Second},
		stubStatusURL: cfg.StubStatusURL,
		interval:      time.Duration(cfg.CollectionIntervalSeconds) * time.Second,
		log:           logrus.WithField("collector", "nginx"),
	}, nil
}

// Collect recolecta métricas de Nginx
func (c *NginxCollector) Collect() (collector.MetricData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.stubStatusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error al crear solicitud HTTP para Nginx: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al realizar solicitud HTTP a Nginx '%s': %w", c.stubStatusURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respuesta inesperada de Nginx: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de Nginx: %w", err)
	}

	// Parsear la salida del stub_status de Nginx
	// Ejemplo de salida:
	// Active connections: 291
	// server accepts handled requests
	//  1156826 1156826 4487778
	// Reading: 6 Writing: 179 Waiting: 106
	lines := strings.Split(string(bodyBytes), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("salida de stub_status de Nginx inesperada: %s", string(bodyBytes))
	}

	metrics := &NginxMetrics{}

	// Línea 1: Active connections
	if len(lines[0]) > 0 {
		activeConnectionsStr := strings.TrimSpace(strings.TrimPrefix(lines[0], "Active connections:"))
		metrics.ActiveConnections, _ = strconv.ParseUint(activeConnectionsStr, 10, 64)
	}

	// Línea 3: server accepts handled requests
	if len(lines[2]) > 0 {
		fields := strings.Fields(lines[2])
		if len(fields) >= 3 {
			metrics.Accepts, _ = strconv.ParseUint(fields[0], 10, 64)
			metrics.Handled, _ = strconv.ParseUint(fields[1], 10, 64)
			metrics.Requests, _ = strconv.ParseUint(fields[2], 10, 64)
		}
	}

	// Línea 4: Reading: X Writing: Y Waiting: Z
	if len(lines[3]) > 0 {
		parts := strings.Fields(lines[3])
		if len(parts) >= 6 {
			metrics.Reading, _ = strconv.ParseUint(parts[1], 10, 64)
			metrics.Writing, _ = strconv.ParseUint(parts[3], 10, 64)
			metrics.Waiting, _ = strconv.ParseUint(parts[5], 10, 64)
		}
	}

	c.log.WithFields(logrus.Fields{
		"active_connections": metrics.ActiveConnections,
		"total_requests":     metrics.Requests,
	}).Debug("Métricas de Nginx recolectadas")

	return metrics, nil
}

// Name devuelve el nombre de este colector
func (c *NginxCollector) Name() string {
	return "nginx"
}

// GetInterval devuelve el intervalo de recolección para este colector
func (c *NginxCollector) GetInterval() time.Duration {
	return c.interval
}
