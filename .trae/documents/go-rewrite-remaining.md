# Go 重写计划 — 剩余实施步骤

## 概述

将 db-patrol 从 Python 完全重写为 Go，支持交叉编译到 linux/arm64，生成单文件静态二进制。

## 当前进度（已完成）

| 模块 | 文件 | 状态 |
|------|------|------|
| 数据模型 | `internal/models/models.go` | 已完成 |
| 工具函数 | `internal/utils/utils.go` | 已完成 |
| 配置加载 | `internal/config/config.go` | 已完成 |
| 配置构建 | `internal/config/builder.go` | 已完成 |
| 连接接口 | `internal/connection/connection.go` | 已完成 |
| PG 连接 | `internal/connection/pg.go` | 已完成 |
| MySQL 连接 | `internal/connection/mysql.go` | 已完成 |
| 巡检器框架 | `internal/inspector/inspector.go` | 已完成 |
| 基本信息巡检 | `internal/inspector/basic.go` | 已完成 |
| 性能巡检 | `internal/inspector/performance.go` | 已完成 |
| Go Module | `go.mod` | 已完成 |

## 剩余工作

### Step 1: SchemaInspector (`internal/inspector/schema.go`)

**源文件**: `db_inspector/inspectors/schema.py` (569行)

**实现内容**:
- `SchemaInspector` struct，实现 `Inspector` 接口
- PG 路径: 表命名/列命名/主键/索引/约束/数据类型/注释/大表 共8项检查
- MySQL 路径: 表命名/列命名/主键/索引/约束/数据类型/注释/大表/引擎字符集 共9项检查
- 返回 `map[string]interface{}`，key 为各检查类别，value 为 `[]models.SchemaIssue`

**关键映射**:
- Python `self.execute_query()` → Go `i.conn.ExecuteQuery()`
- Python `self.connection.config['database']` → Go `i.conn.Config().Database`
- Python `self.config.get('table_size_threshold', 1024)` → Go `i.cfg.TableSizeThreshold`

### Step 2: 健康评分引擎 (`internal/reporter/scoring.go`)

**源文件**: `db_inspector/reporters/scoring.py` (377行)

**实现内容**:
- `CalculateHealthScore(basicInfo, performance, databases, tables) models.HealthScore`
  - 8个评分维度: 连接使用率(15)/缓存命中率(20)/索引命中率(15)/主键完整性(10)/备份数据清理(10)/索引大小占比(15)/无效索引(10)/重复索引(5)
  - 等级判定: 优秀(90-100)/良好(75-89)/一般(60-74)/较差(0-59)
- `GenerateKeyFindings(basicInfo, performance, databases, tables) []models.KeyFinding`
  - 12类发现: 连接使用率/缓存命中率/索引命中率/长事务/锁等待/无主键表/死元组/VACUUM状态/无效索引/重复索引/IO负载/备份数据

**数据类型**: 输入参数使用 `map[string]interface{}`，与 Python dict 保持一致，通过类型断言取值。

### Step 3: 报告生成框架

#### 3a: Reporter 接口与工厂 (`internal/reporter/reporter.go`)

```go
type Reporter interface {
    Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error)
}
func CreateReporter(format, outputDir string) (Reporter, error)
```

#### 3b: JSON Reporter (`internal/reporter/json_reporter.go`)

- 将巡检结果 + 健康评分 + 关键发现序列化为 JSON
- 文件名: `db_inspection_{name}_{timestamp}.json`

#### 3c: Markdown Reporter (`internal/reporter/markdown_reporter.go`)

- 生成 Markdown 格式报告，包含评分表格和发现列表

#### 3d: HTML Reporter (`internal/reporter/html_reporter.go`)

- 使用 `//go:embed templates/report.html.tmpl` 嵌入模板
- 将 Jinja2 模板转换为 Go `text/template` 语法
- 模板变量映射: 保持与 Python 版本相同的 context 结构
- 关键转换:
  - Jinja2 `{{ var }}` → Go `{{.var}}`
  - Jinja2 `{% if %}` → Go `{{if .var}}...{{end}}`
  - Jinja2 `{% for %}` → Go `{{range .items}}...{{end}}`
  - Jinja2 `| format_datetime` → Go template func `formatDatetime`
  - Jinja2 `.lower()` → Go `strings.ToLower` (需预计算)

#### 3e: HTML 模板 (`internal/reporter/templates/report.html.tmpl`)

**源文件**: `db_inspector/reporters/templates/report.html.j2` (~900行)

- 完整保留 CSS 样式和 HTML 结构
- 转换所有 Jinja2 语法为 Go template 语法
- 注册自定义函数: `formatDatetime`, `formatSize`, `lower`, `contains`

### Step 4: CLI 入口与核心控制器

#### 4a: 核心控制器 (`internal/core/core.go`)

**源文件**: `db_inspector/core.py` (226行)

- `DBInspector` struct: 持有 config、results
- `InspectDatabase(dbConfig, inspectionConfig)`: 创建连接 → 获取巡检器 → 逐个执行 → 返回结果
- `InspectAll()`: 遍历所有数据库配置，逐个巡检
- `PrintSummary()`: 打印巡检摘要到终端
- 使用 `colorama` → `fatih/color` 实现彩色终端输出
- 使用 `tablewriter` 格式化表格输出

#### 4b: CLI 入口 (`cmd/root.go` + `main.go`)

**源文件**: `main.py` (176行)

- 使用 cobra 实现 CLI:
  - `--config/-c`: 配置文件路径
  - `--database/-d`: 指定数据库
  - `--format/-f`: 报告格式 (html/markdown/json)
  - `--db-host/--db-port/--db-user/--db-password/--db-name/--db-type/--db-database/--db-schema`: 单库参数
  - `--db-json`: JSON 多库配置
- `DB_PASSWORD` 环境变量支持
- 参数校验逻辑与 Python 版本一致

#### 4c: 入口文件 (`main.go`)

```go
package main
import "db-patrol/cmd"
func main() { cmd.Execute() }
```

### Step 5: 编译验证

1. `go mod tidy` — 下载依赖
2. `go build -o db-patrol.exe .` — Windows 本地编译
3. 交叉编译:
   ```
   set GOOS=linux
   set GOARCH=arm64
   set CGO_ENABLED=0
   go build -o db-patrol-linux-arm64 .
   ```
4. 验证二进制文件大小和可执行性

## 文件创建顺序

1. `internal/inspector/schema.go`
2. `internal/reporter/scoring.go`
3. `internal/reporter/reporter.go`
4. `internal/reporter/json_reporter.go`
5. `internal/reporter/markdown_reporter.go`
6. `internal/reporter/templates/report.html.tmpl`
7. `internal/reporter/html_reporter.go`
8. `internal/core/core.go`
9. `cmd/root.go`
10. `main.go`
11. `go.sum` (go mod tidy 自动生成)

## 验证步骤

- `go vet ./...` — 静态检查
- `go build ./...` — 编译通过
- 交叉编译 linux/arm64 成功
- 二进制文件可在目标平台运行
