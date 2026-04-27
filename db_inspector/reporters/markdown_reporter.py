import os
from datetime import datetime
from typing import Dict, Any, List

from ..utils import format_size
from .base import BaseReporter
from .scoring import calculate_health_score, generate_key_findings


class MarkdownReporter(BaseReporter):

    def generate(self, db_config: Dict[str, Any], results: Dict[str, Any]) -> str:
        """生成 Markdown 报告"""
        lines = []
        
        # 获取数据
        basic_info = results.get('basic_info', {})
        performance = results.get('performance', {})
        instance_info = basic_info.get('instance_info', {})
        databases = basic_info.get('databases', {})
        tables = basic_info.get('tables', {})
        
        # 计算健康评分
        health_score = calculate_health_score(basic_info, performance, databases, tables)
        
        key_findings = generate_key_findings(basic_info, performance, databases, tables)
        
        # 标题
        lines.append(f"# 🔍 数据库实例巡检报告\n")
        lines.append(f"**实例名称**: {db_config.get('name', 'Unknown')}\n")
        lines.append(f"**数据库类型**: {db_config.get('type', 'Unknown')}\n")
        lines.append(f"**连接地址**: {db_config.get('host', 'Unknown')}:{db_config.get('port', 'Unknown')}\n")
        lines.append(f"**生成时间**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
        lines.append("---\n")
        
        # 健康评分
        lines.append(f"## 🏥 健康评分\n")
        score_emoji = {'excellent': '✅', 'good': '🔵', 'average': '🟠', 'poor': '🔴'}
        emoji = score_emoji.get(health_score['level'], '⚪')
        lines.append(f"### {emoji} 评分: {health_score['score']} 分 - {health_score['label']}\n")
        lines.append(f"{health_score['summary']}\n")
        
        # 评分明细
        if health_score.get('details'):
            lines.append("#### 评分明细\n")
            lines.append("| 检查项 | 得分 | 状态 | 详情 |\n")
            lines.append("|--------|------|------|------|\n")
            status_map = {
                'excellent': '✅ 优秀',
                'good': '🔵 良好',
                'warning': '🟠 警告',
                'critical': '🔴 严重'
            }
            for detail in health_score['details']:
                status_text = status_map.get(detail['status'], detail['status'])
                lines.append(f"| {detail['name']} | {detail['score']}/{detail['max_score']} | {status_text} | {detail['detail']} |\n")
            lines.append("\n")
        
        # 关键发现与建议
        if key_findings:
            lines.append(f"## ⚠️ 关键发现与建议\n")
            for finding in key_findings:
                lines.append(f"### {finding['icon']} {finding['title']}\n")
                lines.append(f"{finding['description']}\n")
            lines.append("\n")
        
        # 概览统计
        lines.append("## 📊 概览统计\n")
        connection_status = basic_info.get('connection_status', {})
        lines.append(f"- **连接状态**: {connection_status.get('status', '未知')}\n")
        lines.append(f"- **数据库数量**: {databases.get('total', 0)}\n")
        lines.append(f"- **表总数**: {tables.get('total_count', 0)}\n")
        lines.append(f"- **实例总大小**: {instance_info.get('total_size', 'N/A')}\n")
        lines.append("\n")
        
        # 实例基本信息
        if instance_info:
            lines.append("## 📋 实例基本信息\n")
            version = basic_info.get('version', '未知')
            lines.append(f"- **数据库版本**: {version}\n")
            
            # 尝试从version中提取产品信息
            product_name = instance_info.get('product_name', '')
            product_version = instance_info.get('product_version', '')
            if product_name:
                lines.append(f"- **产品名称**: {product_name}\n")
            if product_version:
                lines.append(f"- **产品版本**: {product_version}\n")
            
            lines.append(f"- **启动时间**: {basic_info.get('uptime', '未知')}\n")
            lines.append(f"- **数据目录**: {instance_info.get('data_directory', 'N/A')}\n")
            lines.append(f"- **监听地址**: {instance_info.get('listen_addresses', 'N/A')}\n")
            lines.append(f"- **端口**: {instance_info.get('port', 'N/A')}\n")
            lines.append(f"- **最大连接数**: {instance_info.get('max_connections', 'N/A')}\n")
            lines.append(f"- **当前连接数**: {instance_info.get('current_connections', 'N/A')}\n")
            lines.append(f"- **共享缓冲区**: {instance_info.get('shared_buffers', 'N/A')}\n")
            lines.append(f"- **数据库当前时间**: {instance_info.get('db_time', 'N/A')}\n")
            lines.append(f"- **时区**: {instance_info.get('timezone', 'N/A')}\n")
            lines.append(f"- **表名大小写**: {instance_info.get('case_sensitive', 'N/A')}\n")
            lines.append("\n")
        
        # 实例配置
        settings = basic_info.get('settings', {})
        if settings:
            lines.append("## ⚙️ 实例配置\n")
            
            # 按类别分组显示
            config_categories = {
                '连接与内存': ['max_connections', 'shared_buffers', 'work_mem', 'maintenance_work_mem', 'effective_cache_size'],
                'WAL配置': ['wal_level', 'max_wal_size', 'min_wal_size', 'wal_buffers'],
                '检查点': ['checkpoint_completion_target', 'checkpoint_timeout'],
                '自动清理': ['autovacuum', 'autovacuum_max_workers', 'autovacuum_naptime'],
                '查询规划': ['random_page_cost', 'default_statistics_target', 'effective_io_concurrency'],
                '并行查询': ['max_parallel_workers_per_gather', 'max_parallel_workers'],
                '日志配置': ['logging_collector', 'log_statement', 'log_min_duration_statement'],
                '其他': ['timezone', 'max_locks_per_transaction']
            }
            
            for category, keys in config_categories.items():
                found_keys = [k for k in keys if k in settings]
                if found_keys:
                    lines.append(f"### {category}\n")
                    for key in found_keys:
                        lines.append(f"- **{key}**: {settings[key]}\n")
                    lines.append("\n")
        
        # 数据库列表
        if databases and databases.get('normal'):
            lines.append(f"## 🗄️ 数据库列表 (共 {databases.get('total', 0)} 个)\n")
            lines.append("| 序号 | 数据库名称 | 大小 | Schema数 | 表数量 | 视图数量 | 触发器数量 | 字符集 |\n")
            lines.append("|-----|-----------|------|----------|--------|----------|------------|--------|\n")
            total_schema_count = 0
            total_table_count = 0
            total_view_count = 0
            total_trigger_count = 0
            total_size_bytes = 0
            for idx, db in enumerate(databases['normal'], 1):
                schema_count = db.get('schema_count', 0) or 0
                table_count = db.get('table_count', 0) or 0
                view_count = db.get('view_count', 0) or 0
                trigger_count = db.get('trigger_count', 0) or 0
                size_bytes = db.get('size_bytes', 0) or 0
                total_schema_count += schema_count
                total_table_count += table_count
                total_view_count += view_count
                total_trigger_count += trigger_count
                total_size_bytes += size_bytes
                lines.append(f"| {idx} | {db.get('name', 'N/A')} | "
                           f"{db.get('size', 'N/A')} | "
                           f"{schema_count} | "
                           f"{table_count} | "
                           f"{view_count} | "
                           f"{trigger_count} | "
                           f"{db.get('encoding', 'N/A')} |\n")
            total_size_formatted = format_size(total_size_bytes)
            lines.append(f"| | **汇总** | **{total_size_formatted}** | **{total_schema_count}** | **{total_table_count}** | **{total_view_count}** | **{total_trigger_count}** | - |\n")
            lines.append("\n")
        
        # 疑似备份库
        if databases and databases.get('backup'):
            lines.append(f"## ⚠️ 疑似备份库 (共 {databases.get('backup_count', 0)} 个)\n")
            lines.append("以下数据库名符合备份库命名规则，建议定期清理以释放空间\n\n")
            lines.append("| 序号 | 数据库名称 | 大小 | Schema数 | 表数量 | 视图数量 | 触发器数量 | 字符集 |\n")
            lines.append("|-----|-----------|------|----------|--------|----------|------------|--------|\n")
            for idx, db in enumerate(databases['backup'], 1):
                lines.append(f"| {idx} | {db.get('name', 'N/A')} | "
                           f"{db.get('size', 'N/A')} | "
                           f"{db.get('schema_count', 0)} | "
                           f"{db.get('table_count', 0)} | "
                           f"{db.get('view_count', 0)} | "
                           f"{db.get('trigger_count', 0)} | "
                           f"{db.get('encoding', 'N/A')} |\n")
            lines.append("\n")
        
        # 数据表清单（按数据库分组，基于1GB阈值动态展示）
        if tables and tables.get('normal'):
            lines.append("## 📊 数据表清单\n")
            lines.append("> **说明**: 行数为估算值（来自 pg_class.reltuples），由 PostgreSQL 统计收集器维护\n\n")
            lines.append("> **展示规则**: 默认展示超过1GB的大表，如果大表少于10个则展示前10个，最多展示50个\n\n")
            
            # 按数据库分组
            tables_by_db = {}
            for table in tables['normal']:
                db_name = table.get('database', 'Unknown')
                if db_name not in tables_by_db:
                    tables_by_db[db_name] = []
                tables_by_db[db_name].append(table)
            
            SIZE_THRESHOLD = 1 * 1024 * 1024 * 1024  # 1GB
            MIN_DISPLAY_COUNT = 10
            MAX_DISPLAY_COUNT = 50
            
            for db_name, db_tables in tables_by_db.items():
                sorted_tables = sorted(db_tables, key=lambda x: x.get('size_bytes', 0), reverse=True)
                large_tables = [t for t in sorted_tables if t.get('size_bytes', 0) >= SIZE_THRESHOLD]
                
                if len(large_tables) < MIN_DISPLAY_COUNT:
                    selected_tables = sorted_tables[:MIN_DISPLAY_COUNT]
                else:
                    selected_tables = large_tables
                
                if len(selected_tables) > MAX_DISPLAY_COUNT:
                    selected_tables = selected_tables[:MAX_DISPLAY_COUNT]
                
                lines.append(f"### 数据库: {db_name} (共 {len(sorted_tables)} 个表, 展示 {len(selected_tables)} 个)\n")
                lines.append("| 排名 | 模式 | 表名 | 大小 | 字段数 | 行数(估算) |\n")
                lines.append("|------|------|------|------|--------|------------|\n")
                for idx, table in enumerate(selected_tables, 1):
                    row_count = table.get('row_count', 0)
                    row_count_str = f"{row_count:,}" if isinstance(row_count, (int, float)) else str(row_count)
                    column_count = table.get('column_count', 'N/A')
                    lines.append(f"| {idx} | "
                               f"{table.get('schema', 'N/A')} | "
                               f"{table.get('table_name', 'N/A')} | "
                               f"{table.get('size', 'N/A')} | "
                               f"{column_count} | "
                               f"{row_count_str} |\n")
                lines.append("\n")
        
        # 疑似备份表
        if tables and tables.get('backup'):
            lines.append(f"## ⚠️ 疑似备份表 (共 {tables.get('backup_count', 0)} 个)\n")
            lines.append("以下表名符合备份表命名规则，建议定期清理以释放空间\n\n")
            lines.append("| 序号 | 所属数据库 | 模式 | 表名 | 大小 |\n")
            lines.append("|-----|-----------|------|------|------|\n")
            for idx, table in enumerate(tables['backup'], 1):
                lines.append(f"| {idx} | {table.get('database', 'N/A')} | "
                           f"{table.get('schema', 'N/A')} | "
                           f"{table.get('table_name', 'N/A')} | "
                           f"{table.get('size', 'N/A')} |\n")
            lines.append("\n")
        
        # 缺少主键或唯一索引的表
        tables_without_pk = basic_info.get('tables_without_pk', {})
        if tables_without_pk:
            total_count = sum(len(tables) for tables in tables_without_pk.values())
            lines.append(f"## ⚠️ 缺少主键或唯一索引的表 (共 {total_count} 个)\n")
            lines.append("以下表缺少主键或唯一索引，建议添加以保证数据完整性和查询性能\n\n")
            lines.append("| 数据库 | 模式 | 数量 | 表名 |\n")
            lines.append("|--------|------|------|------|\n")
            for db_name, db_tables in tables_without_pk.items():
                schema = db_tables[0].get('schema', 'N/A') if db_tables else 'N/A'
                table_names = '、'.join([t.get('table_name', 'N/A') for t in db_tables])
                lines.append(f"| {db_name} | {schema} | {len(db_tables)} | {table_names} |\n")
            lines.append("\n")
        
        # 性能指标
        if performance:
            lines.append("## ⚡ 性能指标\n")
            
            # 连接统计
            connections = performance.get('connections', {})
            if connections:
                lines.append("### 连接统计\n")
                lines.append(f"- **当前连接数**: {connections.get('current', 'N/A')}\n")
                lines.append(f"- **最大连接数**: {connections.get('max', 'N/A')}\n")
                lines.append(f"- **活跃连接**: {connections.get('active', 'N/A')}\n")
                lines.append(f"- **空闲连接**: {connections.get('idle', 'N/A')}\n")
                lines.append(f"- **使用率**: {connections.get('usage_percent', 'N/A')}%\n")
                lines.append(f"- **状态**: {connections.get('status', 'N/A')}\n")
                lines.append("\n")
            
            # 客户端连接详情
            client_connections = performance.get('client_connections', [])
            if client_connections and 'error' not in client_connections[0]:
                lines.append("### 客户端连接详情 (按IP分组)\n")
                lines.append("| 客户端IP | 总连接数 | 活跃 | 空闲 | 访问数据库数 | 访问数据库 | 用户数 | 用户 | 应用数 | 应用 |\n")
                lines.append("|----------|----------|------|------|--------------|------------|--------|------|--------|------|\n")
                for client in client_connections:
                    lines.append(f"| {client.get('client_ip', 'N/A')} | "
                               f"{client.get('total_connections', 0)} | "
                               f"{client.get('active', 0)} | "
                               f"{client.get('idle', 0)} | "
                               f"{client.get('database_count', 0)} | "
                               f"{client.get('databases', 'N/A')} | "
                               f"{client.get('user_count', 0)} | "
                               f"{client.get('users', 'N/A')} | "
                               f"{client.get('application_count', 0)} | "
                               f"{client.get('applications', 'N/A')} |\n")
                lines.append("\n")
            
            # 缓存命中率
            cache_hit = performance.get('cache_hit_ratio', {})
            if cache_hit:
                lines.append("### 缓存命中率\n")
                lines.append(f"- **命中率**: {cache_hit.get('ratio', 'N/A')}%\n")
                lines.append(f"- **缓存命中**: {cache_hit.get('hits', 'N/A')}\n")
                lines.append(f"- **缓存未命中**: {cache_hit.get('misses', 'N/A')}\n")
                lines.append("\n")
            
            # 索引命中率
            index_hit = performance.get('index_hit_ratio', {})
            if index_hit:
                lines.append("### 索引命中率\n")
                lines.append(f"- **命中率**: {index_hit.get('ratio', 'N/A')}%\n")
                lines.append(f"- **索引扫描**: {index_hit.get('idx_scan', 'N/A')}\n")
                lines.append(f"- **顺序扫描**: {index_hit.get('seq_scan', 'N/A')}\n")
                lines.append("\n")
            
            # 长事务检测
            long_transactions = performance.get('long_transactions', [])
            if long_transactions and len(long_transactions) > 0:
                lines.append(f"### ⏱️ 长事务检测 (共 {len(long_transactions)} 个)\n")
                lines.append("| 持续时间 | 状态 | 用户 | 数据库 | 等待事件 |\n")
                lines.append("|----------|------|------|--------|----------|\n")
                for txn in long_transactions:
                    duration = txn.get('duration', 'N/A')
                    state = txn.get('state', 'N/A')
                    user = txn.get('usename', 'N/A')
                    db = txn.get('datname', 'N/A')
                    wait_event = txn.get('wait_event', '无')
                    lines.append(f"| {duration} | {state} | {user} | {db} | {wait_event} |\n")
                lines.append("\n")
            
            # 锁等待分析
            locks = performance.get('locks', [])
            if locks and len(locks) > 0:
                lines.append(f"### 🔒 锁等待分析 (共 {len(locks)} 个)\n")
                lines.append("| 等待时间 | 锁类型 | 等待SQL | 阻塞PID | 阻塞用户 | 阻塞时长 |\n")
                lines.append("|----------|--------|---------|---------|----------|----------|\n")
                for lock in locks:
                    wait_time = lock.get('wait_duration', 'N/A')
                    lock_type = lock.get('locktype', 'N/A')
                    query = lock.get('query', 'N/A')
                    blocking_pid = lock.get('blocking_pid', 'N/A')
                    blocking_user = lock.get('blocking_user', 'N/A')
                    blocking_wait = lock.get('blocking_wait_duration', 'N/A')
                    lines.append(f"| {wait_time} | {lock_type} | {query} | {blocking_pid} | {blocking_user} | {blocking_wait} |\n")
                lines.append("\n")
            
            # 死元组分析
            dead_tuples = performance.get('dead_tuples', {})
            if dead_tuples:
                total_tables = 0
                for db_tables in dead_tuples.values():
                    if isinstance(db_tables, list):
                        total_tables += len(db_tables)
                
                if total_tables > 0:
                    lines.append(f"### 🗑️ 死元组分析 (共 {total_tables} 个表需要关注)\n")
                    for db_name, db_tables in dead_tuples.items():
                        if isinstance(db_tables, list) and len(db_tables) > 0:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 模式 | 表名 | 总行数 | 死元组数 | 死元组比例 | 状态 |\n")
                            lines.append("|------|------|--------|----------|------------|------|\n")
                            for table in db_tables[:20]:
                                severity = table.get('severity', '')
                                severity_icon = "🔴" if severity == "critical" else ("🟡" if severity == "warning" else "🟢")
                                lines.append(f"| {table.get('schemaname', 'N/A')} | "
                                           f"{table.get('relname', 'N/A')} | "
                                           f"{table.get('n_live_tup', 'N/A')} | "
                                           f"{table.get('n_dead_tup', 'N/A')} | "
                                           f"{table.get('dead_ratio', 'N/A')}% | "
                                           f"{severity_icon} {severity} |\n")
                            lines.append("\n")
            
            # VACUUM状态
            vacuum_status = performance.get('vacuum_status', {})
            if vacuum_status:
                total_issues = 0
                for db_tables in vacuum_status.values():
                    if isinstance(db_tables, list):
                        total_issues += len(db_tables)
                
                if total_issues > 0:
                    lines.append(f"### 🧹 VACUUM状态 (共 {total_issues} 个表需要关注)\n")
                    for db_name, db_tables in vacuum_status.items():
                        if isinstance(db_tables, list) and len(db_tables) > 0:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 模式 | 表名 | VACUUM状态 | 最后VACUUM | 最后ANALYZE |\n")
                            lines.append("|------|------|-----------|-----------|-------------|\n")
                            for table in db_tables[:20]:
                                lines.append(f"| {table.get('schemaname', 'N/A')} | "
                                           f"{table.get('relname', 'N/A')} | "
                                           f"{table.get('vacuum_status', 'N/A')} | "
                                           f"{table.get('last_vacuum', 'N/A')} | "
                                           f"{table.get('last_analyze', 'N/A')} |\n")
                            lines.append("\n")
            
            # IO密集表
            io_stats = performance.get('io_stats', {})
            if io_stats:
                total_tables = 0
                for db_tables in io_stats.values():
                    if isinstance(db_tables, list):
                        total_tables += len(db_tables)
                
                if total_tables > 0:
                    lines.append(f"### 💾 Top IO密集表 (共 {total_tables} 个)\n")
                    for db_name, db_tables in io_stats.items():
                        if isinstance(db_tables, list) and len(db_tables) > 0:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 排名 | 模式 | 表名 | 堆读取次数 | 索引读取次数 | 缓存命中次数 | 缓存命中率 | IO等级 |\n")
                            lines.append("|------|------|------|-----------|-------------|-------------|-----------|--------|\n")
                            for idx, table in enumerate(db_tables, 1):
                                lines.append(f"| {idx} | "
                                           f"{table.get('schemaname', 'N/A')} | "
                                           f"{table.get('relname', 'N/A')} | "
                                           f"{table.get('heap_blks_read', 'N/A')} | "
                                           f"{table.get('idx_blks_read', 'N/A')} | "
                                           f"{table.get('heap_blks_hit', 'N/A')} | "
                                           f"{table.get('cache_hit_ratio', 'N/A')}% | "
                                           f"{table.get('io_level', 'N/A')} |\n")
                            lines.append("\n")
            
            # 索引大小分析（按数据库分组）
            index_analysis = performance.get('index_size_analysis', {})
            if index_analysis and isinstance(index_analysis, dict) and 'error' not in index_analysis:
                lines.append("### ⚠️ 索引大小占比分析\n")
                lines.append("**注意**: 索引大小超过数据大小 30% 的表需要关注（只显示数据量>=10000行的表）\n\n")
                
                for db_name, db_analysis in index_analysis.items():
                    if db_analysis and isinstance(db_analysis, list):
                        attention_tables = [t for t in db_analysis if t.get('attention') in ['严重', '关注']]
                        if attention_tables:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 模式 | 表名 | 行数 | 总大小 | 数据大小 | 索引数量 | 索引大小 | 索引占比 | 状态 | 原因 |\n")
                            lines.append("|------|------|------|--------|----------|----------|----------|----------|------|------|\n")
                            for table in attention_tables[:15]:
                                attention = table.get('attention', '')
                                attention_icon = "🔴" if attention == "严重" else "🟡"
                                lines.append(f"| {table.get('schemaname', 'N/A')} | "
                                           f"{table.get('table_name', 'N/A')} | "
                                           f"{table.get('row_count', 'N/A')} | "
                                           f"{table.get('total_size', 'N/A')} | "
                                           f"{table.get('table_size', 'N/A')} | "
                                           f"{table.get('index_count', 'N/A')} | "
                                           f"{table.get('index_size', 'N/A')} | "
                                           f"{table.get('index_ratio', 0)}% | "
                                           f"{attention_icon} {attention} | "
                                           f"{table.get('reason', '')} |\n")
                            lines.append("\n")
            
            # 无效索引
            invalid_indexes = performance.get('invalid_indexes', {})
            if invalid_indexes:
                total_invalid = 0
                for indexes in invalid_indexes.values():
                    if isinstance(indexes, list):
                        total_invalid += len(indexes)
                
                if total_invalid > 0:
                    lines.append(f"### ❌ 无效索引 (共 {total_invalid} 个)\n")
                    lines.append("以下索引处于无效状态(INVALID),不会用于查询但会占用存储空间\n\n")
                    for db_name, indexes in invalid_indexes.items():
                        if isinstance(indexes, list) and len(indexes) > 0:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 模式 | 表名 | 索引名 | 索引大小 |\n")
                            lines.append("|------|------|--------|----------|\n")
                            for idx in indexes[:20]:
                                lines.append(f"| {idx.get('schemaname', 'N/A')} | "
                                           f"{idx.get('tablename', 'N/A')} | "
                                           f"{idx.get('indexname', 'N/A')} | "
                                           f"{idx.get('size', 'N/A')} |\n")
                            lines.append("\n")
            
            # 重复索引
            duplicate_indexes = performance.get('duplicate_indexes', {})
            if duplicate_indexes:
                total_duplicate = 0
                for indexes in duplicate_indexes.values():
                    if isinstance(indexes, list):
                        total_duplicate += len(indexes)
                
                if total_duplicate > 0:
                    lines.append(f"### 🔁 重复索引 (共 {total_duplicate} 个)\n")
                    lines.append("以下表在相同列上存在多个索引,建议保留使用频率高的索引,删除冗余索引\n\n")
                    for db_name, indexes in duplicate_indexes.items():
                        if isinstance(indexes, list) and len(indexes) > 0:
                            lines.append(f"#### 数据库: {db_name}\n")
                            lines.append("| 模式 | 表名 | 列 | 索引名 | 索引大小 | 扫描次数 |\n")
                            lines.append("|------|------|----|--------|----------|----------|\n")
                            for idx in indexes[:20]:
                                lines.append(f"| {idx.get('schemaname', 'N/A')} | "
                                           f"{idx.get('tablename', 'N/A')} | "
                                           f"{idx.get('columns', 'N/A')} | "
                                           f"{idx.get('indexname', 'N/A')} | "
                                           f"{idx.get('size', 'N/A')} | "
                                           f"{idx.get('idx_scan', 'N/A')} |\n")
                            lines.append("\n")
        
        # 页脚
        lines.append("---\n")
        lines.append(f"*由 DB Inspector 自动生成*\n")
        
        # 保存文件
        content = ''.join(lines)
        filename = f"db_inspection_{db_config.get('name', 'report').replace(' ', '_')}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.md"
        filepath = os.path.join(self.output_dir, filename)
        
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)
        
        return filepath
