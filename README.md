# 数据库巡检工具 (DB Inspector)

一款专业的数据库巡检工具，支持 Vastbase (PG/MySQL 模式)、MySQL、PostgreSQL，可生成包含数据库运行状况、性能指标和设计规范检查的详细报告。

## ✨ 安全特性

- 🔒 **敏感信息保护**：数据库连接配置（尤其是密码）不存储在配置文件中
- 🎯 **命令行参数传递**：支持通过命令行参数直接传递数据库连接信息
- 📋 **JSON 批量配置**：支持通过 JSON 字符串传递多个数据库配置
- ⚠️ **安全警告**：从配置文件读取数据库配置时给出安全提示

## 功能特性

### 数据库实例基本运行情况
- ✅ 数据库连接状态检查
- ✅ 版本信息和运行时间
- ✅ 实例配置参数采集（共享缓冲区、时区、监听地址等）
- ✅ 数据库列表及详细信息（大小、表数量、视图数量、触发器数量）
- ✅ 数据表清单（大小、行数、字段数）
- ✅ 智能识别疑似备份库和备份表
- ✅ 检测缺少主键或唯一索引的表

### 数据库性能指标
- ✅ 连接数统计（当前/最大/活跃/空闲）
- ✅ 客户端连接详情（按 IP 分组统计）
- ✅ 缓存命中率与索引命中率分析
- ✅ 当前活动会话监控
- ✅ 锁等待检测（含阻塞链分析）
- ✅ 长事务检测（默认>5分钟，分级显示）
- ✅ 慢查询分析
- ✅ 表统计信息（活元组/死元组）
- ✅ 索引使用统计
- ✅ 索引大小占比分析（识别索引过大的表）
- ✅ 无效索引检测（失效索引、未使用的大索引）
- ✅ 重复索引检测
- ✅ 死元组分析与表膨胀检测
- ✅ VACUUM/ANALYZE 执行情况
- ✅ IO 密集表统计

### 智能检测功能
- 🔍 **备份库检测**：自动识别命名符合备份特征的数据库（强特征/弱特征/白名单）
- 🔍 **备份表检测**：自动识别命名符合备份特征的表（含排除规则）
- 🔍 **无主键表检测**：识别缺少主键或唯一索引的表（按数据库分组）
- 🔍 **索引膨胀检测**：分析索引大小占比，标记需要优化的表
- 🔍 **无效索引检测**：识别失效索引和从未使用的大索引
- 🔍 **重复索引检测**：识别相同列上的多个索引
- 🔍 **长事务检测**：识别运行超过阈值的事务，分级告警
- 🔍 **锁等待检测**：识别阻塞链，分析锁竞争
- 🔍 **表膨胀检测**：分析死元组比例，建议 VACUUM

### 报告输出
- 📄 HTML 报告（美观的可视化界面，支持响应式布局）
- 📄 Markdown 报告（适合版本控制和文档集成）
- 📄 JSON 报告（便于程序化处理和数据交换）

## 安装

### 1. 克隆或下载项目

```bash
cd db-patrol
```

### 2. 安装依赖

```bash
pip install -r requirements.txt
```

依赖列表：
- pymysql >= 1.1.0
- psycopg2-binary >= 2.9.9
- sqlalchemy >= 2.0.0
- jinja2 >= 3.1.0
- pyyaml >= 6.0
- click >= 8.1.0
- tabulate >= 0.9.0
- colorama >= 0.4.6

## 配置

### 配置文件说明

编辑 `config.yaml` 文件配置**巡检参数和报告设置**（注意：数据库连接配置不存储在此文件中，应通过命令行参数传递）：

