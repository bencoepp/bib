# Jobs and Tasks Guide

This document explains the job execution system in Bib, including Tasks, Jobs, Pipelines, and the CEL-based instruction system.

---

## Overview

The bib job system enables distributed data processing through:

- **Tasks** - Reusable templates of CEL instructions
- **Jobs** - Runtime instances that execute Tasks
- **Pipelines** - Collections of Jobs with dependencies (DAG workflows)
- **Instructions** - Individual operations within Tasks

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Job System                              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐  │
│  │    Task      │────►│     Job      │────►│   Worker    │  │
│  │  (Template)  │     │  (Instance)  │     │ (Executor)  │  │
│  └──────────────┘     └──────────────┘     └─────────────┘  │
│         │                    │                    │          │
│         ▼                    ▼                    ▼          │
│  ┌──────────────┐     ┌──────────────┐     ┌─────────────┐  │
│  │ Instructions │     │   Schedule   │     │   Output    │  │
│  │   (CEL ops)  │     │  (when/how)  │     │  (Dataset)  │  │
│  └──────────────┘     └──────────────┘     └─────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Tasks

A **Task** is a reusable sequence of Instructions that defines what operations to perform.

### Task Structure

```yaml
id: "fetch-weather"
name: "Fetch Weather Data"
description: "Downloads weather data from API and transforms it"
version: "1.0.0"
tags:
  - weather
  - etl

input_schema: |
  {
    "type": "object",
    "properties": {
      "api_key": {"type": "string"},
      "location": {"type": "string"}
    },
    "required": ["api_key", "location"]
  }

output_schema: |
  {
    "type": "object",
    "properties": {
      "temperature": {"type": "number"},
      "humidity": {"type": "number"}
    }
  }

instructions:
  - id: fetch
    operation: http.get
    params:
      url: "https://api.weather.com/v1/${location}"
      headers:
        Authorization: "Bearer ${api_key}"
    output_var: response
    on_error: retry
    retry_count: 3
    description: "Fetch weather data from API"

  - id: parse
    operation: json.parse
    params:
      input: "${response.body}"
    output_var: data
    description: "Parse JSON response"

  - id: transform
    operation: transform.map
    params:
      input: "${data}"
      expression: |
        {
          "temperature": item.temp_c,
          "humidity": item.humidity
        }
    output_var: result
    description: "Transform to standard format"

  - id: store
    operation: output.store
    params:
      data: "${result}"
      format: "json"
    description: "Store result"
```

### Task Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | TaskID | Unique identifier |
| `name` | string | Human-readable name |
| `description` | string | What the task does |
| `version` | string | Semantic version |
| `instructions` | []Instruction | Ordered instruction sequence |
| `input_schema` | string | JSON Schema for inputs |
| `output_schema` | string | JSON Schema for outputs |
| `created_by` | UserID | Creator |
| `tags` | []string | Labels for categorization |

---

## Instructions

**Instructions** are individual CEL-based operations. Each instruction performs a specific, safe operation.

### Instruction Structure

```yaml
id: "step-1"
operation: http.get
params:
  url: "https://api.example.com/data"
  headers:
    Accept: application/json
output_var: response
condition: "${should_fetch == true}"
on_error: retry
retry_count: 3
description: "Fetch data from API"
```

### Instruction Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | string | Optional identifier |
| `operation` | InstructionOp | CEL operation type |
| `params` | map[string]any | Operation parameters |
| `output_var` | string | Variable to store result |
| `condition` | string | CEL condition for execution |
| `on_error` | string | Error handling: fail/skip/retry |
| `retry_count` | int | Retries if on_error="retry" |
| `description` | string | Human-readable description |

### Available Operations

#### HTTP Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `http.get` | HTTP GET request | `url`, `headers`, `timeout` |
| `http.post` | HTTP POST request | `url`, `headers`, `body`, `timeout` |

**Example:**
```yaml
operation: http.get
params:
  url: "https://api.example.com/data"
  headers:
    Authorization: "Bearer ${token}"
  timeout: 30s
output_var: response
```

