package main

import (
	"context"
	"encoding/json"
	"flag" // <-- Nueva importación
	"fmt"
	"net/http" // Para el servidor de métricas
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atrox39/logtick/collector" // Usando tus rutas de importación
	"github.com/atrox39/logtick/config"
	"github.com/atrox39/logtick/sender"

	"github.com/prometheus/client_golang/prometheus"          // <-- Nueva importación
	"github.com/prometheus/client_golang/prometheus/promhttp" // <-- Nueva importación
	"github.com/sirupsen/logrus"                              // <-- Nueva importación, reemplaza a "log"
)

const configFilePath = "config.yaml"
const metricsPort = ":9090" // Puerto para el endpoint de métricas de Prometheus

// Definir métricas de Prometheus para el propio agente
var (
	metricsCollected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_metrics_collected_total",
			Help: "Total number of metrics collected by the agent.",
		},
		[]string{"type", "agent_name", "agent_id"}, // Añadimos agent_name y agent_id
	)
	metricsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_metrics_sent_total",
			Help: "Total number of metrics successfully sent by the agent.",
		},
		[]string{"status", "agent_name", "agent_id"}, // Añadimos agent_name y agent_id
	)
	collectionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "agent_collection_duration_seconds",
		Help:    "Duration of metric collection in seconds.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	// Registrar las métricas de Prometheus
	prometheus.MustRegister(metricsCollected)
	prometheus.MustRegister(metricsSent)
	prometheus.MustRegister(collectionDuration)
}

var latestMetrics interface{}

func main() {
	// Definir una bandera de línea de comandos --init
	initAgent := flag.Bool("init", false, "Genera un archivo config.yaml inicial si no existe y sale.")
	flag.Parse() // Parsear los argumentos de la línea de comandos

	// Si la bandera --init está presente, generamos/verificamos el config y salimos
	if *initAgent {
		fmt.Printf("Intentando generar un archivo de configuración en: %s\n", configFilePath)
		// LoadConfig ya maneja la creación de defaults y el AgentID si no existe
		_, err := config.LoadConfig(configFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error al inicializar la configuración: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuración inicial generada/verificada. Puedes modificarla en 'config.yaml'.")
		os.Exit(0) // Salir después de la inicialización
	}

	// 1. Cargar configuración y configurar Logrus
	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		// Ya no necesitamos el os.Stat previo, LoadConfig lo maneja
		logrus.Fatalf("Error al cargar la configuración: %v", err)
	}

	// Configurar el nivel de log de Logrus
	logLevel, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		logrus.Errorf("Nivel de log inválido '%s', usando info por defecto.", cfg.LogLevel)
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{}) // Formato JSON para logs estructurados
	logrus.SetOutput(os.Stdout)                  // Opcional: enviar a stderr o a un archivo

	logrus.WithFields(logrus.Fields{
		"agent_name":       cfg.AgentName,
		"agent_id":         cfg.AgentID,
		"interval_seconds": cfg.IntervalSeconds,
		"target_url":       cfg.TargetURL,
		"log_level":        cfg.LogLevel,
	}).Info("Configuración cargada y logger inicializado.")

	// 2. Inicializar el enviador HTTP
	httpSender := sender.NewHTTPSender(cfg.TargetURL)

	// 3. Configurar contexto para el apagado elegante
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Goroutine para manejar señales del sistema (Ctrl+C, etc.)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logrus.WithField("signal", sig).Info("Señal de terminación recibida. Iniciando apagado...")
		cancel()
	}()

	// 4. Iniciar servidor de métricas de Prometheus
	go func() {
		fs := http.FileServer(http.Dir("./web"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))
		http.Handle("/", fs)
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/api/current_metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if latestMetrics == nil {
				json.NewEncoder(w).Encode(map[string]string{"error": "No metrics available yet."})
				return
			}
			json.NewEncoder(w).Encode(latestMetrics)
		})
		logrus.WithField("port", metricsPort).Info("Servidor de métricas de Prometheus escuchando.")
		err := http.ListenAndServe(metricsPort, nil)
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Error al iniciar el servidor de métricas de Prometheus.")
		}
	}()

	// 5. Bucle principal de recolección y envío
	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop() // Asegura que el ticker se detenga al salir

	logrus.Info("Agente iniciado. Recolectando y enviando métricas...")

	for {
		select {
		case <-ticker.C:
			// Medir la duración de la recolección
			start := time.Now()
			// Pasamos el objeto de configuración completo al colector
			metrics, err := collector.CollectSystemMetrics(cfg)
			collectionDuration.Observe(time.Since(start).Seconds())
			metricsCollected.WithLabelValues("system", cfg.AgentName, cfg.AgentID).Inc()

			if err != nil {
				logrus.WithError(err).Error("Error al recolectar métricas del sistema.")
				continue
			}

			// Asegúrate de que las claves de los logs sean correctas (MemoryUsed en lugar de MemoryUsage)
			logrus.WithFields(logrus.Fields{
				"cpu_percent":    fmt.Sprintf("%.2f%%", metrics.CPUPercent),
				"memory_used_mb": metrics.MemoryUsed,
				"memory_free_mb": metrics.MemoryFree,
			}).Debug("Métricas del sistema recolectadas.")

			// Enviar métricas
			err = httpSender.Send(metrics)
			if err != nil {
				metricsSent.WithLabelValues("failure", cfg.AgentName, cfg.AgentID).Inc()
				logrus.WithError(err).Error("Error al enviar métricas al backend.")
			} else {
				metricsSent.WithLabelValues("success", cfg.AgentName, cfg.AgentID).Inc()
				logrus.Info("Métricas enviadas exitosamente al backend.")
			}
			latestMetrics = metrics
		case <-ctx.Done():
			logrus.Info("Contexto cancelado. Deteniendo el agente.")
			return
		}
	}
}
