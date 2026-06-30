# Dify Log Excel

Portable Dify workflow log receiver. It runs as a local command-line tool, buffers logs in SQLite, and writes daily Excel files under `logs/`.

Chinese documentation: [README.zh-CN.md](README.zh-CN.md)

## Run

Download the package for your system and unzip it anywhere. The app always uses the folder that contains the executable as its home directory, no matter which directory your terminal is currently in.

Examples:

```bash
./dify-log-excel-macos-arm64/dify-log-excel serve
/Users/bling/Tools/dify-log-excel-macos-arm64/dify-log-excel serve
```

If your terminal is already inside the extracted folder:

```bash
./dify-log-excel serve
```

Windows:

```bat
dify-log-excel-windows-amd64\dify-log-excel.exe serve
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
- `logs/YYYY-MM-DD.xlsx`: daily Excel output.

## Commands

```bash
./dify-log-excel serve
./dify-log-excel sync
./dify-log-excel status
./dify-log-excel version
```

## Troubleshooting

If an Excel file is open and cannot be written, the log stays in SQLite. Close Excel and run:

```bash
./dify-log-excel sync
```

If the port is already in use, edit `config.toml` and change `port`.

On macOS, the first run may require approving the app in System Settings because the first release is not code signed.
