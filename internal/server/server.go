package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"dify-log-excel/internal/config"
	"dify-log-excel/internal/logentry"
	"dify-log-excel/internal/store"
)

type Server struct {
	store *store.Store
	cfg   config.Config
	loc   *time.Location
	mux   *http.ServeMux
}

func New(st *store.Store, cfg config.Config, loc *time.Location) http.Handler {
	s := &Server{store: st, cfg: cfg, loc: loc, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/v1/logs", s.handleLogs)
	return s.mux
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if r.Header.Get("X-API-Key") != s.cfg.LogAPIKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	defer r.Body.Close()
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}

	record, err := logentry.Parse(data, s.cfg.MaskFields, s.loc)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	excelDate := record.CreatedAt.In(s.loc).Format("2006-01-02")
	excelPath := filepath.Join(s.cfg.ExcelDir, excelDate+".xlsx")
	if err := s.store.InsertLog(context.Background(), record, excelDate, excelPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"execution_id": record.ExecutionID,
		"log_id":       record.ID,
		"status":       record.Status,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
