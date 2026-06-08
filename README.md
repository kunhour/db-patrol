# 数据库巡检工具 (DB Patrol)

一款专业的数据库巡检工具，支持 Vastbase (PG/MySQL 模式)、MySQL、PostgreSQL，可生成包含数据库运行状况、性能指标和设计规范检查的详细报告。

## 特性

- **单文件部署**：编译为单个静态二进制文件，无需安装任何依赖
- **跨平台支持**：支持 Windows/Linux/macOS，可交叉编译到 ARM64 等架构
- **敏感信息保护**：数据库密码通过命令行参数传递，不存储在配置文件中
- **多种报告格式**：支持 HTML、Markdown、JSON 三种报告格式
- **并发采集**：使用 goroutine 并发采集数据库信息，高效快速

## 支持的数据库

- PostgreSQL
- Vastbase (PG 模式)
- MySQL
- Vastbase (MySQL 模式)

## 快速开始

### 下载

从发布页面下载对应平台的二进制文件：
- `db-patrol.exe` - Windows
- `db-patrol-linux-arm64` - Linux ARM64 (银河麒麟等)
- `db-patrol-darwin-arm64` - macOS ARM64 (M1/M2)

### 使用

```bash
# 查看帮助
./db-patrol --help

# 巡检单个数据库
./db-patrol --db-host 192.168.1.1 --db-port 5432 \
            --db-user admin --db-password "your_password" \
            --db-name "生产库" --db-type postgresql \
            --db-database mydb

# 使用环境变量传递密码（推荐）
export DB_PASSWORD=your_password
./db-patrol --db-host 192.168.1.1 --db-user admin \
            --db-name "生产库" --db-type postgresql \
            --db-database mydb

# 批量巡检多个数据库
./db-patrol --db-json '[
  {"name":"DB1", "type":"postgresql", "host":"192.168.1.10", "user":"admin", "password":"pass1", "database":"db1"},
  {"name":"DB2", "type":"mysql", "host":"192.168.1.11", "user":"root", "password":"pass2", "database":"db2"}
]'
```

### 报告输出

报告默认生成在 `./reports/` 目录下：

```bash
# 指定报告格式
./db-patrol --format html ...    # HTML 报告（默认）
./db-patrol --format markdown ... # Markdown 报告
./db-patrol --format json ...     # JSON 报告
```

## 命令行参数

### 数据库连接参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--db-host` | 数据库主机地址 | 必填 |
| `--db-port` | 数据库端口 | PG:5432 / MySQL:3306 |
| `--db-user` | 数据库用户名 | 必填 |
| `--db-password` | 数据库密码（或设置 `DB_PASSWORD` 环境变量） | 必填 |
| `--db-name` | 数据库标识名称（用于报告） | 使用 database |
| `--db-type` | 数据库类型 | 必填 |
| `--db-database` | 要连接的数据库名 | 必填 |
| `--db-schema` | 数据库 schema (PG) | public |

**`--db-type` 支持的值**：
- `postgresql` - PostgreSQL
- `vastbase_pg` - Vastbase PG 模式
- `mysql` - MySQL
- `vastbase_mysql` - Vastbase MySQL 模式

### 其他参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-c, --config` | 配置文件路径 | config.yaml |
| `-d, --database` | 指定要巡检的数据库名称 | 全部 |
| `-f, --format` | 报告输出格式 (html/markdown/json) | html |
| `--db-json` | JSON 格式的多数据库配置 | - |

## 配置说明

配置文件 `config.yaml` 用于设置巡检参数和报告选项：

```yaml
# 巡检配置
inspection:
  slow_query_threshold: 1.0        # 慢查询阈值（秒）
  max_connections_threshold: 80    # 连接数警告阈值（百分比）
  table_size_threshold: 1024       # 大表阈值（MB）
  long_transaction_threshold: 300  # 长事务阈值（秒）
  
  checks:
    basic_info: true      # 基本信息检查
    performance: true     # 性能检查
    schema: false         # 设计规范检查（默认关闭）

# 报告配置
report:
  format: html            # 输出格式: html, markdown, json
  output_dir: ./reports   # 报告输出目录
```

## 巡检内容

### 基本信息
- 数据库连接状态
- 版本信息和运行时间
- 实例配置参数
- 数据库列表及大小
- 表清单及统计
- 备份库/备份表检测
- 无主键表检测

### 性能指标
- 连接数统计（当前/最大/活跃/空闲）
- 缓存命中率
- 索引命中率
- 锁等待检测
- 长事务检测
- 慢查询分析
- 死元组分析
- VACUUM 状态
- IO 统计
- 索引大小分析
- 无效索引检测
- 重复索引检测

### 设计规范（可选）
- 表命名规范
- 列命名规范
- 主键完整性
- 索引规范
- 数据类型检查
- 注释检查

## 健康评分

报告包含 100 分制的健康评分，基于以下维度：

| 维度 | 分值 | 扣分规则 |
|------|------|----------|
| 连接使用率 | 15 | >90% 扣15, >80% 扣10, >60% 扣5 |
| 缓存命中率 | 20 | <90% 扣20, <95% 扣15, <99% 扣5 |
| 索引命中率 | 15 | <50% 扣15, <70% 扣10, <90% 扣5 |
| 主键完整性 | 10 | >20个表无主键扣10, >10个扣7, >0个扣3 |
| 备份数据清理 | 10 | 备份库>5或备份表>20扣10, 否则扣5 |
| 索引大小占比 | 15 | >10个表严重超标扣15, >5个扣10, >0个扣5 |
| 无效索引 | 10 | 发现即扣10 |
| 重复索引 | 5 | 发现即扣5 |

**评分等级**：
- 优秀：90-100 分
- 良好：75-89 分
- 一般：60-74 分
- 较差：0-59 分

## 编译

如需从源码编译：

```bash
# 安装 Go 1.21+

# 下载依赖
go mod tidy

# 编译当前平台
go build -o db-patrol .

# 交叉编译到 Linux ARM64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o db-patrol-linux-arm64 .

# 交叉编译到 macOS ARM64
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o db-patrol-darwin-arm64 .
```

## 注意事项

1. **权限要求**：巡检账号需要读取系统表的权限（如 `pg_stat_*`、`information_schema`）
2. **慢查询**：PG 需要安装 `pg_stat_statements` 扩展
3. **Schema 巡检**：默认关闭，在配置文件中设置 `checks.schema: true` 启用
4. **生产环境**：建议在业务低峰期执行巡检

## License

MIT License
