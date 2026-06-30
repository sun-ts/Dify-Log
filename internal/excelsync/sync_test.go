package excelsync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dify-log-excel/internal/logentry"
	"dify-log-excel/internal/store"

	"github.com/xuri/excelize/v2"
)

func TestSyncPendingCreatesWorkbookAndDoesNotDuplicateRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Init(ctx); err != nil {
		t.Fatal(err)
	}

	excelPath := filepath.Join(dir, "logs", "2026-06-30.xlsx")
	record := sampleRecord("log-1", "exec-1")
	if err := st.InsertLog(ctx, record, "2026-06-30", excelPath); err != nil {
		t.Fatal(err)
	}

	result, err := SyncPending(ctx, st, 100)
	if err != nil {
		t.Fatalf("SyncPending returned error: %v", err)
	}
	if result.Synced != 1 {
		t.Fatalf("Synced = %d", result.Synced)
	}

	result, err = SyncPending(ctx, st, 100)
	if err != nil {
		t.Fatalf("second SyncPending returned error: %v", err)
	}
	if result.Synced != 0 {
		t.Fatalf("second Synced = %d", result.Synced)
	}

	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	rows, err := f.GetRows("node_logs")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("node_logs row count = %d rows=%#v", len(rows), rows)
	}
	if rows[1][0] != "log-1" || rows[1][1] != "exec-1" {
		t.Fatalf("node row = %#v", rows[1])
	}

	executionRows, err := f.GetRows("executions")
	if err != nil {
		t.Fatal(err)
	}
	if len(executionRows) != 2 {
		t.Fatalf("executions row count = %d", len(executionRows))
	}
}

func sampleRecord(id, executionID string) logentry.Record {
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	duration := int64(123)
	sequence := 1
	return logentry.Record{
		ID:           id,
		ExecutionID:  executionID,
		WorkflowID:   "wf",
		WorkflowName: "workflow",
		AppID:        "app",
		AppName:      "application",
		NodeID:       "node",
		NodeName:     "node name",
		NodeType:     "llm",
		SequenceNo:   &sequence,
		Status:       "success",
		InputJSON:    `{"input":"value"}`,
		OutputJSON:   `{"output":"value"}`,
		MetadataJSON: `{}`,
		StartedAt:    now,
		FinishedAt:   &now,
		DurationMS:   &duration,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
