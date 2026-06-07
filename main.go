package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	godotenv "github.com/joho/godotenv"
	_ "github.com/lib/pq"
	mydb "github.com/nkapila6/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	Queries        mydb.Queries
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

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (apiCfg *apiConfig) admin_metrics(w http.ResponseWriter, r *http.Request) {

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
}

func (apiCfg *apiConfig) admin_reset(w http.ResponseWriter, r *http.Request) {
	apiCfg.reset()
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Resetted: %d", apiCfg.fileserverHits.Load())))
}

func validate_chirp(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	if len(req.Body) > 140 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(struct {
			Error string `json:"error"`
		}{"Chirp is too long"})
		return
	}

	words := []string{"kerfuffle", "sharbert", "fornax"}
	body := strings.Split(req.Body, " ")
	for i, word := range body {
		for _, bad := range words {
			if strings.ToLower(word) == bad {
				body[i] = "****"
			}
		}
	}
	nbody := strings.Join(body, " ")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(struct {
		Body string `json:"cleaned_body"`
		// Valid bool `json:"valid"`
	}{nbody})
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err := db.Ping(); err != nil {
		log.Fatal("could not connect to db: ", err)
	}

	dbQueries := mydb.New(db)

	mux := http.NewServeMux()

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		Queries:        *dbQueries,
	}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir("./app/"))),
	))

	mux.HandleFunc("GET /api/healthz", healthz)
	mux.HandleFunc("GET /admin/metrics", apiCfg.admin_metrics)

	mux.HandleFunc("POST /admin/reset", apiCfg.admin_reset)
	mux.HandleFunc("POST /api/validate_chirp", validate_chirp)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("urmumma")
	}
}
