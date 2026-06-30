package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusCommandInitializesStoreAndPrintsSummary(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"status"}, base, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "node_logs=0") {
		t.Fatalf("stdout = %s", out.String())
	}
	if !strings.Contains(out.String(), filepath.Join(base, "logs")) {
		t.Fatalf("stdout missing logs dir: %s", out.String())
	}
}

func TestUnknownCommandReturnsUsage(t *testing.T) {
	base := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	code := Run([]string{"unknown"}, base, &out, &errOut)
	if code != 2 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(errOut.String(), "usage: dify-log-excel") {
		t.Fatalf("stderr = %s", errOut.String())
	}
}
