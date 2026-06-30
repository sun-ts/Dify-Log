# Dify Log Excel

Dify 工作流日志接收工具。它是一个免安装的本地命令行程序，可以接收 Dify HTTP 节点发送过来的日志，先写入本地 SQLite 缓冲，再同步生成每日 Excel 文件。

## 特点

- 不依赖 Python、Node、数据库服务或其他运行时，解压后直接运行。
- 支持 macOS、Linux、Windows，并提供 Linux ARM64 包。
- 程序始终以可执行文件所在目录作为工作目录，不受当前终端目录影响。
- 日志先落 SQLite，再写 Excel；Excel 被打开导致写入失败时，数据不会丢。
- 支持按字段名脱敏，例如 `password`、`token`、`api_key`、`phone`。

## 下载和运行

从 `dist` 目录选择对应系统的包，解压到任意位置即可使用。比如可以放在桌面、下载目录、工具目录或外接盘里，不需要固定路径。

macOS Apple Silicon:

```bash
./dify-log-excel-macos-arm64/dify-log-excel serve
```

macOS Intel:

```bash
./dify-log-excel-macos-amd64/dify-log-excel serve
```

Linux x86_64:

```bash
./dify-log-excel-linux-amd64/dify-log-excel serve
```

Linux ARM64:

```bash
./dify-log-excel-linux-arm64/dify-log-excel serve
```

Windows:

```bat
dify-log-excel-windows-amd64\dify-log-excel.exe serve
```

如果终端已经进入了解压后的目录，也可以这样运行：

```bash
./dify-log-excel serve
```

Windows 进入解压目录后：

```bat
dify-log-excel.exe serve
```

也可以使用包内的启动脚本：

```bash
./start.command
./start.sh
```

Windows:

```bat
start.bat
```

## 工作目录说明

程序会动态获取自身可执行文件所在目录，并把这个目录作为应用主目录。

例如你从 `/tmp` 运行：

```bash
/Users/bling/Tools/dify-log-excel-macos-arm64/dify-log-excel status
```

输出里的路径仍然会指向：

```text
/Users/bling/Tools/dify-log-excel-macos-arm64/data
/Users/bling/Tools/dify-log-excel-macos-arm64/logs
```

所以用户不需要按某个固定目录摆放，也不需要每次改配置里的绝对路径。

## Dify HTTP 节点配置

在 Dify 工作流里添加 HTTP 请求节点。

请求地址：

```http
POST http://127.0.0.1:8000/api/v1/logs
```

请求头：

```http
X-API-Key: dev-log-api-key
Content-Type: application/json
```

请求体示例：

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

必填字段：

- `node_id`
- `node_name`

常用字段：

- `execution_id`: 同一次工作流运行的唯一标识；不传时程序会自动生成。
- `workflow_id` / `workflow_name`: 工作流标识和名称。
- `app_id` / `app_name`: Dify 应用标识和名称。
- `node_type`: 节点类型，例如 `llm`、`http-request`、`code`。
- `sequence_no`: 节点顺序。
- `status`: 节点状态，不传时默认为 `success`。
- `input_data`: 节点输入，JSON 对象。
- `output_data`: 节点输出，JSON 对象。
- `metadata`: 额外信息，JSON 对象。
- `error_message` / `error_detail`: 失败信息。
- `started_at` / `finished_at`: 开始和结束时间。
- `duration_ms`: 耗时，单位毫秒。

## 输出文件

默认文件位置：

```text
data/dify_logs.db
logs/YYYY-MM-DD.xlsx
```

Excel 文件里包含两个工作表：

- `node_logs`: 每条节点日志明细。
- `executions`: 按 `execution_id` 汇总后的运行记录。

## 配置

配置文件是包内的 `config.toml`：

```toml
host = "127.0.0.1"
port = 8000
log_api_key = "dev-log-api-key"
data_dir = "./data"
excel_dir = "./logs"
timezone = "Asia/Shanghai"
sync_interval_seconds = 5
mask_fields = ["password", "token", "api_key", "phone"]
```

说明：

- `host` / `port`: 本地 HTTP 服务监听地址。
- `log_api_key`: Dify HTTP 节点请求时使用的密钥，对应请求头 `X-API-Key`。
- `data_dir`: SQLite 缓冲目录，相对路径会基于程序所在目录解析。
- `excel_dir`: Excel 输出目录，相对路径会基于程序所在目录解析。
- `timezone`: 日志日期和默认时间使用的时区。
- `sync_interval_seconds`: 自动同步 Excel 的间隔秒数。
- `mask_fields`: 需要脱敏的字段名，匹配时不区分大小写。

## 命令

```bash
./dify-log-excel serve
./dify-log-excel sync
./dify-log-excel status
./dify-log-excel version
```

命令说明：

- `serve`: 启动本地 HTTP 接收服务，并定时同步 Excel。
- `sync`: 手动把未同步日志写入 Excel。
- `status`: 查看日志数量、待同步数量、数据目录、Excel 目录和监听地址。
- `version`: 查看版本。

## 常见问题

如果 Excel 文件正在被打开，程序可能无法写入。此时日志会继续保存在 SQLite 里，不会丢失。关闭 Excel 后执行：

```bash
./dify-log-excel sync
```

如果端口被占用，修改 `config.toml` 里的 `port`。

如果 macOS 第一次运行提示无法打开，这是因为当前包没有做代码签名。可以在系统设置的安全性里允许运行，或者在终端执行：

```bash
xattr -dr com.apple.quarantine ./dify-log-excel
chmod +x ./dify-log-excel
```

如果 Linux 提示没有执行权限：

```bash
chmod +x ./dify-log-excel ./start.sh
```

## 重新打包

开发环境里可以执行：

```bash
GO_BIN=/path/to/go ./scripts/package.sh
```

打包脚本会生成：

```text
dist/dify-log-excel-macos-arm64.zip
dist/dify-log-excel-macos-amd64.zip
dist/dify-log-excel-linux-amd64.tar.gz
dist/dify-log-excel-linux-arm64.tar.gz
dist/dify-log-excel-windows-amd64.zip
```

每次打包都会校验包结构，防止把可执行文件又放进嵌套目录里。