#### File Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `file.read` | Read file contents | `path` |
| `file.write` | Write to file | `path`, `data` |
| `file.delete` | Delete file | `path` |
| `file.list` | List directory | `path`, `pattern` |
| `file.move` | Move file | `from`, `to` |
| `file.copy` | Copy file | `from`, `to` |
| `file.exists` | Check if file exists | `path` |
| `file.size` | Get file size | `path` |
| `file.hash` | Calculate file hash | `path`, `algorithm` |
| `file.unzip` | Extract ZIP archive | `path`, `destination` |
| `file.untar` | Extract TAR archive | `path`, `destination` |
| `file.gunzip` | Decompress gzip | `path`, `destination` |
| `file.bunzip2` | Decompress bzip2 | `path`, `destination` |

**Example:**
```yaml
operation: file.unzip
params:
  path: "${downloaded_file}"
  destination: "/tmp/extracted"
output_var: extracted_files
```

#### Parse Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `csv.parse` | Parse CSV data | `input`, `delimiter`, `has_header` |
| `json.parse` | Parse JSON data | `input` |
| `xml.parse` | Parse XML data | `input` |

**Example:**
```yaml
operation: csv.parse
params:
  input: "${file_content}"
  delimiter: ","
  has_header: true
output_var: parsed_data
```

#### Transform Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `transform.map` | Map over items | `input`, `expression` |
| `transform.filter` | Filter items | `input`, `expression` |
| `transform.reduce` | Reduce to value | `input`, `expression`, `initial` |
| `transform.flatten` | Flatten nested lists | `input` |
| `transform.sort` | Sort items | `input`, `key`, `order` |
| `transform.unique` | Remove duplicates | `input`, `key` |
| `transform.group` | Group by key | `input`, `key` |
| `transform.join` | Join datasets | `left`, `right`, `on` |

**Example:**
```yaml
operation: transform.filter
params:
  input: "${data}"
  expression: "item.value > 100"
output_var: filtered_data
```

#### Validation Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `validate.schema` | Validate JSON schema | `input`, `schema` |
| `validate.not_null` | Check not null | `input`, `fields` |
| `validate.range` | Validate value range | `input`, `min`, `max` |
| `validate.regex` | Match regex pattern | `input`, `pattern` |
| `validate.type` | Check data type | `input`, `type` |
| `validate.checksum` | Verify checksum | `input`, `expected`, `algorithm` |

**Example:**
```yaml
operation: validate.schema
params:
  input: "${data}"
  schema: |
    {
      "type": "object",
      "required": ["id", "name"]
    }
output_var: is_valid
```

#### String Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `string.split` | Split string | `input`, `delimiter` |
| `string.join` | Join strings | `input`, `delimiter` |
| `string.replace` | Replace substring | `input`, `old`, `new` |
| `string.trim` | Trim whitespace | `input` |
| `string.lower` | Convert to lowercase | `input` |
| `string.upper` | Convert to uppercase | `input` |

#### Control Flow Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `control.if` | Conditional execution | `condition`, `then`, `else` |
| `control.foreach` | Loop over items | `items`, `instructions` |
| `control.retry` | Retry with backoff | `instructions`, `max_retries` |
| `control.sleep` | Pause execution | `duration` |

**Example:**
```yaml
operation: control.foreach
params:
  items: "${urls}"
  instructions:
    - operation: http.get
      params:
        url: "${item}"
      output_var: "responses[${index}]"
```

#### Variable Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `var.set` | Set variable | `name`, `value` |
| `var.get` | Get variable | `name` |
| `var.delete` | Delete variable | `name` |

#### Output Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `output.store` | Store final output | `data`, `format` |
| `output.append` | Append to output | `data` |

---

## Jobs

A **Job** is a runtime instance that executes a Task or inline instructions.

### Job Structure

```yaml
id: "job-12345"
type: etl
status: pending
task_id: "fetch-weather"
execution_mode: goroutine

inputs:
  - name: api_key
    value: "secret-key"
  - name: location
    value: "london"

outputs:
  - name: result
    dataset_id: "weather-london"
    create_new_version: true
    version_message: "Daily weather update"

schedule:
  type: cron
  cron_expr: "0 6 * * *"
  timezone: "UTC"
  enabled: true

resource_limits:
  max_memory_mb: 512
  max_cpu_cores: 1.0
  timeout_seconds: 300
  max_retries: 3

priority: 5
created_by: "user-abc"
```

### Job Properties

