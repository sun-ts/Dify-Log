# Dify Workflow Log System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Docker Compose deployable Python service that records Dify workflow node inputs, outputs, timing, errors, and analysis data linked by `execution_id`.

**Architecture:** A FastAPI application provides write APIs, query APIs, simple Jinja2 admin pages, Excel export, health checks, and an in-process retention scheduler. PostgreSQL stores workflow executions, partition-ready node logs, and node events; application services handle API key auth, admin sessions, masking, log writes, queries, metrics, export, and cleanup.

**Tech Stack:** Python 3.12, FastAPI, SQLAlchemy 2.x, Alembic, PostgreSQL 16, psycopg 3, Pydantic Settings, Jinja2, openpyxl, APScheduler, pytest, httpx, Docker Compose.

---

## Scope Check

The approved spec describes one cohesive product: a Dify workflow log and analysis service. It contains multiple modules, but they are not independent products; one implementation plan can produce working, testable software.

## File Structure

- `pyproject.toml`: Python package metadata, runtime dependencies, dev dependencies, pytest config.
- `.env.example`: Example configuration for Docker Compose and local development.
- `docker-compose.yml`: App and PostgreSQL services.
- `Dockerfile`: FastAPI application image.
- `README.md`: Local run, Docker run, Dify HTTP node examples, export usage.
- `alembic.ini`: Alembic command configuration.
- `alembic/env.py`: Alembic migration environment bound to app models.
- `alembic/versions/20260629_0001_initial_schema.py`: PostgreSQL schema, indexes, and monthly partition helper function.
- `src/dify_log_system/__init__.py`: Package marker.
- `src/dify_log_system/main.py`: FastAPI app factory, middleware, routers, scheduler lifecycle.
- `src/dify_log_system/config.py`: Environment-driven settings.
- `src/dify_log_system/database.py`: SQLAlchemy engine, session factory, DB dependency.
- `src/dify_log_system/models.py`: SQLAlchemy models.
- `src/dify_log_system/schemas.py`: Pydantic request and response models.
- `src/dify_log_system/security.py`: API key dependency and admin session helpers.
- `src/dify_log_system/masking.py`: Recursive JSON field masking.
- `src/dify_log_system/partitioning.py`: PostgreSQL node log monthly partition creation.
- `src/dify_log_system/services/logs.py`: Write path for executions, node logs, events.
- `src/dify_log_system/services/queries.py`: Execution and node log search.
- `src/dify_log_system/services/metrics.py`: Summary, workflow, slow-node, and error metrics.
- `src/dify_log_system/services/export.py`: `.xlsx` export.
- `src/dify_log_system/services/retention.py`: Retention cleanup.
- `src/dify_log_system/routers/api_logs.py`: Write API routes.
- `src/dify_log_system/routers/api_queries.py`: Query API routes.
- `src/dify_log_system/routers/api_metrics.py`: Metrics API routes.
- `src/dify_log_system/routers/api_export.py`: Excel export route.
- `src/dify_log_system/routers/web.py`: Login, logout, list, detail, search, metrics pages.
- `src/dify_log_system/templates/*.html`: Admin page templates.
- `tests/`: Unit and integration tests using SQLite for service tests and mocked PostgreSQL-specific partition behavior.

## Task 1: Project Skeleton And Tooling

**Files:**
- Create: `pyproject.toml`
- Create: `src/dify_log_system/__init__.py`
- Create: `tests/test_project_imports.py`

- [ ] **Step 1: Create package metadata**

Create `pyproject.toml`:

```toml
[project]
name = "dify-workflow-log-system"
version = "0.1.0"
description = "Dify workflow node logging and analysis service"
requires-python = ">=3.12"
dependencies = [
  "fastapi>=0.115.0",
  "uvicorn[standard]>=0.30.0",
  "sqlalchemy>=2.0.30",
  "alembic>=1.13.0",
  "psycopg[binary]>=3.2.0",
  "pydantic-settings>=2.4.0",
  "jinja2>=3.1.4",
  "python-multipart>=0.0.9",
  "itsdangerous>=2.2.0",
  "openpyxl>=3.1.5",
  "apscheduler>=3.10.4"
]

[project.optional-dependencies]
dev = [
  "pytest>=8.3.0",
  "httpx>=0.27.0",
  "ruff>=0.6.0"
]

[build-system]
requires = ["setuptools>=70.0"]
build-backend = "setuptools.build_meta"

[tool.setuptools.packages.find]
where = ["src"]

[tool.pytest.ini_options]
testpaths = ["tests"]
pythonpath = ["src"]
addopts = "-q"

[tool.ruff]
line-length = 100
target-version = "py312"
```

- [ ] **Step 2: Write the failing import test**

Create `tests/test_project_imports.py`:

```python
def test_package_imports():
    import dify_log_system

    assert dify_log_system.__version__ == "0.1.0"
```

- [ ] **Step 3: Run test to verify it fails**

Run:

```bash
python -m pip install -e ".[dev]"
pytest tests/test_project_imports.py -q
```

Expected: FAIL with `ModuleNotFoundError: No module named 'dify_log_system'`.

- [ ] **Step 4: Add package marker**

Create `src/dify_log_system/__init__.py`:

```python
__version__ = "0.1.0"
```

- [ ] **Step 5: Run test to verify it passes**

Run:

```bash
pytest tests/test_project_imports.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pyproject.toml src/dify_log_system/__init__.py tests/test_project_imports.py
git commit -m "chore: add python project skeleton"
```

## Task 2: Settings And Configuration

**Files:**
- Create: `src/dify_log_system/config.py`
- Create: `.env.example`
- Create: `tests/test_config.py`

- [ ] **Step 1: Write failing settings tests**

Create `tests/test_config.py`:

```python
from dify_log_system.config import Settings


def test_mask_fields_are_normalized():
    settings = Settings(mask_fields=" password, TOKEN,api_key ,, phone ")

    assert settings.mask_field_names == {"password", "token", "api_key", "phone"}


def test_retention_defaults_are_enabled():
    settings = Settings()

    assert settings.log_retention_enabled is True
    assert settings.log_retention_days == 90
    assert settings.export_max_rows == 50000
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pytest tests/test_config.py -q
```

Expected: FAIL with `ModuleNotFoundError: No module named 'dify_log_system.config'`.

- [ ] **Step 3: Implement settings**

Create `src/dify_log_system/config.py`:

```python
from functools import lru_cache

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "Dify Workflow Log System"
    app_timezone: str = "Asia/Shanghai"
    database_url: str = (
        "postgresql+psycopg://dify_log:dify_log@localhost:5432/dify_log"
    )
    log_api_key: str = "dev-log-api-key"
    admin_username: str = "admin"
    admin_password: str = "dev-admin-password"
    session_secret_key: str = "dev-session-secret"
    mask_fields: str = "password,token,api_key,phone"
    log_retention_enabled: bool = True
    log_retention_days: int = Field(default=90, ge=1)
    export_max_rows: int = Field(default=50000, ge=1)

    @property
    def mask_field_names(self) -> set[str]:
        return {
            item.strip().lower()
            for item in self.mask_fields.split(",")
            if item.strip()
        }


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
```

- [ ] **Step 4: Add example environment**

Create `.env.example`:

```env
DATABASE_URL=postgresql+psycopg://dify_log:dify_log@postgres:5432/dify_log
LOG_API_KEY=dev-log-api-key
ADMIN_USERNAME=admin
ADMIN_PASSWORD=dev-admin-password
SESSION_SECRET_KEY=dev-session-secret
MASK_FIELDS=password,token,api_key,phone
LOG_RETENTION_ENABLED=true
LOG_RETENTION_DAYS=90
EXPORT_MAX_ROWS=50000
APP_TIMEZONE=Asia/Shanghai
```

- [ ] **Step 5: Run test to verify it passes**

Run:

```bash
pytest tests/test_config.py -q
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .env.example src/dify_log_system/config.py tests/test_config.py
git commit -m "feat: add environment settings"
```

## Task 3: Database Models And Alembic Schema

**Files:**
- Create: `src/dify_log_system/database.py`
- Create: `src/dify_log_system/models.py`
- Create: `src/dify_log_system/partitioning.py`
- Create: `alembic.ini`
- Create: `alembic/env.py`
- Create: `alembic/versions/20260629_0001_initial_schema.py`
- Create: `tests/test_models.py`

- [ ] **Step 1: Write failing model tests**

Create `tests/test_models.py`:

```python
from sqlalchemy import create_engine
from sqlalchemy.orm import Session

from dify_log_system.models import Base, NodeLog, WorkflowExecution


def test_models_can_create_execution_and_node_log_in_sqlite():
    engine = create_engine("sqlite+pysqlite:///:memory:", future=True)
    Base.metadata.create_all(engine)

    with Session(engine) as session:
        execution = WorkflowExecution(execution_id="trace-001", workflow_name="Demo")
        session.add(execution)
        session.flush()

        node = NodeLog(
            execution_id="trace-001",
            workflow_execution_id=execution.id,
            node_id="node-1",
            node_name="LLM",
            node_type="llm",
            sequence_no=1,
            status="success",
            input_data={"question": "hello"},
            output_data={"answer": "world"},
        )
        session.add(node)
        session.commit()

        saved = session.query(NodeLog).filter_by(execution_id="trace-001").one()

    assert saved.node_name == "LLM"
    assert saved.input_data == {"question": "hello"}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pytest tests/test_models.py -q
```

Expected: FAIL with `ModuleNotFoundError: No module named 'dify_log_system.models'`.

- [ ] **Step 3: Implement database session helper**

Create `src/dify_log_system/database.py`:

```python
from collections.abc import Generator

from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker

from dify_log_system.config import get_settings


settings = get_settings()
engine = create_engine(settings.database_url, pool_pre_ping=True, future=True)
SessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False, future=True)


def get_db() -> Generator[Session, None, None]:
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
```

- [ ] **Step 4: Implement models**

Create `src/dify_log_system/models.py`:

