import concurrent.futures
import logging
from datetime import datetime, timedelta
from typing import Dict, Any, List
from .base import BaseInspector
from ..utils import format_size

logger = logging.getLogger('db_patrol')


class PerformanceInspector(BaseInspector):

    name = 'performance'
    title = '检查性能指标'

    def inspect(self) -> Dict[str, Any]:
        db_type = self.connection.config['type']
        
        if 'pg' in db_type.lower() or 'postgres' in db_type.lower():
            return self._inspect_pg()
        else:
            return self._inspect_mysql()
    
    def _inspect_pg(self) -> Dict[str, Any]:
        result = {
            'connections': self._get_pg_connections(),
            'client_connections': self._get_pg_client_connections(),
            'cache_hit_ratio': self._get_pg_cache_hit_ratio(),
            'index_hit_ratio': self._get_pg_index_hit_ratio(),
            'activity': self._get_pg_activity(),
            'locks': self._get_pg_locks(),
            'long_transactions': self._get_pg_long_transactions(),
            'slow_queries': self._get_pg_slow_queries(),
            'table_stats': self._get_pg_table_stats(),
            'index_stats': self._get_pg_index_stats(),
        }

        logger.info("    → 分析死元组和VACUUM状态...")

        try:
            query = "SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true"
            db_rows = self.execute_query(query)
            db_names = [r['datname'] for r in db_rows]
        except Exception:
            db_names = []

        deep_results = self._inspect_pg_databases_parallel(db_names)

        result['dead_tuples'] = {}
        result['vacuum_status'] = {}
        result['io_stats'] = {}
        result['index_size_analysis'] = {}
        result['invalid_indexes'] = {}
        result['duplicate_indexes'] = {}

        for db_name, db_data in deep_results.items():
            if db_data.get('dead_tuples'):
                result['dead_tuples'][db_name] = db_data['dead_tuples']
            if db_data.get('vacuum_status'):
                result['vacuum_status'][db_name] = db_data['vacuum_status']
            if db_data.get('io_stats'):
                result['io_stats'][db_name] = db_data['io_stats']
            if db_data.get('index_size_analysis'):
                result['index_size_analysis'][db_name] = db_data['index_size_analysis']
            if db_data.get('invalid_indexes'):
                result['invalid_indexes'][db_name] = db_data['invalid_indexes']
            if db_data.get('duplicate_indexes'):
                result['duplicate_indexes'][db_name] = db_data['duplicate_indexes']

        return result

    def _inspect_pg_databases_parallel(self, db_names: List[str]) -> Dict[str, Dict]:
        results = {}
        max_workers = min(16, len(db_names))
        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_db = {
                executor.submit(self._inspect_pg_database_deep, db_name): db_name
                for db_name in db_names
            }
            for future in concurrent.futures.as_completed(future_to_db):
                db_name = future_to_db[future]
                try:
                    results[db_name] = future.result()
                except Exception:
                    results[db_name] = {}
        return results

    def _inspect_pg_database_deep(self, db_name: str) -> Dict[str, Any]:
        db_config = self.connection.config.copy()
        db_config['database'] = db_name

        data = {
            'dead_tuples': [],
            'vacuum_status': [],
            'io_stats': [],
            'index_size_analysis': [],
            'invalid_indexes': [],
            'duplicate_indexes': []
        }

        try:
            with self._create_connection(db_config) as conn:
                try:
                    query = """
                        SELECT
                            schemaname, relname as table_name,
                            n_live_tup as live_tuples, n_dead_tup as dead_tuples,
                            CASE WHEN n_live_tup + n_dead_tup > 0
                                THEN ROUND(n_dead_tup::numeric / (n_live_tup + n_dead_tup)::numeric * 100, 2)
                                ELSE 0 END as dead_tuple_ratio,
                            last_vacuum, last_autovacuum, last_analyze, last_autoanalyze,
                            pg_size_pretty(pg_total_relation_size(relid)) as table_size
                        FROM pg_stat_user_tables
                        WHERE n_dead_tup > 1000
                           OR (n_live_tup + n_dead_tup > 0 AND n_dead_tup::numeric / (n_live_tup + n_dead_tup)::numeric > 0.1)
                        ORDER BY n_dead_tup DESC LIMIT 30
                    """
                    result = conn.execute_query(query)
                    for row in result:
                        ratio = row.get('dead_tuple_ratio', 0)
                        if ratio > 50:
                            row['severity'] = 'critical'
                            row['severity_label'] = '严重(>50%)'
                            row['suggestion'] = '立即执行VACUUM,表膨胀严重'
                        elif ratio > 30:
                            row['severity'] = 'warning'
                            row['severity_label'] = '警告(>30%)'
                            row['suggestion'] = '建议执行VACUUM FULL或VACUUM'
                        elif ratio > 10:
                            row['severity'] = 'info'
                            row['severity_label'] = '关注(>10%)'
                            row['suggestion'] = '监控自动VACUUM是否正常工作'
                        else:
                            row['severity'] = 'normal'
                            row['severity_label'] = '正常'
                            row['suggestion'] = ''
                    data['dead_tuples'] = result
                except Exception:
                    pass

                try:
                    query = """
                        SELECT
                            schemaname, relname as table_name,
                            n_live_tup as live_tuples,
                            last_vacuum, last_autovacuum, last_analyze, last_autoanalyze,
                            CASE
                                WHEN last_autovacuum IS NULL AND last_vacuum IS NULL THEN '从未执行'
                                WHEN last_autovacuum < NOW() - INTERVAL '7 days' THEN '超过7天'
                                WHEN last_autoanalyze IS NULL AND last_analyze IS NULL THEN '从未执行'
                                WHEN last_autoanalyze < NOW() - INTERVAL '7 days' THEN '超过7天'
                                ELSE '正常'
                            END as vacuum_status
                        FROM pg_stat_user_tables
                        WHERE last_autovacuum IS NULL
                           OR last_vacuum IS NULL
                           OR last_autoanalyze IS NULL
                           OR last_analyze IS NULL
                           OR last_autovacuum < NOW() - INTERVAL '7 days'
                           OR last_autoanalyze < NOW() - INTERVAL '7 days'
                        ORDER BY n_live_tup DESC
                        LIMIT 20
                    """
                    result = conn.execute_query(query)
                    for row in result:
                        status = row.get('vacuum_status', '')
                        if '从未执行' in status:
                            row['severity'] = 'critical'
                            row['suggestion'] = '检查autovacuum是否启用,或手动执行VACUUM ANALYZE'
                        elif '超过7天' in status:
                            row['severity'] = 'warning'
                            row['suggestion'] = '检查autovacuum配置,可能需要调整阈值'
                        else:
                            row['severity'] = 'info'
                            row['suggestion'] = ''
                    data['vacuum_status'] = result
                except Exception:
                    pass

                try:
                    query = """
                        SELECT
                            schemaname, relname as table_name,
                            heap_blks_read as disk_reads, heap_blks_hit as buffer_hits,
                            idx_blks_read as index_disk_reads, idx_blks_hit as index_buffer_hits,
                            toast_blks_read as toast_disk_reads, toast_blks_hit as toast_buffer_hits,
                            CASE WHEN heap_blks_read + heap_blks_hit > 0
                                THEN ROUND(heap_blks_hit::numeric / (heap_blks_read + heap_blks_hit)::numeric * 100, 2)
                                ELSE 0 END as cache_hit_ratio,
                            pg_size_pretty(pg_total_relation_size(relid)) as table_size
                        FROM pg_statio_user_tables
                        WHERE heap_blks_read > 0 OR idx_blks_read > 0
                        ORDER BY (heap_blks_read + idx_blks_read) DESC LIMIT 20
                    """
                    result = conn.execute_query(query)
                    for row in result:
                        total_reads = row.get('disk_reads', 0) + row.get('index_disk_reads', 0)
                        cache_ratio = row.get('cache_hit_ratio', 0)
                        if total_reads > 100000 and cache_ratio < 90:
                            row['io_level'] = 'high'
                            row['io_label'] = '高IO'
                            row['suggestion'] = '该表IO密集且缓存命中率低,建议优化查询或增加shared_buffers'
                        elif total_reads > 50000:
                            row['io_level'] = 'medium'
                            row['io_label'] = '中IO'
                            row['suggestion'] = '该表IO较密集,建议检查是否有缺失索引'
                        else:
                            row['io_level'] = 'low'
                            row['io_label'] = '低IO'
                            row['suggestion'] = ''
                    data['io_stats'] = result
                except Exception:
                    pass

                try:
                    query = """
                        SELECT
                            t.schemaname, t.relname as table_name, t.n_live_tup as row_count,
                            pg_size_pretty(pg_relation_size(t.relid)) as table_size,
                            pg_size_pretty(pg_indexes_size(t.relid)) as index_size,
                            pg_size_pretty(pg_total_relation_size(t.relid)) as total_size,
                            pg_relation_size(t.relid) as table_size_bytes,
                            pg_indexes_size(t.relid) as index_size_bytes,
                            pg_total_relation_size(t.relid) as total_size_bytes,
                            CASE WHEN pg_relation_size(t.relid) > 0
                                THEN ROUND(pg_indexes_size(t.relid)::numeric / pg_relation_size(t.relid)::numeric * 100, 2)
                                ELSE 0 END as index_ratio,
                            (SELECT COUNT(*) FROM pg_indexes WHERE tablename = t.relname AND schemaname = t.schemaname) as index_count
                        FROM pg_stat_user_tables t
                        WHERE pg_relation_size(t.relid) > 0 AND t.n_live_tup >= 10000
                        ORDER BY CASE WHEN pg_relation_size(t.relid) > 0
                            THEN pg_indexes_size(t.relid)::numeric / pg_relation_size(t.relid)::numeric
                            ELSE 0 END DESC
                        LIMIT 30
                    """
                    result = conn.execute_query(query)
                    analysis = []
                    for row in result:
                        ratio = row.get('index_ratio', 0)
                        if ratio > 100:
                            row['attention'] = '严重'
                            row['suggestion'] = f"索引大小({row['index_size']})超过数据大小({row['table_size']}),请检查冗余索引"
                        elif ratio > 50:
                            row['attention'] = '关注'
                            row['suggestion'] = f"索引占比{ratio}%,建议检查索引是否合理"
                        else:
                            row['attention'] = '正常'
                            row['suggestion'] = ''
                        analysis.append(row)
                    data['index_size_analysis'] = analysis
                except Exception:
                    pass

                try:
                    query = """
                        SELECT
                            schemaname, tablename as table_name, indexname as index_name,
                            pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
                            idx_scan as index_scans,
                            CASE WHEN NOT indisvalid THEN '索引失效'
                                 WHEN idx_scan = 0 THEN '从未使用'
                                 ELSE '疑似无效' END as issue_type
                        FROM pg_stat_user_indexes sui
                        JOIN pg_index pi ON sui.indexrelid = pi.indexrelid
                        WHERE NOT indisvalid
                           OR (idx_scan = 0 AND pg_relation_size(sui.indexrelid) > 1048576)
                        ORDER BY pg_relation_size(sui.indexrelid) DESC
                    """
                    result = conn.execute_query(query)
                    for row in result:
                        row['database'] = db_name
                        row['suggestion'] = '建议删除无效索引以释放空间和提升写入性能' if row['issue_type'] == '索引失效' else '建议评估该索引是否仍然需要'
                    data['invalid_indexes'] = result
                except Exception:
                    pass

                try:
                    query = """
                        SELECT
                            a.schemaname, a.tablename as table_name, a.indexname as index_name,
                            a.indexdef as index_definition,
                            pg_size_pretty(pg_relation_size(b.indexrelid)) as index_size,
                            a.idx_scan as index_scans, b.indexrelid
                        FROM pg_stat_user_indexes a
                        JOIN pg_stat_user_indexes b
                            ON a.schemaname = b.schemaname AND a.tablename = b.tablename
                            AND a.indexrelid < b.indexrelid
                        WHERE EXISTS (
                            SELECT 1 FROM pg_index i1
                            JOIN pg_index i2 ON i1.indrelid = i2.indrelid
                            WHERE i1.indexrelid = a.indexrelid
                              AND i2.indexrelid = b.indexrelid
                              AND i1.indkey::text = i2.indkey::text
                              AND i1.indexrelid != i2.indexrelid
                        )
                        ORDER BY a.schemaname, a.tablename, pg_relation_size(b.indexrelid) DESC
                    """
                    result = conn.execute_query(query)
                    for row in result:
                        row['database'] = db_name
                        row['suggestion'] = f"表 {row['table_name']} 存在重复索引,建议保留使用频率较高的索引,删除其他重复索引"
                    data['duplicate_indexes'] = result
                except Exception:
                    pass

        except Exception:
            pass

        return data
    
    def _inspect_mysql(self) -> Dict[str, Any]:
        """MySQL 模式性能检查"""
        return {
            'connections': self._get_mysql_connections(),
            'client_connections': self._get_mysql_client_connections(),
            'cache_hit_ratio': self._get_mysql_cache_hit_ratio(),
            'index_hit_ratio': self._get_mysql_index_hit_ratio(),
            'status': self._get_mysql_status(),
            'slow_queries': self._get_mysql_slow_queries(),
            'table_stats': self._get_mysql_table_stats(),
            'index_stats': self._get_mysql_index_stats(),
            'processlist': self._get_mysql_processlist(),
            'long_transactions': self._get_mysql_long_transactions(),
            'locks': self._get_mysql_locks(),
            'index_size_analysis': self._get_mysql_index_size_analysis(),
            'invalid_indexes': self._get_mysql_invalid_indexes(),
            'duplicate_indexes': self._get_mysql_duplicate_indexes()
        }
    
    def _get_pg_connections(self) -> Dict[str, Any]:
        """获取 PG 连接信息"""
        try:
            # 当前连接数
            current_query = "SELECT count(*) as count FROM pg_stat_activity"
            current = self.execute_query(current_query)
            current_count = current[0]['count'] if current else 0
            
            # 最大连接数
            max_query = "SHOW max_connections"
            max_result = self.execute_query(max_query)
            max_count = int(max_result[0]['max_connections']) if max_result else 100
            
            # 活跃连接
            active_query = "SELECT count(*) as count FROM pg_stat_activity WHERE state = 'active'"
            active = self.execute_query(active_query)
            active_count = active[0]['count'] if active else 0
            
            # 空闲连接
            idle_query = "SELECT count(*) as count FROM pg_stat_activity WHERE state = 'idle'"
            idle = self.execute_query(idle_query)
            idle_count = idle[0]['count'] if idle else 0
            
            usage_percent = (current_count / max_count * 100) if max_count > 0 else 0
            
            return {
                'current': current_count,
                'max': max_count,
                'active': active_count,
                'idle': idle_count,
                'usage_percent': round(usage_percent, 2),
                'status': '警告' if usage_percent > self.config.get('max_connections_threshold', 80) else '正常'
            }
        except Exception as e:
            return {'error': str(e)}
    
    def _get_pg_cache_hit_ratio(self) -> Dict[str, Any]:
        """获取 PG 缓存命中率"""
        try:
            query = """
                SELECT 
                    SUM(heap_blks_read) as heap_read,
                    SUM(heap_blks_hit) as heap_hit,
                    CASE WHEN SUM(heap_blks_read) + SUM(heap_blks_hit) > 0 
                    THEN ROUND(SUM(heap_blks_hit)::numeric / (SUM(heap_blks_read) + SUM(heap_blks_hit))::numeric * 100, 2)
                    ELSE 0 
                    END as cache_hit_ratio
                FROM pg_statio_user_tables
            """
            result = self.execute_query(query)
            
            if result and result[0]:
                ratio = result[0].get('cache_hit_ratio', 0)
                status = '优秀' if ratio >= 99 else ('良好' if ratio >= 95 else ('一般' if ratio >= 90 else '较差'))
                
                return {
                    'ratio': ratio,
                    'heap_read': result[0].get('heap_read', 0),
                    'heap_hit': result[0].get('heap_hit', 0),
                    'status': status,
                    'suggestion': '缓存命中率低于95%,建议增加shared_buffers' if ratio < 95 else '缓存命中率正常'
                }
            return {'ratio': 0, 'status': '未知'}
        except Exception as e:
            return {'error': str(e)}
    
    def _get_pg_index_hit_ratio(self) -> Dict[str, Any]:
        """获取 PG 索引命中率(索引扫描vs顺序扫描)"""
        try:
            query = """
                SELECT 
                    SUM(idx_scan) as idx_scan,
                    SUM(seq_scan) as seq_scan,
                    CASE WHEN SUM(idx_scan) + SUM(seq_scan) > 0 
                    THEN ROUND(SUM(idx_scan)::numeric / (SUM(idx_scan) + SUM(seq_scan))::numeric * 100, 2)
                    ELSE 0 
                    END as index_hit_ratio
                FROM pg_stat_user_tables
            """
            result = self.execute_query(query)
            
            if result and result[0]:
                ratio = result[0].get('index_hit_ratio', 0)
                status = '优秀' if ratio >= 90 else ('良好' if ratio >= 70 else ('一般' if ratio >= 50 else '较差'))
                
                return {
                    'ratio': ratio,
                    'idx_scan': result[0].get('idx_scan', 0),
                    'seq_scan': result[0].get('seq_scan', 0),
                    'status': status,
                    'suggestion': '索引命中率低于70%,建议检查是否有缺失索引或查询未使用索引' if ratio < 70 else '索引命中率正常'
                }
            return {'ratio': 0, 'status': '未知'}
        except Exception as e:
            return {'error': str(e)}
    
    def _get_pg_client_connections(self) -> List[Dict]:
        """获取 PG 客户端连接列表，按 IP 分组统计"""
        try:
            # 获取所有客户端连接详情
            query = """
                SELECT 
                    datname as database,
                    usename as username,
                    COALESCE(client_addr::text, 'local') as client_ip,
                    application_name,
                    state,
                    COUNT(*) as connection_count
                FROM pg_stat_activity
                WHERE pid IS NOT NULL
                GROUP BY datname, usename, client_addr, application_name, state
                ORDER BY connection_count DESC
            """
            result = self.execute_query(query)
            
            # 按 IP 分组统计
            ip_stats = {}
            for row in result:
                ip = row['client_ip']
                if ip not in ip_stats:
                    ip_stats[ip] = {
                        'client_ip': ip,
                        'total_connections': 0,
                        'databases': set(),
                        'users': set(),
                        'applications': set(),
                        'active': 0,
                        'idle': 0
                    }
                
                ip_stats[ip]['total_connections'] += row['connection_count']
                ip_stats[ip]['databases'].add(row['database'])
                ip_stats[ip]['users'].add(row['username'])
                if row['application_name']:
                    ip_stats[ip]['applications'].add(row['application_name'])
                
                if row['state'] == 'active':
                    ip_stats[ip]['active'] += row['connection_count']
                elif row['state'] == 'idle':
                    ip_stats[ip]['idle'] += row['connection_count']
            
            # 转换为列表并格式化
            client_list = []
            for ip, stats in ip_stats.items():
                client_list.append({
                    'client_ip': stats['client_ip'],
                    'total_connections': stats['total_connections'],
                    'database_count': len(stats['databases']),
                    'databases': ', '.join(list(stats['databases'])[:3]) + ('...' if len(stats['databases']) > 3 else ''),
                    'user_count': len(stats['users']),
                    'users': ', '.join(list(stats['users'])[:2]) + ('...' if len(stats['users']) > 2 else ''),
                    'application_count': len(stats['applications']),
                    'applications': ', '.join(list(stats['applications'])[:2]) + ('...' if len(stats['applications']) > 2 else '') if stats['applications'] else 'N/A',
                    'active': stats['active'],
                    'idle': stats['idle']
                })
            
            # 按连接数排序
            client_list.sort(key=lambda x: x['total_connections'], reverse=True)
            
            return client_list
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_pg_activity(self) -> List[Dict]:
        """获取 PG 当前活动"""
        try:
            query = """
                SELECT datname, usename, application_name, client_addr,
                       state, query_start, state_change, query
                FROM pg_stat_activity
                WHERE state IS NOT NULL
                ORDER BY query_start DESC
                LIMIT 20
            """
            return self.execute_query(query)
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_pg_locks(self) -> List[Dict]:
        """获取 PG 锁信息(含阻塞链分析)"""
        try:
            query = """
                SELECT 
                    blocked_locks.pid AS blocked_pid,
                    blocked_activity.usename AS blocked_user,
                    blocking_locks.pid AS blocking_pid,
                    blocking_activity.usename AS blocking_user,
                    blocked_activity.datname AS database,
                    blocked_activity.application_name,
                    blocked_activity.client_addr,
                    blocked_locks.locktype,
                    blocked_locks.mode AS blocked_mode,
                    blocking_locks.mode AS blocking_mode,
                    blocked_activity.state AS blocked_state,
                    EXTRACT(EPOCH FROM (NOW() - blocked_activity.query_start)) AS wait_seconds,
                    LEFT(blocked_activity.query, 200) AS blocked_query,
                    LEFT(blocking_activity.query, 200) AS blocking_query
                FROM pg_catalog.pg_locks blocked_locks
                JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
                JOIN pg_catalog.pg_locks blocking_locks 
                    ON blocking_locks.locktype = blocked_locks.locktype
                    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
                    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
                    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
                    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
                    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
                    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
                    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
                    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
                    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
                    AND blocking_locks.pid != blocked_locks.pid
                JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
                WHERE NOT blocked_locks.granted
                ORDER BY wait_seconds DESC
                LIMIT 20
            """
            result = self.execute_query(query)
            
            for row in result:
                wait_sec = row.get('wait_seconds', 0)
                if wait_sec > 60:
                    row['severity'] = 'critical'
                    row['severity_label'] = '严重(>60秒)'
                elif wait_sec > 10:
                    row['severity'] = 'warning'
                    row['severity_label'] = '警告(>10秒)'
                else:
                    row['severity'] = 'info'
                    row['severity_label'] = '关注'
                
                row['wait_display'] = f"{wait_sec:.1f}秒"
            
            return result
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_pg_slow_queries(self) -> List[Dict]:
        """获取 PG 慢查询"""
        try:
            threshold = self.config.get('slow_query_threshold', 1.0)
            query = """
                SELECT query, calls, total_time, mean_time,
                       max_time, rows
                FROM pg_stat_statements
                WHERE mean_time > %s
                ORDER BY mean_time DESC
                LIMIT 10
            """
            return self.execute_query(query, (threshold * 1000,))
        except Exception:
            # pg_stat_statements 可能未安装
            return []
    
    def _get_pg_table_stats(self) -> List[Dict]:
        """获取 PG 表统计信息"""
        try:
            query = """
                SELECT schemaname, relname as table_name,
                       n_live_tup as live_tuples,
                       n_dead_tup as dead_tuples,
                       last_vacuum, last_autovacuum,
                       last_analyze, last_autoanalyze
                FROM pg_stat_user_tables
                ORDER BY n_live_tup DESC
                LIMIT 20
            """
            return self.execute_query(query)
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_pg_index_stats(self) -> List[Dict]:
        """获取 PG 索引统计信息"""
        try:
            query = """
                SELECT schemaname, relname as table_name,
                       indexrelname as index_name,
                       idx_scan, idx_tup_read, idx_tup_fetch
                FROM pg_stat_user_indexes
                ORDER BY idx_scan DESC
                LIMIT 20
            """
            return self.execute_query(query)
        except Exception as e:
            return [{'error': str(e)}]
    
    # MySQL 相关方法
    def _get_mysql_connections(self) -> Dict[str, Any]:
        """获取 MySQL 连接信息"""
        try:
            # 当前连接数
            threads_query = "SHOW STATUS LIKE 'Threads_connected'"
            threads = self.execute_query(threads_query)
            current = int(threads[0]['Value']) if threads else 0
            
            # 最大连接数
            max_query = "SHOW VARIABLES LIKE 'max_connections'"
            max_result = self.execute_query(max_query)
            max_conn = int(max_result[0]['Value']) if max_result else 151
            
            usage_percent = (current / max_conn * 100) if max_conn > 0 else 0
            
            return {
                'current': current,
                'max': max_conn,
                'usage_percent': round(usage_percent, 2),
                'status': '警告' if usage_percent > self.config.get('max_connections_threshold', 80) else '正常'
            }
        except Exception as e:
            return {'error': str(e)}
    
    def _get_mysql_client_connections(self) -> List[Dict]:
        """获取 MySQL 客户端连接列表，按 IP 分组统计"""
        try:
            query = """
                SELECT 
                    db as database,
                    user as username,
                    host as client_host,
                    command,
                    state,
                    COUNT(*) as connection_count
                FROM information_schema.processlist
                WHERE command != 'Daemon'
                GROUP BY db, user, host, command, state
                ORDER BY connection_count DESC
            """
            result = self.execute_query(query)
            
            # 按 IP 分组统计
            ip_stats = {}
            for row in result:
                # 提取 IP 地址（host 格式通常是 "ip:port"）
                host = row['client_host']
                ip = host.split(':')[0] if ':' in host else host
                
                if ip not in ip_stats:
                    ip_stats[ip] = {
                        'client_ip': ip,
                        'total_connections': 0,
                        'databases': set(),
                        'users': set(),
                        'active': 0,
                        'sleep': 0
                    }
                
                ip_stats[ip]['total_connections'] += row['connection_count']
                if row['database']:
                    ip_stats[ip]['databases'].add(row['database'])
                ip_stats[ip]['users'].add(row['username'])
                
                if row['command'] == 'Sleep':
                    ip_stats[ip]['sleep'] += row['connection_count']
                else:
                    ip_stats[ip]['active'] += row['connection_count']
            
            # 转换为列表并格式化
            client_list = []
            for ip, stats in ip_stats.items():
                client_list.append({
                    'client_ip': stats['client_ip'],
                    'total_connections': stats['total_connections'],
                    'database_count': len(stats['databases']),
                    'databases': ', '.join(list(stats['databases'])[:3]) + ('...' if len(stats['databases']) > 3 else ''),
                    'user_count': len(stats['users']),
                    'users': ', '.join(list(stats['users'])[:2]) + ('...' if len(stats['users']) > 2 else ''),
                    'application_count': 0,
                    'applications': 'N/A',
                    'active': stats['active'],
                    'idle': stats['sleep']
                })
            
            # 按连接数排序
            client_list.sort(key=lambda x: x['total_connections'], reverse=True)
            
            return client_list
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_mysql_status(self) -> Dict[str, Any]:
        """获取 MySQL 状态"""
        try:
            status_vars = ['Queries', 'Questions', 'Slow_queries', 'Uptime']
            result = {}
            for var in status_vars:
                try:
                    res = self.execute_query(f"SHOW STATUS LIKE '{var}'")
                    if res:
                        result[var] = res[0]['Value']
                except Exception:
                    result[var] = 'N/A'
            return result
        except Exception:
            return {}
    
    def _get_mysql_slow_queries(self) -> List[Dict]:
        """获取 MySQL 慢查询"""
        try:
            # 检查慢查询日志是否开启
            result = self.execute_query("SHOW VARIABLES LIKE 'slow_query_log'")
            if not result or result[0]['Value'] != 'ON':
                return [{'message': '慢查询日志未开启'}]
            
            # 这里可以读取慢查询日志文件
            # 简化处理，返回提示信息
            return [{'message': '请检查慢查询日志文件'}]
        except Exception:
            return []
    
    def _get_mysql_table_stats(self) -> List[Dict]:
        """获取 MySQL 表统计信息"""
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT 
                    table_name,
                    engine,
                    table_rows,
                    data_length,
                    index_length,
                    data_free
                FROM information_schema.tables
                WHERE table_schema = %s
                ORDER BY data_length + index_length DESC
                LIMIT 20
            """
            return self.execute_query(query, (db_name,))
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_mysql_index_stats(self) -> List[Dict]:
        """获取 MySQL 索引统计信息"""
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT 
                    table_name,
                    index_name,
                    cardinality
                FROM information_schema.statistics
                WHERE table_schema = %s
                ORDER BY cardinality DESC
                LIMIT 20
            """
            return self.execute_query(query, (db_name,))
        except Exception as e:
            return [{'error': str(e)}]
    
    def _get_mysql_processlist(self) -> List[Dict]:
        """获取 MySQL 进程列表"""
        try:
            query = "SELECT * FROM information_schema.processlist WHERE command != 'Daemon' LIMIT 20"
            return self.execute_query(query)
        except Exception as e:
            return [{'error': str(e)}]

    def _get_mysql_cache_hit_ratio(self) -> Dict[str, Any]:
        """获取 MySQL InnoDB 缓存命中率"""
        try:
            read_requests = self.execute_query("SHOW STATUS LIKE 'Innodb_buffer_pool_read_requests'")
            reads = self.execute_query("SHOW STATUS LIKE 'Innodb_buffer_pool_reads'")

            total_requests = int(read_requests[0]['Value']) if read_requests else 0
            total_reads = int(reads[0]['Value']) if reads else 0

            total = total_requests + total_reads
            ratio = round((total_requests / total) * 100, 2) if total > 0 else 100.0

            status = '优秀' if ratio >= 99 else ('良好' if ratio >= 95 else ('一般' if ratio >= 90 else '较差'))
            return {
                'ratio': ratio,
                'heap_read': total_reads,
                'heap_hit': total_requests,
                'status': status,
                'suggestion': 'InnoDB缓存命中率低于95%,建议增加innodb_buffer_pool_size' if ratio < 95 else '缓存命中率正常'
            }
        except Exception as e:
            return {'error': str(e)}

    def _get_mysql_index_hit_ratio(self) -> Dict[str, Any]:
        """获取 MySQL 索引使用率估算"""
        try:
            read_key = self.execute_query("SHOW STATUS LIKE 'Handler_read_key'")
            read_next = self.execute_query("SHOW STATUS LIKE 'Handler_read_rnd_next'")
            read_first = self.execute_query("SHOW STATUS LIKE 'Handler_read_first'")
            read_prev = self.execute_query("SHOW STATUS LIKE 'Handler_read_prev'")
            read_rnd = self.execute_query("SHOW STATUS LIKE 'Handler_read_rnd'")

            idx_reads = int(read_key[0]['Value']) if read_key else 0
            idx_reads += int(read_first[0]['Value']) if read_first else 0
            idx_reads += int(read_prev[0]['Value']) if read_prev else 0

            seq_reads = int(read_next[0]['Value']) if read_next else 0
            seq_reads += int(read_rnd[0]['Value']) if read_rnd else 0

            total = idx_reads + seq_reads
            ratio = round((idx_reads / total) * 100, 2) if total > 0 else 100.0

            status = '优秀' if ratio >= 90 else ('良好' if ratio >= 70 else ('一般' if ratio >= 50 else '较差'))
            return {
                'ratio': ratio,
                'idx_scan': idx_reads,
                'seq_scan': seq_reads,
                'status': status,
                'suggestion': '索引使用率低于70%,建议检查是否有缺失索引或查询未使用索引' if ratio < 70 else '索引使用率正常'
            }
        except Exception as e:
            return {'error': str(e)}

    def _get_mysql_long_transactions(self) -> List[Dict]:
        """获取 MySQL 长事务"""
        try:
            threshold_seconds = self.config.get('long_transaction_threshold', 300)
            query = """
                SELECT
                    trx_id,
                    trx_mysql_thread_id as thread_id,
                    trx_state as state,
                    trx_tables_locked as tables_locked,
                    trx_rows_locked as rows_locked,
                    TIMESTAMPDIFF(SECOND, trx_started, NOW()) as duration_seconds
                FROM information_schema.innodb_trx
                WHERE TIMESTAMPDIFF(SECOND, trx_started, NOW()) > %s
                ORDER BY trx_started ASC
            """
            result = self.execute_query(query, (threshold_seconds,))
            transactions = []
            for row in result:
                duration_sec = row.get('duration_seconds', 0)
                if duration_sec > 3600:
                    severity = 'critical'
                    severity_label = '严重(>1小时)'
                elif duration_sec > 1800:
                    severity = 'warning'
                    severity_label = '警告(>30分钟)'
                else:
                    severity = 'info'
                    severity_label = '关注(>5分钟)'

                transactions.append({
                    'trx_id': row.get('trx_id'),
                    'thread_id': row.get('thread_id'),
                    'state': row.get('state'),
                    'tables_locked': row.get('tables_locked'),
                    'rows_locked': row.get('rows_locked'),
                    'duration_seconds': duration_sec,
                    'severity': severity,
                    'severity_label': severity_label,
                    'duration_display': f"{duration_sec // 60}分{duration_sec % 60}秒"
                })
            return transactions
        except Exception as e:
            return [{'error': str(e)}]

    def _get_mysql_locks(self) -> List[Dict]:
        """获取 MySQL 锁等待信息"""
        try:
            query = """
                SELECT
                    r.trx_id as waiting_trx_id,
                    r.trx_mysql_thread_id as waiting_thread,
                    b.trx_id as blocking_trx_id,
                    b.trx_mysql_thread_id as blocking_thread,
                    w.lock_mode as waiting_lock_mode,
                    w.lock_type as waiting_lock_type,
                    b_lock.lock_mode as blocking_lock_mode,
                    TIMESTAMPDIFF(SECOND, r.trx_started, NOW()) as wait_seconds
                FROM information_schema.innodb_lock_waits w
                INNER JOIN information_schema.innodb_trx b ON b.trx_id = w.blocking_trx_id
                INNER JOIN information_schema.innodb_trx r ON r.trx_id = w.requesting_trx_id
                INNER JOIN information_schema.innodb_locks b_lock ON b_lock.lock_id = w.blocking_lock_id
                ORDER BY wait_seconds DESC
            """
            result = self.execute_query(query)
            locks = []
            for row in result:
                wait_sec = row.get('wait_seconds', 0)
                if wait_sec > 60:
                    severity = 'critical'
                    severity_label = '严重(>60秒)'
                elif wait_sec > 10:
                    severity = 'warning'
                    severity_label = '警告(>10秒)'
                else:
                    severity = 'info'
                    severity_label = '关注'

                locks.append({
                    'waiting_trx_id': row.get('waiting_trx_id'),
                    'waiting_thread': row.get('waiting_thread'),
                    'blocking_trx_id': row.get('blocking_trx_id'),
                    'blocking_thread': row.get('blocking_thread'),
                    'waiting_lock_mode': row.get('waiting_lock_mode'),
                    'waiting_lock_type': row.get('waiting_lock_type'),
                    'blocking_lock_mode': row.get('blocking_lock_mode'),
                    'wait_seconds': wait_sec,
                    'severity': severity,
                    'severity_label': severity_label,
                    'wait_display': f"{wait_sec:.1f}秒"
                })
            return locks
        except Exception as e:
            return [{'error': str(e)}]

    def _get_mysql_index_size_analysis(self) -> Dict[str, List[Dict]]:
        """获取 MySQL 索引大小占比分析"""
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT
                    table_name,
                    table_rows,
                    data_length,
                    index_length,
                    data_length + index_length as total_size,
                    CASE WHEN data_length > 0
                        THEN ROUND(index_length / data_length * 100, 2)
                        ELSE 0 END as index_ratio
                FROM information_schema.tables
                WHERE table_schema = %s
                  AND table_type = 'BASE TABLE'
                  AND table_rows >= 10000
                  AND data_length > 0
                ORDER BY index_length / data_length DESC
                LIMIT 30
            """
            result = self.execute_query(query, (db_name,))
            analysis = []
            for row in result:
                ratio = row.get('index_ratio', 0)
                if ratio > 100:
                    attention = '严重'
                    suggestion = f"索引大小({format_size(row['index_length'])})超过数据大小({format_size(row['data_length'])}),请检查冗余索引"
                elif ratio > 50:
                    attention = '关注'
                    suggestion = f"索引占比{ratio}%,建议检查索引是否合理"
                else:
                    attention = '正常'
                    suggestion = ''

                analysis.append({
                    'schemaname': db_name,
                    'table_name': row['table_name'],
                    'row_count': row['table_rows'],
                    'table_size': format_size(row['data_length']),
                    'index_size': format_size(row['index_length']),
                    'total_size': format_size(row['total_size']),
                    'table_size_bytes': row['data_length'],
                    'index_size_bytes': row['index_length'],
                    'total_size_bytes': row['total_size'],
                    'index_ratio': ratio,
                    'attention': attention,
                    'suggestion': suggestion
                })

            return {db_name: analysis} if analysis else {}
        except Exception as e:
            return {}

    def _get_mysql_invalid_indexes(self) -> Dict[str, List[Dict]]:
        """获取 MySQL 可能无效的索引（基于 cardinality 为 0 或极少使用）"""
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT
                    table_name,
                    index_name,
                    cardinality,
                    seq_in_index
                FROM information_schema.statistics
                WHERE table_schema = %s
                  AND cardinality = 0
                  AND index_name != 'PRIMARY'
                ORDER BY table_name, index_name
            """
            result = self.execute_query(query, (db_name,))
            indexes = []
            for row in result:
                indexes.append({
                    'schemaname': db_name,
                    'table_name': row['table_name'],
                    'index_name': row['index_name'],
                    'index_size': 'N/A',
                    'index_scans': 0,
                    'issue_type': '基数为0',
                    'suggestion': '该索引基数为0,建议评估是否需要删除',
                    'database': db_name
                })
            return {db_name: indexes} if indexes else {}
        except Exception as e:
            return {}

    def _get_mysql_duplicate_indexes(self) -> Dict[str, List[Dict]]:
        """获取 MySQL 重复索引"""
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT
                    t1.table_name,
                    t1.index_name as index_name_a,
                    t2.index_name as index_name_b,
                    GROUP_CONCAT(t1.column_name ORDER BY t1.seq_in_index) as columns_a,
                    GROUP_CONCAT(t2.column_name ORDER BY t2.seq_in_index) as columns_b
                FROM information_schema.statistics t1
                JOIN information_schema.statistics t2
                    ON t1.table_schema = t2.table_schema
                    AND t1.table_name = t2.table_name
                    AND t1.index_name < t2.index_name
                    AND t1.seq_in_index = t2.seq_in_index
                    AND t1.column_name = t2.column_name
                WHERE t1.table_schema = %s
                GROUP BY t1.table_name, t1.index_name, t2.index_name
                HAVING columns_a = columns_b
                LIMIT 50
            """
            result = self.execute_query(query, (db_name,))
            indexes = []
            for row in result:
                indexes.append({
                    'schemaname': db_name,
                    'table_name': row['table_name'],
                    'index_name': row['index_name_b'],
                    'index_definition': f"{row['index_name_a']} 与 {row['index_name_b']} 列相同: ({row['columns_a']})",
                    'index_size': 'N/A',
                    'index_scans': 0,
                    'suggestion': f"表 {row['table_name']} 存在重复索引,建议保留使用频率较高的索引,删除其他重复索引",
                    'database': db_name
                })
            return {db_name: indexes} if indexes else {}
        except Exception as e:
            return {}

    def _get_pg_long_transactions(self) -> List[Dict]:
        """获取长事务(运行超过阈值的会话)"""
        try:
            threshold_seconds = self.config.get('long_transaction_threshold', 300)  # 默认5分钟
            
            query = """
                SELECT 
                    pid,
                    datname as database,
                    usename as username,
                    client_addr,
                    application_name,
                    state,
                    query_start,
                    NOW() - query_start as duration,
                    EXTRACT(EPOCH FROM (NOW() - query_start)) as duration_seconds,
                    wait_event_type,
                    wait_event,
                    LEFT(query, 200) as query
                FROM pg_stat_activity
                WHERE state != 'idle'
                  AND query NOT LIKE '%%pg_stat_activity%%'
                  AND NOW() - query_start > INTERVAL '%s seconds'
                ORDER BY query_start ASC
            """ % threshold_seconds
            
            result = self.execute_query(query)
            
            for row in result:
                duration_sec = row.get('duration_seconds', 0)
                if duration_sec > 3600:
                    row['severity'] = 'critical'
                    row['severity_label'] = '严重(>1小时)'
                elif duration_sec > 1800:
                    row['severity'] = 'warning'
                    row['severity_label'] = '警告(>30分钟)'
                else:
                    row['severity'] = 'info'
                    row['severity_label'] = '关注(>5分钟)'
                
                row['duration_display'] = str(row.get('duration', ''))
            
            return result
        except Exception as e:
            return [{'error': str(e)}]
