package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql" // Driver de MySQL
	"github.com/sirupsen/logrus"

	"github.com/atrox39/logtick/collector" // Importa el paquete collector para la interfaz
	"github.com/atrox39/logtick/config"
)

// MySQLMetrics contiene las métricas específicas de MySQL
type MySQLMetrics struct {
	Uptime               uint64  `json:"uptime_seconds"`
	ThreadsConnected     uint64  `json:"threads_connected"`
	ThreadsRunning       uint64  `json:"threads_running"`
	Connections          uint64  `json:"total_connections"`
	BytesReceived        uint64  `json:"bytes_received"`
	BytesSent            uint64  `json:"bytes_sent"`
	Queries              uint64  `json:"queries_total"`
	InnodbBufferPoolHits float64 `json:"innodb_buffer_pool_reads_hits_ratio"`
}

// MySQLCollector implementa la interfaz Collector para métricas de MySQL
type MySQLCollector struct {
	db       *sql.DB
	dsn      string
	interval time.Duration
	log      *logrus.Entry // Logger para este colector
}

// NewMySQLCollector crea una nueva instancia de MySQLCollector
func NewMySQLCollector(cfg *config.MySQLConfig) (*MySQLCollector, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("DSN de MySQL no puede estar vacío")
	}

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión MySQL: %w", err)
	}

	// Ping para verificar la conexión inicial
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close() // Cerrar la conexión si el ping falla
		return nil, fmt.Errorf("error al conectar con MySQL DSN '%s': %w", cfg.DSN, err)
	}

	return &MySQLCollector{
		db:       db,
		dsn:      cfg.DSN,
		interval: time.Duration(cfg.CollectionIntervalSeconds) * time.Second,
		log:      logrus.WithField("collector", "mysql"),
	}, nil
}

// Collect recolecta métricas de MySQL
func (c *MySQLCollector) Collect() (collector.MetricData, error) {
	var statusVars map[string]string
	statusVars = make(map[string]string)

	rows, err := c.db.Query("SHOW GLOBAL STATUS")
	if err != nil {
		return nil, fmt.Errorf("error al ejecutar 'SHOW GLOBAL STATUS': %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var varName, value string
		if err := rows.Scan(&varName, &value); err != nil {
			c.log.WithError(err).Warn("Error al escanear fila de estado de MySQL")
			continue
		}
		statusVars[varName] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error de fila después de iterar en MySQL status: %w", err)
	}

	// Funciones auxiliares para convertir a uint64
	parseUint := func(s string) uint64 {
		val, _ := strconv.ParseUint(s, 10, 64)
		return val
	}

	// Calcular InnoDB Buffer Pool Hit Ratio
	innodbReads := parseUint(statusVars["Innodb_buffer_pool_read_requests"])
	innodbHits := innodbReads - parseUint(statusVars["Innodb_buffer_pool_reads"])
	var innodbHitRatio float64
	if innodbReads > 0 {
		innodbHitRatio = float64(innodbHits) / float64(innodbReads) * 100
	}

	metrics := &MySQLMetrics{
		Uptime:               parseUint(statusVars["Uptime"]),
		ThreadsConnected:     parseUint(statusVars["Threads_connected"]),
		ThreadsRunning:       parseUint(statusVars["Threads_running"]),
		Connections:          parseUint(statusVars["Connections"]),
		BytesReceived:        parseUint(statusVars["Bytes_received"]),
		BytesSent:            parseUint(statusVars["Bytes_sent"]),
		Queries:              parseUint(statusVars["Queries"]),
		InnodbBufferPoolHits: innodbHitRatio,
	}

	c.log.WithFields(logrus.Fields{
		"threads_connected": metrics.ThreadsConnected,
		"queries":           metrics.Queries,
	}).Debug("Métricas de MySQL recolectadas")

	return metrics, nil
}

// Name devuelve el nombre de este colector
func (c *MySQLCollector) Name() string {
	return "mysql"
}

// GetInterval devuelve el intervalo de recolección para este colector
func (c *MySQLCollector) GetInterval() time.Duration {
	return c.interval
}