| Property | Type | Description |
|----------|------|-------------|
| `id` | JobID | Unique identifier |
| `type` | JobType | Job category |
| `status` | JobStatus | Current status |
| `task_id` | TaskID | Task to execute |
| `inline_instructions` | []Instruction | Alternative to task_id |
| `execution_mode` | ExecutionMode | How job runs |
| `schedule` | *Schedule | When/how often to run |
| `inputs` | []JobInput | Job inputs |
| `outputs` | []JobOutput | Job outputs |
| `dependencies` | []JobID | Jobs that must complete first |
| `resource_limits` | *ResourceLimits | Execution constraints |
| `priority` | int | Higher = more urgent |

### Job Types

| Type | Description |
|------|-------------|
| `scrape` | Web scraping |
| `transform` | Data transformation |
| `clean` | Data cleaning |
| `analyze` | Data analysis |
| `ml` | Machine learning |
| `etl` | Extract-Transform-Load |
| `ingest` | Data ingestion |
| `export` | Data export |
| `custom` | Custom job type |

### Job Status

| Status | Terminal | Description |
|--------|----------|-------------|
| `pending` | No | Created, not yet queued |
| `queued` | No | In execution queue |
| `running` | No | Currently executing |
| `completed` | Yes | Successfully finished |
| `failed` | Yes | Failed with error |
| `cancelled` | Yes | Cancelled by user |
| `waiting` | No | Waiting for dependencies |
| `retrying` | No | Retrying after failure |

### Execution Modes

| Mode | Description |
|------|-------------|
| `goroutine` | Run as goroutine in bibd process |
| `container` | Run in Docker/Podman container |
| `pod` | Run in Kubernetes pod |

### Resource Limits

```yaml
resource_limits:
  max_memory_mb: 512       # Max memory usage
  max_cpu_cores: 1.0       # Max CPU cores
  timeout_seconds: 300     # Max execution time
  max_retries: 3           # Retry attempts
  max_output_size_mb: 100  # Max output size
```

---

## Schedules

A **Schedule** defines when and how often a Job runs.

### Schedule Types

| Type | Description |
|------|-------------|
| `once` | Run exactly once |
| `cron` | Run on cron schedule |
| `repeat` | Run N times |
| `interval` | Run at fixed intervals |

### Schedule Configuration

```yaml
# Once
schedule:
  type: once

# Cron (every day at 6 AM UTC)
schedule:
  type: cron
  cron_expr: "0 6 * * *"
  timezone: "UTC"
  enabled: true

# Repeat (run 5 times)
schedule:
  type: repeat
  repeat_count: 5
  interval: 1h

# Interval (every 30 minutes)
schedule:
  type: interval
  interval: 30m
  start_at: "2024-01-15T00:00:00Z"
  end_at: "2024-12-31T23:59:59Z"
```

### Schedule Properties

| Property | Type | Description |
|----------|------|-------------|
| `type` | ScheduleType | Schedule type |
| `cron_expr` | string | Cron expression |
| `repeat_count` | int | Number of repeats |
| `interval` | duration | Time between runs |
| `start_at` | *time.Time | When schedule activates |
| `end_at` | *time.Time | When schedule expires |
| `timezone` | string | Timezone for cron |
| `enabled` | bool | Is schedule active |
| `run_count` | int | Completed runs |
| `next_run_at` | *time.Time | Next scheduled run |
| `last_run_at` | *time.Time | Last run time |

---

## Pipelines

A **Pipeline** is a collection of Jobs with dependencies, forming a DAG (Directed Acyclic Graph) workflow.

### Pipeline Structure

```yaml
id: "etl-pipeline"
name: "Daily ETL Pipeline"
description: "Complete daily data processing"
created_by: "user-abc"

jobs:
  - id: extract
    type: scrape
    task_id: "fetch-data"
    
  - id: transform
    type: transform
    task_id: "clean-data"
    dependencies:
      - extract
    
  - id: load
    type: ingest
    task_id: "load-warehouse"
    dependencies:
      - transform
    
  - id: analyze
    type: analyze
    task_id: "run-analytics"
    dependencies:
      - load
```

### Pipeline Visualization

```
┌─────────┐
│ extract │
└────┬────┘
     │
     ▼
┌───────────┐
│ transform │
└─────┬─────┘
      │
      ▼
┌──────┐
│ load │
└───┬──┘
    │
    ▼
┌─────────┐
│ analyze │
└─────────┘
```

### Parallel Execution

Jobs without dependencies can run in parallel:

```yaml
jobs:
  - id: fetch-weather
    task_id: "fetch-weather"
    
  - id: fetch-stocks
    task_id: "fetch-stocks"
    
  - id: merge
    task_id: "merge-data"
    dependencies:
      - fetch-weather
      - fetch-stocks
```

```
┌──────────────┐     ┌─────────────┐
│ fetch-weather│     │ fetch-stocks│
└──────┬───────┘     └──────┬──────┘
       │                    │
       └────────┬───────────┘
                │
                ▼
          ┌─────────┐
          │  merge  │
          └─────────┘
```

### DAG Validation

Pipelines validate for cycles before execution:

```yaml
# INVALID - cyclic dependency
jobs:
  - id: a
    dependencies: [c]
  - id: b
    dependencies: [a]
  - id: c
    dependencies: [b]  # Creates cycle: a -> c -> b -> a
```

---

## Job Inputs and Outputs

### Inputs

Jobs can receive inputs from:

1. **Literal values**
```yaml
inputs:
  - name: api_key
    value: "secret-key"
```

2. **Dataset references**
```yaml
inputs:
  - name: source_data
    dataset_id: "raw-data"
    version_id: "v1.0.0"  # Optional, defaults to latest
```

### Outputs

Jobs can produce outputs to:

1. **New dataset version**
```yaml
outputs:
  - name: result
    dataset_id: "processed-data"
    create_new_version: true
    version_message: "Processed on 2024-01-15"
```

2. **Existing dataset (append)**
```yaml
outputs:
  - name: result
    dataset_id: "aggregated-data"
    create_new_version: false  # Append to existing
```

---

## Example Workflows

### Simple ETL

```yaml
# Task: Transform CSV to JSON
id: "csv-to-json"
name: "CSV to JSON Converter"
version: "1.0.0"

instructions:
  - operation: file.read
    params:
      path: "${input_file}"
    output_var: raw_content

  - operation: csv.parse
    params:
      input: "${raw_content}"
      has_header: true
    output_var: data

  - operation: output.store
    params:
      data: "${data}"
      format: json
```

### Web Scraping

```yaml
id: "scrape-news"
name: "News Scraper"
version: "1.0.0"

instructions:
  - operation: http.get
    params:
      url: "https://news.example.com/api/latest"
    output_var: response
    on_error: retry
    retry_count: 3

  - operation: json.parse
    params:
      input: "${response.body}"
    output_var: articles

  - operation: transform.filter
    params:
      input: "${articles}"
      expression: "item.published_date > '2024-01-01'"
    output_var: recent_articles

  - operation: output.store
    params:
      data: "${recent_articles}"
      format: json
```

### Data Validation Pipeline

```yaml
id: "validate-data"
name: "Data Quality Check"
version: "1.0.0"

instructions:
  - operation: validate.not_null
    params:
      input: "${data}"
      fields: ["id", "name", "email"]
    output_var: null_check

  - operation: control.if
    params:
      condition: "${null_check.has_errors}"
      then:
        - operation: var.set
          params:
            name: status
            value: "failed"
      else:
        - operation: validate.regex
          params:
            input: "${data}"
            field: email
            pattern: "^[a-zA-Z0-9+_.-]+@[a-zA-Z0-9.-]+$"
          output_var: email_check

  - operation: output.store
    params:
      data:
        valid: "${!null_check.has_errors && !email_check.has_errors}"
        errors: "${merge(null_check.errors, email_check.errors)}"
```

---

## Best Practices

### Task Design

1. **Single responsibility** - One task, one purpose
2. **Reusability** - Design tasks to be reused with different inputs
3. **Idempotency** - Tasks should be safe to re-run
4. **Error handling** - Use `on_error` and `retry_count` appropriately

### Job Management

1. **Resource limits** - Always set appropriate limits
2. **Timeouts** - Prevent runaway jobs
3. **Dependencies** - Use pipelines for complex workflows
4. **Monitoring** - Track job status and progress

### Security

1. **Input validation** - Validate inputs with schemas
2. **No shell execution** - CEL operations are sandboxed
3. **Secret management** - Use secret references, not inline values
4. **Output verification** - Validate outputs before storing

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Domain Entities](../concepts/domain-entities.md) | Job, Task, and Instruction definitions |
| [CLI Reference](cli-reference.md) | Job and task CLI commands |
| [Configuration](../getting-started/configuration.md) | Job execution configuration |
| [Architecture Overview](../concepts/architecture.md) | System components overview |

