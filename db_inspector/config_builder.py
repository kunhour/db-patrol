import json
import os
from typing import Dict, Any, List, Optional


REQUIRED_DB_FIELDS = ['type', 'host', 'user', 'password', 'database']
VALID_DB_TYPES = ['vastbase_pg', 'mysql', 'postgresql']


def parse_db_json(db_json: str) -> List[Dict[str, Any]]:
    databases_config = json.loads(db_json)
    if not isinstance(databases_config, list):
        databases_config = [databases_config]
    return databases_config


def resolve_env_passwords(databases_config: List[Dict[str, Any]]) -> List[str]:
    warnings = []
    for i, db_conf in enumerate(databases_config):
        if 'password' in db_conf and db_conf['password'] and db_conf['password'].startswith('$'):
            env_var_name = db_conf['password'][1:]
            env_value = os.environ.get(env_var_name)
            if env_value:
                db_conf['password'] = env_value
            else:
                warnings.append(f"第{i+1}个数据库的密码引用环境变量 ${env_var_name} 未设置")
    return warnings


def validate_db_configs(databases_config: List[Dict[str, Any]]) -> List[str]:
    errors = []
    for i, db_conf in enumerate(databases_config):
        missing_fields = [f for f in REQUIRED_DB_FIELDS if f not in db_conf or db_conf[f] is None]
        if missing_fields:
            errors.append(f"第{i+1}个数据库配置缺少必要字段: {', '.join(missing_fields)}")

        if db_conf.get('type') not in VALID_DB_TYPES:
            errors.append(f"第{i+1}个数据库的类型 {db_conf.get('type')} 不支持，支持: {', '.join(VALID_DB_TYPES)}")

        if 'port' in db_conf and db_conf['port'] is not None:
            if not isinstance(db_conf['port'], int) or db_conf['port'] < 1 or db_conf['port'] > 65535:
                errors.append(f"第{i+1}个数据库的端口 {db_conf['port']} 无效，必须是1-65535之间的整数")

    return errors


def build_single_db_config(
    db_host: str, db_user: str, db_password: str, db_database: str,
    db_type: str, db_name: Optional[str] = None, db_port: Optional[int] = None,
    db_schema: str = 'public'
) -> Dict[str, Any]:
    return {
        'name': db_name or db_database,
        'type': db_type,
        'host': db_host,
        'port': db_port or (3306 if db_type == 'mysql' else 5432),
        'user': db_user,
        'password': db_password,
        'database': db_database,
        'schema': db_schema,
    }
