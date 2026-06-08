# DB Patrol Go 重写计划

## 摘要

将现有 Python 数据库巡检工具完全重写为 Go 实现，解决离线环境部署问题。Go 版本将编译为单个静态二进制文件，无需 Python 运行时和依赖，支持在 Windows 上交叉编译到银河麒麟 ARM64 服务器。

**关键决策**: 目标架构 `linux/arm64`，保留 HTML 报告功能（Go `text/html/template` 重写模板）。

---

## 现状分析

### 现有架构 (Python)

| 模块 | 文件 | 职责 |
|------|------|------|
| CLI 入口 | `main.py` | Click 参数解析、配置加载 |
| 核心控制器 | `db_inspector/core.py` | DBInspector 类，遍历数据库执行巡检 |
| 连接工厂 | `db_inspector/connection.py` | DatabaseConnection ABC，VastbasePG/MySQL 实现 |
| 配置构建 | `db_inspector/config_builder.py` | JSON/CLI 参数解析、验证 |
| 巡检器基类 | `db_inspector/inspectors/base.py` | BaseInspector ABC |
| 基本信息巡检 | `db_inspector/inspectors/basic_info.py` | 数据库/表信息采集、备份检测、主键缺失检测 |
| 性能巡检 | `db_inspector/inspectors/performance.py` | 连接、缓存/索引命中率、锁、慢查询、死元组、VACUUM |
| 设计规范巡检 | `db_inspector/inspectors/schema.py` | 命名规范、主键、索引、约束、数据类型、注释、大表 |
| 巡检器注册表 | `db_inspector/inspectors/__init__.py` | 注册表 + get_inspectors() 工厂 |
| 健康评分 | `db_inspector/reporters/scoring.py` | 100分制评分引擎 + 关键发现生成 |
| HTML 报告 | `db_inspector/reporters/html_reporter.py` | Jinja2 模板渲染 |
| Reporter 注册表 | `db_inspector/reporters/__init__.py` | 注册表 + create_reporter() 工厂 |
| 工具函数 | `db_inspector/utils.py` | format_size() |

### 核心依赖映射 (Python → Go)

| Python 库 | Go 替代 | 说明 |
|-----------|---------|------|
| `psycopg2` / `pymysql` | `github.com/lib/pq` + `database/sql` | PG 驱动；MySQL 用 `github.com/go-sql-driver/mysql` |
| `click` | `github.com/spf13/cobra` | CLI 框架，支持子命令、参数、环境变量 |
| `jinja2` | `html/template` + `text/template` | Go 标准库模板引擎 |
| `pyyaml` | `gopkg.in/yaml.v3` | YAML 解析 |
| `tabulate` | 自研或 `github.com/olekukonko/tablewriter` | 表格输出 |
| `colorama` | `github.com/fatih/color` | 终端彩色输出 |
| `concurrent.futures` | `sync.WaitGroup` + goroutines | Go 原生并发 |

---

## 计划变更

### 1. 项目初始化

**文件**: `go.mod`, `main.go`

**操作**:
```bash
cd d:\db-patrol
go mod init db-patrol
```

**依赖**:
```
github.com/spf13/cobra v1.8.0
github.com/lib/pq v1.10.9
github.com/go-sql-driver/mysql v1.7.1
gopkg.in/yaml.v3 v3.0.1
github.com/fatih/color v1.16.0
github.com/olekukonko/tablewriter v0.0.5
```

---

### 2. 目录结构

```
db-patrol/
├── main.go                      # CLI 入口 (cobra)
├── config.yaml                  # 巡检配置（保留）
├── go.mod
├── go.sum
├── cmd/
│   └── root.go                  # cobra RootCmd 定义、参数绑定
├── internal/
│   ├── config/
│   │   ├── config.go            # 配置结构体 + YAML 加载
│   │   └── builder.go           # CLI 参数 → DBConfig, JSON 解析, 验证
│   ├── connection/
│   │   ├── connection.go        # Connection 接口
│   │   ├── pg.go                # PostgresConnection (pq)
│   │   └── mysql.go             # MySQLConnection (go-sql-driver)
│   ├── inspector/
│   │   ├── inspector.go         # Inspector 接口 + 注册表
│   │   ├── basic.go             # BasicInfoInspector
│   │   ├── performance.go       # PerformanceInspector
│   │   └── schema.go            # SchemaInspector
│   ├── reporter/
│   │   ├── reporter.go          # Reporter 接口 + 注册表
│   │   ├── scoring.go           # 健康评分引擎（Python 逻辑直译）
│   │   ├── html.go              # HTMLReporter (template)
│   │   ├── markdown.go          # MarkdownReporter
│   │   └── json.go              # JSONReporter
│   ├── models/
│   │   └── models.go            # 共享数据结构（DBConfig, InspectionResult 等）
│   └── utils/
│       └── utils.go             # formatSize, formatDuration 等
└── templates/
    └── report.html.tmpl         # HTML 报告模板（Jinja2 → Go template 语法转换）
```

