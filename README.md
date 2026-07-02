# Dify Log Excel

Portable Dify workflow log receiver. It runs as a local command-line tool, buffers logs in SQLite, writes daily Excel files under `data/excel/`, and writes application logs under `logs/`.

Chinese documentation: [README.zh-CN.md](README.zh-CN.md)

## Run

Download the package for your system and unzip it anywhere. The app always uses the folder that contains the executable as its home directory, no matter which directory your terminal is currently in.

Examples:

```bash
./dify-log-excel-macos-arm64/dify-log-excel start
/Users/bling/Tools/dify-log-excel-macos-arm64/dify-log-excel start
```

If your terminal is already inside the extracted folder:

```bash
./dify-log-excel start
```

Windows:

```bat
dify-log-excel-windows-amd64\dify-log-excel.exe start
dify-log-excel-windows-amd64\start.bat
```

macOS:

```bash
./start.command
```

Linux:

```bash
chmod +x start.sh dify-log-excel
./start.sh
```

## Dify HTTP Node

URL:

```http
POST http://127.0.0.1:8000/api/v1/logs
```

Headers:

```http
X-API-Key: dev-log-api-key
Content-Type: application/json
```

Body:

```json
{
  "execution_id": "{{execution_id}}",
  "workflow_id": "{{workflow_id}}",
  "workflow_name": "客户线索分析工作流",
  "node_id": "llm_summary_01",
  "node_name": "线索摘要生成",
  "node_type": "llm",
  "sequence_no": 3,
  "status": "success",
  "input_data": {
    "lead_text": "{{lead_text}}"
  },
  "output_data": {
    "summary": "{{llm_output}}"
  },
  "metadata": {
    "model": "gpt-4.1",
    "tokens": 1280
  }
}
```

## Files

- `config.toml`: local configuration.
- `data/dify_logs.db`: SQLite durable buffer.
- `data/excel/YYYY-MM-DD.xlsx`: daily Excel output.
- `logs/app-YYYY-MM-DD.log`: daily application log.
- `logs/dify-log-excel.pid`: background process PID file.

## Configuration

```toml
host = "127.0.0.1"
port = 8000
log_api_key = "dev-log-api-key"
data_dir = "./data"
excel_dir = "./data/excel"
log_enabled = true
log_level = "info"
log_dir = "./logs"
log_body = true
timezone = "Asia/Shanghai"
sync_interval_seconds = 5
mask_fields = ["password", "token", "api_key", "phone"]
```

`log_level` supports `error`, `info`, and `debug`. When `log_body = true`, the raw full HTTP request body is written to the application log without masking or truncation.

## Commands

```bash
./dify-log-excel start
./dify-log-excel stop
./dify-log-excel restart
./dify-log-excel status
./dify-log-excel serve
./dify-log-excel sync
./dify-log-excel version
```

`start` runs the receiver in the background and returns immediately. `serve` keeps the receiver in the foreground and is mainly useful for debugging.

`status` prints the data directory, Excel directory, log directory, current daily log file, log level, and background process state.

## Troubleshooting

If the API returns `422`, read the JSON `error` field in the response. The most common cause is an invalid JSON body. Browser callers should send `body: JSON.stringify(payload)`. Dify text variables must also expand to valid JSON.

`node_id` and `node_name` are optional in current builds. `input_data`, `output_data`, and `metadata` may be objects, arrays, strings, or numbers.

If an Excel file is open and cannot be written, the log stays in SQLite. Close Excel and run:

```bash
./dify-log-excel sync
```

If the port is already in use, edit `config.toml` and change `port`.

On macOS, the first run may require approving the app in System Settings because the first release is not code signed.
