package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dify-log-excel/internal/config"
	"dify-log-excel/internal/store"
)

func TestPostLogsRequiresAPIKey(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPostLogsWritesSQLiteAndReturnsCompatibleResponse(t *testing.T) {
	handler := newTestHandler(t)
	body := `{"execution_id":"exec-http","node_id":"node","node_name":"Node","status":"success","input_data":{"password":"secret"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(body))
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"execution_id":"exec-http"`) {
		t.Fatalf("response body = %s", rec.Body.String())
	}
}

func TestPostLogsRejectsInvalidBody(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(`{"node_id":""}`))
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	base := t.TempDir()
	st, err := store.Open(filepath.Join(base, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default(base)
	cfg.LogAPIKey = "test-key"
	loc := time.FixedZone("CST", 8*3600)
	return New(st, cfg, loc)
}
