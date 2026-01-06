package main

import (
	"crypto/rand"
	"embed"
	"io/fs"
	"log"
	"math/big"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var frontend embed.FS

func main() {
	fsSub, err := fs.Sub(frontend, "dist")
	if err != nil {
		log.Fatal(err)
	}

	// Serve Captcha Image
	http.HandleFunc("/phis/app/login/voCode", func(w http.ResponseWriter, r *http.Request) {
		dirPath := "phis/app/login"
		entries, err := fs.ReadDir(fsSub, dirPath)
		if err != nil {
			log.Printf("Error reading directory %s: %v", dirPath, err)
			http.Error(w, "Captcha Error", http.StatusInternalServerError)
			return
		}

		var candidates []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "voCode-") && strings.HasSuffix(e.Name(), ".jpeg") {
				candidates = append(candidates, e.Name())
			}
		}

		if len(candidates) == 0 {
			http.Error(w, "No captcha images found", http.StatusNotFound)
			return
		}

		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		selectedName := candidates[n.Int64()]

		// Expected format: voCode-CODE.jpeg
		parts := strings.Split(selectedName, "-")
		if len(parts) >= 2 {
			codePart := parts[1]
			code := strings.TrimSuffix(codePart, ".jpeg")
			
			// Set expected code in cookie (plain text for demo simplicity)
			http.SetCookie(w, &http.Cookie{
				Name:  "expected_captcha",
				Value: code,
				Path:  "/",
			})
			log.Printf("Serving captcha: %s (Code: %s)", selectedName, code)
		}

		// Prevent caching
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Content-Type", "image/jpeg")

		fullPath := path.Join(dirPath, selectedName)
		data, err := fs.ReadFile(fsSub, fullPath)
		if err != nil {
			log.Printf("Error reading file %s: %v", fullPath, err)
			http.Error(w, "Read Error", http.StatusInternalServerError)
			return
		}

		w.Write(data)
	})

	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		cookie := http.Cookie{
			Name:     "session",
			Value:    "loggedin",
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		cookie := http.Cookie{
			Name:     "session",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour),
			HttpOnly: true,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/check-auth", func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("session")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		expected, err := r.Cookie("expected_captcha")
		if err != nil {
			http.Error(w, "Captcha session expired", http.StatusBadRequest)
			return
		}
		
		submitted := r.FormValue("verifyCode") // Must match input name in frontend
		if submitted == "" {
			// Try "captcha" or other common names if needed, but let's stick to one
			submitted = r.FormValue("captcha")
		}

		log.Printf("Verification: Expected '%s', Got '%s'", expected.Value, submitted)

		if !strings.EqualFold(submitted, expected.Value) {
			http.Error(w, "Incorrect Captcha", http.StatusForbidden)
			return
		}
		
		log.Printf("Received valid form submission: %v", r.PostForm)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Form received"))
	})

	// Main file server
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the requested file exists in the embedded filesystem
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			f, err := fsSub.Open(path)
			if err != nil {
				// If the file doesn't exist, serve index.html
				r.URL.Path = "/"
			} else {
				f.Close()
			}
		}
		http.FileServer(http.FS(fsSub)).ServeHTTP(w, r)
	}))

	log.Println("Starting server on :8888")
	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal(err)
	}
}
