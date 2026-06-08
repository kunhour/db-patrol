# AGENTS.md - DB Patrol AI 助手指南

## 项目概述

**db-patrol** 是数据库巡检工具，支持 Vastbase (PG/MySQL)、MySQL、PostgreSQL，生成健康评分报告。

**技术栈**: Go 1.21+ / cobra / pq / go-sql-driver/mysql / yaml.v3 / color / tablewriter

## 项目结构

```
db-patrol/
├── main.go                      # 程序入口
├── go.mod                       # Go 模块定义
├── config.yaml                  # 巡检配置
├── dist/                        # 编译输出目录
├── cmd/
│   └── root.go                  # CLI 入口 (cobra)
└── internal/
    ├── config/
    │   └── config.go            # 配置加载与默认值
    ├── connection/
    │   ├── connection.go        # 数据库连接接口与工厂
    │   ├── pg.go                # PostgreSQL 连接实现
    │   └── mysql.go             # MySQL 连接实现
    ├── core/
    │   └── core.go              # DBInspector 核心控制器
    ├── inspector/
    │   ├── inspector.go         # Inspector 接口与注册表
    │   ├── basic.go             # 基本信息巡检器
    │   ├── performance.go       # 性能巡检器
    │   └── schema.go            # 设计规范巡检器
    ├── models/
    │   └── models.go            # 数据模型定义
    ├── reporter/
    │   ├── reporter.go          # Reporter 接口与工厂
    │   ├── scoring.go           # 健康评分引擎
    │   ├── html_reporter.go     # HTML 报告生成器
    │   ├── markdown_reporter.go # Markdown 报告生成器
    │   ├── json_reporter.go     # JSON 报告生成器
    │   └── templates/
    │       └── report.html.tmpl # HTML 报告模板 (go:embed)
    └── utils/
        └── utils.go             # 工具函数
```

## 架构

**数据流**: CLI (cobra) → DBInspector → CreateConnection → GetInspectors → Inspector.Inspect → CreateReporter → Reporter.Generate

**核心模式**:
- **工厂模式**: `CreateConnection()` 按 `db_type` 创建连接
- **注册表模式**: `GetInspectors()` 从注册表获取所有 Inspector 实例
- **策略模式**: `Inspector` 接口 → `BasicInfoInspector`, `PerformanceInspector`, `SchemaInspector`
- **并发扫描**: goroutine + sync.WaitGroup + channel 信号量 (max 16) 全库并发采集
- **模板嵌入**: `//go:embed` 将 HTML 模板嵌入单二进制

**接口关系**:
- `Connection` 接口 ← `PostgresConnection`, `MySQLConnection`
- `Inspector` 接口 ← `BasicInfoInspector`, `PerformanceInspector`, `SchemaInspector`
- `Reporter` 接口 ← `HTMLReporter`, `MarkdownReporter`, `JSONReporter`

## 代码约定

- **命名**: PascalCase(导出类型/函数) / snake_case(非导出) / snake_case(文件名)
- **错误处理**: 查询失败返回默认值/空结果，**不中断巡检流程**
- **日志**: `fmt.Printf` + `color` 包彩色终端输出
- **模板**: HTML 模板使用 `//go:embed` 嵌入，`text/template` 渲染
- **编码**: 全程 UTF-8

## 扩展指南

| 扩展类型 | 步骤 |
|---------|------|
| 新数据库 | 1. `connection/` 新建文件实现 `Connection` 接口 2. `connection.go` 的 `CreateConnection()` 添加分支 |
| 新巡检项 | 1. `inspector/` 新建文件实现 `Inspector` 接口 2. 设置 `Name()`/`Title()` 3. 实现 `Inspect()` 返回 `map[string]interface{}` 4. `inspector.go` 的 `init()` 中注册 5. `config.yaml` 的 `checks` 添加开关 |
| 新报告格式 | 1. `reporter/` 新建文件实现 `Reporter` 接口 2. 实现 `Generate(dbConfig, results)` 返回文件路径 3. `reporter.go` 的 `CreateReporter()` 中注册 |

## 业务规则

### 备份库检测
- **强特征**: `_backup/_bak/_copy/_old/_test/_temp/_dev`、日期结尾
- **弱特征**: 数字结尾 + 存在相似基础名称
- **白名单**: `emate_dev`

### 备份表检测
- **特征**: `_backup/_bak/_copy/_old/_new/_temp/_tmp`、日期结尾
- **排除**: `_nod_old/_lin_old/_net_old`

### 健康评分 (100 分制)

| 维度 | 分值 | 核心规则 |
|------|------|----------|
| 连接使用率 | 15 | >90% 扣15, >80% 扣10, >60% 扣5 |
| 缓存命中率 | 20 | <90% 扣20, <95% 扣15, <99% 扣5 |
| 索引命中率 | 15 | <50% 扣15, <70% 扣10, <90% 扣5 |
| 主键完整性 | 10 | >20个扣10, >10个扣7, >0个扣3 |
| 备份数据清理 | 10 | 备份库>5或备份表>20 扣10, 否则扣5 |
| 索引大小占比 | 15 | >10个扣15, >5个扣10, >0个扣5 |
| 无效索引 | 10 | 发现即扣10 |
| 重复索引 | 5 | 发现即扣5 |

等级: 优秀(90-100) / 良好(75-89) / 一般(60-74) / 较差(0-59)

## 编译与部署

```bash
# Windows 编译
go build -o dist/db-patrol.exe .

# Linux ARM64 交叉编译
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o dist/db-patrol-linux-arm64 .
```

生成的二进制为单文件静态链接，可直接在目标平台运行，无需任何依赖。

## 注意事项

- **Schema 巡检器**: 已集成到巡检流程，在 `config.yaml` 中通过 `checks.schema: true` 启用（默认关闭）
- **慢查询**: PG 需要 `pg_stat_statements` 扩展
- **数据库类型**: 支持 `vastbase_pg`/`postgresql`/`mysql`/`vastbase_mysql` 四种类型
- **安全**: 密码不存配置文件，推荐命令行参数或 `--db-json` 传递