```yaml
# 巡检配置
inspection:
  # 慢查询阈值（秒）
  slow_query_threshold: 1.0
  
  # 最大连接数警告阈值（百分比）
  max_connections_threshold: 80
  
  # 表大小警告阈值（MB）
  table_size_threshold: 1024
  
  # 检查项开关
  checks:
    basic_info: true      # 基本信息检查
    performance: true     # 性能检查
    schema: false         # 设计规范检查（默认关闭，启用后检查命名规范、主键、索引等）
    table_structure: true # 表结构检查
    index_check: true     # 索引检查
    security: true        # 安全检查

# 报告配置
report:
  format: html            # 输出格式: html, markdown, json
  output_dir: ./reports   # 报告输出目录
  include_suggestions: true  # 是否包含优化建议
```

### 配置项说明

#### 巡检配置

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|-------|------|
| slow_query_threshold | float | 1.0 | 慢查询阈值，超过此值的查询被视为慢查询 |
| max_connections_threshold | int | 80 | 连接数警告阈值，超过此百分比标记为警告 |
| table_size_threshold | int | 1024 | 大表阈值，超过此大小（MB）被视为大表 |
| checks.basic_info | bool | true | 是否执行基本信息检查 |
| checks.performance | bool | true | 是否执行性能检查 |
| checks.schema | bool | false | 是否执行设计规范检查（默认关闭） |
| checks.table_structure | bool | true | 是否执行表结构检查 |
| checks.index_check | bool | true | 是否执行索引检查 |
| checks.security | bool | true | 是否执行安全检查 |

#### 报告配置

| 配置项 | 类型 | 默认值 | 说明 |
|-------|------|-------|------|
| format | string | html | 报告输出格式 |
| output_dir | string | ./reports | 报告输出目录路径 |
| include_suggestions | bool | true | 是否在报告中包含优化建议 |

## 使用方法

### 方式一：通过单独参数传递（推荐）

```bash
python main.py --db-host 192.168.1.1 --db-port 5432 \
               --db-user admin --db-password "your_password" \
               --db-name "Vastbase-生产库" --db-type vastbase_pg \
               --db-database vastbase --db-schema public
```

### 方式二：通过 JSON 批量传递（支持多个数据库）

```bash
python main.py --db-json '[
  {
    "name": "Vastbase-PG-生产库",
    "type": "vastbase_pg",
    "host": "192.168.1.10",
    "port": 5432,
    "user": "admin",
    "password": "password123",
    "database": "vastbase",
    "schema": "public"
  },
  {
    "name": "MySQL-业务库",
    "type": "mysql",
    "host": "192.168.1.11",
    "port": 3306,
    "user": "root",
    "password": "password456",
    "database": "business",
    "charset": "utf8mb4"
  }
]'
```

### 方式三：使用配置文件（不推荐，仅用于开发环境）

```bash
# 如果在 config.yaml 中配置了数据库连接（不推荐生产环境）
python main.py

# 使用自定义配置文件
python main.py -c custom_config.yaml

# 只巡检指定的数据库
python main.py -d "Vastbase-PG-生产库"

# 指定输出格式（覆盖配置文件中的设置）
python main.py -f markdown
```

### 完整的命令行参数

```
Options:
  -c, --config PATH               配置文件路径 [默认: config.yaml]
  -d, --database TEXT             指定要巡检的数据库名称
  -f, --format [html|markdown|json]
                                  报告输出格式
  --db-host TEXT                  数据库主机地址
  --db-port INTEGER               数据库端口
  --db-user TEXT                  数据库用户名
  --db-password TEXT              数据库密码
  --db-name TEXT                  数据库标识名称
  --db-type [vastbase_pg|mysql|postgresql]
                                  数据库类型
  --db-database TEXT              要连接的数据库名
  --db-schema TEXT                数据库schema (PG模式)
  --db-json TEXT                  数据库配置JSON字符串，支持多个数据库
  --help                          显示帮助信息
```

### 数据库连接参数说明

