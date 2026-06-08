# DB Patrol - 数据库巡检工具设计文档

---

## 目录

1. [项目概述](#1-项目概述)
2. [系统架构](#2-系统架构)
3. [模块设计](#3-模块设计)
4. [数据流设计](#4-数据流设计)
5. [配置设计](#5-配置设计)
6. [安全设计](#6-安全设计)
7. [报告生成设计](#7-报告生成设计)
8. [扩展性设计](#8-扩展性设计)

---

## 1. 项目概述

### 1.1 项目背景

DB Patrol 是一款专业的数据库巡检工具，旨在帮助数据库管理员和开发人员全面了解数据库实例的运行状况、性能指标和设计规范合规性。随着企业数据库规模的不断扩大，手动巡检变得越来越困难且容易出错，因此需要一款自动化、可配置、多格式的巡检工具。

### 1.2 项目目标

- **自动化巡检**：一键执行全面的数据库健康检查
- **多数据库支持**：支持 Vastbase (PG/MySQL 模式)、MySQL、PostgreSQL
- **多维度检查**：覆盖基本信息、性能指标、设计规范等多个维度
- **多格式报告**：支持 HTML、Markdown、JSON 三种报告格式
- **安全优先**：敏感配置（如密码）不存储在文件中，通过命令行传递
- **单文件部署**：编译为单个静态二进制文件，无需安装任何依赖，支持交叉编译

### 1.3 功能特性

#### 1.3.1 数据库实例基本运行情况

| 功能模块 | 功能描述 |
|---------|---------|
| 连接状态检查 | 验证数据库连接可用性 |
| 版本信息 | 获取数据库版本和运行时间 |
| 连接监控 | 监控连接数和连接状态 |
| 性能指标 | QPS、TPS、慢查询等关键指标 |
| 表统计 | 表大小、行数、索引使用情况 |
| 锁等待检测 | 检测锁等待和阻塞情况 |

#### 1.3.2 数据库设计规范检查

| 检查项 | 说明 |
|-------|------|
| 表命名规范 | 检查表名是否符合命名规范 |
| 列命名规范 | 检查列名是否符合命名规范 |
| 主键检查 | 检查表是否定义主键 |
| 索引规范 | 检查索引命名和使用规范 |
| 外键约束 | 检查外键约束完整性 |
| 数据类型建议 | 提供数据类型优化建议 |
| 注释完整性 | 检查表和列的注释是否完整 |
| 大表检测 | 识别超过阈值的大表 |
| 存储引擎和字符集 | MySQL 模式下的引擎和字符集检查 |

#### 1.3.3 智能检测功能

- **备份库检测**：自动识别疑似备份的数据库（基于命名规则）
- **备份表检测**：自动识别疑似备份的数据表
- **无主键表检测**：识别缺少主键或唯一索引的表
- **索引大小分析**：分析索引大小占比，识别需要优化的表

#### 1.3.4 安全特性

- **敏感信息保护**：数据库连接配置不存储在配置文件中
- **命令行参数传递**：支持通过命令行参数直接传递连接信息
- **JSON 批量配置**：支持通过 JSON 字符串传递多个数据库配置
- **环境变量支持**：密码支持通过 `DB_PASSWORD` 环境变量传递

### 1.4 技术栈

| 组件 | 版本 | 用途 |
|-----|------|-----|
| Go | 1.21+ | 开发语言 |
| cobra | v1.8.0 | 命令行接口 |
| pq | v1.10.9 | PostgreSQL 数据库连接 |
| go-sql-driver/mysql | v1.7.1 | MySQL 数据库连接 |
| yaml.v3 | v3.0.1 | 配置文件解析 |
| color | v1.16.0 | 终端彩色输出 |
| tablewriter | v0.0.5 | 终端表格输出 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户交互层                                │
│  ┌─────────────┐                                                │
│  │   CLI 入口   │  cmd/root.go - cobra 命令行参数解析              │
│  │             │  - 数据库连接参数解析                             │
│  │             │  - JSON 配置解析                                 │
│  └─────────────┘                                                │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                        核心控制层                                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              DBInspector (核心控制器)                     │   │
│  │  internal/core/core.go                                   │   │
│  │  - 配置加载与管理（支持命令行参数覆盖）                    │   │
│  │  - 巡检任务调度                                           │   │
│  │  - 结果汇总与摘要输出（tablewriter 表格）                  │   │
│  │  - 彩色终端输出（color 包）                               │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────┬────────────────────────────────────────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
┌───────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
│   连接管理层   │  │   巡检执行层  │  │   报告生成层  │
│              │  │             │  │             │
│ • 连接工厂    │  │ • 基本信息   │  │ • HTML      │
│ • PG 连接    │  │ • 性能指标   │  │ • Markdown  │
│ • MySQL 连接 │  │ • 设计规范   │  │ • JSON      │
└──────────────┘  └─────────────┘  └─────────────┘
```

### 2.2 分层架构说明

#### 2.2.1 用户交互层

- **职责**：处理用户输入，提供友好的命令行接口
- **核心组件**：`cmd/root.go`（cobra 命令定义）
- **功能**：
  - 命令行参数解析（配置文件路径、指定数据库、输出格式）
  - 数据库连接参数解析（host、port、user、password、name、type、database、schema）
  - JSON 格式数据库配置解析
  - 环境变量密码引用解析（`$ENV_VAR`）
  - 程序入口和流程控制
  - 错误处理和用户提示

#### 2.2.2 核心控制层

- **职责**：协调各模块工作，管理巡检生命周期
- **核心组件**：`DBInspector` 结构体
- **功能**：
  - 加载和验证配置文件
  - 接收命令行传入的数据库配置（覆盖配置文件）
  - 遍历数据库配置执行巡检
  - 调用报告生成器输出结果
  - 打印巡检摘要（tablewriter 表格）
  - 彩色终端输出（color 包）

#### 2.2.3 连接管理层

- **职责**：管理数据库连接的生命周期
- **核心组件**：`Connection` 接口及其实现
- **功能**：
  - 统一的数据库连接接口
  - 支持多种数据库类型
  - 工厂函数 `CreateConnection()` 按类型创建连接

#### 2.2.4 巡检执行层

- **职责**：执行具体的数据库检查任务
- **核心组件**：各类 Inspector 实现
- **功能**：
  - 基本信息采集
  - 性能指标采集
  - 设计规范检查
  - 并发采集（goroutine + WaitGroup + 信号量）

#### 2.2.5 报告生成层

- **职责**：将巡检结果转换为不同格式的报告
- **核心组件**：各类 Reporter 实现
- **功能**：
  - HTML 可视化报告（`//go:embed` 嵌入模板）
  - Markdown 文档报告
  - JSON 数据报告
  - 健康评分计算

### 2.3 设计模式应用

| 设计模式 | 应用场景 | 实现位置 |
|---------|---------|---------|
| 工厂模式 | 根据配置创建对应的数据库连接 | `CreateConnection()` 函数 |
| 注册表模式 | Inspector 的自动发现与实例化 | `inspector.GetInspectors()` |
| 策略模式 | 支持多种报告格式，可灵活切换 | `Reporter` 接口体系 |
| 接口模式 | 定义巡检器和连接的通用行为 | `Inspector` / `Connection` 接口 |
| 依赖注入 | Connection 可注入到 Inspector | `NewXxxInspector(conn, cfg)` |
| 模板嵌入 | HTML 模板编译进二进制 | `//go:embed templates/report.html.tmpl` |

---

## 3. 模块设计

### 3.1 模块结构图

```
db-patrol/
├── main.go                      # 程序入口
├── go.mod                       # Go 模块定义
├── config.yaml                  # 巡检配置
│
├── cmd/
│   └── root.go                  # CLI 入口 (cobra)
│
└── internal/
    ├── config/
    │   └── config.go            # 配置加载与默认值
    │
    ├── connection/
    │   ├── connection.go        # 数据库连接接口与工厂
    │   ├── pg.go                # PostgreSQL 连接实现
    │   └── mysql.go             # MySQL 连接实现
    │
    ├── core/
    │   └── core.go              # DBInspector 核心控制器
    │
    ├── inspector/
    │   ├── inspector.go         # Inspector 接口与注册表
    │   ├── basic.go             # 基本信息巡检器
    │   ├── performance.go       # 性能巡检器
    │   └── schema.go            # 设计规范巡检器
    │
    ├── models/
    │   └── models.go            # 数据模型定义
    │
    ├── reporter/
    │   ├── reporter.go          # Reporter 接口与工厂
    │   ├── scoring.go           # 健康评分引擎
    │   ├── html_reporter.go     # HTML 报告生成器
    │   ├── markdown_reporter.go # Markdown 报告生成器
    │   ├── json_reporter.go     # JSON 报告生成器
    │   └── templates/
    │       └── report.html.tmpl # HTML 报告模板 (go:embed)
    │
    └── utils/
        └── utils.go             # 工具函数
```

### 3.2 核心模块详细设计

#### 3.2.1 命令行入口模块 (cmd/root.go)

##### 结构

```
┌─────────────────────────────────────────────────────────┐
│                    cobra.Command                          │
├─────────────────────────────────────────────────────────┤
│ Flags:                                                   │
│   --config, -c: string                                   │
│   --database, -d: string                                 │
│   --format, -f: string                                   │
│   --db-host: string                                      │
│   --db-port: int                                         │
│   --db-user: string                                      │
│   --db-password: string (env: DB_PASSWORD)               │
│   --db-name: string                                      │
│   --db-type: string                                      │
│   --db-database: string                                  │
│   --db-schema: string                                    │
│   --db-json: string                                      │
├─────────────────────────────────────────────────────────┤
│ + Execute()                                              │
│ + runRoot(cmd, args) error                               │
│   - 解析命令行参数                                       │
│   - 构建 DBConfig 对象                                   │
│   - 创建 DBInspector 实例                                │
│   - 执行巡检并生成报告                                   │
└─────────────────────────────────────────────────────────┘
```

##### 参数处理流程

1. **单独参数模式**：用户通过 `--db-host`、`--db-user` 等单独参数传递数据库连接信息
   - 验证必填参数（host、user、password、database、type）
   - 设置可选参数默认值（port、name、schema）
   - 构建单个数据库配置对象

2. **JSON 参数模式**：用户通过 `--db-json` 传递 JSON 格式的数据库配置
   - 解析 JSON 字符串
   - 支持单个对象或数组（多个数据库）
   - 验证 JSON 格式正确性

3. **配置文件模式**：未提供命令行数据库参数时，从配置文件读取（不推荐）

#### 3.2.2 数据库连接模块 (connection/)

##### 接口定义

```
┌─────────────────────────────────────────────────────────┐
│                    <<interface>> Connection              │
├─────────────────────────────────────────────────────────┤
│ + Connect() error                                        │
│ + ExecuteQuery(query string, args ...interface{})        │
│   -> ([]map[string]interface{}, error)                   │
│ + Execute(query string, args ...interface{}) error       │
│ + Close() error                                          │
│ + DB() *sql.DB                                           │
│ + Config() models.DBConfig                               │
└─────────────────────────────────────────────────────────┘
                            △
           ┌────────────────┴────────────────┐
           │                                 │
┌──────────┴──────────┐        ┌─────────────┴──────────────┐
│ PostgresConnection   │        │ MySQLConnection            │
├──────────────────────┤        ├────────────────────────────┤
│ BaseConnection       │        │ BaseConnection             │
├──────────────────────┤        ├────────────────────────────┤
│ + Connect()          │        │ + Connect()                │
│ + ExecuteQuery()     │        │ + ExecuteQuery()           │
│ + Execute()          │        │ + Execute()                │
└──────────────────────┘        └────────────────────────────┘
```

##### 职责说明

- **Connection**：定义数据库连接的抽象接口
- **PostgresConnection**：PostgreSQL/Vastbase PG 模式的具体实现，使用 `lib/pq` 驱动
- **MySQLConnection**：MySQL/Vastbase MySQL 模式的具体实现，使用 `go-sql-driver/mysql` 驱动
- **CreateConnection()**：工厂函数，根据 `db_type` 创建对应连接

#### 3.2.3 巡检器模块 (inspector/)

##### 接口定义

```
┌─────────────────────────────────────────────────────────┐
│                    <<interface>> Inspector               │
├─────────────────────────────────────────────────────────┤
│ + Name() string                                          │
│ + Title() string                                         │
│ + Inspect() (map[string]interface{}, error)              │
└─────────────────────────────────────────────────────────┘
                            △
           ┌────────────────┼────────────────┐
           │                │                │
┌──────────┴──────────┐ ┌───┴───────┐ ┌─────┴──────────┐
│ BasicInfoInspector  │ │Performance│ │SchemaInspector │
│  name='basic_info'  │ │Inspector  │ │ name='schema'  │
│  title='检查基本信息'│ │name='per- │ │title='检查设计 │
│                     │ │formance'  │ │  规范'          │
└─────────────────────┘ └───────────┘ └────────────────┘
```

##### 注册表机制

```go
// 注册表
var registry = map[string]Factory{}

// 注册函数
func Register(name string, factory Factory) {
    registry[name] = factory
}

// init() 自动注册
func init() {
    Register("basic_info", func(conn, cfg) Inspector { ... })
    Register("performance", func(conn, cfg) Inspector { ... })
    Register("schema", func(conn, cfg) Inspector { ... })
}
```

##### 扩展机制

新增巡检器类型只需：
1. 创建新文件实现 `Inspector` 接口
2. 设置 `Name()` 和 `Title()` 方法
3. 实现 `Inspect()` 方法
4. 在 `inspector.go` 的 `init()` 中注册
5. 在 `config.yaml` 的 `checks` 下添加开关

#### 3.2.4 基本信息巡检器 (inspector/basic.go)

##### 功能清单

| 方法 | 功能描述 |
|-----|---------|
| `inspectPG()` | PG 模式巡检入口 |
| `inspectMySQL()` | MySQL 模式巡检入口 |
| `checkConnection()` | 检查数据库连接状态 |
| `getPGVersion()` | 获取数据库版本 |
| `getPGInstanceInfo()` | 获取实例信息（大小、连接数、配置等） |
| `getPGUptime()` | 获取运行时间 |
| `inspectPGDatabasesParallel()` | 并发获取所有数据库详细信息 |
| `inspectPGDatabaseAll()` | 单数据库并发采集（表、统计、主键检查） |
| `detectBackupDatabases()` | 检测疑似备份库 |
| `detectBackupTables()` | 检测疑似备份表 |

##### 并发采集机制

```go
// 使用 goroutine + WaitGroup + 信号量控制并发
sem := make(chan struct{}, 16) // 最大并发 16
var wg sync.WaitGroup

for _, dbName := range dbNames {
    wg.Add(1)
    go func(name string) {
        defer wg.Done()
        sem <- struct{}{}        // 获取信号量
        defer func() { <-sem }() // 释放信号量
        // 执行采集...
    }(dbName)
}
wg.Wait()
```

#### 3.2.5 性能巡检器 (inspector/performance.go)

##### 功能清单

| 方法 | 功能描述 |
|-----|---------|
| `getPGConnections()` | 获取连接统计信息 |
| `getPGClientConnections()` | 按 IP 分组统计客户端连接 |
| `getPGActivity()` | 获取当前活动会话 |
| `getPGLocks()` | 获取锁等待信息 |
| `getPGSlowQueries()` | 获取慢查询列表 |
| `getPGTableStats()` | 获取表统计信息（活/死元组） |
| `getPGIndexStats()` | 获取索引使用统计 |
| `inspectPGDatabasesParallel()` | 并发扫描所有数据库 |
| `inspectPGDatabaseDeep()` | 单数据库深度采集（死元组、VACUUM、IO、索引分析） |

##### 索引大小分析逻辑

```
分析指标:
- 行数 >= 1000 的表才进行分析
- 索引占比 > 50%：标记为"严重"
- 索引占比 > 30%：标记为"关注"
- 索引占比 <= 30%：标记为"正常"

原因分析:
- 索引过多：单表索引数量超过 5 个
- 索引过大：索引大小超过数据大小的 50%
```

#### 3.2.6 设计规范巡检器 (inspector/schema.go)

##### 功能清单

| 方法 | 功能描述 |
|-----|---------|
| `checkPGTableNaming()` | 检查表命名规范 |
| `checkPGColumnNaming()` | 检查列命名规范 |
| `checkPGPrimaryKeys()` | 检查主键定义 |
| `checkPGIndexes()` | 检查索引规范 |
| `checkPGConstraints()` | 检查约束完整性 |
| `checkPGDataTypes()` | 检查数据类型建议 |
| `checkPGComments()` | 检查注释完整性 |
| `checkPGLargeTables()` | 检测大表 |

##### 命名规范检查规则

```
表名规范:
- 应全部小写
- 不应包含连字符或空格
- 建议使用小写字母和下划线

列名规范:
- 应全部小写
- 不应以数字开头
- 建议使用小写字母和下划线
```

### 3.3 报告生成模块设计

#### 3.3.1 报告生成器接口

```
┌─────────────────────────────────────────────────────────┐
│                    <<interface>> Reporter                │
├─────────────────────────────────────────────────────────┤
│ + Generate(dbConfig models.DBConfig,                     │
│            results map[string]interface{}) (string, error)│
└─────────────────────────────────────────────────────────┘
                            △
           ┌────────────────┼────────────────┐
           │                │                │
┌──────────┴──────────┐ ┌───┴───────┐ ┌─────┴──────────┐
│    HTMLReporter     │ │ Markdown  │ │  JSONReporter  │
│                     │ │ Reporter  │ │                │
├─────────────────────┤ ├───────────┤ ├────────────────┤
│ //go:embed 嵌入模板 │ │ 纯文本    │ │ json.Marshal   │
│ text/template 渲染  │ │ 格式化    │ │ 序列化         │
└─────────────────────┘ └───────────┘ └────────────────┘

注: HTML 报告模板通过 //go:embed 嵌入二进制，
使用 text/template 渲染
```

#### 3.3.2 HTML 报告生成器

##### 技术特点

- 使用 Go `text/template` 模板引擎渲染 HTML
- `//go:embed` 将模板文件嵌入二进制，无需外部文件
- 响应式设计，支持移动端查看
- 现代化 UI 风格，渐变色卡片设计
- 数据表格支持悬停高亮
- 状态标签使用颜色区分（成功/警告/错误）

##### 模板函数

| 函数 | 说明 |
|-----|------|
| `formatDatetime` | 格式化时间值 |
| `formatSize` | 格式化字节大小 |
| `formatNumber` | 格式化数字（千分位） |
| `isPG` | 判断是否为 PG 类型 |
| `lower` | 转小写 |
| `contains` | 字符串包含判断 |
| `add` | 整数加法 |

##### 报告内容结构

1. **报告头部**：标题、实例名称、数据库类型、生成时间
2. **概览卡片**：连接状态、数据库数量、表总数、实例大小
3. **健康评分**：综合评分（0-100分），基于8个维度
4. **关键发现与建议**：自动检测并高亮严重问题
5. **实例基本信息**：版本、启动时间、数据目录、连接配置等
6. **实例配置**：关键配置参数列表
7. **数据库列表**：所有数据库的详细信息表格
8. **疑似备份库**：高亮显示疑似备份的数据库
9. **数据表清单**：按数据库分组，动态展示
10. **疑似备份表**：高亮显示疑似备份的表
11. **缺少主键的表**：列出需要添加主键的表
12. **性能指标**：连接统计、客户端连接详情、索引大小分析

---

## 4. 数据流设计

### 4.1 巡检流程时序图

```
User       cmd/root.go    DBInspector    Connection    Inspector    Reporter
 |             |               |              |             |            |
 |-- 执行命令 ->|               |              |             |            |
 |             |-- 解析参数 -->|              |             |            |
 |             |   (db-host,   |              |             |            |
 |             |    db-user,   |              |             |            |
 |             |    db-json)   |              |             |            |
 |             |               |              |             |            |
 |             |-- 创建 ------>|              |             |            |
 |             |   (DBConfig)  |              |             |            |
 |             |               |-- 加载配置 -->|             |            |
 |             |               |<-------------|             |            |
 |             |<--------------|              |             |            |
 |             |               |              |             |            |
 |             |               |-- 遍历数据库配置 ----------> |            |
 |             |               |              |             |            |
 |             |               |-- 创建连接 ---------------->|            |
 |             |               |              |-- 连接 ---->|            |
 |             |               |              |<------------|            |
 |             |               |<-------------|             |            |
 |             |               |              |             |            |
 |             |               |-- 执行巡检 --------------->|            |
 |             |               |              |-- 查询 ---->|            |
 |             |               |              |<------------|            |
 |             |               |<-------------|             |            |
 |             |               |              |             |            |
 |             |               |-- 生成报告 ---------------------------->|
 |             |               |              |             |<-----------|
 |             |               |<-------------|             |            |
 |             |<--------------|              |             |            |
 |<-- 输出结果 -|               |              |             |            |
```

### 4.2 数据转换流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   数据库      │ --> │   原始数据    │ --> │   结构化数据  │ --> │   报告输出    │
│  (PostgreSQL)│     │ (map[string] │     │ (检查结果    │     │ (HTML/MD/JSON)│
│              │     │  interface{})│     │  map[string] │     │              │
└──────────────┘     └──────────────┘     │  interface{})│     └──────────────┘
       │                    │             └──────────────┘            │
       │                    │                    │                    │
   SQL 查询            数据提取与           结果分类与           模板渲染或
   执行结果            格式化               阈值判断               序列化
```

### 4.3 巡检结果数据结构

```go
map[string]interface{}{
    "basic_info": map[string]interface{}{
        "instance_info":     models.InstanceInfo{...},
        "version":           "...",
        "connection_status": models.ConnectionStatus{...},
        "uptime":            "...",
        "settings":          map[string]string{...},
        "databases": map[string]interface{}{
            "total":  0,
            "normal": []models.DatabaseInfo{...},
            "backup": []models.DatabaseInfo{...},
        },
        "tables": map[string]interface{}{
            "total_count": 0,
            "normal":      []models.TableInfo{...},
            "backup":      []models.TableInfo{...},
        },
        "tables_without_pk": map[string][]models.TableWithoutPK{...},
    },
    "performance": map[string]interface{}{
        "connections":          models.ConnectionStats{...},
        "client_connections":   []models.ClientConnection{...},
        "cache_hit_ratio":      models.CacheHitRatio{...},
        "index_hit_ratio":      models.IndexHitRatio{...},
        "locks":                []models.LockInfo{...},
        "long_transactions":    []models.LongTransaction{...},
        "dead_tuples":          map[string][]models.DeadTupleInfo{...},
        "vacuum_status":        map[string][]models.VacuumStatus{...},
        "io_stats":             map[string][]models.IOStats{...},
        "index_size_analysis":  map[string][]models.IndexSizeAnalysis{...},
        "invalid_indexes":      map[string][]models.InvalidIndex{...},
        "duplicate_indexes":    map[string][]models.DuplicateIndex{...},
    },
    "schema": map[string]interface{}{
        "table_naming":  []models.SchemaIssue{...},
        "column_naming": []models.SchemaIssue{...},
        "primary_keys":  []models.SchemaIssue{...},
        // ...
    },
}
```

---

## 5. 配置设计

### 5.1 配置文件结构

**重要提示**：数据库连接配置不存储在配置文件中，应通过命令行参数传递。

```yaml
# 巡检配置
inspection:
  slow_query_threshold: 1.0        # 慢查询阈值（秒）
  max_connections_threshold: 80    # 连接数警告阈值（百分比）
  table_size_threshold: 1024       # 大表阈值（MB）
  long_transaction_threshold: 300  # 长事务阈值（秒）
  
  checks:
    basic_info: true               # 启用基本信息检查
    performance: true              # 启用性能检查
    schema: false                  # 启用设计规范检查（默认关闭）

# 报告配置
report:
  format: html                     # 输出格式: html | markdown | json
  output_dir: ./reports            # 输出目录
```

### 5.2 配置项说明

#### 5.2.1 巡检配置

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|-------|------|
| slow_query_threshold | float | 1.0 | 慢查询阈值（秒） |
| max_connections_threshold | int | 80 | 连接数警告阈值（百分比） |
| table_size_threshold | int | 1024 | 大表阈值（MB） |
| long_transaction_threshold | int | 300 | 长事务阈值（秒） |
| checks.basic_info | bool | true | 是否执行基本信息检查 |
| checks.performance | bool | true | 是否执行性能检查 |
| checks.schema | bool | false | 是否执行设计规范检查 |

#### 5.2.2 报告配置

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|-------|------|
| format | string | html | 报告输出格式 |
| output_dir | string | ./reports | 报告输出目录路径 |

### 5.3 配置加载流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  命令行参数   │ --> │  参数合并     │ --> │  配置验证    │ --> │  AppConfig   │
│  (db-host,    │     │              │     │              │     │  结构体       │
│   db-user...) │     │              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       │                    │                    │                    │
       ▼                    ▼                    ▼                    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  配置文件     │ --> │ YAML 解析    │ --> │  设置默认值   │ --> │ 存储在        │
│ config.yaml  │     │ (yaml.v3)    │     │              │     │ DBInspector   │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘

优先级：命令行参数 > 配置文件
```

### 5.4 数据库配置数据结构

```go
// 单个数据库配置
type DBConfig struct {
    Name     string `json:"name" yaml:"name"`
    Type     string `json:"type" yaml:"type"`
    Host     string `json:"host" yaml:"host"`
    Port     int    `json:"port" yaml:"port"`
    User     string `json:"user" yaml:"user"`
    Password string `json:"password" yaml:"password"`
    Database string `json:"database" yaml:"database"`
    Schema   string `json:"schema" yaml:"schema"`
}

// 多个数据库配置（JSON 数组）
[
    {"name":"DB1", "type":"postgresql", "host":"192.168.1.10", ...},
    {"name":"DB2", "type":"mysql", "host":"192.168.1.11", ...}
]
```

---

## 6. 安全设计

### 6.1 安全设计原则

1. **最小化暴露**：敏感信息（如密码）不应持久化存储
2. **传输安全**：通过命令行参数或环境变量传递敏感信息
3. **单文件部署**：编译为静态二进制，无需安装依赖，减少攻击面
4. **向后兼容**：保留配置文件方式但标记为不推荐

### 6.2 敏感信息保护机制

#### 6.2.1 命令行参数安全特性

- **环境变量支持**：密码可通过 `DB_PASSWORD` 环境变量传递
- **JSON 配置环境变量引用**：`--db-json` 中密码字段支持 `$ENV_VAR` 引用
- **不存储密码**：配置文件不包含数据库连接密码

#### 6.2.2 环境变量引用示例

```bash
# 设置环境变量
export DB_PASSWORD=your_password

# 在 JSON 中引用
./db-patrol --db-json '[
  {"name":"DB1", "type":"mysql", "host":"192.168.1.1", "password":"$DB_PASSWORD", ...}
]'
```

### 6.3 推荐的安全使用模式

#### 模式一：环境变量 + 命令行参数（生产环境推荐）

```bash
# 设置环境变量
export DB_PASSWORD=$(vault get db/password)

# 执行巡检
./db-patrol --db-host 192.168.1.10 --db-user admin \
            --db-password $DB_PASSWORD --db-database vastbase \
            --db-type vastbase_pg
```

#### 模式二：CI/CD 流水线集成

```yaml
# .gitlab-ci.yml 示例
db_inspection:
  stage: monitoring
  script:
    - ./db-patrol-linux-arm64 --db-host $VAULT_DB_HOST
                              --db-user $VAULT_DB_USER
                              --db-password $VAULT_DB_PASSWORD
                              --db-database $VAULT_DB_NAME
                              --db-type vastbase_pg
  only:
    - schedules
```

### 6.4 安全边界考虑

| 场景 | 风险 | 缓解措施 |
|-----|------|---------|
| 命令行历史泄露 | 密码可能保存在 shell 历史中 | 使用环境变量传递密码 |
| 进程列表可见 | 命令行参数可能通过 ps 命令可见 | 使用环境变量或 stdin |
| 报告文件敏感 | 报告中包含数据库结构信息 | 限制报告目录权限 |

---

## 7. 报告生成设计

### 7.1 报告模板设计

#### 7.1.1 HTML 模板嵌入机制

```go
//go:embed templates/report.html.tmpl
var templateFS embed.FS

// 解析模板
tmpl, err := template.New("report.html.tmpl").
    Funcs(funcMap).
    ParseFS(templateFS, "templates/report.html.tmpl")
```

优势：
- 模板编译进二进制，无需外部文件
- 单文件部署，适合离线环境
- 模板与代码版本一致，不会出现版本不匹配

#### 7.1.2 样式设计原则

- **色彩系统**：使用渐变色头部，状态色区分（绿/橙/红）
- **布局系统**：CSS Grid 实现响应式卡片布局
- **表格设计**：斑马纹、悬停高亮、固定表头
- **打印优化**：避免卡片和区块分页断开

### 7.2 报告生成流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  巡检结果    │ --> │  数据准备    │ --> │  模板渲染    │ --> │  文件输出    │
│  (map)       │     │              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       │                    │                    │                    │
       │                    │                    │                    │
   原始数据            struct → map         template.Execute()   os.WriteFile
   嵌套结构            转换显示字段         替换变量              UTF-8编码
                      计算健康评分
```

### 7.3 报告文件命名规范

```
db_inspection_{实例名}_{时间戳}.{格式}

示例：
- db_inspection_segh_yy_20260608_130804.html
- db_inspection_segh_yy_20260608_130804.md
- db_inspection_segh_yy_20260608_130804.json
```

---

## 8. 扩展性设计

### 8.1 添加新的数据库类型支持

#### 步骤

1. **创建连接实现**：实现 `Connection` 接口

```go
// internal/connection/oracle.go
type OracleConnection struct {
    BaseConnection
}

func (c *OracleConnection) Connect() error {
    // 实现连接逻辑
}

func (c *OracleConnection) ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
    // 实现查询逻辑
}
```

2. **注册连接工厂**：在 `CreateConnection()` 中添加类型判断

```go
func CreateConnection(cfg models.DBConfig) (Connection, error) {
    switch cfg.Type {
    case "oracle":
        conn := &OracleConnection{BaseConnection{Config_: cfg}}
        return conn, conn.Connect()
    // ...
    }
}
```

### 8.2 添加新的巡检项

#### 步骤

1. **创建巡检器**：实现 `Inspector` 接口

```go
// internal/inspector/security.go
type SecurityInspector struct {
    conn connection.Connection
    cfg  models.InspectionConfig
}

func (i *SecurityInspector) Name() string  { return "security" }
func (i *SecurityInspector) Title() string { return "检查安全配置" }

func (i *SecurityInspector) Inspect() (map[string]interface{}, error) {
    return map[string]interface{}{
        "user_privileges": i.checkUserPrivileges(),
        "ssl_config":      i.checkSSLConfig(),
    }, nil
}
```

2. **注册到注册表**：在 `inspector.go` 的 `init()` 中注册

```go
func init() {
    // ... 已有注册
    Register("security", func(conn connection.Connection, cfg models.InspectionConfig) Inspector {
        return NewSecurityInspector(conn, cfg)
    })
}
```

3. **更新配置**：在 `config.yaml` 中添加开关

```yaml
inspection:
  checks:
    security: true
```

### 8.3 添加新的报告格式

#### 步骤

1. **创建报告生成器**：实现 `Reporter` 接口

```go
// internal/reporter/csv_reporter.go
type CSVReporter struct {
    outputDir string
}

func (r *CSVReporter) Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error) {
    // 实现 CSV 生成逻辑
    return filepath, nil
}
```

2. **注册到工厂**：在 `CreateReporter()` 中添加

```go
func CreateReporter(format, outputDir string) (Reporter, error) {
    switch format {
    case "csv":
        return NewCSVReporter(outputDir), nil
    // ...
    }
}
```

### 8.4 交叉编译支持

Go 天然支持交叉编译，无需额外工具链：

```bash
# Linux ARM64 (银河麒麟)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o db-patrol-linux-arm64 .

# Linux x86_64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o db-patrol-linux-amd64 .

# macOS ARM64 (M1/M2)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o db-patrol-darwin-arm64 .

# Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o db-patrol.exe .
```

---

## 附录

### A. 术语表

| 术语 | 说明 |
|-----|------|
| Vastbase | 海量数据公司开发的关系型数据库，兼容 PostgreSQL 和 MySQL 协议 |
| Schema | 数据库中的逻辑命名空间，用于组织表、视图等对象 |
| 巡检 | 定期检查数据库的健康状态和性能指标 |
| 慢查询 | 执行时间超过设定阈值的 SQL 查询 |
| 死元组 | PostgreSQL 中已被更新或删除但尚未清理的行版本 |
| goroutine | Go 语言的轻量级线程 |
| embed | Go 1.16+ 的编译时文件嵌入特性 |

### B. 参考资料

- [Go 官方文档](https://go.dev/doc/)
- [PostgreSQL 官方文档](https://www.postgresql.org/docs/)
- [cobra 文档](https://github.com/spf13/cobra)
- [lib/pq 文档](https://github.com/lib/pq)
- [go-sql-driver/mysql 文档](https://github.com/go-sql-driver/mysql)
- [Go text/template 文档](https://pkg.go.dev/text/template)

---

**文档版本**: 2.0  
**最后更新**: 2026-06-08  
**更新内容**: 从 Python 重写为 Go，新增交叉编译支持、单文件部署、go:embed 模板嵌入
