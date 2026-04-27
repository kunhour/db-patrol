import os
import json
from datetime import datetime
from typing import Dict, Any, List
from jinja2 import Environment, FileSystemLoader

from ..utils import format_size
from .base import BaseReporter
from .scoring import calculate_health_score, generate_key_findings

def format_datetime(value):
    if value is None:
        return 'N/A'
    if isinstance(value, datetime):
        return value.strftime('%Y-%m-%d %H:%M:%S%z')
    return str(value)

_TEMPLATE_DIR = os.path.join(os.path.dirname(__file__), 'templates')
jinja_env = Environment(loader=FileSystemLoader(_TEMPLATE_DIR))
jinja_env.filters['format_datetime'] = format_datetime



class HTMLReporter(BaseReporter):

    def generate(self, db_config: Dict[str, Any], results: Dict[str, Any]) -> str:
        """生成 HTML 报告"""
        template = jinja_env.get_template('report.html.j2')
        
        # 获取数据
        basic_info = results.get('basic_info', {})
        performance = results.get('performance', {})
        instance_info = basic_info.get('instance_info', {})
        databases = basic_info.get('databases', {})
        tables = basic_info.get('tables', {})
        
        # 计算健康评分
        health_score = calculate_health_score(basic_info, performance, databases, tables)
        
        # 生成关键发现与建议
        key_findings = generate_key_findings(basic_info, performance, databases, tables)
        
        # 按数据库分组表,基于1GB阈值动态展示
        tables_by_database = {}
        tables_stats_by_database = {}  # 存储每个库的表统计信息
        
        # 配置参数
        SIZE_THRESHOLD = 1 * 1024 * 1024 * 1024  # 1GB阈值
        MIN_DISPLAY_COUNT = 10  # 最少展示10个表
        MAX_DISPLAY_COUNT = 50  # 最多展示50个表
        
        if tables and 'normal' in tables:
            # 先按数据库分组
            tables_grouped = {}
            for table in tables['normal']:
                db_name = table.get('database', 'Unknown')
                if db_name not in tables_grouped:
                    tables_grouped[db_name] = []
                tables_grouped[db_name].append(table)
            
            # 对每个数据库的表进行动态筛选
            for db_name, db_tables in tables_grouped.items():
                # 按大小排序
                sorted_tables = sorted(
                    db_tables, 
                    key=lambda x: x.get('size_bytes', 0), 
                    reverse=True
                )
                
                # 统计超过1GB的表
                large_tables = [t for t in sorted_tables if t.get('size_bytes', 0) >= SIZE_THRESHOLD]
                
                # 如果大表太少,补充到最小数量
                if len(large_tables) < MIN_DISPLAY_COUNT:
                    selected_tables = sorted_tables[:MIN_DISPLAY_COUNT]
                else:
                    selected_tables = large_tables
                
                # 如果太多,限制最大数量
                if len(selected_tables) > MAX_DISPLAY_COUNT:
                    selected_tables = selected_tables[:MAX_DISPLAY_COUNT]
                
                tables_by_database[db_name] = selected_tables
                
                # 记录统计信息
                tables_stats_by_database[db_name] = {
                    'total_count': len(sorted_tables),
                    'display_count': len(selected_tables),
                    'large_count': len(large_tables),
                    'threshold_mb': SIZE_THRESHOLD / (1024 * 1024)
                }
        
        # 计算数据库列表的汇总数据
        databases_summary = {}
        if databases and 'normal' in databases:
            normal_dbs = databases['normal']
            total_schema_count = sum(db.get('schema_count', 0) or 0 for db in normal_dbs)
            total_table_count = sum(db.get('table_count', 0) or 0 for db in normal_dbs)
            total_view_count = sum(db.get('view_count', 0) or 0 for db in normal_dbs)
            total_trigger_count = sum(db.get('trigger_count', 0) or 0 for db in normal_dbs)
            
            # 计算总大小（转换为字节后汇总）
            total_size_bytes = 0
            for db in normal_dbs:
                size_bytes = db.get('size_bytes', 0) or 0
                total_size_bytes += size_bytes
            
            # 格式化总大小
            total_size_formatted = format_size(total_size_bytes)
            
            databases_summary = {
                'total_schema_count': total_schema_count,
                'total_table_count': total_table_count,
                'total_view_count': total_view_count,
                'total_trigger_count': total_trigger_count,
                'total_size': total_size_formatted,
                'total_size_bytes': total_size_bytes
            }
        
        context = {
            'report_title': db_config.get('name', 'Unknown'),
            'db_name': db_config.get('name', 'Unknown'),
            'db_type': db_config.get('type', 'Unknown'),
            'db_host': db_config.get('host', 'Unknown'),
            'db_port': db_config.get('port', 'Unknown'),
            'generated_at': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
            'connection_status': basic_info.get('connection_status', {}).get('status', '未知'),
            'database_count': databases.get('total', 0),
            'table_count': tables.get('total_count', 0),
            'total_size': instance_info.get('total_size', 'N/A'),
            'instance_info': instance_info,
            'version': basic_info.get('version', ''),
            'uptime': basic_info.get('uptime', ''),
            'settings': basic_info.get('settings', {}),
            'databases': databases,
            'databases_summary': databases_summary,
            'tables': tables,
            'tables_by_database': tables_by_database,
            'tables_stats_by_database': tables_stats_by_database,
            'backup_tables': tables.get('backup', []),
            'backup_tables_count': tables.get('backup_count', 0),
            'tables_without_pk': basic_info.get('tables_without_pk', {}),
            'performance': performance,
            'health_score': health_score,
            'key_findings': key_findings
        }
        
        html_content = template.render(**context)
        
        # 保存文件
        filename = f"db_inspection_{db_config.get('name', 'report').replace(' ', '_')}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.html"
        filepath = os.path.join(self.output_dir, filename)
        
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(html_content)
        
        return filepath