| 参数 | 必填 | 说明 |
|-----|-----|------|
| `--db-host` | 是 | 数据库服务器地址 |
| `--db-user` | 是 | 连接用户名 |
| `--db-password` | 是 | 连接密码 |
| `--db-database` | 是 | 默认连接的数据库 |
| `--db-type` | 是 | 数据库类型：`vastbase_pg`、`mysql`、`postgresql` |
| `--db-name` | 否 | 实例显示名称，默认使用数据库名 |
| `--db-port` | 否 | 端口号，PG默认5432，MySQL默认3306 |
| `--db-schema` | 否 | PG 模式下的默认 Schema，默认 public |

## 项目结构

```
db-patrol/
├── config.yaml              # 配置文件（仅巡检和报告配置）
├── main.py                  # 主入口（CLI）
├── requirements.txt         # 依赖列表
├── build.py                 # 打包脚本
├── README.md               # 说明文档
├── USER_GUIDE.md           # 用户使用手册
├── docs/                   # 文档目录
│   └── design_document.md  # 设计文档
│
├── db_inspector/           # 核心代码包
│   ├── __init__.py
│   ├── connection.py       # 数据库连接管理（工厂模式）
│   ├── core.py            # 巡检核心逻辑控制器
│   ├── config_builder.py  # CLI 配置构建与校验
│   ├── log.py             # 统一日志模块
│   ├── utils.py           # 工具函数
│   │
│   ├── inspectors/        # 巡检器模块（注册表模式）
│   │   ├── __init__.py    # Inspector 注册表 + get_inspectors() 工厂
│   │   ├── base.py        # 巡检器抽象基类（含 connection_factory 注入）
│   │   ├── basic_info.py  # 基本信息巡检器（版本、配置、数据库列表、表清单、备份检测、主键检查）
│   │   ├── performance.py # 性能巡检器（连接、缓存、索引、慢查询、锁、长事务、死元组、VACUUM、IO、无效/重复索引）
│   │   └── schema.py      # 设计规范巡检器（已集成，默认关闭）
│   │
│   └── reporters/         # 报告生成器模块（注册表模式）
│       ├── __init__.py    # Reporter 注册表 + create_reporter() 工厂
│       ├── base.py        # 报告生成器抽象基类
│       ├── scoring.py     # 健康评分引擎
│       ├── html_reporter.py      # HTML 报告生成器（Jinja2 FileSystemLoader）
│       ├── markdown_reporter.py  # Markdown 报告生成器
│       ├── json_reporter.py      # JSON 报告生成器
│       └── templates/
│           └── report.html.j2   # HTML 报告模板
│
├── reports/               # 生成的报告目录（自动创建）
├── build/                 # 打包构建目录（自动清理）
└── dist/                  # 打包输出目录
    ├── db-inspector.pyz          # zipapp 主程序
    ├── db-inspector.bat          # Windows 启动脚本
    ├── db-inspector.sh           # Linux/Mac 启动脚本
    └── db-inspector-package.zip  # 完整分发包
```

## 报告内容说明

### HTML 报告

HTML 报告采用现代化 UI 设计，包含以下章节：

1. **概览卡片**：连接状态、数据库数量、表总数、实例总大小
2. **健康评分**：综合评分（0-100分），基于8个维度（连接使用率、缓存命中率、索引命中率、主键完整性、备份数据清理、索引大小占比、无效索引、重复索引）
3. **关键发现与建议**：自动检测并高亮严重问题（连接使用率告警、缓存命中率低、长事务、锁等待、缺少主键等）
4. **实例基本信息**：版本、启动时间、数据目录、监听地址、端口、连接数、共享缓冲区、时区、大小写配置
5. **实例配置**：关键配置参数列表（连接、内存、WAL、检查点、自动清理、查询规划器、并行查询、日志等）
6. **数据库列表**：所有数据库的详细信息（大小、Schema数、表数量、视图数量、触发器数量、字符集）
7. **疑似备份库**：高亮显示命名符合备份特征的数据库
8. **数据表清单**：按数据库分组，动态展示（基于1GB阈值，最少10个，最多50个表）
9. **疑似备份表**：高亮显示命名符合备份特征的表
10. **缺少主键或唯一索引的表**：按数据库分组，列出需要添加主键的表
11. **性能指标**：
    - 连接统计（当前/最大/活跃/空闲/使用率/状态）
    - 客户端连接详情（按 IP 分组，显示访问的数据库、用户、应用）
    - 缓存命中率与索引命中率分析
    - 长事务检测（运行超过5分钟的事务，分级显示）
    - 锁等待检测（含阻塞链分析，分级显示）
    - 死元组分析与表膨胀检测
    - VACUUM/ANALYZE 执行情况
    - IO 密集表统计
    - 索引大小占比分析（标记索引占比超过 30% 的表）
    - 无效索引检测（失效索引、从未使用的大索引）
    - 重复索引检测（相同列上的多个索引）
    - 慢查询分析（依赖 pg_stat_statements）
    - 表统计信息（活元组/死元组）
    - 索引使用统计

