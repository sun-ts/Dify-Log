# Dify 工作流节点日志与分析系统设计

日期：2026-06-29

## 1. 背景

本系统用于记录 Dify 工作流中关键节点的入参、出参、状态、错误和耗时。Dify 在需要记录日志的节点后增加 HTTP 请求节点，调用本系统的日志接口完成数据写入。系统后续通过统一的 `execution_id` 串联一次工作流执行中的所有节点日志，支持按完整执行路线排查问题，并提供基础分析和 Excel 导出能力。

后端技术选型为 Python，数据库选型为 PostgreSQL，部署方式为 Docker Compose。

## 2. 目标

- 记录 Dify 工作流节点的入参、出参、执行状态、错误信息、开始时间、结束时间和耗时。
- 支持一次工作流执行生成或复用一个 `execution_id`，所有节点日志通过该 ID 关联。
- 支持两种写入模式：一次性记录节点日志，以及 start/finish 两段式记录。
- 提供简单 Web 页面，用于查询执行列表、执行详情、节点日志和基础分析指标。
- 支持按时间范围、工作流、状态等条件导出 Excel，便于离线比对和排查问题。
- 支持字段脱敏、API Key 认证、管理员登录和可配置日志保留策略。
- 从第一版开始为长期日志存储和分析查询预留结构，包括索引、分区和统计扩展空间。

## 3. 非目标

第一版不实现以下能力：

- 多租户权限系统。
- 多 API Key 和按工作流授权。
- 独立前端工程或复杂大屏。
- 对象存储归档。
- 自动识别敏感信息。
- 告警通知。
- 分布式队列和独立 worker 容器。

这些能力作为第二阶段或后续平台化扩展。

## 4. 总体架构

第一版采用偏分析平台化的设计，但部署保持简单。Docker Compose 启动两个核心服务：

- `app`：FastAPI 应用，包含日志采集 API、查询 API、Web 页面、Excel 导出和后台定时任务。
- `postgres`：PostgreSQL 数据库。

逻辑模块划分：

- Dify 接入层：Dify 工作流通过 HTTP 请求节点调用日志接口。
- 日志采集 API：接收节点日志、校验 API Key、校验请求体。
- 日志处理服务：生成或复用 `execution_id`，执行字段脱敏，标准化状态，计算耗时，写入数据库。
- PostgreSQL 存储层：保存执行记录、节点日志、节点事件和后续统计数据。
- 查询与页面层：提供管理员登录后的日志查询、执行链路查看、节点搜索和基础分析。
- 分析与生命周期任务：执行日志清理、基础统计查询和后续聚合任务。

后续数据量增大时，可以将清理和统计任务拆成独立 worker，不影响第一版 API 契约和核心数据模型。

## 5. 核心概念

### 5.1 execution_id

`execution_id` 是一次 Dify 工作流执行的对外追踪 ID。它贯穿整条执行路线，所有节点日志都必须关联该字段。

生成规则：

- 如果 Dify 请求中传入 `execution_id`，系统直接使用该值。
- 如果请求中未传入 `execution_id`，系统生成新的 UUID 字符串并在响应中返回。

推荐在 Dify 工作流开始处生成或获取 `execution_id`，后续所有日志 HTTP 节点都传递该值。

### 5.2 日志状态

工作流执行状态：

- `running`
- `success`
- `failed`
- `partial`

节点日志状态：

- `running`
- `success`
- `failed`
- `skipped`

### 5.3 写入模式

系统同时支持两种写入模式：

- 一次性记录：节点执行结束后调用 `POST /api/v1/logs`，提交入参、出参、状态和耗时。
- 两段式记录：节点开始前调用 `POST /api/v1/logs/start`，节点结束后调用 `POST /api/v1/logs/{log_id}/finish`。

一次性记录适合大多数节点。两段式记录适合耗时敏感、失败率高或需要精确定位中断位置的节点。

## 6. 数据模型

### 6.1 workflow_executions

记录一次完整工作流执行。

核心字段：

