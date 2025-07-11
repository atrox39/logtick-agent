package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync" // <-- Nueva importación para WaitGroup
	"syscall"
	"time"

	"github.com/atrox39/logtick/collector"       // Interfaz Collector y MetricData
	"github.com/atrox39/logtick/collector/mysql" // Colector de MySQL
	"github.com/atrox39/logtick/collector/nginx" // Colector de Nginx
	"github.com/atrox39/logtick/config"
	"github.com/atrox39/logtick/sender"
	"github.com/atrox39/logtick/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const configFilePath = "config.yaml"
const metricsPort = ":9090" // Puerto para el endpoint de métricas de Prometheus y la UI

// Definir métricas de Prometheus para el propio agente
var (
	metricsCollected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_metrics_collected_total",
			Help: "Total number of metric collections performed by the agent.",
		},
		[]string{"type", "agent_name", "agent_id"},
	)
	metricsSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_metrics_sent_total",
			Help: "Total number of metric reports successfully sent by the agent.",
		},
		[]string{"status", "agent_name", "agent_id"},
	)
	collectionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "agent_collection_duration_seconds",
			Help:    "Duration of metric collection in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"}, // Etiqueta para el tipo de colector (system, mysql, nginx)
	)
	// Nueva métrica para el estado del colector (up/down)
	collectorStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_collector_status",
			Help: "Status of each metric collector (1 = up, 0 = down).",
		},
		[]string{"type", "agent_name", "agent_id"},
	)
)

func init() {
	// Registrar las métricas de Prometheus
	prometheus.MustRegister(metricsCollected)
	prometheus.MustRegister(metricsSent)
	prometheus.MustRegister(collectionDuration)
	prometheus.MustRegister(collectorStatus)
}

// AgentReport encapsula todas las métricas recolectadas para un envío consolidado
type AgentReport struct {
	AgentID   string                   `json:"agent_id"`
	AgentName string                   `json:"agent_name"`
	Timestamp int64                    `json:"timestamp"`
	System    *collector.SystemMetrics `json:"system_metrics,omitempty"`
	MySQL     *mysql.MySQLMetrics      `json:"mysql_metrics,omitempty"`
	Nginx     *nginx.NginxMetrics      `json:"nginx_metrics,omitempty"`
	// Añadir más tipos de métricas aquí según se implementen los colectores
}

// Variable global para almacenar las últimas métricas para la UI interna
var latestAgentReport *AgentReport
var mu sync.RWMutex // Mutex para proteger latestAgentReport