### Markdown 报告

适合在 GitLab、GitHub、Markdown 编辑器中查看，包含：
- 结构化的检查结果
- 表格形式的统计数据
- 问题列表和优化建议
- 便于版本控制和 diff 对比

### JSON 报告

便于程序化处理，包含：
- 完整的元数据（实例名、类型、生成时间等）
- 结构化的巡检结果
- 便于与其他系统集成或自动化分析

## 智能检测规则

### 备份库检测

命名符合以下特征的数据库被视为疑似备份库：
- **强特征**：包含 `_backup`、`_bak`、`_copy`、`_old`、`_test`、`_temp`、`_dev` 关键字，或以日期结尾（8位/6位/4位数字）
- **弱特征**：以数字结尾（3位以上），且存在相似基础名称的数据库
- **白名单**：`emate_dev` 等明确排除的正常库

### 备份表检测

命名符合以下特征的表被视为疑似备份表：
- 包含 `_backup`、`_bak`、`_copy`、`_old`、`_new`、`_temp`、`_tmp` 关键字
- 以 `_` + 4位/6位/8位数字结尾
- **排除项**：`_nod_old`、`_lin_old`、`_net_old` 等正常命名模式

### 索引大小分析

对数据量 >= 1000 行的表进行索引大小分析：
- **严重**：索引占比 > 50%，或索引数量 > 10个，建议重建索引或清理冗余索引
- **关注**：索引占比 > 30%，建议关注索引使用情况
- **正常**：索引占比 <= 30%

### 健康评分体系

HTML 报告包含综合健康评分（0-100分），基于以下8个维度：

| 维度 | 权重 | 评分标准 |
|------|------|----------|
| 连接使用率 | 15分 | >90%扣15分，>80%扣10分，>60%扣5分 |
| 缓存命中率 | 20分 | <90%扣20分，<95%扣15分，<99%扣5分 |
| 索引命中率 | 15分 | <50%扣15分，<70%扣10分，<90%扣5分 |
| 主键完整性 | 10分 | >20个表无主键扣10分，>10个扣7分，>0个扣3分 |
| 备份数据清理 | 10分 | 备份库>5或备份表>20扣10分，否则扣5分 |
| 索引大小占比 | 15分 | >10个表严重超标扣15分，>5个扣10分，>0个扣5分 |
| 无效索引 | 10分 | 发现无效索引扣10分 |
| 重复索引 | 5分 | 发现重复索引扣5分 |

评分等级：
- **优秀**（90-100分）：数据库运行状态良好
- **良好**（75-89分）：数据库运行状态较好，部分指标需关注
- **一般**（60-74分）：数据库存在一些问题，建议优化
- **较差**（0-59分）：数据库存在较多问题，需要立即优化

## 注意事项

1. **权限要求**：巡检账号需要足够的权限读取系统表和视图（如 `pg_stat_activity`、`pg_stat_user_tables`、`pg_statio_user_tables` 等）

2. **生产环境**：建议在业务低峰期执行巡检。巡检采用并发采集，约8秒完成

3. **慢查询分析**：PG 模式需要安装 `pg_stat_statements` 扩展才能获取慢查询详情。如未安装，慢查询部分将为空