- `id`：UUID 主键。
- `execution_id`：对外追踪 ID，唯一。
- `workflow_id`：Dify 工作流 ID。
- `workflow_name`：工作流名称。
- `app_id`：Dify 应用 ID。
- `app_name`：Dify 应用名称。
- `status`：执行状态。
- `started_at`：开始时间，`timestamptz`。
- `finished_at`：结束时间，`timestamptz`。
- `duration_ms`：总耗时，毫秒。
- `metadata`：JSONB 扩展字段。
- `created_at`：创建时间，`timestamptz`。
- `updated_at`：更新时间，`timestamptz`。

索引：

- `execution_id` 唯一索引。
- `workflow_id, created_at` 组合索引。
- `status, created_at` 组合索引。
- `created_at` 索引。

### 6.2 node_logs

记录一次节点执行。

核心字段：

- `id`：UUID 主键。
- `execution_id`：对外追踪 ID。
- `workflow_execution_id`：关联 `workflow_executions.id`。
- `workflow_id`：冗余工作流 ID，便于查询。
- `workflow_name`：冗余工作流名称。
- `app_id`：冗余应用 ID。
- `app_name`：冗余应用名称。
- `node_id`：Dify 节点 ID。
- `node_name`：节点名称。
- `node_type`：节点类型，例如 `llm`、`http`、`code`、`knowledge_retrieval`。
- `sequence_no`：节点顺序。
- `status`：节点状态。
- `input_data`：JSONB 入参。
- `output_data`：JSONB 出参。
- `error_message`：错误摘要。
- `error_detail`：错误详情。
- `started_at`：开始时间，`timestamptz`。
- `finished_at`：结束时间，`timestamptz`。
- `duration_ms`：节点耗时，毫秒。
- `metadata`：JSONB 扩展字段，例如模型、token、重试次数。
- `created_at`：创建时间，`timestamptz`。
- `updated_at`：更新时间，`timestamptz`。

索引：

- `execution_id` 索引。
- `workflow_id, created_at` 组合索引。
- `node_id, created_at` 组合索引。
- `status, created_at` 组合索引。
- `duration_ms` 索引。
- `created_at` 索引。

分区策略：

- `node_logs` 是最大表，第一版按 `created_at` 做月分区。
- 清理过期数据时优先删除过期分区。
- 如果月分区不满足后续数据量要求，再演进为日分区。

### 6.3 node_log_events

记录节点事件明细，用于审计和后续更细粒度分析。

核心字段：

- `id`：UUID 主键。
- `node_log_id`：关联 `node_logs.id`。
- `execution_id`：对外追踪 ID。
- `event_type`：`start`、`finish`、`error`、`retry`、`custom`。
- `event_data`：JSONB 事件数据。
- `created_at`：创建时间，`timestamptz`。

第一版写入 start、finish、error 三类基础事件，页面默认不单独展示事件列表。

### 6.4 分析统计表

第一版统计接口优先使用实时 SQL 聚合。后续数据量增加后引入以下每日聚合表：

- `daily_workflow_metrics`：按天和工作流统计调用量、成功量、失败量、平均耗时、P95/P99 耗时。
- `daily_node_metrics`：按天和节点统计调用量、失败率、平均耗时、P95/P99 耗时、慢调用数。

## 7. API 设计

所有写入接口使用 Header：

```http
X-API-Key: <LOG_API_KEY>
```

### 7.1 一次性记录节点日志

```http
POST /api/v1/logs
```

请求示例：

```json
{
  "execution_id": "b9af4d70-0d21-4c33-9a87-6df4a936018f",
  "workflow_id": "workflow_001",
  "workflow_name": "客户线索分析工作流",
  "app_id": "app_001",
  "app_name": "CRM 助手",
  "node_id": "llm_summary_01",
  "node_name": "线索摘要生成",
  "node_type": "llm",
  "sequence_no": 3,
  "status": "success",
  "input_data": {
    "lead_text": "示例线索文本"
  },
  "output_data": {
    "summary": "示例摘要"
  },
  "metadata": {
    "model": "gpt-4.1",
    "tokens": 1280
  }
}
```

