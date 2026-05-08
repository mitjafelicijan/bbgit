package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const Version = "0.1.0"

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{w, http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, lrw.statusCode, time.Since(start))
	})
}

var (
	GlobalConfig *Config
)

func main() {
	var showVersion bool
	flag.StringVar(&ConfigPath, "c", "config.yaml", "custom config path")
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.Parse()

	if showVersion {
		fmt.Printf("bbgit version %s\n", Version)
		return
	}

	var err error
	GlobalConfig, err = loadConfig(ConfigPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	LoadCache()
	StartSaver()

	mux := http.NewServeMux()

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("GET /{$}", homeHandler)
	mux.HandleFunc("GET /r/{name}", repoHandler)
	mux.HandleFunc("GET /r/{name}/readme", readmeHandler)
	mux.HandleFunc("GET /r/{name}/license", licenseHandler)
	mux.HandleFunc("GET /r/{name}/markers", markersHandler)
	mux.HandleFunc("GET /r/{name}/commits.rss", repoCommitsRSSHandler)
	mux.HandleFunc("GET /r/{name}/tags.rss", repoTagsRSSHandler)
	mux.HandleFunc("GET /r/{name}/files/{path...}", filesHandler)
	mux.HandleFunc("GET /r/{name}/blob/{path...}", blobHandler)
	mux.HandleFunc("GET /r/{name}/raw/{path...}", rawHandler)
	mux.HandleFunc("GET /r/{name}/archive/{path...}", archiveHandler)
	mux.HandleFunc("GET /r/{name}/c/{hash}", commitHandler)
	mux.HandleFunc("GET /r/{name}/c/{hash}/patch", patchHandler)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down... saving cache.")
		if err := SaveCache(); err != nil {
			log.Printf("Error saving cache on shutdown: %v", err)
		}
		os.Exit(0)
	}()

	fmt.Printf("Server starting on :8080 (config: %s)...\n", ConfigPath)
	if err := http.ListenAndServe(":8080", loggingMiddleware(mux)); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
