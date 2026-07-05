package daemon

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsUseLogDir(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "logs")

	paths := PathsFor(logDir)

	if paths.PIDFile != filepath.Join(logDir, "dify-log-excel.pid") {
		t.Fatalf("PIDFile = %q", paths.PIDFile)
	}
	if paths.LogFile != filepath.Join(logDir, "dify-log-excel.out.log") {
		t.Fatalf("LogFile = %q", paths.LogFile)
	}
}

func TestWriteReadAndClearPID(t *testing.T) {
	paths := PathsFor(filepath.Join(t.TempDir(), "logs"))

	if err := WritePID(paths.PIDFile, 12345); err != nil {
		t.Fatalf("WritePID returned error: %v", err)
	}
	pid, ok, err := ReadPID(paths.PIDFile)
	if err != nil {
		t.Fatalf("ReadPID returned error: %v", err)
	}
	if !ok || pid != 12345 {
		t.Fatalf("ReadPID = (%d, %v), want (12345, true)", pid, ok)
	}

	if err := ClearPID(paths.PIDFile); err != nil {
		t.Fatalf("ClearPID returned error: %v", err)
	}
	_, ok, err = ReadPID(paths.PIDFile)
	if err != nil {
		t.Fatalf("ReadPID after clear returned error: %v", err)
	}
	if ok {
		t.Fatal("ReadPID after clear returned ok=true")
	}
}

func TestReadPIDRejectsInvalidContent(t *testing.T) {
	paths := PathsFor(filepath.Join(t.TempDir(), "logs"))
	if err := os.MkdirAll(filepath.Dir(paths.PIDFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.PIDFile, []byte("not-a-pid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := ReadPID(paths.PIDFile)
	if err == nil {
		t.Fatal("expected invalid pid file to fail")
	}
	if !strings.Contains(err.Error(), "invalid pid") {
		t.Fatalf("error = %v", err)
	}
}

func TestStopURLUsesLoopbackForWildcardHosts(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "::"} {
		got := StopURL(host, 8000)
		if got != "http://127.0.0.1:8000/api/v1/admin/stop" {
			t.Fatalf("StopURL(%q) = %q", host, got)
		}
	}
}

func TestStopURLUsesConfiguredHost(t *testing.T) {
	got := StopURL("localhost", 9001)
	if got != "http://localhost:9001/api/v1/admin/stop" {
		t.Fatalf("StopURL localhost = %q", got)
	}

	got = StopURL("::1", 9001)
	if got != "http://[::1]:9001/api/v1/admin/stop" {
		t.Fatalf("StopURL ::1 = %q", got)
	}
}

func TestRequestStopPostsAPIKey(t *testing.T) {
	var method string
	var apiKey string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		method = r.Method
		apiKey = r.Header.Get("X-API-Key")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})}

	err := RequestStopWithClient(context.Background(), client, "http://127.0.0.1:8000/api/v1/admin/stop", "secret-key")
	if err != nil {
		t.Fatalf("RequestStop returned error: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %q", method)
	}
	if apiKey != "secret-key" {
		t.Fatalf("api key = %q", apiKey)
	}
}

func TestRequestStopRejectsNonOK(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("nope")),
			Header:     make(http.Header),
		}, nil
	})}

	err := RequestStopWithClient(context.Background(), client, "http://127.0.0.1:8000/api/v1/admin/stop", "bad-key")
	if err == nil {
		t.Fatal("expected RequestStop to fail")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
