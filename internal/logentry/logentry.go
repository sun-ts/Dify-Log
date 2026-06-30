package logentry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaskValue = "***MASKED***"

type Request struct {
	ExecutionID  string         `json:"execution_id"`
	WorkflowID   string         `json:"workflow_id"`
	WorkflowName string         `json:"workflow_name"`
	AppID        string         `json:"app_id"`
	AppName      string         `json:"app_name"`
	NodeID       string         `json:"node_id"`
	NodeName     string         `json:"node_name"`
	NodeType     string         `json:"node_type"`
	SequenceNo   *int           `json:"sequence_no"`
	Status       string         `json:"status"`
	InputData    map[string]any `json:"input_data"`
	OutputData   map[string]any `json:"output_data"`
	ErrorMessage string         `json:"error_message"`
	ErrorDetail  string         `json:"error_detail"`
	StartedAt    *time.Time     `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at"`
	DurationMS   *int64         `json:"duration_ms"`
	Metadata     map[string]any `json:"metadata"`
}

type Record struct {
	ID           string
	ExecutionID  string
	WorkflowID   string
	WorkflowName string
	AppID        string
	AppName      string
	NodeID       string
	NodeName     string
	NodeType     string
	SequenceNo   *int
	Status       string
	InputJSON    string
	OutputJSON   string
	MetadataJSON string
	ErrorMessage string
	ErrorDetail  string
	StartedAt    time.Time
	FinishedAt   *time.Time
	DurationMS   *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func Parse(body []byte, maskFields []string, loc *time.Location) (Record, error) {
	var req Request
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return Record{}, fmt.Errorf("invalid JSON body: %w", err)
	}

	now := time.Now().In(loc)
	if strings.TrimSpace(req.NodeID) == "" {
		return Record{}, errors.New("node_id is required")
	}
	if strings.TrimSpace(req.NodeName) == "" {
		return Record{}, errors.New("node_name is required")
	}
	if req.ExecutionID == "" {
		req.ExecutionID = uuid.NewString()
	}
	if req.Status == "" {
		req.Status = "success"
	}
	if req.StartedAt == nil {
		req.StartedAt = &now
	}

	inputJSON, err := CompactJSON(MaskJSON(req.InputData, maskFields))
	if err != nil {
		return Record{}, fmt.Errorf("encode input_data: %w", err)
	}
	outputJSON, err := CompactJSON(MaskJSON(req.OutputData, maskFields))
	if err != nil {
		return Record{}, fmt.Errorf("encode output_data: %w", err)
	}
	metadataJSON, err := CompactJSON(MaskJSON(req.Metadata, maskFields))
	if err != nil {
		return Record{}, fmt.Errorf("encode metadata: %w", err)
	}

	return Record{
		ID:           uuid.NewString(),
		ExecutionID:  req.ExecutionID,
		WorkflowID:   req.WorkflowID,
		WorkflowName: req.WorkflowName,
		AppID:        req.AppID,
		AppName:      req.AppName,
		NodeID:       req.NodeID,
		NodeName:     req.NodeName,
		NodeType:     req.NodeType,
		SequenceNo:   req.SequenceNo,
		Status:       req.Status,
		InputJSON:    inputJSON,
		OutputJSON:   outputJSON,
		MetadataJSON: metadataJSON,
		ErrorMessage: req.ErrorMessage,
		ErrorDetail:  req.ErrorDetail,
		StartedAt:    req.StartedAt.In(loc),
		FinishedAt:   req.FinishedAt,
		DurationMS:   req.DurationMS,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func CompactJSON(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func MaskJSON(value any, fields []string) any {
	fieldSet := map[string]struct{}{}
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field != "" {
			fieldSet[field] = struct{}{}
		}
	}
	return maskValue(value, fieldSet)
}

func maskValue(value any, fields map[string]struct{}) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if _, ok := fields[strings.ToLower(key)]; ok {
				out[key] = MaskValue
				continue
			}
			out[key] = maskValue(item, fields)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = maskValue(item, fields)
		}
		return out
	default:
		return typed
	}
}
