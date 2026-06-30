package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dify-log-excel/internal/logentry"
)

func TestInsertLogCreatesExecutionNodeAndSyncState(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	record := sampleRecord("log-1", "exec-1", "success")

	if err := st.InsertLog(ctx, record, "2026-06-30", filepath.Join(t.TempDir(), "2026-06-30.xlsx")); err != nil {
		t.Fatalf("InsertLog returned error: %v", err)
	}

	summary, err := st.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ExecutionCount != 1 || summary.NodeLogCount != 1 || summary.PendingCount != 1 {
		t.Fatalf("summary = %#v", summary)
	}

	pending, err := st.PendingLogs(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Record.ID != "log-1" {
		t.Fatalf("pending = %#v", pending)
	}
}

func TestMarkSyncedPreventsPendingReplay(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	record := sampleRecord("log-2", "exec-2", "failed")

	if err := st.InsertLog(ctx, record, "2026-06-30", "logs/2026-06-30.xlsx"); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkSynced(ctx, "log-2", "logs/2026-06-30.xlsx", time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	pending, err := st.PendingLogs(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %#v", pending)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "dify_logs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return st
}

func sampleRecord(id, executionID, status string) logentry.Record {
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	return logentry.Record{
		ID:           id,
		ExecutionID:  executionID,
		WorkflowID:   "wf",
		WorkflowName: "workflow",
		NodeID:       "node",
		NodeName:     "node name",
		Status:       status,
		InputJSON:    `{"input":"value"}`,
		OutputJSON:   `{"output":"value"}`,
		MetadataJSON: `{}`,
		StartedAt:    now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
