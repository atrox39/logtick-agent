package process

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"

	"github.com/atrox39/logtick/collector"
	"github.com/atrox39/logtick/config"
)

// ProcessInfo contiene métricas de un proceso individual
type ProcessInfo struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float32 `json:"memory_percent"`   // Porcentaje de memoria utilizada
	MemoryRSS     uint64  `json:"memory_rss_bytes"` // Resident Set Size
	NumThreads    int32   `json:"num_threads"`
	Status        string  `json:"status"`
}

// ProcessMetrics contiene las métricas específicas de los procesos monitoreados
type ProcessMetrics struct {
	MonitoredProcesses map[string][]ProcessInfo `json:"monitored_processes"` // Mapa por nombre de proceso
}

// ProcessCollector implementa la interfaz Collector para métricas de procesos
type ProcessCollector struct {
	processNames []string
	interval     time.Duration
	log          *logrus.Entry
}

// NewProcessCollector crea una nueva instancia de ProcessCollector
func NewProcessCollector(cfg *config.ProcessConfig) (*ProcessCollector, error) {
	if len(cfg.ProcessNames) == 0 {
		return nil, fmt.Errorf("se requiere al menos un nombre de proceso para monitorear")
	}

	return &ProcessCollector{
		processNames: cfg.ProcessNames,
		interval:     time.Duration(cfg.CollectionIntervalSeconds) * time.Second,
		log:          logrus.WithField("collector", "process"),
	}, nil
}

// Collect recolecta métricas de procesos
func (c *ProcessCollector) Collect() (collector.MetricData, error) {
	allProcs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("error al obtener la lista de procesos: %w", err)
	}

	monitored := make(map[string][]ProcessInfo)

	for _, p := range allProcs {
		pName, err := p.Name()
		if err != nil {
			// Podría ser un proceso zombie o sin permisos, lo ignoramos
			continue
		}

		// Normalizar el nombre del proceso para comparar (ej. "mysqld" vs "mysqld_safe")
		normalizedPName := strings.ToLower(pName)

		for _, targetName := range c.processNames {
			normalizedTargetName := strings.ToLower(targetName)

			if strings.Contains(normalizedPName, normalizedTargetName) { // Usamos Contains para mayor flexibilidad
				// Recolectar métricas del proceso
				cpuPercent, _ := p.CPUPercent() // Requiere llamar dos veces para obtener delta real, 0ms en primera llamada
				memPercent, _ := p.MemoryPercent()
				memInfo, _ := p.MemoryInfo()
				numThreads, _ := p.NumThreads()
				status, _ := p.Status()

				info := ProcessInfo{
					PID:           p.Pid,
					Name:          pName,
					CPUPercent:    cpuPercent,
					MemoryPercent: memPercent,
					MemoryRSS:     memInfo.RSS,
					NumThreads:    numThreads,
					Status:        strings.Join(status, ","), // Status puede ser un slice de strings
				}
				monitored[targetName] = append(monitored[targetName], info)
				break // Ya encontramos una coincidencia para este proceso, pasar al siguiente PID
			}
		}
	}

	metrics := &ProcessMetrics{
		MonitoredProcesses: monitored,
	}

	if len(metrics.MonitoredProcesses) == 0 {
		c.log.Debug("No se encontraron procesos monitoreados en esta ronda.")
	} else {
		c.log.WithField("processes_found", len(metrics.MonitoredProcesses)).Debug("Métricas de procesos recolectadas.")
	}

	return metrics, nil
}

// Name devuelve el nombre de este colector
func (c *ProcessCollector) Name() string {
	return "process"
}

// GetInterval devuelve el intervalo de recolección para este colector
func (c *ProcessCollector) GetInterval() time.Duration {
	return c.interval
}
