# Dify 本地 Excel 日志外挂设计

日期：2026-06-30

## 1. 背景

当前项目已经实现为一个完整的 Web 日志系统：FastAPI 接收 Dify 工作流节点日志，PostgreSQL 持久化，Web 管理页查询，指标页分析，并支持 Excel 导出。这个形态适合长期平台化运维，但对“作为 Dify 应用旁边的轻量外挂，把数据最终存入 Excel”来说偏重。

新的目标是将系统收缩为跨平台本地程序：用户在 macOS、Ubuntu 或 Windows 上通过命令行启动一个轻量 HTTP 接收器，Dify 工作流继续使用 HTTP 节点上报日志。程序先把日志可靠写入本地 SQLite，再增量同步到按日期生成的 Excel 文件。

SQLite 是可靠缓冲层，Excel 是面向人工查看、排查和归档的最终文件。

## 2. 目标

- 提供命令行程序 `dify-log-excel`，支持在 macOS、Ubuntu 和 Windows 上运行。
- 保持兼容当前 `POST /api/v1/logs` 接口和现有 JSON 字段，降低 Dify 工作流迁移成本。
- 使用 `X-API-Key` 做写入认证。
- 将请求数据先写入本地 SQLite，避免 Excel 文件被占用、写入失败或程序重启导致数据丢失。
- 按日期生成 Excel 文件，例如 `logs/2026-06-30.xlsx`。
- 每个 Excel 文件包含 `executions` 和 `node_logs` 两个 sheet。
- 支持手动同步、状态查看和后台定时同步。
- 保留字段脱敏能力，避免敏感字段直接落入 SQLite 和 Excel。

## 3. 非目标

第一版不实现以下能力：

- Web 管理页面。
- PostgreSQL、Alembic 迁移和 Docker Compose 部署。
- 指标分析页面、复杂查询 API 和保留策略调度器。
- 原生 Dify 插件包形态。
- Windows 服务、macOS launchd 或 Linux systemd 常驻安装。
- 多用户权限系统、多 API Key 和按工作流授权。
- Excel 中复杂图表、透视表或格式化报表。

这些能力可以作为后续增强；第一版重点是稳定、轻量、跨平台和易接入。

## 4. 总体架构

程序由四个核心模块组成：

- HTTP 接收器：基于 FastAPI，保留现有日志写入接口，负责认证、请求校验和响应。
- 本地存储层：使用 SQLite 保存执行记录、节点日志和 Excel 同步状态。
- Excel 同步器：定时扫描未同步记录，按日期追加到对应 `.xlsx` 文件，成功后更新同步状态。
- 命令行入口：提供 `serve`、`sync` 和 `status` 三个命令。

运行方式：

```bash
dify-log-excel serve
```

默认监听：

```http
POST http://127.0.0.1:8000/api/v1/logs
```

默认目录结构：

```text
data/dify_logs.db
logs/2026-06-30.xlsx
logs/2026-07-01.xlsx
```

## 5. 命令行设计

### 5.1 serve

```bash
dify-log-excel serve
```

启动 HTTP 接收器和后台 Excel 同步器。

启动流程：

1. 加载配置。
2. 创建 `DATA_DIR` 和 `EXCEL_DIR`。
3. 初始化 SQLite 表结构。
4. 启动后台同步线程或异步任务。
5. 启动 FastAPI HTTP 服务。

退出流程：

1. 捕获 `Ctrl+C` 或进程终止信号。
2. 停止接收新请求。
3. 尽量执行一次最终同步。
4. 关闭 SQLite 连接并退出。

### 5.2 sync

```bash
dify-log-excel sync
```

手动扫描 SQLite 中未同步的日志并写入 Excel。该命令适合在 Excel 文件关闭后手动补同步。

### 5.3 status

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

## 6. 配置设计

配置优先从环境变量和 `.env` 文件读取，命令行参数可覆盖关键项。

默认配置：

```text
HOST=127.0.0.1
PORT=8000
LOG_API_KEY=dev-log-api-key
DATA_DIR=./data
EXCEL_DIR=./logs
APP_TIMEZONE=Asia/Shanghai
SYNC_INTERVAL_SECONDS=5
MASK_FIELDS=password,token,api_key,phone
```

路径处理使用 Python `pathlib`，不依赖操作系统专属目录格式。

## 7. API 兼容设计

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

响应格式也保持兼容：

```json
{
  "execution_id": "example-execution-id",
  "log_id": "example-log-id",
  "status": "success"
}
```

如果请求未传 `execution_id`，程序生成 UUID 字符串并返回。

