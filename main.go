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
	"time"

	"github.com/google/uuid"
	godotenv "github.com/joho/godotenv"
	_ "github.com/lib/pq"
	mydb "github.com/nkapila6/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	Queries        mydb.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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

func createChirp(queries mydb.Queries, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	req := struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
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

	user, err := queries.CreateChirp(r.Context(), mydb.CreateChirpParams{Body: nbody, UserID: req.UserID})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}{user.ID, user.CreatedAt, user.UpdatedAt, user.Body, user.UserID})

	// json.NewEncoder(w).Encode(struct {
	// 	Body string `json:"cleaned_body"`
	// 	// Valid bool `json:"valid"`
	// }{nbody})
}

func createUser(queries mydb.Queries, w http.ResponseWriter, r *http.Request) {
	// Add a new endpoint to your server, POST /api/users, which allows users to be created. It accepts an email as JSON in the request body and returns the user's ID, email, and timestamps in the response body.

	// read into struct
	user := User{}
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		json.NewEncoder(w).Encode(struct {
			Error string `json:"error"`
		}{"Something went wrong"})
		return
	}

	user1, err := queries.CreateUser(r.Context(), user.Email)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Println(user1)

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		ID        uuid.UUID `json:"id"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}{user1.ID, user1.Email, user1.CreatedAt, user1.UpdatedAt})

}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("could not open db: ", err)
	}
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
	// mux.HandleFunc("POST /api/validate_chirp", validate_chirp)
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		createUser(*dbQueries, w, r)
	})
	mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		createChirp(*dbQueries, w, r)
	})

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal("urmumma")
	}
}
