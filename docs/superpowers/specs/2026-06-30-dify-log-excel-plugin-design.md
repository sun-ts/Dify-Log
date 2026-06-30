# Dify 便携式 Excel 日志外挂设计

日期：2026-06-30

## 1. 背景

当前项目已经实现为一个完整的 Web 日志系统：FastAPI 接收 Dify 工作流节点日志，PostgreSQL 持久化，Web 管理页查询，指标页分析，并支持 Excel 导出。这个形态适合长期平台化运维，但对“作为 Dify 应用旁边的外挂，把数据最终存入 Excel”来说偏重。

新的目标是将系统收缩为一个无需安装运行时的便携包。用户下载对应系统的压缩包，解压后直接运行二进制文件或双击启动脚本。Dify 工作流继续使用 HTTP 节点上报日志，程序先把日志可靠写入本地 SQLite，再增量同步到按日期生成的 Excel 文件。

SQLite 是可靠缓冲层，Excel 是面向人工查看、排查和归档的最终文件。Go 编译出的单文件程序负责 HTTP 接收、SQLite 读写、Excel 生成和命令行操作。

## 2. 产品形态

发布包按平台区分：

```text
dify-log-excel-macos-arm64.zip
dify-log-excel-macos-amd64.zip
dify-log-excel-linux-amd64.tar.gz
dify-log-excel-windows-amd64.zip
```

每个包解压后包含：

```text
dify-log-excel/
  dify-log-excel          # macOS/Linux
  dify-log-excel.exe      # Windows
  config.toml
  config.example.toml
  start.sh
  start.command
  start.bat
  data/
  logs/
  README.md
```

懂命令行的用户可以直接运行：

```bash
./dify-log-excel serve
```

普通用户可以使用启动脚本：

- Windows：双击 `start.bat`。
- macOS：双击 `start.command`。
- Ubuntu：在终端运行 `./start.sh`，支持文件管理器可执行脚本的环境也可以双击。

第一版不要求用户安装 Python、Node.js、Docker、PostgreSQL、SQLite 或其他运行时。

## 3. 目标

- 使用 Go 实现跨平台单文件可执行程序。
- 提供 `dify-log-excel` 命令，支持 macOS、Ubuntu 和 Windows。
- 同时支持命令行启动和双击脚本启动。
- 保持兼容当前 `POST /api/v1/logs` 接口和现有 JSON 字段，降低 Dify 工作流迁移成本。
- 使用 `X-API-Key` 做写入认证。
- 将请求数据先写入本地 SQLite，避免 Excel 文件被占用、写入失败或程序重启导致数据丢失。
- 按日期生成 Excel 文件，例如 `logs/2026-06-30.xlsx`。
- 每个 Excel 文件包含 `executions` 和 `node_logs` 两个 sheet。
- 支持手动同步、状态查看和后台定时同步。
- 保留字段脱敏能力，避免敏感字段直接落入 SQLite 和 Excel。

## 4. 非目标

第一版不实现以下能力：

- Web 管理页面。
- PostgreSQL、Alembic 迁移和 Docker Compose 部署。
- Python 运行时、虚拟环境或 pip 安装流程。
- Node.js、npm 或桌面 Electron 壳。
- 指标分析页面、复杂查询 API 和保留策略调度器。
- 原生 Dify 插件包形态。
- Windows 服务、macOS launchd 或 Linux systemd 常驻安装。
- 代码签名、公证或自动处理系统安全拦截提示。
- 多用户权限系统、多 API Key 和按工作流授权。
- Excel 中复杂图表、透视表或格式化报表。

这些能力可以作为后续增强；第一版重点是稳定、轻量、解压即用和跨平台。

## 5. 技术选型

- 语言：Go。
- HTTP：Go 标准库 `net/http`。
- SQLite：纯 Go SQLite 驱动 `modernc.org/sqlite`，避免用户安装系统 SQLite 或 CGO 运行依赖。
- Excel：`github.com/xuri/excelize/v2`。
- 配置：`config.toml`，使用 `github.com/pelletier/go-toml/v2` 编译进二进制。
- CLI：使用 Go 标准库解析 `serve`、`sync`、`status` 子命令，第一版不引入大型 CLI 框架。
- 测试：Go `testing` 包，必要时使用 `httptest`。