## 8. SQLite 数据模型

### 8.1 workflow_executions

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

### 8.2 node_logs

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

### 8.3 excel_sync_state

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

## 9. Excel 文件设计

Excel 文件按应用时区中的日志创建日期生成：

```text
logs/YYYY-MM-DD.xlsx
```

每个文件包含两个 sheet。

### 9.1 executions

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

### 9.2 node_logs

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

JSON 字段使用 `json.dumps(..., ensure_ascii=False)` 写入单元格，便于中文内容直接查看。

## 10. 数据流

### 10.1 写入流程

1. Dify HTTP 节点调用 `POST /api/v1/logs`。
2. 接收器校验 `X-API-Key`。
3. Pydantic 校验请求体，填充默认时间和状态。
4. 对 `input_data`、`output_data` 和 `metadata` 做递归字段脱敏。
5. 在 SQLite 中 upsert `workflow_executions`。
6. 插入 `node_logs`。
7. 插入对应 `excel_sync_state`，状态为 `pending`。
8. 返回 `execution_id`、`log_id` 和 `status`。

### 10.2 同步流程

1. 同步器按 `SYNC_INTERVAL_SECONDS` 扫描 pending 或 failed 记录。
2. 按 `excel_date` 分组。
3. 打开或创建对应 Excel 文件。
4. 追加未同步的 `node_logs` 行。
5. 追加前读取 `node_logs` sheet 中已有的 `log_id`，跳过已存在行，避免重复写入。
6. 更新或重建 `executions` sheet。
7. 保存 Excel 文件。
8. 成功后将对应状态更新为 `synced`。
9. 失败后记录 `last_error` 和 `retry_count`，等待下轮同步。

## 11. 异常处理

- Excel 文件被打开或锁定：SQLite 数据保持 pending 或 failed，接口仍返回成功，因为数据已经可靠落地。
- 请求字段缺失或格式错误：返回 `422`，不写入 SQLite。
- API Key 错误：返回 `401`。
- SQLite 写入失败：返回 `500`，因为数据没有可靠落地。
- Excel 同步失败：不影响 HTTP 写入；下一轮或手动 `sync` 会重试。
- 程序退出：停止新请求并尽量执行一次最终同步。
- 重复请求：第一版允许重复写入，保持与当前行为一致。后续如 Dify 请求能提供稳定 `request_id`，再增加幂等约束。

## 12. 脱敏规则

程序沿用当前递归字段脱敏思路：

- 将 `MASK_FIELDS` 解析为小写字段名集合。
- 遍历 JSON object、array 和嵌套结构。
- 当字段名小写后命中集合时，将值替换为固定掩码字符串。
- 脱敏发生在写入 SQLite 之前，因此 SQLite 和 Excel 都不会保存原始敏感值。

## 13. 跨平台要求

- Python 版本保持 `>=3.12`。
- 文件路径全部使用 `pathlib.Path`。
- SQLite 使用 Python 标准库 `sqlite3`，减少运行依赖。
- Excel 写入使用 `openpyxl`。
- 命令行入口使用 Python 包的 console script。
- 不依赖 shell 专属语法、系统服务管理器或平台专属文件锁。
- Windows 上 Excel 文件被打开时，写入失败按可重试同步错误处理。

## 14. 从当前实现迁移

保留：

- `POST /api/v1/logs` 的请求和响应契约。
- 字段脱敏逻辑。
- Excel 生成能力中的字段选择和 JSON 序列化方式。
- FastAPI app factory 和测试客户端思路。

替换：

- PostgreSQL 存储替换为 SQLite。
- Excel 导出接口替换为后台同步器和手动 `sync` 命令。
- Web 页面、查询 API、指标 API、Docker Compose 和 Alembic 从第一版运行路径中移除。

包命名从平台化的 `dify-workflow-log-system` 调整为更贴近目标的 `dify-log-excel`。源码包名使用 `dify_log_excel`，避免新实现和旧平台化模块边界混在一起。

## 15. 测试策略

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

## 16. 第一版完成标准

第一版完成后，用户可以：

1. 安装或以 editable 模式运行程序。
2. 执行 `dify-log-excel serve`。
3. 在 Dify HTTP 节点中继续调用 `POST /api/v1/logs`。
4. 在 `data/dify_logs.db` 中获得可靠本地缓冲。
5. 在 `logs/YYYY-MM-DD.xlsx` 中看到每日节点日志和执行汇总。
6. 当 Excel 文件被占用导致同步失败后，关闭文件并执行 `dify-log-excel sync` 补同步。