---

### 3. 核心模块详细设计

#### 3.1 `internal/models/models.go`

定义所有共享结构体。将 Python dict 转为 Go struct，便于类型安全和模板渲染。

**关键结构体**:
- `DBConfig` - 数据库连接配置
- `InspectionConfig` - 巡检配置（checks 开关、阈值）
- `ReportConfig` - 报告配置（format, output_dir）
- `AppConfig` - 顶层配置，包含以上三个
- `BasicInfoResult` - basic_info 巡检结果
- `PerformanceResult` - performance 巡检结果
- `SchemaResult` - schema 巡检结果
- `HealthScore` - 健康评分结果
- `KeyFinding` - 关键发现
- `TableInfo`, `DatabaseInfo`, `ConnectionStatus` 等

**注意**: 所有数值字段使用 `interface{}` 或 `sql.Null*` 类型以兼容 PG/MySQL 的不同返回类型。

#### 3.2 `internal/config/config.go`

- 加载 `config.yaml`（`gopkg.in/yaml.v3`）
- 结构体标签 `yaml:"xxx"`
- 支持从嵌入文件系统读取（Go 1.16+ `embed`），方便单二进制分发

#### 3.3 `internal/config/builder.go`

将 Python `config_builder.py` 的逻辑直译：
- `ParseDBJSON(jsonStr string) ([]DBConfig, error)`
- `ResolveEnvPasswords(dbs []DBConfig) []string`
- `ValidateDBConfigs(dbs []DBConfig) []string`
- `BuildSingleDBConfig(...)` - 从 CLI flag 构建单条配置

#### 3.4 `internal/connection/`

**接口设计**:
```go
type Connection interface {
    Connect() error
    ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error)
    Execute(query string, args ...interface{}) error
    Close() error
}
```

**实现**:
- `PostgresConnection`: 使用 `database/sql` + `github.com/lib/pq`
- `MySQLConnection`: 使用 `database/sql` + `github.com/go-sql-driver/mysql`

**工厂函数**:
```go
func CreateConnection(cfg models.DBConfig) (Connection, error)
```

#### 3.5 `internal/inspector/`

**接口**:
```go
type Inspector interface {
    Name() string
    Title() string
    Inspect() (map[string]interface{}, error)
}
```

**注册表**:
```go
var registry = map[string]func(conn connection.Connection, cfg models.InspectionConfig) Inspector{}
func Register(name string, factory ...)
func GetInspectors(conn connection.Connection, cfg models.InspectionConfig) []Inspector
```

**巡检器实现策略**:
将 Python 中每个 `_inspect_pg()` / `_inspect_mysql()` 方法对直译为 Go 中的两个方法。

**BasicInfoInspector** 关键逻辑:
- `_inspectPG()`: 查询 `pg_database`, 并行遍历各库获取表信息（goroutine + `sync.WaitGroup`）
- `_inspectMySQL()`: 查询 `information_schema.schemata`, 遍历获取表信息
- `_detectBackupDatabases()`: 正则检测备份库（Python re → Go `regexp`）
- `_detectBackupTables()`: 正则检测备份表
- `_getDBBaseName()`, `_hasSimilarDB()`, `_getBaseTableName()`: 字符串处理

**PerformanceInspector** 关键逻辑:
- `_inspectPG()`: 连接数、缓存命中率、索引命中率、活动会话、锁、慢查询、表/索引统计
- `_inspectPGDatabaseDeep()`: 死元组、VACUUM状态、IO统计、索引大小分析、无效索引、重复索引
- 并发采集：使用 `sync.WaitGroup` + goroutine（替代 ThreadPoolExecutor）
- `_inspectMySQL()`: 对应 MySQL 状态变量查询

**SchemaInspector** 关键逻辑:
- 表命名、列命名、主键、索引、约束、数据类型、注释、大表检查
- PG 和 MySQL 分别实现

#### 3.6 `internal/reporter/scoring.go`

将 Python `calculate_health_score()` 和 `generate_key_findings()` 直译为 Go。

- 输入改为 Go struct（`BasicInfoResult`, `PerformanceResult`）
- 评分逻辑 1:1 保留
- 等级划分：优秀(90-100) / 良好(75-89) / 一般(60-74) / 较差(0-59)

#### 3.7 `internal/reporter/html.go`

**模板方案**:
- 将现有 `report.html.j2` 转换为 Go `html/template` 语法
- 差异：Jinja2 `{% for %}` → `{{range}}`, `{{ var }}` → `{{.Var}}`, 过滤器通过 Go 函数实现
- `formatSize`, `formatDatetime` 等作为 `template.FuncMap` 注册

