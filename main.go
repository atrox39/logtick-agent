package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atrox39/logtick/collector"
	"github.com/atrox39/logtick/config"
	"github.com/atrox39/logtick/sender"
)

const configFilePath = "config.yaml"

func main() {
	// Check if flag --init is set
	if os.Args[1] == "--init" {
		log.Println("Initializing config file...")
		cfg := &config.Config{
			AgentName:       "agent-1",
			IntervalSeconds: 5,
			TargetUrl:       "http://localhost:4003/metrics",
		}
		err := config.SaveConfig(cfg, configFilePath)
		if err != nil {
			log.Fatalf("Failed to save configuration: %v", err)
		}
		log.Println("Config file initialized successfully")
		return
	}
	// Preload config
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", configFilePath)
	}
	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded: interval %d seconds, target URL %s", cfg.IntervalSeconds, cfg.TargetUrl)
	httpSender := sender.NewHTTPSender(cfg.TargetUrl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Shutdown signal received: %s", sig)
		cancel()
	}()
	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	for {
		select {
		case <-ticker.C:
			metrics, err := collector.CollectSystemMetrics(cfg)
			if err != nil {
				log.Printf("Failed to collect metrics: %v", err)
				continue
			}
			log.Printf("Collected metrics: CPU %f, Memory Usage %d, Memory Free %d", metrics.CPUPercent, metrics.MemoryUsage, metrics.MemoryFree)
			// Send metrics
			err = httpSender.Send(metrics)
			if err != nil {
				log.Printf("Failed to send metrics: %v", err)
			} else {
				log.Println("Metrics sent successfully")
			}
		case <-ctx.Done():
			log.Println("Shutting down...")
			return
		}
	}
}
