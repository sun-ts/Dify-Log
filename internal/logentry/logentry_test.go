package logentry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseRequestGeneratesExecutionIDAndMasksFields(t *testing.T) {
	body := []byte(`{
		"workflow_id": "wf_1",
		"workflow_name": "客户线索分析",
		"node_id": "llm_1",
		"node_name": "摘要",
		"node_type": "llm",
		"sequence_no": 2,
		"status": "success",
		"input_data": {"password": "secret", "nested": {"token": "abc"}},
		"output_data": {"summary": "中文结果"},
		"metadata": {"model": "gpt", "phone": "13800000000"}
	}`)

	record, err := Parse(body, []string{"password", "token", "phone"}, time.FixedZone("CST", 8*3600))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if record.ExecutionID == "" {
		t.Fatal("ExecutionID was not generated")
	}
	if record.NodeID != "llm_1" {
		t.Fatalf("NodeID = %q", record.NodeID)
	}
	if !strings.Contains(record.InputJSON, `"password":"***MASKED***"`) {
		t.Fatalf("InputJSON not masked: %s", record.InputJSON)
	}
	if !strings.Contains(record.MetadataJSON, `"phone":"***MASKED***"`) {
		t.Fatalf("MetadataJSON not masked: %s", record.MetadataJSON)
	}
}

func TestParseDefaultsMissingNodeFields(t *testing.T) {
	record, err := Parse([]byte(`{"workflow_name":"前端调用","input_data":{"question":"hello"}}`), nil, time.UTC)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if record.NodeID != "http_request" {
		t.Fatalf("NodeID = %q", record.NodeID)
	}
	if record.NodeName != "HTTP Request" {
		t.Fatalf("NodeName = %q", record.NodeName)
	}
}

func TestParseAcceptsNonObjectDataFields(t *testing.T) {
	body := []byte(`{
		"node_id": "frontend",
		"node_name": "前端调用",
		"input_data": "plain text input",
		"output_data": ["item1", "item2"],
		"metadata": 123
	}`)

	record, err := Parse(body, []string{"password"}, time.UTC)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if record.InputJSON != `"plain text input"` {
		t.Fatalf("InputJSON = %s", record.InputJSON)
	}
	if record.OutputJSON != `["item1","item2"]` {
		t.Fatalf("OutputJSON = %s", record.OutputJSON)
	}
	if record.MetadataJSON != `123` {
		t.Fatalf("MetadataJSON = %s", record.MetadataJSON)
	}
}

func TestCompactJSONPreservesChinese(t *testing.T) {
	value := map[string]any{"summary": "中文结果"}
	data, err := CompactJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	if data != `{"summary":"中文结果"}` {
		t.Fatalf("CompactJSON = %s", data)
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(data), &decoded); err != nil {
		t.Fatal(err)
	}
}
