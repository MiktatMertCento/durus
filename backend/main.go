package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

func main() {
	addr := envOr("ADDR", ":8080")
	staticDir := envOr("STATIC_DIR", "./static")
	dbURL := databaseURL()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	db, err := openDB(ctx, dbURL)
	cancel()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	bootCtx, bootCancel := context.WithTimeout(context.Background(), 10*time.Second)
	store, err := NewStore(bootCtx, db)
	bootCancel()
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	hub := NewHub(store)
	stopTick := make(chan struct{})
	go hub.tickLoop(stopTick)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pingCancel()
		if err := store.Ping(pingCtx); err != nil {
			http.Error(w, `{"ok":false}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/ws", hub.handleWS)
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, hub.store.View())
	})
	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, _ *http.Request) {
		items, err := hub.store.ListSessions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)
	})
	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		arch, err := hub.store.GetSession(id)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, arch)
	})
	registerStatic(mux, staticDir)

	server := &http.Server{
		Addr:              addr,
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (postgres)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	close(stopTick)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func registerStatic(mux *http.ServeMux, staticDir string) {
	info, err := os.Stat(staticDir)
	if err != nil || !info.IsDir() {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("duruş api — connect via /ws"))
		})
		return
	}

	fileServer := http.FileServer(http.Dir(staticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		clean := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		if clean != "/" {
			full := path.Join(staticDir, clean)
			if !strings.HasPrefix(full, path.Clean(staticDir)+"/") && full != path.Clean(staticDir) {
				http.NotFound(w, r)
				return
			}
			if st, err := os.Stat(full); err != nil || st.IsDir() {
				http.ServeFile(w, r, path.Join(staticDir, "index.html"))
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
