# 数据库巡检工具 - 用户使用手册

## 目录
1. [快速开始](#快速开始)
2. [系统要求](#系统要求)
3. [安装说明](#安装说明)
4. [使用方法](#使用方法)
5. [命令行参数](#命令行参数)
6. [配置说明](#配置说明)
7. [示例](#示例)
8. [常见问题](#常见问题)

---

## 快速开始

```bash
# 1. 解压分发包
unzip db-inspector-package.zip

# 2. 运行巡检（以 PostgreSQL 为例）
python db-inspector.pyz --db-host 192.168.1.1 --db-port 5432 \
  --db-user admin --db-password "your_password" \
  --db-database mydb --db-type postgresql

# 3. 查看报告
# 报告将生成在 ./reports/ 目录下
```

---

## 系统要求

| 项目 | 要求 |
|------|------|
| Python | 3.8 或更高版本 |
| 操作系统 | Windows / Linux / macOS |
| 内存 | 至少 512MB 可用内存 |
| 磁盘空间 | 至少 100MB 可用空间 |

---

## 安装说明

### 方式一：使用 zipapp（推荐）

1. 解压 `db-inspector-package.zip`
2. 确保已安装 Python 3.8+
3. 首次运行时会自动安装数据库驱动依赖

### 方式二：手动安装依赖（可选）

如果希望提前安装依赖，可以运行：

```bash
pip install pymysql psycopg2-binary
```

---

## 使用方法

### Windows

```bash
# 使用批处理脚本
db-inspector.bat --help

# 或直接运行
db-inspector.bat --db-host 192.168.1.1 --db-port 5432 --db-user admin --db-password "pass" --db-database mydb --db-type postgresql
```

### Linux / macOS

```bash
# 赋予执行权限
chmod +x db-inspector.sh

# 运行
./db-inspector.sh --db-host 192.168.1.1 --db-port 5432 --db-user admin --db-password "pass" --db-database mydb --db-type postgresql
```

### 直接运行 Python

```bash
python db-inspector.pyz [选项]
```

---

## 命令行参数

### 数据库连接参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--db-host` | 数据库主机地址 | `192.168.1.1` |
| `--db-port` | 数据库端口 | `5432` (PostgreSQL) / `3306` (MySQL) |
| `--db-user` | 数据库用户名 | `admin` |
| `--db-password` | 数据库密码 | `your_password` |
| `--db-database` | 要连接的数据库名 | `mydb` |
| `--db-type` | 数据库类型 | `postgresql` / `mysql` / `vastbase_pg` |
| `--db-name` | 数据库标识名称（用于报告） | `生产库` |
| `--db-schema` | 数据库 schema | `public` |

### 其他参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-c, --config` | 配置文件路径 | `config.yaml` |
| `-d, --database` | 指定要巡检的数据库名称 | `生产库` |
| `-f, --format` | 报告输出格式 | `html` / `markdown` / `json` |
| `--db-json` | JSON 格式的多数据库配置 | 见下方示例 |

---

## 配置说明

### 配置文件 (config.yaml)

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
    basic_info: true      # 基础信息
    performance: true     # 性能检查
    schema: false         # 设计规范检查（默认关闭）
    table_structure: true # 表结构检查
    index_check: true     # 索引检查
    security: true        # 安全检查

# 报告配置
report:
  format: html           # 输出格式
  output_dir: ./reports  # 输出目录
  include_suggestions: true  # 包含优化建议
```

---

## 示例

### 示例 1：巡检单个 PostgreSQL 数据库

```bash
python db-inspector.pyz \
  --db-host 192.168.10.70 \
  --db-port 5432 \
  --db-user db_user \
  --db-password "Vbase@1234" \
  --db-database segh_yy \
  --db-name "生产环境PostgreSQL" \
  --db-type postgresql
```

### 示例 2：巡检单个 MySQL 数据库

```bash
python db-inspector.pyz \
  --db-host 192.168.1.100 \
  --db-port 3306 \
  --db-user root \
  --db-password "mysql_pass" \
  --db-database myapp \
  --db-name "MySQL生产库" \
  --db-type mysql
```

### 示例 3：使用 JSON 配置巡检多个数据库

```bash
python db-inspector.pyz --db-json '[
  {
    "name": "PostgreSQL生产库",
    "type": "postgresql",
    "host": "192.168.1.10",
    "port": 5432,
    "user": "postgres",
    "password": "pass1",
    "database": "prod_db"
  },
  {
    "name": "MySQL测试库",
    "type": "mysql",
    "host": "192.168.1.20",
    "port": 3306,
    "user": "root",
    "password": "pass2",
    "database": "test_db"
  }
]'
```

### 示例 4：输出 Markdown 格式报告

```bash
python db-inspector.pyz \
  --db-host 192.168.1.1 \
  --db-user admin \
  --db-password "pass" \
  --db-database mydb \
  --db-type postgresql \
  --format markdown
```

---

## 常见问题

### Q1: 首次运行提示安装依赖？

**A:** 这是正常现象。工具需要 `pymysql` 和 `psycopg2-binary` 来连接数据库。首次运行时会自动安装，或您可以提前运行：

```bash
pip install pymysql psycopg2-binary
```

### Q2: 如何查看生成的报告？

**A:** 报告默认生成在 `./reports/` 目录下：
- HTML 报告：用浏览器打开 `.html` 文件
- Markdown 报告：用文本编辑器或 Markdown 查看器打开
- JSON 报告：用文本编辑器或 JSON 查看器打开

### Q3: 支持哪些数据库类型？

**A:** 目前支持：
- PostgreSQL (`postgresql`)
- MySQL (`mysql`)
- Vastbase PG 模式 (`vastbase_pg`)

### Q4: 如何指定配置文件？

**A:** 使用 `-c` 或 `--config` 参数：

```bash
python db-inspector.pyz -c /path/to/config.yaml --db-host ...
```

### Q5: 巡检需要哪些数据库权限？

**A:** 建议授予以下权限：
- 连接数据库权限
- 读取系统表权限（如 `pg_stat_*` 或 `information_schema`）
- 读取表结构权限

### Q6: 报告中的中文显示乱码？

**A:** 确保您的终端或浏览器使用 UTF-8 编码。Windows 用户建议使用 PowerShell 或 Git Bash。

---

## 技术支持

如有问题，请查看项目文档或联系技术支持。

---

**版本：** 1.0.0  
**更新日期：** 2026-04-22
