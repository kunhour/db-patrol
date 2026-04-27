# AGENTS.md - DB Patrol AI 助手指南

## 项目概述

**db-patrol** 是数据库巡检工具，支持 Vastbase (PG/MySQL)、MySQL、PostgreSQL，生成健康评分报告。

**技术栈**: Python 3.8+ / pymysql / psycopg2 / Click 8.1+ / Jinja2 3.1+ / PyYAML 6.0 / Colorama / Tabulate

## 项目结构

```
db-patrol/
├── main.py                  # CLI 入口 (Click)
├── config.yaml              # 巡检配置
├── build.py                 # 打包脚本
├── db_inspector/
│   ├── __init__.py          # 包初始化 (logger 初始化)
│   ├── connection.py        # 数据库连接 (工厂模式 + 上下文管理器)
│   ├── core.py              # DBInspector 核心控制器
│   ├── config_builder.py    # CLI 配置构建与校验
│   ├── log.py               # 统一日志模块 (ColoramaStreamHandler)
│   ├── utils.py             # format_size() 工具函数
│   ├── inspectors/          # 巡检器 (注册表模式 + 策略模式)
│   │   ├── __init__.py      # Inspector 注册表 + get_inspectors() 工厂
│   │   ├── base.py          # BaseInspector (ABC) + connection_factory 注入
│   │   ├── basic_info.py    # 基本信息巡检
│   │   ├── performance.py   # 性能巡检
│   │   └── schema.py        # 设计规范巡检
│   └── reporters/
│       ├── __init__.py      # Reporter 注册表 + create_reporter() 工厂
│       ├── base.py          # BaseReporter (ABC)
│       ├── scoring.py       # 健康评分引擎
│       ├── html_reporter.py # HTML 报告 (Jinja2 FileSystemLoader)
│       ├── markdown_reporter.py
│       ├── json_reporter.py
│       └── templates/
│           └── report.html.j2  # HTML 报告模板
└── dist/                    # 打包输出
```

## 架构

**数据流**: CLI → DBInspector → create_connection → get_inspectors() → Inspector.inspect() → create_reporter() → Reporter.generate()

**核心模式**:
- **工厂模式**: `create_connection()` 按 `db_type` 创建连接 (`vastbase_pg` → VastbasePGConnection, `vastbase_mysql` → VastbaseMySQLConnection)
- **注册表模式**: `get_inspectors()` 从注册表获取所有 Inspector 实例; `create_reporter()` 从注册表获取 Reporter 实例
- **策略模式**: `BaseInspector`(ABC) → `BasicInfoInspector`, `PerformanceInspector`, `SchemaInspector`
- **上下文管理器**: `DatabaseConnection` 支持 `with` 语句
- **并发扫描**: `ThreadPoolExecutor`(max 16) 全库并发采集
- **依赖注入**: `connection_factory` 可注入到 Inspector，支持测试 mock 和连接池扩展

**类关系**:
- `DatabaseConnection`(ABC) ← `VastbasePGConnection`, `VastbaseMySQLConnection`
- `BaseInspector`(ABC) ← `BasicInfoInspector`, `PerformanceInspector`, `SchemaInspector`
- `BaseReporter`(ABC) ← `HTMLReporter`, `MarkdownReporter`, `JSONReporter`

## 代码约定

- **命名**: PascalCase(类) / snake_case(函数) / UPPER_SNAKE_CASE(常量) / `_`前缀(私有)
- **类型注解**: 所有函数/方法必须标注返回类型
- **错误处理**: `except Exception as e` 捕获异常，查询失败返回默认值/空结果，**不中断巡检流程**; 禁止裸 `except:`
- **日志**: Inspector 内部使用 `logging.getLogger('db_patrol')` 而非 `print()`; core.py 保留 colorama 彩色终端输出
- **模板**: HTML 报告模板存放在 `reporters/templates/` 目录，使用 Jinja2 `FileSystemLoader` 加载
- **编码**: 全程 UTF-8

## 扩展指南

| 扩展类型 | 步骤 |
|---------|------|
| 新数据库 | 1. `connection.py` 新建连接类继承 `DatabaseConnection` 2. 实现 `connect()/execute_query()/execute()` 3. `create_connection()` 添加分支 |
| 新巡检项 | 1. `inspectors/` 新建类继承 `BaseInspector` 2. 设置 `name`/`title` 类属性 3. 实现 `inspect()` 返回 `Dict` 4. `inspectors/__init__.py` 的 `get_inspectors()` 中注册 5. `config.yaml` 的 `checks` 添加开关 |
| 新报告格式 | 1. `reporters/` 新建类继承 `BaseReporter` 2. 实现 `generate(db_config, results)` 返回文件路径 3. `reporters/__init__.py` 的 `create_reporter()` 中注册 |

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

## 注意事项

- **Schema 巡检器**: `schema.py` 已集成到巡检流程，在 `config.yaml` 中通过 `checks.schema: true` 启用（默认关闭）
- **慢查询**: PG 需要 `pg_stat_statements` 扩展
- **数据库类型**: CLI 支持 `vastbase_pg`/`postgresql`/`mysql`/`vastbase_mysql` 四种类型，但 `create_connection()` 目前仅实现了 `vastbase_pg` 和 `vastbase_mysql` 两种连接
- **安全**: 密码不存配置文件，推荐命令行参数或 `--db-json` 传递
