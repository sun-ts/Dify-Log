package excelsync

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"dify-log-excel/internal/logentry"
	"dify-log-excel/internal/store"

	"github.com/xuri/excelize/v2"
)

var nodeHeaders = []string{"log_id", "execution_id", "sequence_no", "workflow_id", "workflow_name", "app_id", "app_name", "node_id", "node_name", "node_type", "status", "started_at", "finished_at", "duration_ms", "error_message", "error_detail", "input_data", "output_data", "metadata", "created_at"}
var executionHeaders = []string{"execution_id", "workflow_id", "workflow_name", "app_id", "app_name", "status", "started_at", "finished_at", "duration_ms", "node_count", "failed_node_count", "created_at", "updated_at"}

type Result struct {
	Synced int
	Failed int
}

func SyncPending(ctx context.Context, st *store.Store, limit int) (Result, error) {
	pending, err := st.PendingLogs(ctx, limit)
	if err != nil {
		return Result{}, err
	}
	result := Result{}
	byPath := map[string][]store.PendingLog{}
	for _, item := range pending {
		byPath[item.ExcelPath] = append(byPath[item.ExcelPath], item)
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		items := byPath[path]
		written, err := writeWorkbook(path, items)
		now := time.Now().UTC()
		if err != nil {
			for _, item := range items {
				_ = st.MarkFailed(ctx, item.Record.ID, err.Error(), now)
				result.Failed++
			}
			continue
		}
		for _, item := range written {
			if err := st.MarkSynced(ctx, item.Record.ID, path, now); err != nil {
				return result, err
			}
			result.Synced++
		}
	}
	return result, nil
}

func writeWorkbook(path string, items []store.PendingLog) ([]store.PendingLog, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := openOrCreate(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	existing, err := existingLogIDs(f)
	if err != nil {
		return nil, err
	}

	var written []store.PendingLog
	for _, item := range items {
		if existing[item.Record.ID] {
			written = append(written, item)
			continue
		}
		row := nodeRow(item.Record)
		next := nextRow(f, "node_logs")
		if err := setRow(f, "node_logs", next, row); err != nil {
			return nil, err
		}
		existing[item.Record.ID] = true
		written = append(written, item)
	}
	if err := rebuildExecutions(f); err != nil {
		return nil, err
	}
	if err := f.SaveAs(path); err != nil {
		return nil, err
	}
	return written, nil
}

func openOrCreate(path string) (*excelize.File, error) {
	if _, err := os.Stat(path); err == nil {
		return excelize.OpenFile(path)
	}
	f := excelize.NewFile()
	defaultSheet := f.GetSheetName(0)
	if err := f.SetSheetName(defaultSheet, "node_logs"); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet("executions"); err != nil {
		return nil, err
	}
	if err := setRow(f, "node_logs", 1, nodeHeaders); err != nil {
		return nil, err
	}
	if err := setRow(f, "executions", 1, executionHeaders); err != nil {
		return nil, err
	}
	return f, nil
}

func existingLogIDs(f *excelize.File) (map[string]bool, error) {
	rows, err := f.GetRows("node_logs")
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for i, row := range rows {
		if i == 0 || len(row) == 0 {
			continue
		}
		result[row[0]] = true
	}
	return result, nil
}

func rebuildExecutions(f *excelize.File) error {
	rows, err := f.GetRows("node_logs")
	if err != nil {
		return err
	}
	type agg struct {
		first     []string
		count     int
		failures  int
		createdAt string
		updatedAt string
	}
	aggregates := map[string]*agg{}
	for i, row := range rows {
		if i == 0 || len(row) < len(nodeHeaders) {
			continue
		}
		executionID := row[1]
		item := aggregates[executionID]
		if item == nil {
			item = &agg{first: row, createdAt: row[19], updatedAt: row[19]}
			aggregates[executionID] = item
		}
		item.count++
		if row[10] == "failed" {
			item.failures++
		}
		if row[19] > item.updatedAt {
			item.updatedAt = row[19]
		}
	}
	index, err := f.GetSheetIndex("executions")
	if err == nil && index != -1 {
		f.DeleteSheet("executions")
	}
	if _, err := f.NewSheet("executions"); err != nil {
		return err
	}
	if err := setRow(f, "executions", 1, executionHeaders); err != nil {
		return err
	}

	executionIDs := make([]string, 0, len(aggregates))
	for executionID := range aggregates {
		executionIDs = append(executionIDs, executionID)
	}
	sort.Strings(executionIDs)

	rowNum := 2
	for _, executionID := range executionIDs {
		item := aggregates[executionID]
		row := []string{
			executionID, item.first[3], item.first[4], item.first[5], item.first[6], item.first[10],
			item.first[11], item.first[12], item.first[13],
			strconv.Itoa(item.count), strconv.Itoa(item.failures), item.createdAt, item.updatedAt,
		}
		if err := setRow(f, "executions", rowNum, row); err != nil {
			return err
		}
		rowNum++
	}
	return nil
}

func nodeRow(record logentry.Record) []string {
	return []string{
		record.ID, record.ExecutionID, intPtr(record.SequenceNo), record.WorkflowID, record.WorkflowName, record.AppID, record.AppName,
		record.NodeID, record.NodeName, record.NodeType, record.Status,
		record.StartedAt.Format(time.RFC3339Nano), timePtr(record.FinishedAt), int64Ptr(record.DurationMS),
		record.ErrorMessage, record.ErrorDetail, record.InputJSON, record.OutputJSON, record.MetadataJSON,
		record.CreatedAt.Format(time.RFC3339Nano),
	}
}

func setRow(f *excelize.File, sheet string, row int, values []string) error {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return err
	}
	interfaces := make([]interface{}, len(values))
	for i, value := range values {
		interfaces[i] = value
	}
	return f.SetSheetRow(sheet, cell, &interfaces)
}

func nextRow(f *excelize.File, sheet string) int {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return 1
	}
	return len(rows) + 1
}

func intPtr(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func int64Ptr(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func timePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