4. **长事务检测**：默认检测运行超过5分钟（300秒）的事务，可通过配置文件调整

5. **连接配置**：确保数据库服务器允许从巡检工具所在机器连接

6. **字符编码**：配置文件和报告默认使用 UTF-8 编码

7. **安全建议**：
   - 生产环境务必使用命令行参数传递数据库密码
   - 避免将密码写入脚本文件或配置文件
   - 考虑使用环境变量结合命令行参数的方式

8. **Schema 巡检器**：项目包含 `schema.py` 设计规范巡检器（检查命名规范、主键、索引、约束、数据类型、注释等），已集成到主流程中，在 `config.yaml` 中设置 `checks.schema: true` 即可启用

## 打包部署

本项目使用 Python 标准库 `zipapp` 实现单文件打包，打包后的文件包含纯 Python 依赖和默认配置，可跨平台分发运行。

### 一键打包

项目根目录提供了 `build.py` 一键打包脚本：

```bash
python build.py
```

### 打包说明
- **自动安装依赖**：自动安装纯 Python 依赖（SQLAlchemy、Jinja2、Click 等）到打包文件中
- **包含默认配置**：自动包含 `config.yaml` 默认配置文件
- **自动清理**：每次打包自动清空 build 和 dist 目录旧文件
- **输出文件**：
  - `dist/db-inspector.pyz` - 主程序（约 2.7 MB）
  - `dist/db-inspector.bat` - Windows 启动脚本
  - `dist/db-inspector.sh` - Linux/Mac 启动脚本
  - `dist/db-inspector-package.zip` - 完整分发包

### 跨平台运行
打包后的 `.pyz` 文件支持 Windows/Linux/macOS 全平台运行，目标环境只需安装 Python 3.8+ 即可：

```bash
# 直接使用 zipapp
python db-inspector.pyz --help

# Windows 使用批处理脚本
db-inspector.bat --help

# Linux/macOS 使用 shell 脚本
chmod +x db-inspector.sh
./db-inspector.sh --help
```

### 首次运行
打包文件不包含二进制依赖（数据库驱动），首次运行时会自动安装：

```bash
# 首次运行会自动安装 pymysql 和 psycopg2-binary
python db-inspector.pyz --db-host 192.168.1.1 --db-user admin ...

# 或提前手动安装依赖
pip install pymysql psycopg2-binary
```

### 分发包内容
`db-inspector-package.zip` 包含：
- `db-inspector.pyz` - 主程序
- `db-inspector.bat` / `db-inspector.sh` - 启动脚本
- `config.yaml` - 默认配置文件
- `USER_GUIDE.md` - 用户使用手册

## 扩展开发

### 添加新的数据库类型支持

1. 在 `db_inspector/connection.py` 中创建新的连接类，继承 `DatabaseConnection`
2. 实现 `connect()`、`execute_query()`、`execute()` 方法
3. 在 `create_connection()` 函数中添加类型判断

### 添加新的巡检项

1. 在 `db_inspector/inspectors/` 中创建新的巡检器类，继承 `BaseInspector`
2. 设置 `name` 和 `title` 类属性（用于注册表和终端显示）
3. 实现 `inspect()` 方法返回检查结果字典
4. 在 `inspectors/__init__.py` 的 `get_inspectors()` 中注册
5. 在 `config.yaml` 的 `checks` 下添加开关

### 添加新的报告格式

1. 在 `db_inspector/reporters/` 中创建新的报告生成器类，继承 `BaseReporter`
2. 实现 `generate(db_config, results)` 方法返回文件路径
3. 在 `reporters/__init__.py` 的 `create_reporter()` 中注册

## 设计文档

详细的设计文档请参见 [docs/design_document.md](docs/design_document.md)，包含：
- 系统架构设计
- 模块详细设计
- 数据流设计
- 配置设计
- 扩展性设计

## License

MIT License

---

**版本**: 1.2  
**最后更新**: 2026-04-23
