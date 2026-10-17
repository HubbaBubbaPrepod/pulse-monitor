package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}
func apiError(w http.ResponseWriter, status int, message string) {
	code := map[int]string{400: "error.invalid_input", 401: "error.unauthorized", 403: "error.forbidden", 404: "error.not_found", 405: "error.method_not_allowed", 409: "error.conflict", 429: "error.rate_limited"}[status]
	if code == "" {
		code = "error.internal"
	}
	switch message {
	case "Неверный email или пароль":
		code = "error.invalid_credentials"
	case "Владелец панели уже зарегистрирован":
		code = "error.registration_closed"
	case "Укажите имя, email и пароль от 12 символов, до 72 байт":
		code = "error.weak_password"
	}
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("expected one JSON value")
	}
	return nil
}
func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	a.authRoutes(mux)
	a.agentRoutes(mux)
	a.eventRoutes(mux)
	a.retentionRoutes(mux)
	a.preferenceRoutes(mux)
	a.moduleRoutes(mux)
	a.marketplaceRoutes(mux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "source": a.dataSource()})
	})
	mux.HandleFunc("GET /api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"servers": a.serverList(), "source": a.dataSource(), "startedAt": a.started.Format(time.RFC3339Nano)})
	})
	mux.HandleFunc("PUT /api/servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		var s server
		if err := decodeJSON(w, r, &s); err != nil {
			apiError(w, 400, "Invalid server JSON")
			return
		}
		if s.ID != r.PathValue("id") {
			apiError(w, 400, "Server ID mismatch")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		list := a.serverList()
		if slices.ContainsFunc(list, func(item server) bool { return item.ID != s.ID && strings.EqualFold(item.Name, s.Name) }) {
			apiError(w, 409, "Server name already exists")
			return
		}
		i := slices.IndexFunc(list, func(item server) bool { return item.ID == s.ID })
		if i < 0 || list[i].Address != s.Address {
			s.Source = "waiting"
			s.OS = ""
			s.Online = false
			s.CPU = 0
			s.Memory = 0
			s.Disk = 0
		} else {
			old := list[i]
			s.Source = old.Source
			s.OS = old.OS
			s.Online = old.Online
			s.CPU = old.CPU
			s.Memory = old.Memory
			s.Disk = old.Disk
		}
		s.LogRetentionDays = nil
		if err := validateServer(s); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		if i >= 0 && list[i].Address != s.Address {
			if err := a.resetAgent(s.ID, true); err != nil {
				apiError(w, 500, "Could not reset agent")
				return
			}
		}
		if i < 0 {
			if len(list) >= 50 {
				apiError(w, 409, "Maximum 50 servers")
				return
			}
			list = append(list, s)
		} else {
			list[i] = s
		}
		if i < 0 {
			if err := a.storage.setRetention(r.Context(), s.ID, 30); err != nil {
				apiError(w, 500, "Could not save log retention")
				return
			}
		}
		if err := a.persistServers(list); err != nil {
			apiError(w, 500, "Could not save server")
			return
		}
		a.mu.Lock()
		a.servers = list
		if i < 0 {
			a.retentions[s.ID] = 30
		}
		a.mu.Unlock()
		writeJSON(w, 200, s)
	})
	mux.HandleFunc("DELETE /api/servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		list := a.serverList()
		i := slices.IndexFunc(list, func(s server) bool { return s.ID == r.PathValue("id") })
		if i < 0 {
			apiError(w, 404, "Server not found")
			return
		}
		list = slices.Delete(list, i, i+1)
		if err := a.resetAgent(r.PathValue("id"), false); err != nil {
			apiError(w, 500, "Could not revoke agent")
			return
		}
		if err := a.persistServers(list); err != nil {
			apiError(w, 500, "Could not delete server")
			return
		}
		a.mu.Lock()
		a.servers = list
		a.mu.Unlock()
		writeJSON(w, 200, map[string]bool{"deleted": true})
	})
	mux.HandleFunc("GET /api/servers/{id}/snapshot", func(w http.ResponseWriter, r *http.Request) {
		s, ok := a.findServer(r.PathValue("id"))
		if !ok {
			apiError(w, 404, "Server not found")
			return
		}
		period := 60
		if raw := r.URL.Query().Get("period"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || !slices.Contains([]int{15, 60, 360, 1440}, n) {
				apiError(w, 400, "Invalid chart period")
				return
			}
			period = n
		}
		writeJSON(w, 200, a.snapshot(s, period))
	})
	mux.HandleFunc("GET /api/servers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := a.findServer(id); !ok {
			apiError(w, 404, "Server not found")
			return
		}
		q := r.URL.Query()
		limit := 100
		offset := 0
		var err error
		if q.Get("limit") != "" {
			limit, err = strconv.Atoi(q.Get("limit"))
			if err != nil || limit < 1 || limit > 200 {
				apiError(w, 400, "Invalid log limit")
				return
			}
		}
		if q.Get("offset") != "" {
			offset, err = strconv.Atoi(q.Get("offset"))
			if err != nil || offset < 0 || offset > 1000000 {
				apiError(w, 400, "Invalid log offset")
				return
			}
		}
		if q.Get("before") != "" {
			if _, err := time.Parse(time.RFC3339Nano, q.Get("before")); err != nil {
				apiError(w, 400, "Invalid time boundary")
				return
			}
		}
		if q.Get("level") != "" && !slices.Contains([]string{"INFO", "WARN", "ERROR"}, q.Get("level")) {
			apiError(w, 400, "Invalid log level")
			return
		}
		if q.Get("category") != "" && !slices.Contains([]string{"system", "security", "telemetry", "agent"}, q.Get("category")) {
			apiError(w, 400, "Invalid log category")
			return
		}
		page, err := a.storage.queryLogs(r.Context(), id, q.Get("level"), q.Get("source"), q.Get("q"), q.Get("before"), offset, limit, q.Get("category"))
		if err != nil {
			apiError(w, 500, "Could not read log archive")
			return
		}
		writeJSON(w, 200, page)
	})
	mux.HandleFunc("GET /api/servers/{id}/stream", a.stream)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		localHost := host == "localhost" || (ip != nil && ip.IsLoopback())
		if (a.publicHost != "" && r.Host != a.publicHost) || (a.publicHost == "" && !localHost) {
			apiError(w, 403, "Host not allowed")
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			same := err == nil && u.Host == r.Host && u.Scheme == scheme
			dev := a.publicHost == "" && r.TLS == nil && (origin == "http://127.0.0.1:3000" || origin == "http://localhost:3000")
			if !same && !dev {
				apiError(w, 403, "Origin not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		agent := strings.HasPrefix(r.URL.Path, "/api/agent/")
		public := agent || r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/status" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/register"
		mutation := r.Method != "GET" && r.Method != "HEAD"
		if mutation && (r.Header.Get("Sec-Fetch-Site") == "cross-site" || ((r.Method == "POST" || r.Method == "PUT") && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"))) {
			apiError(w, 403, "Invalid request origin or content type")
			return
		}
		if !public {
			session, ok := a.currentSession(r)
			if !ok {
				apiError(w, 401, "Требуется вход в панель")
				return
			}
			if mutation && !validCSRF(r, session) {
				apiError(w, 403, "Недействительный токен сессии")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *app) stream(w http.ResponseWriter, r *http.Request) {
	s, ok := a.findServer(r.PathValue("id"))
	if !ok {
		apiError(w, 404, "Server not found")
		return
	}
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	timer := time.NewTicker(500 * time.Millisecond)
	defer timer.Stop()
	for {
		if _, ok := a.currentSession(r); !ok {
			return
		}
		if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return
		}
		data, err := json.Marshal(a.snapshot(s, 60))
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", data); err != nil {
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
		}
		var exists bool
		s, exists = a.findServer(s.ID)
		if !exists {
			return
		}
	}
}

// Collection continues when browsers disconnect or pause their views.
func (a *app) collect(ctx context.Context) error {
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	for {
		for _, s := range a.serverList() {
			if err := a.checkAgentOffline(s); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
	}
}