所有第三方库都在构建期打入可执行文件；最终用户只接触发布包，不需要安装依赖。

## 6. 总体架构

程序由五个核心模块组成：

- HTTP 接收器：基于 `net/http`，保留现有日志写入接口，负责认证、请求校验和响应。
- 本地存储层：使用 SQLite 保存执行记录、节点日志和 Excel 同步状态。
- Excel 同步器：定时扫描未同步记录，按日期追加到对应 `.xlsx` 文件，成功后更新同步状态。
- 命令行入口：提供 `serve`、`sync` 和 `status` 三个命令。
- 发布包脚本：提供 `start.bat`、`start.sh` 和 `start.command`，让非命令行用户可以直接启动。

运行后默认监听：

```http
POST http://127.0.0.1:8000/api/v1/logs
```

默认数据目录：

```text
data/dify_logs.db
logs/2026-06-30.xlsx
logs/2026-07-01.xlsx
```

## 7. 命令行设计

### 7.1 serve

```bash
dify-log-excel serve
```

启动 HTTP 接收器和后台 Excel 同步器。

启动流程：

1. 定位程序所在目录。
2. 读取同目录下的 `config.toml`；如果不存在，则从内置默认值启动并提示用户可以复制 `config.example.toml`。
3. 创建 `data` 和 `logs` 目录。
4. 初始化 SQLite 表结构。
5. 启动后台同步循环。
6. 启动 HTTP 服务。

退出流程：

1. 捕获 `Ctrl+C` 或进程终止信号。
2. 停止接收新请求。
3. 尽量执行一次最终同步。
4. 关闭 SQLite 连接并退出。

### 7.2 sync

```bash
dify-log-excel sync
```

手动扫描 SQLite 中未同步的日志并写入 Excel。该命令适合在 Excel 文件关闭后手动补同步。

### 7.3 status

```bash
dify-log-excel status
```

输出本地状态：

- workflow execution 总数。
- node log 总数。
- 未同步 node log 数。
- 最近一次同步时间。
- 最近一次同步错误。
- 当前数据目录和 Excel 目录。
- HTTP 监听地址。

## 8. 双击启动脚本

### 8.1 Windows start.bat

`start.bat` 进入脚本所在目录后运行：

```bat
dify-log-excel.exe serve
pause
```

`pause` 用于保留窗口，让用户能看到监听地址或错误信息。

### 8.2 macOS start.command

`start.command` 进入脚本所在目录后运行：

```bash
./dify-log-excel serve
```

发布包中应保留可执行权限。首次运行时，如果系统提示安全限制，README 说明用户如何在系统设置中允许运行。

### 8.3 Ubuntu start.sh

`start.sh` 进入脚本所在目录后运行：

```bash
./dify-log-excel serve
```

README 同时提供终端命令：

```bash
chmod +x start.sh dify-log-excel
./start.sh
```

## 9. 配置设计

`config.toml` 放在程序同目录：

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

配置规则：

- 相对路径以程序所在目录为基准，而不是用户当前终端目录。
- `config.example.toml` 与默认配置保持一致。
- `host` 默认只监听本机，减少误暴露风险。
- `port` 冲突时启动失败并输出清晰错误。
- `log_api_key` 为空字符串时拒绝启动，避免无认证写入。

## 10. API 兼容设计

第一版保持当前写入接口：

```http
POST /api/v1/logs
X-API-Key: <LOG_API_KEY>
Content-Type: application/json
```

请求体兼容当前字段：

- `execution_id`
- `workflow_id`
- `workflow_name`
- `app_id`
- `app_name`
- `node_id`
- `node_name`
- `node_type`
- `sequence_no`
- `status`
- `input_data`
- `output_data`
- `error_message`
- `error_detail`
- `started_at`
- `finished_at`
- `duration_ms`
- `metadata`

响应格式保持兼容：

```json
{
  "execution_id": "example-execution-id",
  "log_id": "example-log-id",
  "status": "success"
}
```

如果请求未传 `execution_id`，程序生成 UUID 字符串并返回。

## 11. SQLite 数据模型

### 11.1 workflow_executions

记录一次工作流执行的聚合信息。

核心字段：

