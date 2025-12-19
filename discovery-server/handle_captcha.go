package main

import (
	"io"
	"net/http"
)

// handleSolveCaptchaRequest receives an image and returns the solved text.
func handleSolveCaptchaRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	solution := SolveCaptcha(body)
	w.Write([]byte(solution))
}
