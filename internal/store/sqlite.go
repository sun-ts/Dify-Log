package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"dify-log-excel/internal/logentry"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type PendingLog struct {
	Record    logentry.Record
	ExcelDate string
	ExcelPath string
}

type Summary struct {
	ExecutionCount int
	NodeLogCount   int
	PendingCount   int
	LastSyncError  string
	LastSyncedAt   sql.NullTime
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Init(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS workflow_executions (
			id TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL UNIQUE,
			workflow_id TEXT,
			workflow_name TEXT,
			app_id TEXT,
			app_name TEXT,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			duration_ms INTEGER,
			metadata_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS node_logs (
			id TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL,
			workflow_id TEXT,
			workflow_name TEXT,
			app_id TEXT,
			app_name TEXT,
			node_id TEXT NOT NULL,
			node_name TEXT NOT NULL,
			node_type TEXT,
			sequence_no INTEGER,
			status TEXT NOT NULL,
			input_data_json TEXT NOT NULL,
			output_data_json TEXT NOT NULL,
			error_message TEXT,
			error_detail TEXT,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			duration_ms INTEGER,
			metadata_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS excel_sync_state (
			node_log_id TEXT PRIMARY KEY,
			sync_status TEXT NOT NULL,
			excel_date TEXT NOT NULL,
			excel_path TEXT NOT NULL,
			synced_at TEXT,
			last_error TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_node_logs_execution_id ON node_logs(execution_id)`,
		`CREATE INDEX IF NOT EXISTS idx_node_logs_created_at ON node_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_logs_workflow_created ON node_logs(workflow_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_logs_status_created ON node_logs(status, created_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) InsertLog(ctx context.Context, record logentry.Record, excelDate, excelPath string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `INSERT INTO workflow_executions (
		id, execution_id, workflow_id, workflow_name, app_id, app_name, status,
		started_at, finished_at, duration_ms, metadata_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(execution_id) DO UPDATE SET
		workflow_id=excluded.workflow_id,
		workflow_name=excluded.workflow_name,
		app_id=excluded.app_id,
		app_name=excluded.app_name,
		status=excluded.status,
		updated_at=excluded.updated_at`,
		record.ExecutionID, record.ExecutionID, record.WorkflowID, record.WorkflowName, record.AppID, record.AppName,
		record.Status, formatTime(record.StartedAt), formatOptionalTime(record.FinishedAt), nullableInt64(record.DurationMS),
		record.MetadataJSON, formatTime(record.CreatedAt), formatTime(record.UpdatedAt),
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO node_logs (
		id, execution_id, workflow_id, workflow_name, app_id, app_name,
		node_id, node_name, node_type, sequence_no, status,
		input_data_json, output_data_json, error_message, error_detail,
		started_at, finished_at, duration_ms, metadata_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.ExecutionID, record.WorkflowID, record.WorkflowName, record.AppID, record.AppName,
		record.NodeID, record.NodeName, record.NodeType, nullableInt(record.SequenceNo), record.Status,
		record.InputJSON, record.OutputJSON, record.ErrorMessage, record.ErrorDetail,
		formatTime(record.StartedAt), formatOptionalTime(record.FinishedAt), nullableInt64(record.DurationMS),
		record.MetadataJSON, formatTime(record.CreatedAt), formatTime(record.UpdatedAt),
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO excel_sync_state (
		node_log_id, sync_status, excel_date, excel_path, retry_count, updated_at
	) VALUES (?, 'pending', ?, ?, 0, ?)`,
		record.ID, excelDate, excelPath, formatTime(record.UpdatedAt),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) PendingLogs(ctx context.Context, limit int) ([]PendingLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		n.id, n.execution_id, n.workflow_id, n.workflow_name, n.app_id, n.app_name,
		n.node_id, n.node_name, n.node_type, n.sequence_no, n.status,
		n.input_data_json, n.output_data_json, n.error_message, n.error_detail,
		n.started_at, n.finished_at, n.duration_ms, n.metadata_json, n.created_at, n.updated_at,
		e.excel_date, e.excel_path
		FROM node_logs n
		JOIN excel_sync_state e ON e.node_log_id = n.id
		WHERE e.sync_status IN ('pending', 'failed')
		ORDER BY n.created_at ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PendingLog
	for rows.Next() {
		var item PendingLog
		var sequence sql.NullInt64
		var finished sql.NullString
		var duration sql.NullInt64
		var startedAt, createdAt, updatedAt string
		if err := rows.Scan(
			&item.Record.ID, &item.Record.ExecutionID, &item.Record.WorkflowID, &item.Record.WorkflowName, &item.Record.AppID, &item.Record.AppName,
			&item.Record.NodeID, &item.Record.NodeName, &item.Record.NodeType, &sequence, &item.Record.Status,
			&item.Record.InputJSON, &item.Record.OutputJSON, &item.Record.ErrorMessage, &item.Record.ErrorDetail,
			&startedAt, &finished, &duration, &item.Record.MetadataJSON, &createdAt, &updatedAt,
			&item.ExcelDate, &item.ExcelPath,
		); err != nil {
			return nil, err
		}
		if sequence.Valid {
			v := int(sequence.Int64)
			item.Record.SequenceNo = &v
		}
		if duration.Valid {
			v := duration.Int64
			item.Record.DurationMS = &v
		}
		item.Record.StartedAt = parseStoredTime(startedAt)
		item.Record.CreatedAt = parseStoredTime(createdAt)
		item.Record.UpdatedAt = parseStoredTime(updatedAt)
		if finished.Valid {
			v := parseStoredTime(finished.String)
			item.Record.FinishedAt = &v
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) MarkSynced(ctx context.Context, nodeLogID, excelPath string, syncedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE excel_sync_state
		SET sync_status='synced', excel_path=?, synced_at=?, last_error=NULL, updated_at=?
		WHERE node_log_id=?`, excelPath, formatTime(syncedAt), formatTime(syncedAt), nodeLogID)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, nodeLogID string, errText string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE excel_sync_state
		SET sync_status='failed', last_error=?, retry_count=retry_count+1, updated_at=?
		WHERE node_log_id=?`, errText, formatTime(updatedAt), nodeLogID)
	return err
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_executions`).Scan(&summary.ExecutionCount); err != nil {
		return Summary{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_logs`).Scan(&summary.NodeLogCount); err != nil {
		return Summary{}, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM excel_sync_state WHERE sync_status IN ('pending', 'failed')`).Scan(&summary.PendingCount); err != nil {
		return Summary{}, err
	}
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(last_error, '') FROM excel_sync_state WHERE last_error IS NOT NULL ORDER BY updated_at DESC LIMIT 1`).Scan(&summary.LastSyncError)
	var lastSynced sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT synced_at FROM excel_sync_state WHERE synced_at IS NOT NULL ORDER BY synced_at DESC LIMIT 1`).Scan(&lastSynced)
	if lastSynced.Valid {
		summary.LastSyncedAt = sql.NullTime{Time: parseStoredTime(lastSynced.String), Valid: true}
	}
	return summary, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func parseStoredTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func (s Summary) String() string {
	return fmt.Sprintf("executions=%d node_logs=%d pending=%d last_error=%s", s.ExecutionCount, s.NodeLogCount, s.PendingCount, s.LastSyncError)
}
