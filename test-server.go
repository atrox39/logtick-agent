package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

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
	log.Println("Server started on :4003")
	log.Fatal(http.ListenAndServe(":4003", nil))
}
