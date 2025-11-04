package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"
)

//go:embed all:frontend/dist
var frontend embed.FS

func main() {
	fs, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(fs)))

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		cookie := http.Cookie{
			Name:     "session",
			Value:    "loggedin",
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
		}
		http.SetCookie(w, &cookie)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		cookie := http.Cookie{
			Name:     "session",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour),
			HttpOnly: true,
		}
		http.SetCookie(w, &cookie)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/check-auth", func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("session")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	log.Println("Starting server on :8888")
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal(err)
	}
}