**模板加载**:
- 开发期：从 `./templates/` 目录加载（`os.ReadDir`）
- 发布期：使用 `//go:embed templates/*` 嵌入二进制，无需外部模板文件

**输出**: 生成 `db_inspection_<name>_<timestamp>.html`

#### 3.8 `cmd/root.go`

使用 `cobra` 定义命令行：
- `--config, -c`
- `--database, -d`
- `--format, -f` (html|markdown|json)
- `--db-host`, `--db-port`, `--db-user`, `--db-password`, `--db-name`, `--db-type`, `--db-database`, `--db-schema`
- `--db-json`

参数校验逻辑 1:1 从 Python 迁移。

---

### 4. 并发模型替换

Python `ThreadPoolExecutor(max_workers=16)` → Go:

```go
func inspectPGDatabasesParallel(dbNames []string) map[string]DBDeepResult {
    results := make(map[string]DBDeepResult)
    var mu sync.Mutex
    var wg sync.WaitGroup
    sem := make(chan struct{}, 16) // 信号量限制并发数

    for _, dbName := range dbNames {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            
            data := inspectPGDatabaseDeep(name)
            mu.Lock()
            results[name] = data
            mu.Unlock()
        }(dbName)
    }
    wg.Wait()
    return results
}
```

---

### 5. 模板转换策略

现有 HTML 报告模板约 ~400 行 Jinja2。转换要点：

| Jinja2 | Go template |
|--------|-------------|
| `{% for item in list %}` | `{{range .List}}` |
| `{% endfor %}` | `{{end}}` |
| `{{ item.name }}` | `{{.Name}}` |
| `{% if condition %}` | `{{if .Condition}}` |
| `{% elif %}` | `{{else if ...}}` |
| `item\|format_datetime` | 通过 `template.FuncMap` 注册 `formatDatetime` 函数 |
| `{% set var = value %}` | 使用 `{{$var := .Value}}`（模板变量）或预处理到 struct |

模板将放在 `templates/report.html.tmpl`，并通过 `//go:embed` 嵌入。

---

### 6. 交叉编译命令

在 Windows 上编译 ARM64 Linux 二进制：

```bash
# 设置环境变量并编译
$env:GOOS="linux"
$env:GOARCH="arm64"
$env:CGO_ENABLED="0"
go build -ldflags="-s -w" -o db-patrol-arm64 .

# 打包
tar czvf db-patrol-linux-arm64.tar.gz db-patrol-arm64 config.yaml
```

服务器上运行：
```bash
chmod +x db-patrol-arm64
./db-patrol-arm64 --db-host 10.19.10.37 --db-port 5432 --db-user user_db --db-password "Vbase@1234" --db-name vastbase --db-type vastbase_pg --db-database vastbase
```

---

### 7. 实现顺序（推荐）

1. **基础设施**: `go.mod`, `models`, `utils`, `config`
2. **数据库连接**: `connection` 包（PG + MySQL）
3. **巡检器基类 + BasicInfo**: `inspector` 包框架 + BasicInfoInspector
4. **Performance + Schema**: 其余两个巡检器
5. **评分引擎**: `reporter/scoring.go`
6. **报告生成**: JSON → Markdown → HTML（模板）
7. **CLI 入口**: `cmd/root.go`, `main.go`
8. **集成测试**: 端到端测试
9. **交叉编译验证**: Windows → ARM64 Linux

---

### 8. 假设与决策

| 决策 | 理由 |
|------|------|
| 使用 `database/sql` + 驱动 | Go 标准数据库接口，最稳定 |
| 使用 `html/template` 而非第三方 | 标准库，静态编译，无额外依赖 |
| 使用 `//go:embed` 嵌入模板 | 单二进制分发，无需外部模板文件 |
| `CGO_ENABLED=0` | 完全静态链接，跨平台兼容 |
| 保留 YAML 配置 | 与现有 `config.yaml` 兼容 |
| 结构体字段大写（导出） | Go template 只能访问导出字段 |
| `map[string]interface{}` 作为巡检结果 | 兼容 Python dict 的灵活性，减少类型转换工作 |

---

### 9. 验证步骤

1. **本地单元测试**:
   ```bash
   go test ./...
   ```

2. **本地运行验证**（Windows）:
   ```bash
   go run . --db-host <测试库> --db-user ... --db-type vastbase_pg --db-database ...
   ```

3. **交叉编译验证**:
   ```bash
   GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o db-patrol-arm64 .
   file db-patrol-arm64  # 应显示 ELF 64-bit LSB executable, ARM aarch64
   ```

4. **ARM64 服务器验证**:
   - 上传二进制到麒麟服务器
   - 运行巡检，验证 HTML 报告生成正常
   - 验证健康评分与 Python 版本一致
