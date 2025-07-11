package sender

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// LogMessage representa una estructura de mensaje de log simple
type LogMessage struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Timestamp int64  `json:"timestamp"`
	Service   string `json:"service"` // e.g., "mysql", "nginx", "system"
	Message   string `json:"message"` // The actual log line
	Level     string `json:"level"`   // e.g., "info", "warn", "error"
}

// WebSocketLogSender gestiona la conexión WebSocket para logs en tiempo real
type WebSocketLogSender struct {
	wsURL             string
	conn              *websocket.Conn
	mu                sync.Mutex // Protege el acceso a 'conn'
	log               *logrus.Entry
	agentID           string
	agentName         string
	reconnectInterval time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewWebSocketLogSender crea una nueva instancia del sender de logs por WebSocket
func NewWebSocketLogSender(ctx context.Context, wsURL string, agentID string, agentName string) *WebSocketLogSender {
	ctx, cancel := context.WithCancel(ctx)
	s := &WebSocketLogSender{
		wsURL:             wsURL,
		log:               logrus.WithField("sender", "websocket_logs"),
		agentID:           agentID,
		agentName:         agentName,
		reconnectInterval: 5 * time.Second, // Intentar reconectar cada 5 segundos
		ctx:               ctx,
		cancel:            cancel,
	}
	go s.connectLoop() // Iniciar bucle de conexión en goroutine separada
	return s
}

// connectLoop intenta establecer y mantener la conexión WebSocket
func (s *WebSocketLogSender) connectLoop() {
	ticker := time.NewTicker(s.reconnectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.log.Info("Deteniendo el bucle de conexión WebSocket.")
			s.disconnect()
			return
		case <-ticker.C:
			if s.conn == nil {
				s.connect()
			}
		}
	}
}

// connect establece la conexión WebSocket
func (s *WebSocketLogSender) connect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		return // Ya conectado
	}

	s.log.Infof("Intentando conectar a WebSocket: %s", s.wsURL)
	u, err := url.Parse(s.wsURL)
	if err != nil {
		s.log.WithError(err).Error("URL WebSocket inválida.")
		return
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		s.log.WithError(err).Warn("No se pudo conectar al servidor WebSocket. Reintentando...")
		return
	}

	s.conn = c
	s.log.Info("Conexión WebSocket establecida exitosamente.")
	// Goroutine para monitorear la conexión y manejar cierres
	go s.readPump()
}

// readPump monitorea la conexión para cierres del lado del servidor
func (s *WebSocketLogSender) readPump() {
	defer func() {
		s.disconnect()
		s.log.Warn("Conexión WebSocket cerrada o error de lectura. Intentando reconectar...")
		// No se necesita llamar a connect() aquí, el connectLoop se encargará.
	}()

	for {
		select {
		case <-s.ctx.Done():
			return // Contexto cancelado, salir
		default:
			// Leer mensajes para detectar el cierre del lado del servidor.
			// No esperamos recibir mensajes, solo que no haya errores de lectura.
			_, _, err := s.conn.ReadMessage()
			if err != nil {
				// Error de lectura (ej. conexión cerrada), salir del bucle.
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.log.WithError(err).Error("Error de lectura inesperado en WebSocket.")
				}
				return
			}
		}
	}
}

// disconnect cierra la conexión WebSocket si está abierta
func (s *WebSocketLogSender) disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		s.log.Info("Conexión WebSocket cerrada.")
	}
}

// SendLog envía un mensaje de log a través del WebSocket
func (s *WebSocketLogSender) SendLog(service, message, level string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		s.log.Debug("No hay conexión WebSocket para enviar log.")
		return
	}

	logMsg := LogMessage{
		AgentID:   s.agentID,
		AgentName: s.agentName,
		Timestamp: time.Now().Unix(),
		Service:   service,
		Message:   message,
		Level:     level,
	}

	data, err := json.Marshal(logMsg)
	if err != nil {
		s.log.WithError(err).Error("Error al serializar el mensaje de log para WebSocket.")
		return
	}

	err = s.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		s.log.WithError(err).Error("Error al enviar mensaje de log por WebSocket. Marcando conexión para reconexión.")
		s.disconnect() // Cerrar la conexión, el bucle de conexión intentará reconectar
	} else {
		s.log.WithFields(logrus.Fields{
			"service": service,
			"level":   level,
			"message": message,
		}).Debug("Log enviado por WebSocket.")
	}
}

// Close cierra el sender y la conexión WebSocket
func (s *WebSocketLogSender) Close() {
	s.cancel() // Cancela el contexto para detener el connectLoop
	s.disconnect()
	s.log.Info("Sender de logs WebSocket cerrado.")
}
