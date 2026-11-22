//go:build linux

package station

import (
	"embed"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

//go:embed web/*
var web embed.FS

func decode(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/analysis", func(w http.ResponseWriter, r *http.Request) {
		var request AnalysisRequest
		if err := decode(w, r, &request); err != nil {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
			return
		}
		job, err := s.StartAnalysis(request)
		if err != nil {
			jsonResponse(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 202, job)
	})
	mux.HandleFunc("GET /api/patterns", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, Patterns()) })
	mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		result, err := s.Create()
		if err != nil {
			jsonResponse(w, 503, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 201, result)
	})
	mux.HandleFunc("GET /api/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		result, err := s.Get(r.PathValue("id"))
		if err != nil {
			jsonResponse(w, 404, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, result)
	})
	mux.HandleFunc("POST /api/commands", func(w http.ResponseWriter, r *http.Request) {
		var cmd Command
		if err := decode(w, r, &cmd); err != nil {
			jsonResponse(w, 400, map[string]string{"error": err.Error()})
			return
		}
		result, err := s.Apply(r.Context(), cmd)
		if err != nil {
			jsonResponse(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, 200, result)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Path
		if file == "/" || file == "/experimental/station" {
			file = "/index.html"
		}
		if file != "/index.html" && file != "/app.js" && file != "/app.css" {
			http.NotFound(w, r)
			return
		}
		b, err := web.ReadFile("web" + file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasSuffix(file, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(file, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		_, _ = w.Write(b)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The lab is loopback only. Reject rebinding and cross-origin browser
		// requests; Android uses adb reverse, not an unauthenticated LAN port.
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "loopback host required", 403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || u.Host != r.Host {
				http.Error(w, "same origin required", 403)
				return
			}
		}
		if r.Method == "POST" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "application/json required", 415)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