- `id`：文本 UUID 主键。
- `execution_id`：对外追踪 ID，唯一。
- `workflow_id`
- `workflow_name`
- `app_id`
- `app_name`
- `status`
- `started_at`
- `finished_at`
- `duration_ms`
- `metadata_json`
- `created_at`
- `updated_at`

### 11.2 node_logs

记录每次节点日志。

核心字段：

- `id`：文本 UUID 主键。
- `execution_id`
- `workflow_id`
- `workflow_name`
- `app_id`
- `app_name`
- `node_id`
- `node_name`
- `node_type`
- `sequence_no`
- `status`
- `input_data_json`
- `output_data_json`
- `error_message`
- `error_detail`
- `started_at`
- `finished_at`
- `duration_ms`
- `metadata_json`
- `created_at`
- `updated_at`

索引：

- `execution_id`
- `created_at`
- `workflow_id, created_at`
- `status, created_at`

### 11.3 excel_sync_state

记录每条节点日志的 Excel 同步状态。

核心字段：

- `node_log_id`：关联 `node_logs.id`，唯一。
- `sync_status`：`pending`、`synced`、`failed`。
- `excel_date`：写入的日期，例如 `2026-06-30`。
- `excel_path`：目标 Excel 文件路径。
- `synced_at`
- `last_error`
- `retry_count`
- `updated_at`

同步成功后状态为 `synced`。同步失败时保留可重试状态，并记录错误。

## 12. Excel 文件设计

Excel 文件按配置时区中的日志创建日期生成：

```text
logs/YYYY-MM-DD.xlsx
```

每个文件包含两个 sheet。

### 12.1 executions

列：

- `execution_id`
- `workflow_id`
- `workflow_name`
- `app_id`
- `app_name`
- `status`
- `started_at`
- `finished_at`
- `duration_ms`
- `node_count`
- `failed_node_count`
- `created_at`
- `updated_at`

同步器每次写入节点日志后，更新或重建当天文件中的 `executions` sheet，使其反映当天相关执行的聚合状态。

### 12.2 node_logs

列：

- `log_id`
- `execution_id`
- `sequence_no`
- `workflow_id`
- `workflow_name`
- `app_id`
- `app_name`
- `node_id`
- `node_name`
- `node_type`
- `status`
- `started_at`
- `finished_at`
- `duration_ms`
- `error_message`
- `error_detail`
- `input_data`
- `output_data`
- `metadata`
- `created_at`

JSON 字段使用紧凑 JSON 字符串写入单元格，保留中文字符，便于直接查看。

## 13. 数据流

### 13.1 写入流程

1. Dify HTTP 节点调用 `POST /api/v1/logs`。
2. 接收器校验 `X-API-Key`。
3. 解析 JSON 请求体，校验必填字段和字段类型。
4. 填充默认时间和状态。
5. 对 `input_data`、`output_data` 和 `metadata` 做递归字段脱敏。
6. 在 SQLite 中 upsert `workflow_executions`。
7. 插入 `node_logs`。
8. 插入对应 `excel_sync_state`，状态为 `pending`。
9. 返回 `execution_id`、`log_id` 和 `status`。

### 13.2 同步流程

1. 同步器按 `sync_interval_seconds` 扫描 pending 或 failed 记录。
2. 按 `excel_date` 分组。
3. 打开或创建对应 Excel 文件。
4. 追加前读取 `node_logs` sheet 中已有的 `log_id`。
5. 跳过已存在行，避免重复写入。
6. 追加未同步的 `node_logs` 行。
7. 更新或重建 `executions` sheet。
8. 保存 Excel 文件。
9. 成功后将对应状态更新为 `synced`。
10. 失败后记录 `last_error` 和 `retry_count`，等待下轮同步。

## 14. 异常处理

- Excel 文件被打开或锁定：SQLite 数据保持 pending 或 failed，接口仍返回成功，因为数据已经可靠落地。
- 请求字段缺失或格式错误：返回 `422`，不写入 SQLite。
- API Key 错误：返回 `401`。
- SQLite 写入失败：返回 `500`，因为数据没有可靠落地。
- Excel 同步失败：不影响 HTTP 写入；下一轮或手动 `sync` 会重试。
- 端口被占用：`serve` 启动失败并输出监听地址和端口冲突提示。
- 配置文件无法解析：启动失败并输出配置文件路径和解析错误。
- 程序退出：停止新请求并尽量执行一次最终同步。
- 重复请求：第一版允许重复写入，保持与当前行为一致。后续如 Dify 请求能提供稳定 `request_id`，再增加幂等约束。

