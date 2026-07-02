package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dify-log-excel/internal/applog"
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

func TestPostLogsAcceptsMissingNodeFields(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(`{"workflow_name":"前端调用","input_data":"hello"}`))
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPostLogsRejectsMalformedJSON(t *testing.T) {
	handler := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(`{"input_data": hello}`))
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPostLogsWritesRequestErrorToLog(t *testing.T) {
	var logs bytes.Buffer
	handler := newTestHandlerWithLogger(t, nil, &logs)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/logs", strings.NewReader(`{"input_data": hello}`))
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	logText := logs.String()
	for _, want := range []string{"POST", "/api/v1/logs", "status=422", "invalid JSON body", "request_body=", "input_data", "hello"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("log missing %q: %s", want, logText)
		}
	}
}

func TestAdminStopInvokesCallbackForLoopbackRequest(t *testing.T) {
	stopped := make(chan struct{}, 1)
	handler := newTestHandlerWithStop(t, func() {
		stopped <- struct{}{}
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stop", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case <-stopped:
	default:
		t.Fatal("stop callback was not invoked")
	}
}

func TestAdminStopRejectsRemoteAddress(t *testing.T) {
	called := false
	handler := newTestHandlerWithStop(t, func() {
		called = true
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stop", nil)
	req.RemoteAddr = "203.0.113.10:4567"
	req.Header.Set("X-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("stop callback was invoked for non-loopback request")
	}
}

func TestAdminStopRequiresAPIKey(t *testing.T) {
	called := false
	handler := newTestHandlerWithStop(t, func() {
		called = true
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stop", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("stop callback was invoked without API key")
	}
}

func newTestHandler(t *testing.T) http.Handler {
	return newTestHandlerWithStop(t, nil)
}

func newTestHandlerWithStop(t *testing.T, onStop func()) http.Handler {
	return newTestHandlerWithLogger(t, onStop, nil)
}

func newTestHandlerWithLogger(t *testing.T, onStop func(), writer io.Writer) http.Handler {
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
	var logger *applog.Logger
	if writer != nil {
		var err error
		logger, err = applog.New(writer, true, "debug")
		if err != nil {
			t.Fatal(err)
		}
	}
	return NewWithLogger(st, cfg, loc, onStop, logger)
}
