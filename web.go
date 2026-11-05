package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"time"
)

const maxRequestBody = 4 << 10

//go:embed web/templates/*.html web/static/*
var webFiles embed.FS

type webApp struct {
	cluster  *cluster
	template *template.Template
}

func newWebHandler(cluster *cluster) http.Handler {
	app := &webApp{
		cluster:  cluster,
		template: template.Must(template.ParseFS(webFiles, "web/templates/*.html")),
	}
	staticFiles, err := fs.Sub(webFiles, "web/static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	mux.HandleFunc("GET /", app.index)
	mux.HandleFunc("GET /api/state", app.state)
	mux.HandleFunc("GET /api/events", app.events)
	mux.HandleFunc("POST /api/network", app.configureNetwork)
	mux.HandleFunc("POST /api/nodes", app.configureNode)
	return mux
}

func (a *webApp) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.template.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func (a *webApp) state(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.cluster.snapshot())
}

func (a *webApp) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	stream, unsubscribe := a.cluster.events.subscribe()
	defer unsubscribe()

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-stream:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = w.Write([]byte(": keep-alive\n\n"))
			flusher.Flush()
		}
	}
}

func (a *webApp) configureNetwork(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Mode networkMode `json:"mode"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.cluster.network.setMode(input.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a.cluster.snapshot())
}

func (a *webApp) configureNode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID      id   `json:"id"`
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := a.cluster.network.setNode(input.ID, input.Enabled); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a.cluster.snapshot())
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "JSON body must contain one object")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