响应示例：

```json
{
  "execution_id": "b9af4d70-0d21-4c33-9a87-6df4a936018f",
  "log_id": "083d4ac8-3c04-4e42-8753-c85054bd3220",
  "status": "success"
}
```

### 7.2 记录节点开始

```http
POST /api/v1/logs/start
```

响应返回 `execution_id` 和 `log_id`。

### 7.3 记录节点结束

```http
POST /api/v1/logs/{log_id}/finish
```

请求体包含 `status`、`output_data`、`error_message`、`error_detail` 和 `metadata`。系统根据开始和结束时间计算 `duration_ms`。

### 7.4 标记工作流结束

```http
POST /api/v1/executions/{execution_id}/finish
```

用于显式标记一次工作流执行完成，并计算总耗时。Dify 工作流结束节点推荐调用该接口。

### 7.5 查询接口

管理员登录后可调用：

```http
GET /api/v1/executions
GET /api/v1/executions/{execution_id}
GET /api/v1/executions/{execution_id}/nodes
GET /api/v1/logs/{log_id}
```

列表查询支持分页和筛选：

- 时间范围。
- `execution_id`。
- `workflow_id`。
- `workflow_name`。
- `node_id`。
- `node_name`。
- `status`。
- 最小耗时。

### 7.6 分析接口

```http
GET /api/v1/metrics/summary
GET /api/v1/metrics/workflows
GET /api/v1/metrics/nodes/slow
GET /api/v1/metrics/errors
```

第一版返回实时聚合结果。

### 7.7 Excel 导出接口

```http
GET /api/v1/export/executions.xlsx
```

支持筛选参数：

- `start_time`
- `end_time`
- `workflow_id`
- `workflow_name`
- `status`
- `execution_id`

导出文件包含两个 sheet：

- `executions`：一次执行一行，包括 `execution_id`、工作流、状态、开始时间、结束时间、总耗时、节点数、失败节点数。
- `node_logs`：一个节点一行，包括 `execution_id`、节点顺序、节点名称、节点类型、状态、耗时、错误信息、入参 JSON、出参 JSON。

导出受 `EXPORT_MAX_ROWS` 限制。第一版默认最大导出 50000 行节点明细。

### 7.8 健康检查

```http
GET /health
```

检查应用状态和数据库连接。

## 8. Dify 接入方式

推荐接入流程：

1. 在工作流开始处生成 `execution_id`，或让第一个日志请求不传 `execution_id`，由日志系统生成并返回。
2. 每个需要记录的业务节点后增加 HTTP 请求节点，调用 `POST /api/v1/logs`。
3. 对耗时敏感或容易失败的节点，使用 start/finish 两段式记录。
4. 工作流结束时调用 `POST /api/v1/executions/{execution_id}/finish`。
5. 失败分支也调用日志系统，至少记录 `execution_id`、节点信息、`status=failed`、错误信息和入参。

HTTP 节点请求示例：

```http
POST http://dify-log-app:8000/api/v1/logs
X-API-Key: ${LOG_API_KEY}
Content-Type: application/json
```

## 9. Web 页面设计

第一版使用 FastAPI + Jinja2 模板渲染，避免引入独立前端构建链。

页面包括：

- 登录页：管理员账号密码登录。
- 执行列表页：展示最近执行记录，支持时间范围、执行 ID、工作流、状态筛选。
- 执行详情页：通过 `execution_id` 展示节点时间线，支持展开节点入参、出参和错误详情。
- 节点日志搜索页：按节点名称、节点 ID、状态、耗时阈值和时间范围搜索。
- 分析概览页：展示今日执行次数、失败次数、失败率、平均耗时、慢节点 Top 10、错误 Top 10、工作流调用量 Top 10。

执行列表页和节点搜索页提供 Excel 导出入口，导出当前筛选条件对应的数据。

## 10. 安全与脱敏

### 10.1 API Key

写入接口统一校验 `X-API-Key`。密钥通过环境变量 `LOG_API_KEY` 配置。校验失败返回 `401`，不写入数据。