```python
from __future__ import annotations

import uuid
from datetime import datetime, timezone
from typing import Any

from sqlalchemy import DateTime, ForeignKey, ForeignKeyConstraint, Integer, String, Text, Uuid
from sqlalchemy.dialects import postgresql
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column
from sqlalchemy.types import JSON


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


JsonDict = dict[str, Any]
JSONB = JSON().with_variant(postgresql.JSONB, "postgresql")


class Base(DeclarativeBase):
    pass


class WorkflowExecution(Base):
    __tablename__ = "workflow_executions"

    id: Mapped[uuid.UUID] = mapped_column(Uuid(as_uuid=True), primary_key=True, default=uuid.uuid4)
    execution_id: Mapped[str] = mapped_column(String(128), unique=True, nullable=False, index=True)
    workflow_id: Mapped[str | None] = mapped_column(String(128), nullable=True, index=True)
    workflow_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    app_id: Mapped[str | None] = mapped_column(String(128), nullable=True)
    app_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    status: Mapped[str] = mapped_column(String(32), nullable=False, default="running", index=True)
    started_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, default=utc_now)
    finished_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    duration_ms: Mapped[int | None] = mapped_column(Integer, nullable=True)
    extra_metadata: Mapped[JsonDict] = mapped_column("metadata", JSONB, nullable=False, default=dict)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, default=utc_now)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, default=utc_now, onupdate=utc_now
    )


class NodeLog(Base):
    __tablename__ = "node_logs"

    id: Mapped[uuid.UUID] = mapped_column(Uuid(as_uuid=True), primary_key=True, default=uuid.uuid4)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), primary_key=True, default=utc_now)
    execution_id: Mapped[str] = mapped_column(String(128), nullable=False, index=True)
    workflow_execution_id: Mapped[uuid.UUID | None] = mapped_column(
        Uuid(as_uuid=True), ForeignKey("workflow_executions.id"), nullable=True
    )
    workflow_id: Mapped[str | None] = mapped_column(String(128), nullable=True, index=True)
    workflow_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    app_id: Mapped[str | None] = mapped_column(String(128), nullable=True)
    app_name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    node_id: Mapped[str] = mapped_column(String(128), nullable=False, index=True)
    node_name: Mapped[str] = mapped_column(String(255), nullable=False)
    node_type: Mapped[str | None] = mapped_column(String(64), nullable=True)
    sequence_no: Mapped[int | None] = mapped_column(Integer, nullable=True)
    status: Mapped[str] = mapped_column(String(32), nullable=False, default="running", index=True)
    input_data: Mapped[JsonDict] = mapped_column(JSONB, nullable=False, default=dict)
    output_data: Mapped[JsonDict] = mapped_column(JSONB, nullable=False, default=dict)
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    error_detail: Mapped[str | None] = mapped_column(Text, nullable=True)
    started_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, default=utc_now)
    finished_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    duration_ms: Mapped[int | None] = mapped_column(Integer, nullable=True)
    extra_metadata: Mapped[JsonDict] = mapped_column("metadata", JSONB, nullable=False, default=dict)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, default=utc_now, onupdate=utc_now
    )


class NodeLogEvent(Base):
    __tablename__ = "node_log_events"
    __table_args__ = (
        ForeignKeyConstraint(
            ["node_log_id", "node_log_created_at"],
            ["node_logs.id", "node_logs.created_at"],
        ),
    )

    id: Mapped[uuid.UUID] = mapped_column(Uuid(as_uuid=True), primary_key=True, default=uuid.uuid4)
    node_log_id: Mapped[uuid.UUID] = mapped_column(Uuid(as_uuid=True), nullable=False)
    node_log_created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    execution_id: Mapped[str] = mapped_column(String(128), nullable=False, index=True)
    event_type: Mapped[str] = mapped_column(String(32), nullable=False)
    event_data: Mapped[JsonDict] = mapped_column(JSONB, nullable=False, default=dict)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False, default=utc_now)
```

- [ ] **Step 5: Add partition helper**

Create `src/dify_log_system/partitioning.py`:

```python
from datetime import datetime, timezone

from sqlalchemy import text
from sqlalchemy.orm import Session


def month_bounds(value: datetime) -> tuple[datetime, datetime, str]:
    normalized = value.astimezone(timezone.utc)
    start = datetime(normalized.year, normalized.month, 1, tzinfo=timezone.utc)
    if normalized.month == 12:
        end = datetime(normalized.year + 1, 1, 1, tzinfo=timezone.utc)
    else:
        end = datetime(normalized.year, normalized.month + 1, 1, tzinfo=timezone.utc)
    suffix = f"y{start.year}m{start.month:02d}"
    return start, end, suffix


def ensure_node_logs_partition(session: Session, value: datetime) -> None:
    if session.bind is None or session.bind.dialect.name != "postgresql":
        return
    start, end, suffix = month_bounds(value)
    table_name = f"node_logs_{suffix}"
    session.execute(
        text(
            f"""
            CREATE TABLE IF NOT EXISTS {table_name}
            PARTITION OF node_logs
            FOR VALUES FROM (:start_at) TO (:end_at)
            """
        ),
        {"start_at": start, "end_at": end},
    )
```

- [ ] **Step 6: Add Alembic configuration**

Create `alembic.ini`:

```ini
[alembic]
script_location = alembic
prepend_sys_path = .
sqlalchemy.url = postgresql+psycopg://dify_log:dify_log@localhost:5432/dify_log

[loggers]
keys = root,sqlalchemy,alembic

[handlers]
keys = console

[formatters]
keys = generic

[logger_root]
level = WARN
handlers = console

[logger_sqlalchemy]
level = WARN
handlers =
qualname = sqlalchemy.engine

[logger_alembic]
level = INFO
handlers =
qualname = alembic

[handler_console]
class = StreamHandler
args = (sys.stderr,)
level = NOTSET
formatter = generic

[formatter_generic]
format = %(levelname)-5.5s [%(name)s] %(message)s
datefmt = %H:%M:%S
```

Create `alembic/env.py`:

```python
from logging.config import fileConfig

from alembic import context
from sqlalchemy import engine_from_config, pool

from dify_log_system.config import get_settings
from dify_log_system.models import Base

config = context.config
if config.config_file_name is not None:
    fileConfig(config.config_file_name)

target_metadata = Base.metadata
settings = get_settings()
config.set_main_option("sqlalchemy.url", settings.database_url)


def run_migrations_offline() -> None:
    context.configure(
        url=settings.database_url,
        target_metadata=target_metadata,
        literal_binds=True,
        dialect_opts={"paramstyle": "named"},
    )
    with context.begin_transaction():
        context.run_migrations()


def run_migrations_online() -> None:
    connectable = engine_from_config(
        config.get_section(config.config_ini_section, {}),
        prefix="sqlalchemy.",
        poolclass=pool.NullPool,
    )
    with connectable.connect() as connection:
        context.configure(connection=connection, target_metadata=target_metadata)
        with context.begin_transaction():
            context.run_migrations()


if context.is_offline_mode():
    run_migrations_offline()
else:
    run_migrations_online()
```

- [ ] **Step 7: Add initial migration**

Create `alembic/versions/20260629_0001_initial_schema.py`:

```python
from collections.abc import Sequence

from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

revision: str = "20260629_0001"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.execute('CREATE EXTENSION IF NOT EXISTS "uuid-ossp"')
    op.create_table(
        "workflow_executions",
        sa.Column("id", postgresql.UUID(as_uuid=True), primary_key=True, server_default=sa.text("uuid_generate_v4()")),
        sa.Column("execution_id", sa.String(length=128), nullable=False),
        sa.Column("workflow_id", sa.String(length=128)),
        sa.Column("workflow_name", sa.String(length=255)),
        sa.Column("app_id", sa.String(length=128)),
        sa.Column("app_name", sa.String(length=255)),
        sa.Column("status", sa.String(length=32), nullable=False, server_default="running"),
        sa.Column("started_at", sa.DateTime(timezone=True), nullable=False, server_default=sa.text("now()")),
        sa.Column("finished_at", sa.DateTime(timezone=True)),
        sa.Column("duration_ms", sa.Integer()),
        sa.Column("metadata", postgresql.JSONB(astext_type=sa.Text()), nullable=False, server_default=sa.text("'{}'::jsonb")),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False, server_default=sa.text("now()")),
        sa.Column("updated_at", sa.DateTime(timezone=True), nullable=False, server_default=sa.text("now()")),
    )
    op.create_index("ix_workflow_executions_execution_id", "workflow_executions", ["execution_id"], unique=True)
    op.create_index("ix_workflow_executions_workflow_created", "workflow_executions", ["workflow_id", "created_at"])
    op.create_index("ix_workflow_executions_status_created", "workflow_executions", ["status", "created_at"])
    op.create_index("ix_workflow_executions_created_at", "workflow_executions", ["created_at"])

    op.execute(
        """
        CREATE TABLE node_logs (
            id uuid NOT NULL DEFAULT uuid_generate_v4(),
            created_at timestamptz NOT NULL DEFAULT now(),
            execution_id varchar(128) NOT NULL,
            workflow_execution_id uuid REFERENCES workflow_executions(id),
            workflow_id varchar(128),
            workflow_name varchar(255),
            app_id varchar(128),
            app_name varchar(255),
            node_id varchar(128) NOT NULL,
            node_name varchar(255) NOT NULL,
            node_type varchar(64),
            sequence_no integer,
            status varchar(32) NOT NULL DEFAULT 'running',
            input_data jsonb NOT NULL DEFAULT '{}'::jsonb,
            output_data jsonb NOT NULL DEFAULT '{}'::jsonb,
            error_message text,
            error_detail text,
            started_at timestamptz NOT NULL DEFAULT now(),
            finished_at timestamptz,
            duration_ms integer,
            metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
            updated_at timestamptz NOT NULL DEFAULT now(),
            PRIMARY KEY (id, created_at)
        ) PARTITION BY RANGE (created_at)
        """
    )
    op.execute(
        """
        CREATE TABLE IF NOT EXISTS node_logs_default
        PARTITION OF node_logs DEFAULT
        """
    )
    op.create_index("ix_node_logs_execution_id", "node_logs", ["execution_id"])
    op.create_index("ix_node_logs_workflow_created", "node_logs", ["workflow_id", "created_at"])
    op.create_index("ix_node_logs_node_created", "node_logs", ["node_id", "created_at"])
    op.create_index("ix_node_logs_status_created", "node_logs", ["status", "created_at"])
    op.create_index("ix_node_logs_duration_ms", "node_logs", ["duration_ms"])
    op.create_index("ix_node_logs_created_at", "node_logs", ["created_at"])

    op.create_table(
        "node_log_events",
        sa.Column("id", postgresql.UUID(as_uuid=True), primary_key=True, server_default=sa.text("uuid_generate_v4()")),
        sa.Column("node_log_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("node_log_created_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("execution_id", sa.String(length=128), nullable=False),
        sa.Column("event_type", sa.String(length=32), nullable=False),
        sa.Column("event_data", postgresql.JSONB(astext_type=sa.Text()), nullable=False, server_default=sa.text("'{}'::jsonb")),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False, server_default=sa.text("now()")),
        sa.ForeignKeyConstraint(["node_log_id", "node_log_created_at"], ["node_logs.id", "node_logs.created_at"]),
    )
    op.create_index("ix_node_log_events_execution_id", "node_log_events", ["execution_id"])


def downgrade() -> None:
    op.drop_table("node_log_events")
    op.execute("DROP TABLE IF EXISTS node_logs CASCADE")
    op.drop_table("workflow_executions")
```

- [ ] **Step 8: Run model tests**

Run:

