package zoo

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// openAPISpec is the zoo's contract, served at GET /openapi.yaml.
//
//go:embed openapi.yaml
var openAPISpec []byte

// API holds the zoo's dependencies and implements its HTTP handlers.
type API struct {
	logger  *slog.Logger
	animals *animalStore
	feeds   *feedStore
	Address string
}

// Run serves the zoo on addr until ctx is cancelled or the server fails.
// The zoo is a benchmark target, so cancellation just closes the server;
// there is no graceful drain.
func Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	stop := context.AfterFunc(ctx, func() { _ = srv.Close() })
	defer stop()

	fmt.Println("Zoo API is live")

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// handler returns the zoo's routes. It is separate from Run so tests can
// exercise it with httptest.
func handler() http.Handler {
	mux := http.NewServeMux()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	apiCfg := &API{
		logger:  logger,
		animals: newAnimalStore(),
		feeds:   newFeedStore(),
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openAPISpec)
	})
	mux.HandleFunc("GET /animals", apiCfg.ListAnimals)
	mux.HandleFunc("POST /animals", apiCfg.CreateAnimal)
	mux.HandleFunc("GET /animals/{id}", apiCfg.FindAnimal)
	mux.HandleFunc("PUT /animals/{id}", apiCfg.PutAnimal)
	mux.HandleFunc("PATCH /animals/{id}", apiCfg.PatchAnimal)
	mux.HandleFunc("DELETE /animals/{id}", apiCfg.DeleteAnimal)

	mux.HandleFunc("GET /feeds", apiCfg.ListFeeds)
	mux.HandleFunc("POST /feeds", apiCfg.CreateFeed)
	mux.HandleFunc("GET /feeds/{id}", apiCfg.FindFeed)
	mux.HandleFunc("PUT /feeds/{id}", apiCfg.PutFeed)
	mux.HandleFunc("POST /feeds/{id}/restock", apiCfg.RestockFeed)
	mux.HandleFunc("DELETE /feeds/{id}", apiCfg.DeleteFeed)

	mux.HandleFunc("GET /animals/{id}/feedings", apiCfg.ListFeedings)
	mux.HandleFunc("POST /animals/{id}/feedings", apiCfg.CreateFeeding)

	return mux
}
