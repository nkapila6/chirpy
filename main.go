package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) reset() {
	cfg.fileserverHits.Store(0)
}

func main() {
	mux := http.NewServeMux()

	var apiCfg apiConfig
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir("./app/")))))

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		// w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`<html>
						  <body>
						    <h1>Welcome, Chirpy Admin</h1>
						    <p>Chirpy has been visited %d times!</p>
						  </body>
						</html>`, apiCfg.fileserverHits.Load())))
		// w.Write([]byte(fmt.Sprintf("Hits: %d", apiCfg.fileserverHits.Load())))
	})

	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		apiCfg.reset()
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Resetted: %d", apiCfg.fileserverHits.Load())))
	})

	mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		req := struct {
			Body string `json:"body"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)

			json.NewEncoder(w).Encode(struct {
				Error string `json:"error"`
			}{"Something went wrong"})
			return
		}

		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		if len(req.Body) > 140 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(struct {
				Error string `json:"error"`
			}{"Chirp is too long"})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(struct {
			Valid bool `json:"valid"`
		}{true})
	})

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("urmumma")
	}
}
