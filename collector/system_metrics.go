package collector

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/atrox39/logtick/config" // Importar la configuración de tu proyecto
)

// SystemMetrics contiene las métricas recolectadas
type SystemMetrics struct {
	AgentID    string  `json:"agent_id"`
	AgentName  string  `json:"agent_name"`
	Timestamp  int64   `json:"timestamp"`
	CPUPercent float64 `json:"cpu_percent"`
	MemoryUsed uint64  `json:"memory_used_mb"` // En MB
	MemoryFree uint64  `json:"memory_free_mb"` // En MB
}

// CollectSystemMetrics recolecta métricas de CPU y memoria, usando la configuración
func CollectSystemMetrics(cfg *config.Config) (*SystemMetrics, error) {
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
		AgentID:    cfg.AgentID,
		AgentName:  cfg.AgentName,
		Timestamp:  time.Now().Unix(),
		CPUPercent: cpuPercent,
		MemoryUsed: vMem.Used / 1024 / 1024,
		MemoryFree: vMem.Free / 1024 / 1024,
	}

	return metrics, nil
}