func main() {
	initAgent := flag.Bool("init", false, "Genera un archivo config.yaml inicial si no existe y sale.")
	server := flag.Bool("server", false, "Inicia el servidor de pruebas para recibir métricas.")
	flag.Parse()

	if *initAgent {
		fmt.Printf("Intentando generar un archivo de configuración en: %s\n", configFilePath)
		_, err := config.LoadConfig(configFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error al inicializar la configuración: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuración inicial generada/verificada. Puedes modificarla en 'config.yaml'.")
		os.Exit(0)
	}

	if *server {
		utils.Server()
		os.Exit(0)
		return
	}

	// 1. Cargar configuración y configurar Logrus
	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		logrus.Fatalf("Error al cargar la configuración: %v", err)
	}

	logLevel, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		logrus.Errorf("Nivel de log inválido '%s', usando info por defecto.", cfg.LogLevel)
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)

	logrus.WithFields(logrus.Fields{
		"agent_name":        cfg.AgentName,
		"agent_id":          cfg.AgentID,
		"global_interval_s": cfg.IntervalSeconds,
		"target_url":        cfg.TargetURL,
		"log_level":         cfg.LogLevel,
	}).Info("Configuración cargada y logger inicializado.")

	// 2. Inicializar el enviador HTTP
	httpSender := sender.NewHTTPSender(cfg.TargetURL)

	// 3. Configurar contexto para el apagado elegante
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logrus.WithField("signal", sig).Info("Señal de terminación recibida. Iniciando apagado...")
		cancel()
	}()

	// 4. Iniciar servidor de métricas de Prometheus y UI
	go func() {
		fs := http.FileServer(http.Dir("./web"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))
		http.Handle("/", fs) // Sirve index.html por defecto
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/api/current_metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			mu.RLock() // Bloquear para lectura
			report := latestAgentReport
			mu.RUnlock()

			if report == nil {
				json.NewEncoder(w).Encode(map[string]string{"error": "No metrics available yet."})
				return
			}
			json.NewEncoder(w).Encode(report)
		})
		logrus.WithField("port", metricsPort).Info("Servidor de métricas y UI escuchando.")
		err := http.ListenAndServe(metricsPort, nil)
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Error al iniciar el servidor de métricas y UI.")
		}
	}()

	// 5. Inicializar colectores activos
	var activeCollectors []collector.Collector

	// Colector de métricas del sistema (siempre activo)
	activeCollectors = append(activeCollectors, collector.NewSystemCollector(cfg))
	logrus.Info("Colector de sistema inicializado.")
	collectorStatus.WithLabelValues("system", cfg.AgentName, cfg.AgentID).Set(0) // Inicialmente 'down' hasta la primera recolección exitosa

	// Colector de MySQL
	if cfg.MySQL != nil && cfg.MySQL.Enabled {
		mysqlCollector, err := mysql.NewMySQLCollector(cfg.MySQL)
		if err != nil {
			logrus.WithError(err).Error("No se pudo inicializar el colector de MySQL. Será omitido.")
			collectorStatus.WithLabelValues("mysql", cfg.AgentName, cfg.AgentID).Set(0)
		} else {
			activeCollectors = append(activeCollectors, mysqlCollector)
			logrus.Info("Colector de MySQL inicializado.")
			collectorStatus.WithLabelValues("mysql", cfg.AgentName, cfg.AgentID).Set(0) // Inicialmente 'down'
		}
	}

	// Colector de Nginx
	if cfg.Nginx != nil && cfg.Nginx.Enabled {
		nginxCollector, err := nginx.NewNginxCollector(cfg.Nginx)
		if err != nil {
			logrus.WithError(err).Error("No se pudo inicializar el colector de Nginx. Será omitido.")
			collectorStatus.WithLabelValues("nginx", cfg.AgentName, cfg.AgentID).Set(0)
		} else {
			activeCollectors = append(activeCollectors, nginxCollector)
			logrus.Info("Colector de Nginx inicializado.")
			collectorStatus.WithLabelValues("nginx", cfg.AgentName, cfg.AgentID).Set(0) // Inicialmente 'down'
		}
	}

	if len(activeCollectors) == 0 {
		logrus.Warn("No hay colectores de métricas activos. El agente solo servirá la UI y Prometheus.")
	}

	// 6. Bucle principal de recolección y envío para cada colector
	logrus.Info("Agente iniciado. Recolectando y enviando métricas...")

	var wg sync.WaitGroup // Usamos un WaitGroup para esperar que todas las goroutines de colectores terminen al apagado

	// Crear un mapa para los últimos datos recolectados de cada tipo para la UI
	currentCollectedData := make(map[string]interface{})
	var uiDataMutex sync.RWMutex // Mutex para proteger currentCollectedData

	for _, col := range activeCollectors {
		wg.Add(1) // Añadir uno al WaitGroup por cada goroutine de colector
		go func(c collector.Collector) {
			defer wg.Done() // Asegurar que Done() se llama cuando la goroutine termina

			ticker := time.NewTicker(c.GetInterval())
			defer ticker.Stop()

			logrus.Infof("Iniciando goroutine para el colector '%s' con intervalo de %s", c.Name(), c.GetInterval())

			for {
				select {
				case <-ticker.C:
					// Medir la duración de la recolección
					start := time.Now()
					collectedMetrics, err := c.Collect() // Recolectar métricas

					collectionDuration.WithLabelValues(c.Name()).Observe(time.Since(start).Seconds())
					metricsCollected.WithLabelValues(c.Name(), cfg.AgentName, cfg.AgentID).Inc()

					if err != nil {
						logrus.WithError(err).Errorf("Error al recolectar métricas del colector '%s'.", c.Name())
						collectorStatus.WithLabelValues(c.Name(), cfg.AgentName, cfg.AgentID).Set(0) // Marcar colector como down
						continue
					}
					collectorStatus.WithLabelValues(c.Name(), cfg.AgentName, cfg.AgentID).Set(1) // Marcar colector como up

					logrus.WithField("collector_name", c.Name()).Debug("Métricas recolectadas.")

					// Actualizar el mapa para la UI
					uiDataMutex.Lock()
					currentCollectedData[c.Name()] = collectedMetrics
					uiDataMutex.Unlock()

					// Construir el AgentReport consolidado antes de enviar
					// Siempre enviamos el reporte completo con los últimos datos disponibles
					fullReport := &AgentReport{
						AgentID:   cfg.AgentID,
						AgentName: cfg.AgentName,
						Timestamp: time.Now().Unix(),
					}
					// Asignar los datos recolectados al reporte según su tipo
					// Esto asegura que el reporte enviado contenga los últimos datos
					// de todos los colectores que han tenido una recolección exitosa.
					uiDataMutex.RLock()
					if sysMetrics, ok := currentCollectedData["system"].(*collector.SystemMetrics); ok {
						fullReport.System = sysMetrics
					}
					if mysqlMetrics, ok := currentCollectedData["mysql"].(*mysql.MySQLMetrics); ok {
						fullReport.MySQL = mysqlMetrics
					}
					if nginxMetrics, ok := currentCollectedData["nginx"].(*nginx.NginxMetrics); ok {
						fullReport.Nginx = nginxMetrics
					}
					// ... añadir más tipos de métricas aquí ...
					uiDataMutex.RUnlock()

					// Actualizar la variable global latestAgentReport para la UI
					mu.Lock()
					latestAgentReport = fullReport // La UI obtendrá el reporte más reciente
					mu.Unlock()

					// Enviar métricas
					err = httpSender.Send(fullReport)
					if err != nil {
						metricsSent.WithLabelValues("failure", cfg.AgentName, cfg.AgentID).Inc()
						logrus.WithError(err).Errorf("Error al enviar métricas de '%s' al backend.", c.Name())
					} else {
						metricsSent.WithLabelValues("success", cfg.AgentName, cfg.AgentID).Inc()
						logrus.Infof("Métricas de '%s' enviadas exitosamente al backend.", c.Name())
					}

				case <-ctx.Done():
					logrus.Infof("Contexto cancelado para el colector '%s'. Deteniendo.", c.Name())
					return // Salir de la goroutine del colector
				}
			}
		}(col) // Pasar el colector a la goroutine
	}

	// Esperar a que todas las goroutines de colectores terminen antes de salir del main
	wg.Wait()
	logrus.Info("Todas las goroutines de colectores han terminado. Apagado completado.")
}