## 15. 脱敏规则

程序沿用当前递归字段脱敏思路：

- 将 `mask_fields` 解析为小写字段名集合。
- 遍历 JSON object、array 和嵌套结构。
- 当字段名小写后命中集合时，将值替换为固定掩码字符串。
- 脱敏发生在写入 SQLite 之前，因此 SQLite 和 Excel 都不会保存原始敏感值。

## 16. 跨平台要求

- 开发和构建使用 Go 1.22 或更高版本；发布包中不包含 Go 工具链。
- 文件路径使用 Go `filepath`，相对路径以程序所在目录为基准。
- SQLite 使用纯 Go 驱动，发布包不依赖系统 SQLite。
- Excel 写入使用 `excelize`。
- HTTP 服务使用标准库，减少运行时依赖。
- 不依赖 shell 专属语法、系统服务管理器或平台专属文件锁。
- Windows 上 Excel 文件被打开时，写入失败按可重试同步错误处理。
- 发布包中的脚本使用平台原生命令，避免要求用户安装 Bash、PowerShell 扩展或其他工具。

## 17. 发布包构建

第一版发布四类包：

- macOS arm64。
- macOS amd64。
- Linux amd64。
- Windows amd64。

每个包都包含：

- 对应平台的可执行文件。
- `config.toml`。
- `config.example.toml`。
- 对应平台可用的启动脚本。
- `data/.gitkeep` 或空目录。
- `logs/.gitkeep` 或空目录。
- README。

构建脚本负责：

1. 运行测试。
2. 为目标平台构建二进制。
3. 复制配置、脚本和 README。
4. 生成 zip 或 tar.gz。
5. 输出包文件名和校验信息。

## 18. 从当前实现迁移

保留：

- `POST /api/v1/logs` 的请求和响应契约。
- 字段脱敏逻辑。
- Excel 生成能力中的字段选择和 JSON 序列化方式。
- 测试客户端覆盖接口行为的思路。

替换：

- Python/FastAPI 实现替换为 Go `net/http`。
- PostgreSQL 存储替换为本地 SQLite。
- Excel 导出接口替换为后台同步器和手动 `sync` 命令。
- Web 页面、查询 API、指标 API、Docker Compose 和 Alembic 从第一版运行路径中移除。

包命名从平台化的 `dify-workflow-log-system` 调整为更贴近目标的 `dify-log-excel`。Go module 和命令名都使用 `dify-log-excel`。

## 19. 测试策略

测试覆盖以下行为：

- `POST /api/v1/logs` 接受当前字段并返回兼容响应。
- 缺少或错误 API Key 返回 `401`。
- 格式错误请求返回 `422`。
- 成功请求写入 SQLite 的 `workflow_executions`、`node_logs` 和 `excel_sync_state`。
- 同步器生成 `logs/YYYY-MM-DD.xlsx`。
- `node_logs` sheet 写入完整节点日志。
- `executions` sheet 反映当天执行聚合信息。
- 已同步记录不会重复追加。
- Excel 保存失败时同步状态保留为可重试，并记录错误。
- `dify-log-excel status` 输出总数、未同步数和最后错误。
- 相对路径基于程序目录解析，而不是当前工作目录。
- 构建脚本能生成目标平台发布包。

## 20. 第一版完成标准

第一版完成后，用户可以：

1. 下载对应系统的压缩包。
2. 解压包，不安装任何语言运行时或数据库。
3. 命令行执行 `dify-log-excel serve`，或双击平台启动脚本。
4. 在 Dify HTTP 节点中继续调用 `POST /api/v1/logs`。
5. 在 `data/dify_logs.db` 中获得可靠本地缓冲。
6. 在 `logs/YYYY-MM-DD.xlsx` 中看到每日节点日志和执行汇总。
7. 当 Excel 文件被占用导致同步失败后，关闭文件并执行 `dify-log-excel sync` 补同步。
