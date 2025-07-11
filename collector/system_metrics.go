package collector

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/atrox39/logtick/config" // Importar la configuración de tu proyecto
)

// MetricData es una interfaz que define el contrato para los datos recolectados.
type MetricData interface{}

type Collector interface {
	Name() string
	GetInterval() time.Duration
	Collect() (MetricData, error)
}

// SystemMetrics contiene las métricas recolectadas del sistema.
// Ya no incluirá AgentID, AgentName ni Timestamp, ya que se manejarán
// a nivel de "AgentReport" antes del envío al backend.
type SystemMetrics struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryUsed uint64  `json:"memory_used_mb"` // En MB
	MemoryFree uint64  `json:"memory_free_mb"` // En MB
}

// SystemCollector implementa la interfaz Collector para métricas del sistema.
type SystemCollector struct {
	interval time.Duration
}

// NewSystemCollector crea una nueva instancia de SystemCollector.
// Recibe la configuración global para obtener el intervalo.
func NewSystemCollector(cfg *config.Config) *SystemCollector {
	return &SystemCollector{
		interval: time.Duration(cfg.IntervalSeconds) * time.Second,
	}
}

// Collect recolecta métricas de CPU y memoria.
// Implementa el método Collect() de la interfaz Collector.
func (c *SystemCollector) Collect() (MetricData, error) {
	// Obtener uso de CPU
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("error al obtener uso de CPU: %w", err)
	}
	cpuPercent := cpuPercents[0]

	// Obtener uso de memoria
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("error al obtener uso de memoria: %w", err)
	}

	metrics := &SystemMetrics{
		CPUPercent: cpuPercent,
		MemoryUsed: vMem.Used / 1024 / 1024,
		MemoryFree: vMem.Free / 1024 / 1024,
	}

	return metrics, nil
}

// Name devuelve el nombre de este colector.
// Implementa el método Name() de la interfaz Collector.
func (c *SystemCollector) Name() string {
	return "system"
}

// GetInterval devuelve el intervalo de recolección para este colector.
// Implementa el método GetInterval() de la interfaz Collector.
func (c *SystemCollector) GetInterval() time.Duration {
	return c.interval
}
