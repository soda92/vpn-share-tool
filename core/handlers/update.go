package handlers

import (
	_ "embed"
	"log"
	"net/http"
)

type TriggerUpdateHandler struct {
	TriggerUpdate func() (bool, error)
}

func (h *TriggerUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		updated, err := h.TriggerUpdate()
		if err != nil {
			log.Printf("Remote trigger update failed: %v", err)
		} else if updated {
			log.Printf("Remote triggered update success. Exiting.")
			// The process will exit in TriggerUpdate usually, but if not:
			// os.Exit(0)
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Update triggered"))
}