```bash
pytest tests/test_models.py -q
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add alembic.ini alembic src/dify_log_system/database.py src/dify_log_system/models.py src/dify_log_system/partitioning.py tests/test_models.py
git commit -m "feat: add database models and migrations"
```

## Task 4: Schemas, Masking, And Security

**Files:**
- Create: `src/dify_log_system/schemas.py`
- Create: `src/dify_log_system/masking.py`
- Create: `src/dify_log_system/security.py`
- Create: `tests/test_masking.py`
- Create: `tests/test_security.py`

- [ ] **Step 1: Write failing masking tests**

Create `tests/test_masking.py`:

```python
from dify_log_system.masking import mask_json


def test_mask_json_recursively_masks_configured_fields():
    data = {
        "password": "secret",
        "profile": {"phone": "13800000000", "name": "Alice"},
        "items": [{"token": "abc"}, {"value": 3}],
    }

    masked = mask_json(data, {"password", "phone", "token"})

    assert masked["password"] == "***MASKED***"
    assert masked["profile"]["phone"] == "***MASKED***"
    assert masked["profile"]["name"] == "Alice"
    assert masked["items"][0]["token"] == "***MASKED***"
    assert data["password"] == "secret"
```

- [ ] **Step 2: Write failing security tests**

Create `tests/test_security.py`:

```python
import pytest
from fastapi import HTTPException

from dify_log_system.security import require_admin, verify_api_key_value


def test_verify_api_key_accepts_matching_value():
    assert verify_api_key_value("abc", "abc") is None


def test_verify_api_key_rejects_missing_or_wrong_value():
    with pytest.raises(HTTPException) as error:
        verify_api_key_value(None, "abc")

    assert error.value.status_code == 401


def test_require_admin_rejects_missing_session():
    class FakeRequest:
        session = {}

    with pytest.raises(HTTPException) as error:
        require_admin(FakeRequest())

    assert error.value.status_code == 401

    with pytest.raises(HTTPException) as error:
        verify_api_key_value("wrong", "abc")

    assert error.value.status_code == 401
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
pytest tests/test_masking.py tests/test_security.py -q
```

Expected: FAIL with missing modules.

- [ ] **Step 4: Implement schemas**

Create `src/dify_log_system/schemas.py`:

```python
from datetime import datetime
from typing import Any
from uuid import UUID

from pydantic import BaseModel, Field


JsonDict = dict[str, Any]


class NodeLogBase(BaseModel):
    execution_id: str | None = None
    workflow_id: str | None = None
    workflow_name: str | None = None
    app_id: str | None = None
    app_name: str | None = None
    node_id: str = Field(min_length=1, max_length=128)
    node_name: str = Field(min_length=1, max_length=255)
    node_type: str | None = None
    sequence_no: int | None = None
    input_data: JsonDict = Field(default_factory=dict)
    metadata: JsonDict = Field(default_factory=dict)
    started_at: datetime | None = None


class NodeLogCreate(NodeLogBase):
    status: str = "success"
    output_data: JsonDict = Field(default_factory=dict)
    error_message: str | None = None
    error_detail: str | None = None
    finished_at: datetime | None = None
    duration_ms: int | None = None


class NodeLogStart(NodeLogBase):
    pass


class NodeLogFinish(BaseModel):
    status: str = "success"
    output_data: JsonDict = Field(default_factory=dict)
    error_message: str | None = None
    error_detail: str | None = None
    metadata: JsonDict = Field(default_factory=dict)
    finished_at: datetime | None = None


class ExecutionFinish(BaseModel):
    status: str = "success"
    finished_at: datetime | None = None
    metadata: JsonDict = Field(default_factory=dict)


class NodeLogWriteResponse(BaseModel):
    execution_id: str
    log_id: UUID
    status: str


class ExecutionResponse(BaseModel):
    execution_id: str
    status: str


class ExecutionListItem(BaseModel):
    execution_id: str
    workflow_id: str | None
    workflow_name: str | None
    status: str
    started_at: datetime
    finished_at: datetime | None
    duration_ms: int | None
    node_count: int
    failed_node_count: int


class NodeLogDetail(BaseModel):
    id: UUID
    execution_id: str
    node_id: str
    node_name: str
    node_type: str | None
    sequence_no: int | None
    status: str
    input_data: JsonDict
    output_data: JsonDict
    error_message: str | None
    error_detail: str | None
    started_at: datetime
    finished_at: datetime | None
    duration_ms: int | None
    metadata: JsonDict
```

- [ ] **Step 5: Implement masking**

Create `src/dify_log_system/masking.py`:

```python
from copy import deepcopy
from typing import Any

MASK_VALUE = "***MASKED***"


def mask_json(value: Any, mask_fields: set[str]) -> Any:
    copied = deepcopy(value)
    return _mask_value(copied, {field.lower() for field in mask_fields})


def _mask_value(value: Any, mask_fields: set[str]) -> Any:
    if isinstance(value, dict):
        return {
            key: MASK_VALUE if str(key).lower() in mask_fields else _mask_value(item, mask_fields)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [_mask_value(item, mask_fields) for item in value]
    return value
```

- [ ] **Step 6: Implement security helpers**

Create `src/dify_log_system/security.py`:

```python
import secrets
from typing import Annotated

from fastapi import Depends, HTTPException, Request, status
from fastapi.security import APIKeyHeader

from dify_log_system.config import Settings, get_settings


api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)


def verify_api_key_value(provided: str | None, expected: str) -> None:
    if not provided or not secrets.compare_digest(provided, expected):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid API key",
        )


def require_api_key(
    provided: Annotated[str | None, Depends(api_key_header)],
    settings: Annotated[Settings, Depends(get_settings)],
) -> None:
    verify_api_key_value(provided, settings.log_api_key)


def is_admin_logged_in(request: Request) -> bool:
    return bool(request.session.get("admin_logged_in"))


def require_admin(request: Request) -> None:
    if not is_admin_logged_in(request):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Admin login required")
```

- [ ] **Step 7: Run tests**

Run:

```bash
pytest tests/test_masking.py tests/test_security.py -q
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add src/dify_log_system/schemas.py src/dify_log_system/masking.py src/dify_log_system/security.py tests/test_masking.py tests/test_security.py
git commit -m "feat: add schemas masking and security helpers"
```

## Task 5: Log Write Service And API Routes

**Files:**
- Create: `src/dify_log_system/services/logs.py`
- Create: `src/dify_log_system/routers/api_logs.py`
- Create: `src/dify_log_system/main.py`
- Create: `tests/conftest.py`
- Create: `tests/test_log_api.py`

- [ ] **Step 1: Write failing API tests**

Create `tests/conftest.py`:

```python
import pytest
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker

from dify_log_system.database import get_db
from dify_log_system.main import create_app
from dify_log_system.models import Base
from dify_log_system.security import require_admin


@pytest.fixture()
def db_session():
    engine = create_engine("sqlite+pysqlite:///:memory:", future=True)
    TestingSessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False, future=True)
    Base.metadata.create_all(engine)
    with TestingSessionLocal() as session:
        yield session


@pytest.fixture()
def client(db_session: Session):
    app = create_app(start_scheduler=False)

    def override_get_db():
        yield db_session

    app.dependency_overrides[get_db] = override_get_db
    app.dependency_overrides[require_admin] = lambda: None
    with TestClient(app) as test_client:
        yield test_client
```

Create `tests/test_log_api.py`:

```python
def test_post_log_generates_execution_id(client):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "node_id": "node-1",
            "node_name": "Question Parser",
            "node_type": "code",
            "status": "success",
            "input_data": {"question": "hello"},
            "output_data": {"parsed": True},
        },
    )

    assert response.status_code == 200
    body = response.json()
    assert body["execution_id"]
    assert body["status"] == "success"
    assert body["log_id"]


def test_post_log_reuses_execution_id_and_masks_fields(client):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "execution_id": "trace-abc",
            "node_id": "node-1",
            "node_name": "HTTP",
            "status": "success",
            "input_data": {"token": "secret", "name": "Alice"},
            "output_data": {"ok": True},
        },
    )

    assert response.status_code == 200
    assert response.json()["execution_id"] == "trace-abc"


def test_post_log_rejects_wrong_api_key(client):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "wrong"},
        json={"node_id": "node-1", "node_name": "HTTP"},
    )

    assert response.status_code == 401


def test_start_and_finish_node_log(client):
    started = client.post(
        "/api/v1/logs/start",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "execution_id": "trace-123",
            "node_id": "node-2",
            "node_name": "LLM",
            "input_data": {"prompt": "hello"},
        },
    )
    assert started.status_code == 200

    log_id = started.json()["log_id"]
    finished = client.post(
        f"/api/v1/logs/{log_id}/finish",
        headers={"X-API-Key": "dev-log-api-key"},
        json={"status": "success", "output_data": {"answer": "world"}},
    )

    assert finished.status_code == 200
    assert finished.json()["execution_id"] == "trace-123"
    assert finished.json()["status"] == "success"
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
pytest tests/test_log_api.py -q
```

Expected: FAIL with `ModuleNotFoundError: No module named 'dify_log_system.main'`.

- [ ] **Step 3: Implement log service**

Create `src/dify_log_system/services/logs.py`:

```python
from datetime import datetime, timezone
from uuid import UUID, uuid4

from fastapi import HTTPException, status
from sqlalchemy import func, select
from sqlalchemy.orm import Session

from dify_log_system.config import Settings
from dify_log_system.masking import mask_json
from dify_log_system.models import NodeLog, NodeLogEvent, WorkflowExecution
from dify_log_system.partitioning import ensure_node_logs_partition
from dify_log_system.schemas import ExecutionFinish, NodeLogCreate, NodeLogFinish, NodeLogStart


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def duration_ms(started_at: datetime, finished_at: datetime) -> int:
    return max(0, int((finished_at - started_at).total_seconds() * 1000))


def get_or_create_execution(
    db: Session,
    settings: Settings,
    execution_id: str | None,
    workflow_id: str | None,
    workflow_name: str | None,
    app_id: str | None,
    app_name: str | None,
    started_at: datetime | None,
    metadata: dict,
) -> WorkflowExecution:
    trace_id = execution_id or str(uuid4())
    execution = db.execute(
        select(WorkflowExecution).where(WorkflowExecution.execution_id == trace_id)
    ).scalar_one_or_none()
    masked_metadata = mask_json(metadata or {}, settings.mask_field_names)
    if execution is not None:
        execution.workflow_id = execution.workflow_id or workflow_id
        execution.workflow_name = execution.workflow_name or workflow_name
        execution.app_id = execution.app_id or app_id
        execution.app_name = execution.app_name or app_name
        execution.extra_metadata = {**execution.extra_metadata, **masked_metadata}
        return execution

    execution = WorkflowExecution(
        execution_id=trace_id,
        workflow_id=workflow_id,
        workflow_name=workflow_name,
        app_id=app_id,
        app_name=app_name,
        status="running",
        started_at=started_at or utc_now(),
        extra_metadata=masked_metadata,
    )
    db.add(execution)
    db.flush()
    return execution


def create_node_log(db: Session, settings: Settings, payload: NodeLogCreate) -> NodeLog:
    now = utc_now()
    started_at = payload.started_at or now
    finished_at = payload.finished_at or now
    execution = get_or_create_execution(
        db,
        settings,
        payload.execution_id,
        payload.workflow_id,
        payload.workflow_name,
        payload.app_id,
        payload.app_name,
        started_at,
        payload.metadata,
    )
    ensure_node_logs_partition(db, now)
    log = NodeLog(
        execution_id=execution.execution_id,
        workflow_execution_id=execution.id,
        workflow_id=payload.workflow_id,
        workflow_name=payload.workflow_name,
        app_id=payload.app_id,
        app_name=payload.app_name,
        node_id=payload.node_id,
        node_name=payload.node_name,
        node_type=payload.node_type,
        sequence_no=payload.sequence_no,
        status=payload.status,
        input_data=mask_json(payload.input_data, settings.mask_field_names),
        output_data=mask_json(payload.output_data, settings.mask_field_names),
        error_message=payload.error_message,
        error_detail=payload.error_detail,
        started_at=started_at,
        finished_at=finished_at,
        duration_ms=payload.duration_ms if payload.duration_ms is not None else duration_ms(started_at, finished_at),
        extra_metadata=mask_json(payload.metadata, settings.mask_field_names),
    )
    db.add(log)
    db.flush()
    add_event(db, log, "finish", {"status": payload.status})
    update_execution_status(db, execution.execution_id)
    db.commit()
    db.refresh(log)
    return log


def start_node_log(db: Session, settings: Settings, payload: NodeLogStart) -> NodeLog:
    started_at = payload.started_at or utc_now()
    execution = get_or_create_execution(
        db,
        settings,
        payload.execution_id,
        payload.workflow_id,
        payload.workflow_name,
        payload.app_id,
        payload.app_name,
        started_at,
        payload.metadata,
    )
    ensure_node_logs_partition(db, started_at)
    log = NodeLog(
        execution_id=execution.execution_id,
        workflow_execution_id=execution.id,
        workflow_id=payload.workflow_id,
        workflow_name=payload.workflow_name,
        app_id=payload.app_id,
        app_name=payload.app_name,
        node_id=payload.node_id,
        node_name=payload.node_name,
        node_type=payload.node_type,
        sequence_no=payload.sequence_no,
        status="running",
        input_data=mask_json(payload.input_data, settings.mask_field_names),
        output_data={},
        started_at=started_at,
        extra_metadata=mask_json(payload.metadata, settings.mask_field_names),
    )
    db.add(log)
    db.flush()
    add_event(db, log, "start", {})
    db.commit()
    db.refresh(log)
    return log


def finish_node_log(db: Session, settings: Settings, log_id: UUID, payload: NodeLogFinish) -> NodeLog:
    log = db.execute(select(NodeLog).where(NodeLog.id == log_id)).scalar_one_or_none()
    if log is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node log not found")
    finished_at = payload.finished_at or utc_now()
    log.status = payload.status
    log.output_data = mask_json(payload.output_data, settings.mask_field_names)
    log.error_message = payload.error_message
    log.error_detail = payload.error_detail
    log.finished_at = finished_at
    log.duration_ms = duration_ms(log.started_at, finished_at)
    log.extra_metadata = {**log.extra_metadata, **mask_json(payload.metadata, settings.mask_field_names)}
    add_event(db, log, "finish" if payload.status != "failed" else "error", {"status": payload.status})
    update_execution_status(db, log.execution_id)
    db.commit()
    db.refresh(log)
    return log


def finish_execution(db: Session, execution_id: str, payload: ExecutionFinish) -> WorkflowExecution:
    execution = db.execute(
        select(WorkflowExecution).where(WorkflowExecution.execution_id == execution_id)
    ).scalar_one_or_none()
    if execution is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Execution not found")
    finished_at = payload.finished_at or utc_now()
    execution.status = payload.status
    execution.finished_at = finished_at
    execution.duration_ms = duration_ms(execution.started_at, finished_at)
    execution.extra_metadata = {**execution.extra_metadata, **payload.metadata}
    db.commit()
    db.refresh(execution)
    return execution


def add_event(db: Session, log: NodeLog, event_type: str, event_data: dict) -> None:
    db.add(
        NodeLogEvent(
            node_log_id=log.id,
            node_log_created_at=log.created_at,
            execution_id=log.execution_id,
            event_type=event_type,
            event_data=event_data,
        )
    )


def update_execution_status(db: Session, execution_id: str) -> None:
    execution = db.execute(
        select(WorkflowExecution).where(WorkflowExecution.execution_id == execution_id)
    ).scalar_one()
    failed_count = db.execute(
        select(func.count()).select_from(NodeLog).where(
            NodeLog.execution_id == execution_id,
            NodeLog.status == "failed",
        )
    ).scalar_one()
    running_count = db.execute(
        select(func.count()).select_from(NodeLog).where(
            NodeLog.execution_id == execution_id,
            NodeLog.status == "running",
        )
    ).scalar_one()
    if failed_count:
        execution.status = "failed"
    elif running_count:
        execution.status = "running"
    else:
        execution.status = "success"
```

- [ ] **Step 4: Implement write routes**

Create `src/dify_log_system/routers/api_logs.py`:

```python
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from dify_log_system.config import Settings, get_settings
from dify_log_system.database import get_db
from dify_log_system.schemas import ExecutionFinish, ExecutionResponse, NodeLogCreate, NodeLogFinish, NodeLogStart, NodeLogWriteResponse
from dify_log_system.security import require_api_key
from dify_log_system.services.logs import create_node_log, finish_execution, finish_node_log, start_node_log

router = APIRouter(prefix="/api/v1", tags=["logs"], dependencies=[Depends(require_api_key)])


@router.post("/logs", response_model=NodeLogWriteResponse)
def post_log(
    payload: NodeLogCreate,
    db: Annotated[Session, Depends(get_db)],
    settings: Annotated[Settings, Depends(get_settings)],
) -> NodeLogWriteResponse:
    log = create_node_log(db, settings, payload)
    return NodeLogWriteResponse(execution_id=log.execution_id, log_id=log.id, status=log.status)


@router.post("/logs/start", response_model=NodeLogWriteResponse)
def post_log_start(
    payload: NodeLogStart,
    db: Annotated[Session, Depends(get_db)],
    settings: Annotated[Settings, Depends(get_settings)],
) -> NodeLogWriteResponse:
    log = start_node_log(db, settings, payload)
    return NodeLogWriteResponse(execution_id=log.execution_id, log_id=log.id, status=log.status)


@router.post("/logs/{log_id}/finish", response_model=NodeLogWriteResponse)
def post_log_finish(
    log_id: UUID,
    payload: NodeLogFinish,
    db: Annotated[Session, Depends(get_db)],
    settings: Annotated[Settings, Depends(get_settings)],
) -> NodeLogWriteResponse:
    log = finish_node_log(db, settings, log_id, payload)
    return NodeLogWriteResponse(execution_id=log.execution_id, log_id=log.id, status=log.status)


@router.post("/executions/{execution_id}/finish", response_model=ExecutionResponse)
def post_execution_finish(
    execution_id: str,
    payload: ExecutionFinish,
    db: Annotated[Session, Depends(get_db)],
) -> ExecutionResponse:
    execution = finish_execution(db, execution_id, payload)
    return ExecutionResponse(execution_id=execution.execution_id, status=execution.status)
```

- [ ] **Step 5: Implement FastAPI app factory**

Create `src/dify_log_system/main.py`:

```python
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.routers import api_logs


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name)
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(api_logs.router)
    return app


app = create_app()
```

Create `src/dify_log_system/routers/__init__.py`:

```python
```

Create `src/dify_log_system/services/__init__.py`:

```python
```

- [ ] **Step 6: Run API tests**

Run:

```bash
pytest tests/test_log_api.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/main.py src/dify_log_system/routers src/dify_log_system/services tests/conftest.py tests/test_log_api.py
git commit -m "feat: add log write api"
```

## Task 6: Query Services And Query API

**Files:**
- Create: `src/dify_log_system/services/queries.py`
- Create: `src/dify_log_system/routers/api_queries.py`
- Modify: `src/dify_log_system/main.py`
- Create: `tests/test_query_api.py`

- [ ] **Step 1: Write failing query API tests**

Create `tests/test_query_api.py`:

```python
def seed_log(client, execution_id="trace-query", status="success", sequence_no=1):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "execution_id": execution_id,
            "workflow_id": "wf-1",
            "workflow_name": "Workflow One",
            "node_id": f"node-{sequence_no}",
            "node_name": f"Node {sequence_no}",
            "node_type": "llm",
            "sequence_no": sequence_no,
            "status": status,
            "input_data": {"value": sequence_no},
            "output_data": {"ok": status == "success"},
        },
    )
    assert response.status_code == 200
    return response.json()


def test_list_executions_contains_node_counts(client):
    seed_log(client, status="failed")

    response = client.get("/api/v1/executions")

    assert response.status_code == 200
    body = response.json()
    assert body["items"][0]["execution_id"] == "trace-query"
    assert body["items"][0]["node_count"] == 1
    assert body["items"][0]["failed_node_count"] == 1


def test_execution_nodes_are_sorted(client):
    seed_log(client, sequence_no=2)
    seed_log(client, sequence_no=1)

    response = client.get("/api/v1/executions/trace-query/nodes")

    assert response.status_code == 200
    names = [item["node_name"] for item in response.json()["items"]]
    assert names == ["Node 1", "Node 2"]
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
pytest tests/test_query_api.py -q
```

Expected: FAIL with `404 Not Found` for `/api/v1/executions`.

- [ ] **Step 3: Implement query service**

Create `src/dify_log_system/services/queries.py`:

