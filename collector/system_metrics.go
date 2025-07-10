package collector

import (
	"fmt"

	"github.com/atrox39/logtick/config"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemMetrics struct {
	AgentName   string  `json:"agent_name"`
	AgentID     string  `json:"agent_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryUsage uint64  `json:"memory_usage"`
	MemoryFree  uint64  `json:"memory_free"`
}

func CollectSystemMetrics(cfg *config.Config) (*SystemMetrics, error) {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get cpu percent: %w", err)
	}
	vMem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual memory: %w", err)
	}
	metrics := &SystemMetrics{
		AgentName:   cfg.AgentName,
		AgentID:     cfg.AgentID,
		CPUPercent:  cpuPercent[0],
		MemoryUsage: vMem.Used,
		MemoryFree:  vMem.Available,
	}
	return metrics, nil
}
