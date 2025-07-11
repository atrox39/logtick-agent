package utils

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func Server() {

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error al leer el cuerpo de la solicitud", http.StatusInternalServerError)
			return
		}

		var metrics map[string]interface{}
		err = json.Unmarshal(body, &metrics)
		if err != nil {
			http.Error(w, "Error al parsear JSON", http.StatusBadRequest)
			return
		}

		log.Printf("Métricas recibidas: %+v", metrics)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Métricas recibidas OK"))
	})

	// Adding websocket endpoint
	http.HandleFunc("/ws/logs", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Error al actualizar la conexión WebSocket:", err)
			return
		}
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error al leer el mensaje WebSocket:", err)
				break
			}
			log.Printf("Mensaje recibido: %s", message)
		}
	})

	log.Println("Server started on :4003")
	log.Fatal(http.ListenAndServe(":4003", nil))
}
