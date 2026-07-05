package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"dify-log-excel/internal/applog"
	"dify-log-excel/internal/config"
	"dify-log-excel/internal/logentry"
	"dify-log-excel/internal/store"
)

type Server struct {
	store *store.Store
	cfg   config.Config
	loc   *time.Location
	mux   *http.ServeMux
	stop  func()
	log   *applog.Logger
}

func New(st *store.Store, cfg config.Config, loc *time.Location) http.Handler {
	return NewWithControl(st, cfg, loc, nil)
}

func NewWithControl(st *store.Store, cfg config.Config, loc *time.Location, onStop func()) http.Handler {
	return NewWithLogger(st, cfg, loc, onStop, nil)
}

func NewWithLogger(st *store.Store, cfg config.Config, loc *time.Location, onStop func(), logger *applog.Logger) http.Handler {
	s := &Server{store: st, cfg: cfg, loc: loc, mux: http.NewServeMux(), stop: onStop, log: logger}
	s.mux.HandleFunc("/api/v1/logs", s.handleLogs)
	if onStop != nil {
		s.mux.HandleFunc("/api/v1/admin/stop", s.handleStop)
	}
	return s.mux
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		s.logRequest(r, http.StatusMethodNotAllowed, started, "method not allowed", nil, nil)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Header.Get("X-API-Key") != s.cfg.LogAPIKey {
		s.logRequest(r, http.StatusUnauthorized, started, "unauthorized", nil, nil)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	defer r.Body.Close()
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		s.logRequest(r, http.StatusUnprocessableEntity, started, err.Error(), nil, nil)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	record, err := logentry.Parse(data, s.cfg.MaskFields, s.loc)
	if err != nil {
		s.logRequest(r, http.StatusUnprocessableEntity, started, err.Error(), data, nil)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	excelDate := record.CreatedAt.In(s.loc).Format("2006-01-02")
	excelPath := filepath.Join(s.cfg.ExcelDir, excelDate+".xlsx")
	if err := s.store.InsertLog(context.Background(), record, excelDate, excelPath); err != nil {
		s.logRequest(r, http.StatusInternalServerError, started, err.Error(), data, &record)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.logRequest(r, http.StatusOK, started, "", data, &record)
	writeJSON(w, http.StatusOK, map[string]string{
		"execution_id": record.ExecutionID,
		"log_id":       record.ID,
		"status":       record.Status,
	})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodPost {
		s.logRequest(r, http.StatusMethodNotAllowed, started, "method not allowed", nil, nil)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Header.Get("X-API-Key") != s.cfg.LogAPIKey {
		s.logRequest(r, http.StatusUnauthorized, started, "unauthorized", nil, nil)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !isLoopbackRemote(r.RemoteAddr) {
		s.logRequest(r, http.StatusForbidden, started, "local requests only", nil, nil)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "local requests only"})
		return
	}
	s.logRequest(r, http.StatusOK, started, "", nil, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
	s.stop()
}

func (s *Server) logRequest(r *http.Request, status int, started time.Time, message string, body []byte, record *logentry.Record) {
	if s.log == nil {
		return
	}
	duration := time.Since(started).Round(time.Millisecond)
	fields := []any{r.Method, r.URL.Path, status, r.RemoteAddr, duration}
	format := "request method=%s path=%s status=%d remote=%s duration=%s"
	if record != nil {
		format += " execution_id=%q node_id=%q node_name=%q"
		fields = append(fields, record.ExecutionID, record.NodeID, record.NodeName)
	}
	if message != "" {
		format += " error=%q"
		fields = append(fields, message)
	}
	if s.cfg.LogBody && body != nil {
		format += " request_body=%q"
		fields = append(fields, string(body))
	}
	if status >= 400 {
		s.log.Error(format, fields...)
	} else {
		s.log.Info(format, fields...)
	}
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