```python
from datetime import datetime
from uuid import UUID

from fastapi import HTTPException, status
from sqlalchemy import Select, case, func, select
from sqlalchemy.orm import Session

from dify_log_system.models import NodeLog, WorkflowExecution


def apply_time_filters(query: Select, model, start_time: datetime | None, end_time: datetime | None) -> Select:
    if start_time is not None:
        query = query.where(model.created_at >= start_time)
    if end_time is not None:
        query = query.where(model.created_at <= end_time)
    return query


def list_executions(
    db: Session,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    execution_id: str | None = None,
    workflow_id: str | None = None,
    workflow_name: str | None = None,
    status_value: str | None = None,
    limit: int = 50,
    offset: int = 0,
) -> dict:
    node_counts = (
        select(
            NodeLog.execution_id.label("execution_id"),
            func.count(NodeLog.id).label("node_count"),
            func.sum(case((NodeLog.status == "failed", 1), else_=0)).label("failed_node_count"),
        )
        .group_by(NodeLog.execution_id)
        .subquery()
    )
    query = (
        select(
            WorkflowExecution,
            func.coalesce(node_counts.c.node_count, 0).label("node_count"),
            func.coalesce(node_counts.c.failed_node_count, 0).label("failed_node_count"),
        )
        .outerjoin(node_counts, node_counts.c.execution_id == WorkflowExecution.execution_id)
        .order_by(WorkflowExecution.started_at.desc())
    )
    query = apply_time_filters(query, WorkflowExecution, start_time, end_time)
    if execution_id:
        query = query.where(WorkflowExecution.execution_id == execution_id)
    if workflow_id:
        query = query.where(WorkflowExecution.workflow_id == workflow_id)
    if workflow_name:
        query = query.where(WorkflowExecution.workflow_name.ilike(f"%{workflow_name}%"))
    if status_value:
        query = query.where(WorkflowExecution.status == status_value)
    rows = db.execute(query.limit(limit).offset(offset)).all()
    items = [
        {
            "execution_id": execution.execution_id,
            "workflow_id": execution.workflow_id,
            "workflow_name": execution.workflow_name,
            "status": execution.status,
            "started_at": execution.started_at,
            "finished_at": execution.finished_at,
            "duration_ms": execution.duration_ms,
            "node_count": int(node_count),
            "failed_node_count": int(failed_node_count),
        }
        for execution, node_count, failed_node_count in rows
    ]
    return {"items": items, "limit": limit, "offset": offset}


def get_execution(db: Session, execution_id: str) -> WorkflowExecution:
    execution = db.execute(
        select(WorkflowExecution).where(WorkflowExecution.execution_id == execution_id)
    ).scalar_one_or_none()
    if execution is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Execution not found")
    return execution


def list_execution_nodes(db: Session, execution_id: str) -> dict:
    query = (
        select(NodeLog)
        .where(NodeLog.execution_id == execution_id)
        .order_by(NodeLog.sequence_no.asc().nulls_last(), NodeLog.started_at.asc())
    )
    items = [node_to_dict(row) for row in db.execute(query).scalars().all()]
    return {"items": items}


def get_node_log(db: Session, log_id: UUID) -> dict:
    log = db.execute(select(NodeLog).where(NodeLog.id == log_id)).scalar_one_or_none()
    if log is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node log not found")
    return node_to_dict(log)


def search_node_logs(
    db: Session,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    node_id: str | None = None,
    node_name: str | None = None,
    status_value: str | None = None,
    min_duration_ms: int | None = None,
    limit: int = 50,
    offset: int = 0,
) -> dict:
    query = select(NodeLog).order_by(NodeLog.started_at.desc())
    query = apply_time_filters(query, NodeLog, start_time, end_time)
    if node_id:
        query = query.where(NodeLog.node_id == node_id)
    if node_name:
        query = query.where(NodeLog.node_name.ilike(f"%{node_name}%"))
    if status_value:
        query = query.where(NodeLog.status == status_value)
    if min_duration_ms is not None:
        query = query.where(NodeLog.duration_ms >= min_duration_ms)
    items = [node_to_dict(row) for row in db.execute(query.limit(limit).offset(offset)).scalars().all()]
    return {"items": items, "limit": limit, "offset": offset}


def node_to_dict(log: NodeLog) -> dict:
    return {
        "id": log.id,
        "execution_id": log.execution_id,
        "node_id": log.node_id,
        "node_name": log.node_name,
        "node_type": log.node_type,
        "sequence_no": log.sequence_no,
        "status": log.status,
        "input_data": log.input_data,
        "output_data": log.output_data,
        "error_message": log.error_message,
        "error_detail": log.error_detail,
        "started_at": log.started_at,
        "finished_at": log.finished_at,
        "duration_ms": log.duration_ms,
        "metadata": log.extra_metadata,
    }
```

- [ ] **Step 4: Implement query routes**

Create `src/dify_log_system/routers/api_queries.py`:

```python
from datetime import datetime
from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Query
from sqlalchemy.orm import Session

from dify_log_system.database import get_db
from dify_log_system.security import require_admin
from dify_log_system.services import queries

router = APIRouter(prefix="/api/v1", tags=["queries"], dependencies=[Depends(require_admin)])


@router.get("/executions")
def get_executions(
    db: Annotated[Session, Depends(get_db)],
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    execution_id: str | None = None,
    workflow_id: str | None = None,
    workflow_name: str | None = None,
    status: str | None = None,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0),
):
    return queries.list_executions(
        db, start_time, end_time, execution_id, workflow_id, workflow_name, status, limit, offset
    )


@router.get("/executions/{execution_id}")
def get_execution(execution_id: str, db: Annotated[Session, Depends(get_db)]):
    execution = queries.get_execution(db, execution_id)
    return {
        "execution_id": execution.execution_id,
        "workflow_id": execution.workflow_id,
        "workflow_name": execution.workflow_name,
        "status": execution.status,
        "started_at": execution.started_at,
        "finished_at": execution.finished_at,
        "duration_ms": execution.duration_ms,
        "metadata": execution.extra_metadata,
    }


@router.get("/executions/{execution_id}/nodes")
def get_execution_nodes(execution_id: str, db: Annotated[Session, Depends(get_db)]):
    return queries.list_execution_nodes(db, execution_id)


@router.get("/logs/{log_id}")
def get_log(log_id: UUID, db: Annotated[Session, Depends(get_db)]):
    return queries.get_node_log(db, log_id)


@router.get("/logs")
def get_logs(
    db: Annotated[Session, Depends(get_db)],
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    node_id: str | None = None,
    node_name: str | None = None,
    status: str | None = None,
    min_duration_ms: int | None = None,
    limit: int = Query(default=50, ge=1, le=200),
    offset: int = Query(default=0, ge=0),
):
    return queries.search_node_logs(
        db, start_time, end_time, node_id, node_name, status, min_duration_ms, limit, offset
    )
```

- [ ] **Step 5: Register query router**

Modify `src/dify_log_system/main.py`:

```python
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.routers import api_logs, api_queries


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name)
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(api_logs.router)
    app.include_router(api_queries.router)
    return app


app = create_app()
```

- [ ] **Step 6: Run query tests**

Run:

```bash
pytest tests/test_query_api.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/services/queries.py src/dify_log_system/routers/api_queries.py src/dify_log_system/main.py tests/test_query_api.py
git commit -m "feat: add execution and node query api"
```

## Task 7: Metrics API

**Files:**
- Create: `src/dify_log_system/services/metrics.py`
- Create: `src/dify_log_system/routers/api_metrics.py`
- Modify: `src/dify_log_system/main.py`
- Create: `tests/test_metrics_api.py`

- [ ] **Step 1: Write failing metrics tests**

Create `tests/test_metrics_api.py`:

```python
def create_metric_log(client, execution_id, node_id, status, duration_ms):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "execution_id": execution_id,
            "workflow_id": "wf-metrics",
            "workflow_name": "Metrics Workflow",
            "node_id": node_id,
            "node_name": node_id,
            "status": status,
            "duration_ms": duration_ms,
            "error_message": "bad input" if status == "failed" else None,
        },
    )
    assert response.status_code == 200


def test_metrics_summary_counts_failures(client):
    create_metric_log(client, "trace-1", "node-a", "success", 100)
    create_metric_log(client, "trace-2", "node-b", "failed", 300)

    response = client.get("/api/v1/metrics/summary")

    assert response.status_code == 200
    body = response.json()
    assert body["execution_count"] == 2
    assert body["failed_execution_count"] == 1
    assert body["failure_rate"] == 0.5


def test_slow_nodes_are_sorted(client):
    create_metric_log(client, "trace-1", "node-fast", "success", 100)
    create_metric_log(client, "trace-2", "node-slow", "success", 900)

    response = client.get("/api/v1/metrics/nodes/slow")

    assert response.status_code == 200
    assert response.json()["items"][0]["node_id"] == "node-slow"
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
pytest tests/test_metrics_api.py -q
```

Expected: FAIL with `404 Not Found` for metrics routes.

- [ ] **Step 3: Implement metrics service**

Create `src/dify_log_system/services/metrics.py`:

```python
from datetime import datetime

from sqlalchemy import case, desc, func, select
from sqlalchemy.orm import Session

from dify_log_system.models import NodeLog, WorkflowExecution


def summary(db: Session, start_time: datetime | None = None, end_time: datetime | None = None) -> dict:
    execution_query = select(WorkflowExecution)
    node_query = select(NodeLog)
    if start_time is not None:
        execution_query = execution_query.where(WorkflowExecution.created_at >= start_time)
        node_query = node_query.where(NodeLog.created_at >= start_time)
    if end_time is not None:
        execution_query = execution_query.where(WorkflowExecution.created_at <= end_time)
        node_query = node_query.where(NodeLog.created_at <= end_time)
    executions = execution_query.subquery()
    nodes = node_query.subquery()
    row = db.execute(
        select(
            func.count(executions.c.execution_id),
            func.sum(case((executions.c.status == "failed", 1), else_=0)),
        )
    ).one()
    node_row = db.execute(select(func.avg(nodes.c.duration_ms))).one()
    execution_count = int(row[0] or 0)
    failed_count = int(row[1] or 0)
    return {
        "execution_count": execution_count,
        "failed_execution_count": failed_count,
        "failure_rate": failed_count / execution_count if execution_count else 0,
        "average_node_duration_ms": int(node_row[0] or 0),
    }


def workflow_metrics(db: Session, limit: int = 10) -> dict:
    rows = db.execute(
        select(
            WorkflowExecution.workflow_id,
            WorkflowExecution.workflow_name,
            func.count(WorkflowExecution.id).label("execution_count"),
            func.sum(case((WorkflowExecution.status == "failed", 1), else_=0)).label("failed_count"),
        )
        .group_by(WorkflowExecution.workflow_id, WorkflowExecution.workflow_name)
        .order_by(desc("execution_count"))
        .limit(limit)
    ).all()
    return {
        "items": [
            {
                "workflow_id": workflow_id,
                "workflow_name": workflow_name,
                "execution_count": int(execution_count),
                "failed_count": int(failed_count or 0),
            }
            for workflow_id, workflow_name, execution_count, failed_count in rows
        ]
    }


def slow_nodes(db: Session, limit: int = 10) -> dict:
    rows = db.execute(
        select(
            NodeLog.node_id,
            NodeLog.node_name,
            NodeLog.workflow_name,
            NodeLog.duration_ms,
            NodeLog.execution_id,
        )
        .where(NodeLog.duration_ms.is_not(None))
        .order_by(NodeLog.duration_ms.desc())
        .limit(limit)
    ).all()
    return {
        "items": [
            {
                "node_id": node_id,
                "node_name": node_name,
                "workflow_name": workflow_name,
                "duration_ms": duration_ms,
                "execution_id": execution_id,
            }
            for node_id, node_name, workflow_name, duration_ms, execution_id in rows
        ]
    }


def error_metrics(db: Session, limit: int = 10) -> dict:
    rows = db.execute(
        select(NodeLog.error_message, func.count(NodeLog.id).label("count"))
        .where(NodeLog.status == "failed")
        .group_by(NodeLog.error_message)
        .order_by(desc("count"))
        .limit(limit)
    ).all()
    return {"items": [{"error_message": message or "unknown", "count": int(count)} for message, count in rows]}
```