### 10.2 管理员登录

Web 页面通过管理员账号密码登录：

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`

登录后使用 Cookie Session。生产部署建议放在 HTTPS 反向代理后。

### 10.3 字段脱敏

通过配置项维护需要脱敏的字段名：

```env
MASK_FIELDS=password,token,api_key,phone
```

系统在保存 `input_data`、`output_data` 和 `metadata` 前递归扫描 JSON 字段名。命中字段后将值替换为 `***MASKED***`。

第一版不自动识别敏感内容，避免误伤业务字段。

## 11. 生命周期策略

自动清理由两个配置控制：

```env
LOG_RETENTION_ENABLED=true
LOG_RETENTION_DAYS=90
```

策略：

- 开启后，后台任务每天清理早于保留天数的数据。
- `node_logs` 按月分区时，优先删除完全过期的分区。
- 未完全过期的分区中如有零散过期数据，再按条件删除。
- 关闭后永久保留数据。

## 12. Docker Compose 部署

第一版包含：

- `app`：FastAPI 应用容器。
- `postgres`：PostgreSQL 容器。

关键环境变量：

```env
DATABASE_URL=postgresql+psycopg://dify_log:dify_log@postgres:5432/dify_log
LOG_API_KEY=change-me
ADMIN_USERNAME=admin
ADMIN_PASSWORD=change-me
SESSION_SECRET_KEY=change-me
MASK_FIELDS=password,token,api_key,phone
LOG_RETENTION_ENABLED=true
LOG_RETENTION_DAYS=90
EXPORT_MAX_ROWS=50000
APP_TIMEZONE=Asia/Shanghai
```

部署交付物：

- `docker-compose.yml`
- `.env.example`
- `README.md`
- Alembic migration。
- 应用源码。

## 13. 测试策略

第一版需要覆盖：

- API Key 校验成功和失败。
- 自动生成 `execution_id`。
- 传入 `execution_id` 时复用该值。
- 一次性日志写入。
- start/finish 两段式写入和耗时计算。
- 字段脱敏递归处理。
- 执行链路按 `sequence_no` 和时间排序。
- 查询筛选和分页。
- Excel 导出 sheet、列和最大行数限制。
- 日志保留任务开关和保留天数。
- `/health` 数据库连通性。

## 14. 第一版验收标准

- 使用 Docker Compose 可以启动应用和 PostgreSQL。
- Dify HTTP 节点可以成功调用日志写入接口。
- 未传 `execution_id` 时，系统返回新 ID。
- 多个节点传入同一个 `execution_id` 后，可以在页面查看完整执行路线。
- 页面可登录、筛选执行记录、查看节点入参出参和错误详情。
- 可以按时间范围导出 Excel，文件包含 `executions` 和 `node_logs` 两个 sheet。
- 配置 `MASK_FIELDS` 后，命中字段在数据库和页面中均显示为 `***MASKED***`。
- 配置关闭自动清理时不删除历史数据。
- 配置开启自动清理时按保留天数清理过期数据。

## 15. 里程碑

### 里程碑 1：基础工程与数据库

建立 FastAPI 工程、Docker Compose、PostgreSQL、SQLAlchemy、Alembic 和基础表结构。

### 里程碑 2：日志写入闭环

实现 API Key 校验、一次性写入、start/finish 写入、`execution_id` 生成与复用、字段脱敏。

### 里程碑 3：查询与详情页面

实现管理员登录、执行列表、执行详情时间线、节点日志搜索。

### 里程碑 4：Excel 导出与基础分析

实现筛选导出 `.xlsx`、基础分析接口和分析概览页。

### 里程碑 5：生命周期与文档

实现日志保留任务、健康检查、README 和 Dify 接入示例。

## 16. 后续扩展

- 多 API Key 和按应用或工作流隔离。
- 独立 worker 容器处理清理、统计和归档。
- 对象存储归档旧分区。
- 管理员操作审计日志。
- 更完整的统计看板。
- 失败率、慢节点和异常错误告警。
- JSON 字段展开导出配置。
