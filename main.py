#!/usr/bin/env python3
"""
数据库巡检工具
支持 Vastbase PG/MySQL 模式、MySQL、PostgreSQL
"""

import click
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

def get_default_config_path():
    cwd_config = os.path.join(os.getcwd(), 'config.yaml')
    if os.path.exists(cwd_config):
        return cwd_config
    
    script_dir = os.path.dirname(os.path.abspath(__file__))
    script_config = os.path.join(script_dir, 'config.yaml')
    if os.path.exists(script_config):
        return script_config
    
    if script_dir.endswith('.pyz') or '.pyz' in script_dir:
        return 'config.yaml'
    
    return 'config.yaml'

from db_inspector.core import DBInspector
from db_inspector.config_builder import (
    parse_db_json, resolve_env_passwords, validate_db_configs, build_single_db_config
)


@click.command()
@click.option('--config', '-c', default=get_default_config_path, help='配置文件路径（用于巡检和报告配置）')
@click.option('--database', '-d', help='指定要巡检的数据库名称')
@click.option('--format', '-f', type=click.Choice(['html', 'markdown', 'json']), 
              help='报告输出格式')
@click.option('--db-host', help='数据库主机地址')
@click.option('--db-port', type=int, help='数据库端口')
@click.option('--db-user', help='数据库用户名')
@click.option('--db-password', envvar='DB_PASSWORD', help='数据库密码（也可通过 DB_PASSWORD 环境变量传递）')
@click.option('--db-name', help='数据库标识名称')
@click.option('--db-type', type=click.Choice(['vastbase_pg', 'mysql', 'postgresql']), 
              help='数据库类型')
@click.option('--db-database', help='要连接的数据库名')
@click.option('--db-schema', default='public', help='数据库schema')
@click.option('--db-json', help='数据库配置JSON字符串，支持多个数据库（例如: [{"name": "DB1", "type": "mysql", ...}]）')
def main(config, database, format, db_host, db_port, db_user, db_password, 
         db_name, db_type, db_database, db_schema, db_json):
    """
    数据库巡检工具
    
    示例:
        # 使用配置文件（不推荐，数据库配置应通过参数传递）
        python main.py -c config.yaml
        
        # 通过参数传递单个数据库配置
        python main.py --db-host 192.168.1.1 --db-port 5432 --db-user admin --db-password pass \\
                       --db-name "测试库" --db-type vastbase_pg --db-database segh_yy
        
        # 通过环境变量传递密码（推荐，避免密码出现在命令行）
        export DB_PASSWORD=your_password
        python main.py --db-host 192.168.1.1 --db-user admin --db-name "测试库" \\
                       --db-type vastbase_pg --db-database segh_yy
        
        # 通过JSON传递多个数据库配置（密码字段支持 $ENV_VAR 引用环境变量）
        python main.py --db-json '[{"name":"DB1", "type":"mysql", "host":"192.168.1.1", "password":"$DB_PWD", ...}]'
    """
    databases_config = None

    if db_json and (db_host or db_user or db_password or db_database):
        click.echo("错误: 不能同时使用 --db-json 参数和单独的数据库连接参数", err=True)
        click.echo("请选择要么使用 --db-json 传递配置，要么使用单独的 --db-host 等参数", err=True)
        sys.exit(1)

    if db_json:
        try:
            databases_config = parse_db_json(db_json)
        except json.JSONDecodeError as e:
            click.echo(f"错误: 数据库JSON格式无效: {e}", err=True)
            sys.exit(1)

        warnings = resolve_env_passwords(databases_config)
        for w in warnings:
            click.echo(f"警告: {w}", err=True)

        errors = validate_db_configs(databases_config)
        for e in errors:
            click.echo(f"错误: {e}", err=True)
        if errors:
            click.echo(f"需要的字段: type, host, user, password, database", err=True)
            sys.exit(1)

    elif db_host or db_user or db_password or db_database:
        required_params = {
            'db_host': db_host,
            'db_user': db_user,
            'db_password': db_password,
            'db_database': db_database,
            'db_type': db_type,
        }

        missing_params = [k for k, v in required_params.items() if v is None]
        if missing_params:
            click.echo(f"错误: 缺少必要的数据库参数: {', '.join(missing_params)}", err=True)
            click.echo("请提供: --db-host, --db-user, --db-password, --db-database, --db-type", err=True)
            sys.exit(1)

        if db_port is not None and (db_port < 1 or db_port > 65535):
            click.echo(f"错误: 端口 {db_port} 无效，必须是1-65535之间的整数", err=True)
            sys.exit(1)

        databases_config = [build_single_db_config(
            db_host=db_host, db_user=db_user, db_password=db_password,
            db_database=db_database, db_type=db_type, db_name=db_name,
            db_port=db_port, db_schema=db_schema
        )]
    
    # 检查配置文件是否存在（支持普通文件和 zipapp 内部文件）
    config_exists = os.path.exists(config)
    if not config_exists:
        # 尝试通过 load_config_file 检查（支持 zipapp）
        try:
            from db_inspector.core import load_config_file
            load_config_file(config)
            config_exists = True
        except FileNotFoundError:
            pass
    
    if not config_exists:
        click.echo(f"错误: 配置文件不存在: {config}", err=True)
        click.echo("请创建配置文件或指定正确的路径")
        sys.exit(1)
    
    # 创建巡检器
    inspector = DBInspector(config, databases_config=databases_config)
    
    # 校验是否有可巡检的数据库
    databases = inspector.config.get('databases', [])
    if not databases or len(databases) == 0:
        click.echo("错误: 未找到任何可巡检的数据库配置", err=True)
        click.echo("请通过以下方式之一提供数据库配置:", err=True)
        click.echo("1. 使用 --db-json 参数传递JSON格式的数据库配置", err=True)
        click.echo("2. 使用 --db-host/--db-user 等单独参数传递数据库配置", err=True)
        click.echo("3. 在配置文件中配置 databases 字段", err=True)
        sys.exit(1)
    
    # 如果指定了格式，覆盖配置
    if format:
        inspector.config['report']['format'] = format
    
    # 传递快速模式选项到巡检配置
    if 'inspection' not in inspector.config:
        inspector.config['inspection'] = {}
    
    # 如果只巡检指定数据库
    if database:
        databases = inspector.config.get('databases', [])
        db_config = next((db for db in databases if db.get('name') == database), None)
        if not db_config:
            click.echo(f"错误: 未找到数据库配置: {database}", err=True)
            sys.exit(1)
        
        inspection_config = inspector.config.get('inspection', {})
        result = inspector.inspect_database(db_config, inspection_config)
        inspector._generate_report(db_config, result)
    else:
        # 巡检所有数据库
        inspector.inspect_all()
        inspector.print_summary()


if __name__ == '__main__':
    main()