- [ ] **Step 4: Implement metrics routes**

Create `src/dify_log_system/routers/api_metrics.py`:

```python
from datetime import datetime
from typing import Annotated

from fastapi import APIRouter, Depends, Query
from sqlalchemy.orm import Session

from dify_log_system.database import get_db
from dify_log_system.security import require_admin
from dify_log_system.services import metrics

router = APIRouter(prefix="/api/v1/metrics", tags=["metrics"], dependencies=[Depends(require_admin)])


@router.get("/summary")
def get_summary(
    db: Annotated[Session, Depends(get_db)],
    start_time: datetime | None = None,
    end_time: datetime | None = None,
):
    return metrics.summary(db, start_time, end_time)


@router.get("/workflows")
def get_workflows(db: Annotated[Session, Depends(get_db)], limit: int = Query(default=10, ge=1, le=100)):
    return metrics.workflow_metrics(db, limit)


@router.get("/nodes/slow")
def get_slow_nodes(db: Annotated[Session, Depends(get_db)], limit: int = Query(default=10, ge=1, le=100)):
    return metrics.slow_nodes(db, limit)


@router.get("/errors")
def get_errors(db: Annotated[Session, Depends(get_db)], limit: int = Query(default=10, ge=1, le=100)):
    return metrics.error_metrics(db, limit)
```

- [ ] **Step 5: Register metrics router**

Modify `src/dify_log_system/main.py`:

```python
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.routers import api_logs, api_metrics, api_queries


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name)
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(api_logs.router)
    app.include_router(api_queries.router)
    app.include_router(api_metrics.router)
    return app


app = create_app()
```

- [ ] **Step 6: Run metrics tests**

Run:

```bash
pytest tests/test_metrics_api.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/services/metrics.py src/dify_log_system/routers/api_metrics.py src/dify_log_system/main.py tests/test_metrics_api.py
git commit -m "feat: add metrics api"
```

## Task 8: Excel Export

**Files:**
- Create: `src/dify_log_system/services/export.py`
- Create: `src/dify_log_system/routers/api_export.py`
- Modify: `src/dify_log_system/main.py`
- Create: `tests/test_export_api.py`

- [ ] **Step 1: Write failing export test**

Create `tests/test_export_api.py`:

```python
from io import BytesIO

from openpyxl import load_workbook


def test_export_contains_execution_and_node_sheets(client):
    response = client.post(
        "/api/v1/logs",
        headers={"X-API-Key": "dev-log-api-key"},
        json={
            "execution_id": "trace-export",
            "workflow_name": "Export Workflow",
            "node_id": "node-export",
            "node_name": "Exporter",
            "status": "success",
            "input_data": {"x": 1},
            "output_data": {"y": 2},
        },
    )
    assert response.status_code == 200

    exported = client.get("/api/v1/export/executions.xlsx?execution_id=trace-export")

    assert exported.status_code == 200
    workbook = load_workbook(BytesIO(exported.content))
    assert workbook.sheetnames == ["executions", "node_logs"]
    assert workbook["executions"]["A1"].value == "execution_id"
    assert workbook["executions"]["A2"].value == "trace-export"
    assert workbook["node_logs"]["A2"].value == "trace-export"
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pytest tests/test_export_api.py -q
```

Expected: FAIL with `404 Not Found` for export route.

- [ ] **Step 3: Implement export service**

Create `src/dify_log_system/services/export.py`:

```python
import json
from datetime import datetime
from io import BytesIO

from openpyxl import Workbook
from sqlalchemy import select
from sqlalchemy.orm import Session

from dify_log_system.models import NodeLog, WorkflowExecution


def build_workbook(
    db: Session,
    export_max_rows: int,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    workflow_id: str | None = None,
    workflow_name: str | None = None,
    status_value: str | None = None,
    execution_id: str | None = None,
) -> bytes:
    workbook = Workbook()
    executions_sheet = workbook.active
    executions_sheet.title = "executions"
    node_sheet = workbook.create_sheet("node_logs")
    executions_sheet.append([
        "execution_id", "workflow_id", "workflow_name", "status", "started_at",
        "finished_at", "duration_ms", "node_count", "failed_node_count"
    ])
    node_sheet.append([
        "execution_id", "sequence_no", "node_id", "node_name", "node_type", "status",
        "duration_ms", "error_message", "input_data", "output_data"
    ])

    execution_query = select(WorkflowExecution).order_by(WorkflowExecution.started_at.desc())
    if start_time is not None:
        execution_query = execution_query.where(WorkflowExecution.created_at >= start_time)
    if end_time is not None:
        execution_query = execution_query.where(WorkflowExecution.created_at <= end_time)
    if workflow_id:
        execution_query = execution_query.where(WorkflowExecution.workflow_id == workflow_id)
    if workflow_name:
        execution_query = execution_query.where(WorkflowExecution.workflow_name.ilike(f"%{workflow_name}%"))
    if status_value:
        execution_query = execution_query.where(WorkflowExecution.status == status_value)
    if execution_id:
        execution_query = execution_query.where(WorkflowExecution.execution_id == execution_id)

    executions = db.execute(execution_query.limit(export_max_rows)).scalars().all()
    execution_ids = [execution.execution_id for execution in executions]
    for execution in executions:
        node_logs = db.execute(
            select(NodeLog).where(NodeLog.execution_id == execution.execution_id)
        ).scalars().all()
        failed_count = len([node for node in node_logs if node.status == "failed"])
        executions_sheet.append([
            execution.execution_id,
            execution.workflow_id,
            execution.workflow_name,
            execution.status,
            execution.started_at.isoformat() if execution.started_at else "",
            execution.finished_at.isoformat() if execution.finished_at else "",
            execution.duration_ms,
            len(node_logs),
            failed_count,
        ])

    written_nodes = 0
    if execution_ids:
        node_query = (
            select(NodeLog)
            .where(NodeLog.execution_id.in_(execution_ids))
            .order_by(NodeLog.execution_id.asc(), NodeLog.sequence_no.asc().nulls_last(), NodeLog.started_at.asc())
            .limit(export_max_rows)
        )
        for node in db.execute(node_query).scalars():
            node_sheet.append([
                node.execution_id,
                node.sequence_no,
                node.node_id,
                node.node_name,
                node.node_type,
                node.status,
                node.duration_ms,
                node.error_message,
                json.dumps(node.input_data, ensure_ascii=False),
                json.dumps(node.output_data, ensure_ascii=False),
            ])
            written_nodes += 1
            if written_nodes >= export_max_rows:
                break

    output = BytesIO()
    workbook.save(output)
    return output.getvalue()
```

- [ ] **Step 4: Implement export route**

Create `src/dify_log_system/routers/api_export.py`:

```python
from datetime import datetime
from typing import Annotated

from fastapi import APIRouter, Depends
from fastapi.responses import Response
from sqlalchemy.orm import Session

from dify_log_system.config import Settings, get_settings
from dify_log_system.database import get_db
from dify_log_system.security import require_admin
from dify_log_system.services.export import build_workbook

router = APIRouter(prefix="/api/v1/export", tags=["export"], dependencies=[Depends(require_admin)])


@router.get("/executions.xlsx")
def export_executions(
    db: Annotated[Session, Depends(get_db)],
    settings: Annotated[Settings, Depends(get_settings)],
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    workflow_id: str | None = None,
    workflow_name: str | None = None,
    status: str | None = None,
    execution_id: str | None = None,
) -> Response:
    content = build_workbook(
        db,
        settings.export_max_rows,
        start_time,
        end_time,
        workflow_id,
        workflow_name,
        status,
        execution_id,
    )
    return Response(
        content=content,
        media_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        headers={"Content-Disposition": 'attachment; filename="dify-workflow-logs.xlsx"'},
    )
```

- [ ] **Step 5: Register export router**

Modify `src/dify_log_system/main.py`:

```python
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.routers import api_export, api_logs, api_metrics, api_queries


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name)
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(api_logs.router)
    app.include_router(api_queries.router)
    app.include_router(api_metrics.router)
    app.include_router(api_export.router)
    return app


app = create_app()
```

- [ ] **Step 6: Run export test**

Run:

```bash
pytest tests/test_export_api.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/services/export.py src/dify_log_system/routers/api_export.py src/dify_log_system/main.py tests/test_export_api.py
git commit -m "feat: add excel export"
```

## Task 9: Web Admin Pages

**Files:**
- Create: `src/dify_log_system/routers/web.py`
- Create: `src/dify_log_system/templates/base.html`
- Create: `src/dify_log_system/templates/login.html`
- Create: `src/dify_log_system/templates/executions.html`
- Create: `src/dify_log_system/templates/execution_detail.html`
- Create: `src/dify_log_system/templates/logs.html`
- Create: `src/dify_log_system/templates/metrics.html`
- Modify: `src/dify_log_system/main.py`
- Create: `tests/test_web_pages.py`

- [ ] **Step 1: Write failing web tests**

Create `tests/test_web_pages.py`:

```python
def test_login_page_loads(client):
    response = client.get("/login")

    assert response.status_code == 200
    assert "Dify Workflow Log System" in response.text


def test_admin_login_and_execution_list(client):
    login = client.post("/login", data={"username": "admin", "password": "dev-admin-password"}, follow_redirects=False)

    assert login.status_code == 303

    response = client.get("/")
    assert response.status_code == 200
    assert "Executions" in response.text


def test_execution_page_requires_login(client):
    response = client.get("/", follow_redirects=False)

    assert response.status_code == 303
    assert response.headers["location"] == "/login"
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
pytest tests/test_web_pages.py -q
```

Expected: FAIL with `404 Not Found` for `/login`.

- [ ] **Step 3: Implement web router**

Create `src/dify_log_system/routers/web.py`:

```python
import secrets
from typing import Annotated

from fastapi import APIRouter, Depends, Form, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from sqlalchemy.orm import Session

from dify_log_system.config import Settings, get_settings
from dify_log_system.database import get_db
from dify_log_system.security import is_admin_logged_in
from dify_log_system.services import metrics, queries

router = APIRouter(tags=["web"])
templates = Jinja2Templates(directory="src/dify_log_system/templates")


def redirect_to_login() -> RedirectResponse:
    return RedirectResponse(url="/login", status_code=303)


@router.get("/login", response_class=HTMLResponse)
def login_page(request: Request):
    return templates.TemplateResponse("login.html", {"request": request, "error": None})


@router.post("/login")
def login(
    request: Request,
    settings: Annotated[Settings, Depends(get_settings)],
    username: str = Form(...),
    password: str = Form(...),
):
    if secrets.compare_digest(username, settings.admin_username) and secrets.compare_digest(password, settings.admin_password):
        request.session["admin_logged_in"] = True
        return RedirectResponse(url="/", status_code=303)
    return templates.TemplateResponse("login.html", {"request": request, "error": "用户名或密码错误"}, status_code=401)


@router.post("/logout")
def logout(request: Request):
    request.session.clear()
    return RedirectResponse(url="/login", status_code=303)


@router.get("/", response_class=HTMLResponse)
def executions_page(request: Request, db: Annotated[Session, Depends(get_db)]):
    if not is_admin_logged_in(request):
        return redirect_to_login()
    data = queries.list_executions(db, limit=50, offset=0)
    return templates.TemplateResponse("executions.html", {"request": request, "items": data["items"]})


@router.get("/executions/{execution_id}", response_class=HTMLResponse)
def execution_detail_page(request: Request, execution_id: str, db: Annotated[Session, Depends(get_db)]):
    if not is_admin_logged_in(request):
        return redirect_to_login()
    execution = queries.get_execution(db, execution_id)
    nodes = queries.list_execution_nodes(db, execution_id)["items"]
    return templates.TemplateResponse(
        "execution_detail.html",
        {"request": request, "execution": execution, "nodes": nodes},
    )


@router.get("/logs", response_class=HTMLResponse)
def logs_page(request: Request, db: Annotated[Session, Depends(get_db)]):
    if not is_admin_logged_in(request):
        return redirect_to_login()
    data = queries.search_node_logs(db, limit=50, offset=0)
    return templates.TemplateResponse("logs.html", {"request": request, "items": data["items"]})


@router.get("/metrics", response_class=HTMLResponse)
def metrics_page(request: Request, db: Annotated[Session, Depends(get_db)]):
    if not is_admin_logged_in(request):
        return redirect_to_login()
    return templates.TemplateResponse(
        "metrics.html",
        {
            "request": request,
            "summary": metrics.summary(db),
            "slow_nodes": metrics.slow_nodes(db)["items"],
            "errors": metrics.error_metrics(db)["items"],
            "workflows": metrics.workflow_metrics(db)["items"],
        },
    )
```

- [ ] **Step 4: Add templates**

Create `src/dify_log_system/templates/base.html`:

```html
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Dify Workflow Log System</title>
  <style>
    body { margin: 0; font-family: Arial, sans-serif; color: #1f2937; background: #f8fafc; }
    header { background: #0f172a; color: white; padding: 14px 24px; display: flex; justify-content: space-between; align-items: center; }
    nav a { color: white; margin-right: 16px; text-decoration: none; }
    main { padding: 24px; }
    table { width: 100%; border-collapse: collapse; background: white; }
    th, td { padding: 10px; border-bottom: 1px solid #e5e7eb; text-align: left; vertical-align: top; }
    th { background: #f1f5f9; }
    .status-failed { color: #b91c1c; font-weight: bold; }
    .status-success { color: #047857; font-weight: bold; }
    pre { white-space: pre-wrap; background: #0f172a; color: #e5e7eb; padding: 12px; overflow: auto; }
    .cards { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
    .card { background: white; border: 1px solid #e5e7eb; padding: 16px; border-radius: 6px; }
    button, input { padding: 8px 10px; }
  </style>
</head>
<body>
  <header>
    <div>Dify Workflow Log System</div>
    <nav>
      <a href="/">Executions</a>
      <a href="/logs">Node Logs</a>
      <a href="/metrics">Metrics</a>
      <form action="/logout" method="post" style="display:inline"><button type="submit">Logout</button></form>
    </nav>
  </header>
  <main>{% block content %}{% endblock %}</main>
</body>
</html>
```

Create `src/dify_log_system/templates/login.html`:

```html
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Dify Workflow Log System</title>
  <style>
    body { font-family: Arial, sans-serif; background: #f8fafc; display: grid; place-items: center; min-height: 100vh; margin: 0; }
    form { background: white; border: 1px solid #e5e7eb; padding: 24px; width: 320px; border-radius: 6px; }
    label, input, button { display: block; width: 100%; box-sizing: border-box; margin-top: 10px; }
    input, button { padding: 10px; }
    .error { color: #b91c1c; }
  </style>
</head>
<body>
  <form method="post" action="/login">
    <h1>Dify Workflow Log System</h1>
    {% if error %}<p class="error">{{ error }}</p>{% endif %}
    <label>Username<input name="username"></label>
    <label>Password<input name="password" type="password"></label>
    <button type="submit">Login</button>
  </form>
</body>
</html>
```

Create `src/dify_log_system/templates/executions.html`:

```html
{% extends "base.html" %}
{% block content %}
<h1>Executions</h1>
<p><a href="/api/v1/export/executions.xlsx">Export Excel</a></p>
<table>
  <thead><tr><th>Started</th><th>Execution ID</th><th>Workflow</th><th>Status</th><th>Nodes</th><th>Failed</th><th>Duration</th></tr></thead>
  <tbody>
  {% for item in items %}
    <tr>
      <td>{{ item.started_at }}</td>
      <td><a href="/executions/{{ item.execution_id }}">{{ item.execution_id }}</a></td>
      <td>{{ item.workflow_name or item.workflow_id or "" }}</td>
      <td class="status-{{ item.status }}">{{ item.status }}</td>
      <td>{{ item.node_count }}</td>
      <td>{{ item.failed_node_count }}</td>
      <td>{{ item.duration_ms or "" }}</td>
    </tr>
  {% endfor %}
  </tbody>
</table>
{% endblock %}
```

Create `src/dify_log_system/templates/execution_detail.html`:

```html
{% extends "base.html" %}
{% block content %}
<h1>Execution {{ execution.execution_id }}</h1>
<p>Status: <strong class="status-{{ execution.status }}">{{ execution.status }}</strong></p>
<table>
  <thead><tr><th>Seq</th><th>Node</th><th>Type</th><th>Status</th><th>Duration</th><th>Error</th></tr></thead>
  <tbody>
  {% for node in nodes %}
    <tr>
      <td>{{ node.sequence_no or "" }}</td>
      <td>{{ node.node_name }}</td>
      <td>{{ node.node_type or "" }}</td>
      <td class="status-{{ node.status }}">{{ node.status }}</td>
      <td>{{ node.duration_ms or "" }}</td>
      <td>{{ node.error_message or "" }}</td>
    </tr>
    <tr>
      <td colspan="6">
        <details><summary>Input / Output</summary>
          <h3>Input</h3><pre>{{ node.input_data }}</pre>
          <h3>Output</h3><pre>{{ node.output_data }}</pre>
          {% if node.error_detail %}<h3>Error Detail</h3><pre>{{ node.error_detail }}</pre>{% endif %}
        </details>
      </td>
    </tr>
  {% endfor %}
  </tbody>
</table>
{% endblock %}
```

Create `src/dify_log_system/templates/logs.html`:

```html
{% extends "base.html" %}
{% block content %}
<h1>Node Logs</h1>
<table>
  <thead><tr><th>Started</th><th>Execution</th><th>Node</th><th>Status</th><th>Duration</th><th>Error</th></tr></thead>
  <tbody>
  {% for item in items %}
    <tr>
      <td>{{ item.started_at }}</td>
      <td><a href="/executions/{{ item.execution_id }}">{{ item.execution_id }}</a></td>
      <td>{{ item.node_name }}</td>
      <td class="status-{{ item.status }}">{{ item.status }}</td>
      <td>{{ item.duration_ms or "" }}</td>
      <td>{{ item.error_message or "" }}</td>
    </tr>
  {% endfor %}
  </tbody>
</table>
{% endblock %}
```

Create `src/dify_log_system/templates/metrics.html`:

```html
{% extends "base.html" %}
{% block content %}
<h1>Metrics</h1>
<section class="cards">
  <div class="card"><strong>Executions</strong><br>{{ summary.execution_count }}</div>
  <div class="card"><strong>Failed</strong><br>{{ summary.failed_execution_count }}</div>
  <div class="card"><strong>Failure Rate</strong><br>{{ summary.failure_rate }}</div>
  <div class="card"><strong>Avg Node Duration</strong><br>{{ summary.average_node_duration_ms }} ms</div>
</section>
<h2>Slow Nodes</h2>
<table><thead><tr><th>Node</th><th>Workflow</th><th>Duration</th><th>Execution</th></tr></thead><tbody>
{% for item in slow_nodes %}
<tr><td>{{ item.node_name }}</td><td>{{ item.workflow_name or "" }}</td><td>{{ item.duration_ms }}</td><td>{{ item.execution_id }}</td></tr>
{% endfor %}
</tbody></table>
<h2>Errors</h2>
<table><thead><tr><th>Error</th><th>Count</th></tr></thead><tbody>
{% for item in errors %}
<tr><td>{{ item.error_message }}</td><td>{{ item.count }}</td></tr>
{% endfor %}
</tbody></table>
{% endblock %}
```

- [ ] **Step 5: Register web router**

Modify `src/dify_log_system/main.py`:

```python
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.routers import api_export, api_logs, api_metrics, api_queries, web


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name)
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(api_logs.router)
    app.include_router(api_queries.router)
    app.include_router(api_metrics.router)
    app.include_router(api_export.router)
    app.include_router(web.router)
    return app


app = create_app()
```

- [ ] **Step 6: Run web tests**

Run:

```bash
pytest tests/test_web_pages.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/routers/web.py src/dify_log_system/templates src/dify_log_system/main.py tests/test_web_pages.py
git commit -m "feat: add admin web pages"
```

## Task 10: Retention Job And Health Check

**Files:**
- Create: `src/dify_log_system/services/retention.py`
- Create: `src/dify_log_system/routers/health.py`
- Modify: `src/dify_log_system/main.py`
- Create: `tests/test_retention_and_health.py`

- [ ] **Step 1: Write failing tests**

Create `tests/test_retention_and_health.py`:

```python
from datetime import datetime, timedelta, timezone

from dify_log_system.models import NodeLog, WorkflowExecution
from dify_log_system.services.retention import cleanup_old_logs


def test_health_check_returns_ok(client):
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json()["status"] == "ok"


def test_cleanup_old_logs_removes_old_execution(db_session):
    old_time = datetime.now(timezone.utc) - timedelta(days=120)
    execution = WorkflowExecution(execution_id="old-trace", created_at=old_time, started_at=old_time)
    db_session.add(execution)
    db_session.flush()
    db_session.add(
        NodeLog(
            execution_id="old-trace",
            workflow_execution_id=execution.id,
            node_id="old-node",
            node_name="Old",
            status="success",
            created_at=old_time,
            started_at=old_time,
        )
    )
    db_session.commit()

    result = cleanup_old_logs(db_session, retention_days=90)

    assert result["deleted_executions"] == 1
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
pytest tests/test_retention_and_health.py -q
```

Expected: FAIL with missing retention service and `/health` route.

- [ ] **Step 3: Implement retention service**

Create `src/dify_log_system/services/retention.py`:

```python
from datetime import datetime, timedelta, timezone

from sqlalchemy import delete, select
from sqlalchemy.orm import Session

from dify_log_system.models import NodeLog, NodeLogEvent, WorkflowExecution


def cleanup_old_logs(db: Session, retention_days: int) -> dict:
    cutoff = datetime.now(timezone.utc) - timedelta(days=retention_days)
    old_execution_ids = [
        row[0]
        for row in db.execute(
            select(WorkflowExecution.execution_id).where(WorkflowExecution.created_at < cutoff)
        ).all()
    ]
    if not old_execution_ids:
        return {"deleted_executions": 0, "deleted_node_logs": 0, "deleted_events": 0}
    deleted_events = db.execute(
        delete(NodeLogEvent).where(NodeLogEvent.execution_id.in_(old_execution_ids))
    ).rowcount or 0
    deleted_nodes = db.execute(
        delete(NodeLog).where(NodeLog.execution_id.in_(old_execution_ids))
    ).rowcount or 0
    deleted_executions = db.execute(
        delete(WorkflowExecution).where(WorkflowExecution.execution_id.in_(old_execution_ids))
    ).rowcount or 0
    db.commit()
    return {
        "deleted_executions": deleted_executions,
        "deleted_node_logs": deleted_nodes,
        "deleted_events": deleted_events,
    }
```

- [ ] **Step 4: Implement health route**

Create `src/dify_log_system/routers/health.py`:

```python
from typing import Annotated

from fastapi import APIRouter, Depends
from sqlalchemy import text
from sqlalchemy.orm import Session

from dify_log_system.database import get_db

router = APIRouter(tags=["health"])


@router.get("/health")
def health(db: Annotated[Session, Depends(get_db)]):
    db.execute(text("select 1"))
    return {"status": "ok", "database": "ok"}
```

- [ ] **Step 5: Register health route and scheduler lifecycle**

Modify `src/dify_log_system/main.py`:

```python
from contextlib import asynccontextmanager

from apscheduler.schedulers.background import BackgroundScheduler
from fastapi import FastAPI
from starlette.middleware.sessions import SessionMiddleware

from dify_log_system.config import get_settings
from dify_log_system.database import SessionLocal
from dify_log_system.routers import api_export, api_logs, api_metrics, api_queries, health, web
from dify_log_system.services.retention import cleanup_old_logs


def run_retention_job() -> None:
    settings = get_settings()
    if not settings.log_retention_enabled:
        return
    with SessionLocal() as db:
        cleanup_old_logs(db, settings.log_retention_days)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    scheduler = None
    if settings.log_retention_enabled and app.state.start_scheduler:
        scheduler = BackgroundScheduler(timezone=settings.app_timezone)
        scheduler.add_job(run_retention_job, "interval", days=1, id="retention-cleanup", replace_existing=True)
        scheduler.start()
    yield
    if scheduler is not None:
        scheduler.shutdown(wait=False)


def create_app(start_scheduler: bool = True) -> FastAPI:
    settings = get_settings()
    app = FastAPI(title=settings.app_name, lifespan=lifespan)
    app.state.start_scheduler = start_scheduler
    app.add_middleware(SessionMiddleware, secret_key=settings.session_secret_key)
    app.include_router(health.router)
    app.include_router(api_logs.router)
    app.include_router(api_queries.router)
    app.include_router(api_metrics.router)
    app.include_router(api_export.router)
    app.include_router(web.router)
    return app


app = create_app()
```

- [ ] **Step 6: Run tests**

Run:

```bash
pytest tests/test_retention_and_health.py -q
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add src/dify_log_system/services/retention.py src/dify_log_system/routers/health.py src/dify_log_system/main.py tests/test_retention_and_health.py
git commit -m "feat: add retention cleanup and health check"
```

## Task 11: Docker Compose And Runtime Docs

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `README.md`
- Modify: `.env.example`

- [ ] **Step 1: Add Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential libpq-dev \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml ./
COPY src ./src
COPY alembic.ini ./
COPY alembic ./alembic

RUN pip install --no-cache-dir -e .

EXPOSE 8000

CMD ["sh", "-c", "alembic upgrade head && uvicorn dify_log_system.main:app --host 0.0.0.0 --port 8000"]
```

- [ ] **Step 2: Add Docker Compose**

Create `docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: dify_log
      POSTGRES_PASSWORD: dify_log
      POSTGRES_DB: dify_log
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dify_log -d dify_log"]
      interval: 5s
      timeout: 5s
      retries: 10
    ports:
      - "5432:5432"

  app:
    build: .
    env_file:
      - .env
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "8000:8000"

volumes:
  postgres_data:
```

- [ ] **Step 3: Ensure `.env.example` uses container database host**

Replace `.env.example` with:

```env
DATABASE_URL=postgresql+psycopg://dify_log:dify_log@postgres:5432/dify_log
LOG_API_KEY=dev-log-api-key
ADMIN_USERNAME=admin
ADMIN_PASSWORD=dev-admin-password
SESSION_SECRET_KEY=dev-session-secret
MASK_FIELDS=password,token,api_key,phone
LOG_RETENTION_ENABLED=true
LOG_RETENTION_DAYS=90
EXPORT_MAX_ROWS=50000
APP_TIMEZONE=Asia/Shanghai
```

- [ ] **Step 4: Add README**

Create `README.md`:

```markdown
# Dify Workflow Log System

Python + PostgreSQL service for recording Dify workflow node inputs, outputs, status, timing, errors, and analysis data linked by `execution_id`.

## Run With Docker Compose

```bash
cp .env.example .env
docker compose up --build
```

Open:

- Web admin: http://localhost:8000/
- Health: http://localhost:8000/health

Default development login:

- Username: `admin`
- Password: `dev-admin-password`

## Dify HTTP Node Example

URL:

```http
POST http://app:8000/api/v1/logs
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

## Start And Finish Mode

Start:

```http
POST /api/v1/logs/start
```

Finish:

```http
POST /api/v1/logs/{log_id}/finish
```

## Excel Export

```http
GET /api/v1/export/executions.xlsx?start_time=2026-06-29T00:00:00%2B08:00&end_time=2026-06-30T00:00:00%2B08:00
```

The workbook contains:

- `executions`
- `node_logs`

## Configuration

| Variable | Meaning |
| --- | --- |
| `DATABASE_URL` | SQLAlchemy PostgreSQL connection URL |
| `LOG_API_KEY` | Dify write API key |
| `ADMIN_USERNAME` | Web admin username |
| `ADMIN_PASSWORD` | Web admin password |
| `SESSION_SECRET_KEY` | Cookie session signing key |
| `MASK_FIELDS` | Comma-separated JSON field names masked before persistence |
| `LOG_RETENTION_ENABLED` | Enables daily cleanup job |
| `LOG_RETENTION_DAYS` | Retention window in days |
| `EXPORT_MAX_ROWS` | Max rows for Excel node detail export |
| `APP_TIMEZONE` | Scheduler timezone |
```

- [ ] **Step 5: Validate Docker configuration**

Run:

```bash
docker compose config
```

Expected: command exits with code 0 and prints normalized compose config.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile docker-compose.yml README.md .env.example
git commit -m "chore: add docker compose deployment"
```

## Task 12: Full Verification And Fix Pass

**Files:**
- Modify only files required by failing checks.

- [ ] **Step 1: Run the full Python test suite**

Run:

```bash
pytest
```

Expected: PASS.

- [ ] **Step 2: Run lint**

Run:

```bash
ruff check .
```

Expected: PASS.

- [ ] **Step 3: Run Alembic upgrade against Docker PostgreSQL**

Run:

```bash
cp .env.example .env
docker compose up -d postgres
DATABASE_URL=postgresql+psycopg://dify_log:dify_log@localhost:5432/dify_log alembic upgrade head
```

Expected: `Running upgrade  -> 20260629_0001`.

- [ ] **Step 4: Run app container**

Run:

```bash
docker compose up --build -d app
curl -s http://localhost:8000/health
```

Expected:

```json
{"status":"ok","database":"ok"}
```

- [ ] **Step 5: Smoke-test write API**

Run:

```bash
curl -s -X POST http://localhost:8000/api/v1/logs \
  -H "X-API-Key: dev-log-api-key" \
  -H "Content-Type: application/json" \
  -d '{"execution_id":"smoke-trace","node_id":"smoke-node","node_name":"Smoke Node","status":"success","input_data":{"token":"secret"},"output_data":{"ok":true}}'
```

Expected response contains:

```json
{"execution_id":"smoke-trace"
```

- [ ] **Step 6: Smoke-test query API**

Run:

```bash
curl -s http://localhost:8000/api/v1/executions/smoke-trace/nodes
```

Expected response contains:

```json
"node_name":"Smoke Node"
```

- [ ] **Step 7: Smoke-test Excel export**

Run:

```bash
curl -s -o /tmp/dify-workflow-logs.xlsx "http://localhost:8000/api/v1/export/executions.xlsx?execution_id=smoke-trace"
python - <<'PY'
from openpyxl import load_workbook
wb = load_workbook('/tmp/dify-workflow-logs.xlsx')
print(wb.sheetnames)
print(wb['executions']['A2'].value)
PY
```

Expected:

```text
['executions', 'node_logs']
smoke-trace
```

- [ ] **Step 8: Commit verification fixes**

If Step 1 through Step 7 required code changes, run:

```bash
git add .
git commit -m "fix: complete verification fixes"
```

If no files changed, run:

```bash
git status --short
```

Expected: no output.

## Self-Review Notes

- Spec coverage: this plan covers write APIs, `execution_id` generation and reuse, start/finish mode, PostgreSQL schema, node events, masking, admin login, query pages, metrics, Excel export, retention, health check, Docker Compose, README, and verification.
- Scope: one deployable service remains a single cohesive implementation plan.
- Type consistency: request models use `metadata`; SQLAlchemy models use `extra_metadata` mapped to the database column named `metadata`, avoiding SQLAlchemy's reserved declarative `metadata` attribute.
- PostgreSQL partitioning: migration creates `node_logs` as a range-partitioned table with a default partition; runtime partition creation is guarded so SQLite tests do not execute PostgreSQL DDL.
