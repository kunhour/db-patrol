import concurrent.futures
import logging
from typing import Dict, Any, List, Tuple
from .base import BaseInspector
from ..utils import format_size

logger = logging.getLogger('db_patrol')


class BasicInfoInspector(BaseInspector):

    name = 'basic_info'
    title = '检查基本信息'

    def inspect(self) -> Dict[str, Any]:
        """采集数据库基本信息"""
        db_type = self.connection.config['type']
        
        if 'pg' in db_type.lower() or 'postgres' in db_type.lower():
            return self._inspect_pg()
        else:
            return self._inspect_mysql()
    
    def _inspect_pg(self) -> Dict[str, Any]:
        try:
            query = """
                SELECT 
                    d.datname as name,
                    pg_database_size(d.datname) as size,
                    pg_encoding_to_char(d.encoding) as encoding,
                    d.datcollate as collation,
                    d.datctype as ctype,
                    d.datistemplate as is_template,
                    d.datallowconn as allow_conn
                FROM pg_database d
                WHERE d.datistemplate = false
                ORDER BY pg_database_size(d.datname) DESC
            """
            db_rows = self.execute_query(query)
        except Exception:
            db_rows = []

        databases = []
        db_names = []
        for row in db_rows:
            db_name = row['name']
            db_names.append(db_name)
            databases.append({
                'name': db_name,
                'size': format_size(row['size']),
                'size_bytes': row['size'],
                'encoding': row['encoding'],
                'collation': row['collation'],
                'ctype': row['ctype'],
                'schema_count': 0,
                'table_count': 0,
                'view_count': 0,
                'trigger_count': 0,
                'is_backup': False
            })

        all_tables = []
        tables_without_pk = {}

        logger.info("    → 获取所有数据库的表信息...")
        db_results = self._inspect_pg_databases_parallel(db_names)

        for db in databases:
            db_name = db['name']
            if db_name in db_results:
                stats = db_results[db_name]['stats']
                db['schema_count'] = stats.get('schema_count', 0)
                db['table_count'] = stats.get('table_count', 0)
                db['view_count'] = stats.get('view_count', 0)
                db['trigger_count'] = stats.get('trigger_count', 0)

        for db_name, result in db_results.items():
            all_tables.extend(result['tables'])
            if result['tables_without_pk']:
                tables_without_pk[db_name] = result['tables_without_pk']

        all_tables.sort(key=lambda x: x.get('size_bytes', 0), reverse=True)

        databases = self._detect_backup_databases(databases)
        all_tables = self._detect_backup_tables(all_tables)

        normal_databases = [db for db in databases if not db.get('is_backup')]
        backup_databases = [db for db in databases if db.get('is_backup')]
        normal_tables = [t for t in all_tables if not t.get('is_backup')]
        backup_tables = [t for t in all_tables if t.get('is_backup')]

        backup_db_total_size = sum(db.get('size_bytes', 0) for db in backup_databases)
        backup_db_total_tables = sum(db.get('table_count', 0) for db in backup_databases)
        backup_db_total_views = sum(db.get('view_count', 0) for db in backup_databases)
        backup_db_total_triggers = sum(db.get('trigger_count', 0) for db in backup_databases)
        backup_table_total_size = sum(table.get('size_bytes', 0) for table in backup_tables)

        results = {
            'instance_info': self._get_pg_instance_info(),
            'version': self._get_pg_version(),
            'connection_status': self._check_connection(),
            'uptime': self._get_pg_uptime(),
            'settings': self._get_pg_settings(),
            'databases': {
                'total': len(databases),
                'normal': normal_databases,
                'backup': backup_databases,
                'normal_count': len(normal_databases),
                'backup_count': len(backup_databases),
                'backup_total_size': format_size(backup_db_total_size),
                'backup_total_tables': backup_db_total_tables,
                'backup_total_views': backup_db_total_views,
                'backup_total_triggers': backup_db_total_triggers
            },
            'tables': {
                'all': all_tables,
                'normal': normal_tables,
                'backup': backup_tables,
                'total_count': len(all_tables),
                'normal_count': len(normal_tables),
                'backup_count': len(backup_tables),
                'backup_total_size': format_size(backup_table_total_size)
            },
            'tables_without_pk': tables_without_pk
        }
        return results
    
    def _inspect_mysql(self) -> Dict[str, Any]:
        """MySQL 模式基本信息"""
        logger.info("    → 获取数据库列表...")
        databases = self._get_mysql_databases()
        db_names = [db['name'] for db in databases]

        logger.info("    → 获取所有数据库的表信息...")
        all_tables = []
        tables_without_pk = {}

        for db_name in db_names:
            db_tables = self._get_mysql_tables(db_name)
            db_tables_without_pk = self._get_mysql_tables_without_pk(db_name)
            all_tables.extend(db_tables)
            if db_tables_without_pk:
                tables_without_pk[db_name] = db_tables_without_pk

            for db in databases:
                if db['name'] == db_name:
                    db['table_count'] = len(db_tables)
                    break

        all_tables.sort(key=lambda x: x.get('size_bytes', 0), reverse=True)

        databases = self._detect_backup_databases(databases)
        all_tables = self._detect_backup_tables(all_tables)

        normal_databases = [db for db in databases if not db.get('is_backup')]
        backup_databases = [db for db in databases if db.get('is_backup')]
        normal_tables = [t for t in all_tables if not t.get('is_backup')]
        backup_tables = [t for t in all_tables if t.get('is_backup')]

        backup_db_total_size = sum(db.get('size_bytes', 0) for db in backup_databases)
        backup_db_total_tables = sum(db.get('table_count', 0) for db in backup_databases)
        backup_table_total_size = sum(table.get('size_bytes', 0) for table in backup_tables)

        results = {
            'instance_info': self._get_mysql_instance_info(),
            'version': self._get_mysql_version(),
            'connection_status': self._check_connection(),
            'uptime': self._get_mysql_uptime(),
            'settings': self._get_mysql_settings(),
            'databases': {
                'total': len(databases),
                'normal': normal_databases,
                'backup': backup_databases,
                'normal_count': len(normal_databases),
                'backup_count': len(backup_databases),
                'backup_total_size': format_size(backup_db_total_size),
                'backup_total_tables': backup_db_total_tables,
                'backup_total_views': 0,
                'backup_total_triggers': 0
            },
            'tables': {
                'all': all_tables,
                'normal': normal_tables,
                'backup': backup_tables,
                'total_count': len(all_tables),
                'normal_count': len(normal_tables),
                'backup_count': len(backup_tables),
                'backup_total_size': format_size(backup_table_total_size)
            },
            'tables_without_pk': tables_without_pk
        }
        return results
    
    def _check_connection(self) -> Dict[str, Any]:
        """检查连接状态"""
        try:
            self.execute_query("SELECT 1")
            return {'status': '正常', 'message': '连接成功'}
        except Exception as e:
            return {'status': '异常', 'message': str(e)}
    
    def _get_pg_version(self) -> str:
        """获取 PG 版本"""
        try:
            result = self.execute_query("SELECT version()")
            return result[0]['version'] if result else '未知'
        except Exception:
            return '未知'
    
    def _get_pg_instance_info(self) -> Dict[str, Any]:
        """获取数据库实例信息"""
        try:
            info = {}
            
            # 产品版本(完整版本字符串)
            result = self.execute_query("SELECT version()")
            full_version = result[0]['version'] if result else '未知'
            info['full_version'] = full_version
            
            # 提取产品名称和版本号
            # PostgreSQL格式: PostgreSQL 9.2.4 on x86_64...
            # Vastbase格式: Vastbase 3.0 (基于PostgreSQL)
            import re
            version_match = re.match(r'^(PostgreSQL|Vastbase|openGauss)\s+([\d.]+)', full_version)
            if version_match:
                info['product_name'] = version_match.group(1)
                info['product_version'] = version_match.group(2)
            else:
                info['product_name'] = 'PostgreSQL'
                info['product_version'] = '未知'
            
            # 实例名称
            result = self.execute_query("SELECT current_database()")
            info['current_database'] = result[0]['current_database'] if result else 'N/A'
            
            # 实例总大小（所有数据库）
            result = self.execute_query("SELECT SUM(pg_database_size(datname)) as total_size FROM pg_database WHERE datistemplate = false")
            total_size = result[0]['total_size'] if result else 0
            info['total_size'] = format_size(total_size)
            info['total_size_bytes'] = total_size
            
            # 数据库数量
            result = self.execute_query("SELECT COUNT(*) as count FROM pg_database WHERE datistemplate = false")
            info['database_count'] = result[0]['count'] if result else 0
            
            # 总连接数限制
            result = self.execute_query("SHOW max_connections")
            info['max_connections'] = result[0]['max_connections'] if result else 'N/A'
            
            # 当前连接数
            result = self.execute_query("SELECT COUNT(*) as count FROM pg_stat_activity")
            info['current_connections'] = result[0]['count'] if result else 0
            
            # 共享内存
            result = self.execute_query("SHOW shared_buffers")
            info['shared_buffers'] = result[0]['shared_buffers'] if result else 'N/A'
            
            # 数据库当前时间
            result = self.execute_query("SELECT NOW() as db_time")
            info['db_time'] = result[0]['db_time'] if result else 'N/A'
            
            # 时区
            result = self.execute_query("SHOW timezone")
            info['timezone'] = result[0]['TimeZone'] if result else 'N/A'
            
            # 数据目录
            result = self.execute_query("SHOW data_directory")
            info['data_directory'] = result[0]['data_directory'] if result else 'N/A'
            
            # 监听地址
            result = self.execute_query("SHOW listen_addresses")
            info['listen_addresses'] = result[0]['listen_addresses'] if result else 'N/A'
            
            # 端口
            result = self.execute_query("SHOW port")
            info['port'] = result[0]['port'] if result else 'N/A'
            
            # 大小写敏感配置（Vastbase 特有参数，PostgreSQL 使用标准行为）
            try:
                result = self.execute_query("SHOW enable_case_sensitive")
                if result:
                    enable_case_sensitive = result[0]['enable_case_sensitive']
                    # 转换为中文描述
                    if enable_case_sensitive == 'on':
                        info['case_sensitive'] = '区分大小写'
                    elif enable_case_sensitive == 'off':
                        info['case_sensitive'] = '忽略大小写'
                    else:
                        info['case_sensitive'] = enable_case_sensitive
                else:
                    info['case_sensitive'] = '标准行为(加引号区分)'
            except Exception:
                # PostgreSQL 标准行为：不加引号自动转小写，加引号区分大小写
                info['case_sensitive'] = '标准行为(加引号区分)'
            
            return info
        except Exception as e:
            return {'error': str(e)}
    
    def _get_pg_uptime(self) -> str:
        """获取 PG 运行时间"""
        try:
            query = "SELECT pg_postmaster_start_time() as start_time"
            result = self.execute_query(query)
            if result and result[0]['start_time']:
                return str(result[0]['start_time'])
        except Exception:
            pass
        return '未知'

    def _inspect_pg_databases_parallel(self, db_names: List[str]) -> Dict[str, Dict]:
        results = {}
        max_workers = min(16, len(db_names))
        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            future_to_db = {
                executor.submit(self._inspect_pg_database_all, db_name): db_name
                for db_name in db_names
            }
            for future in concurrent.futures.as_completed(future_to_db):
                db_name = future_to_db[future]
                try:
                    results[db_name] = future.result()
                except Exception:
                    results[db_name] = {'stats': {}, 'tables': [], 'tables_without_pk': []}
        return results

    def _inspect_pg_database_all(self, db_name: str) -> Dict[str, Any]:
        db_config = self.connection.config.copy()
        db_config['database'] = db_name

        stats = {'table_count': 0, 'view_count': 0, 'trigger_count': 0, 'schema_count': 0}
        tables = []
        tables_without_pk = []

        try:
            with self._create_connection(db_config) as conn:
                stats_query = """
                    SELECT
                        (SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public') as table_count,
                        (SELECT COUNT(*) FROM pg_views WHERE schemaname = 'public') as view_count,
                        (SELECT COUNT(*) FROM pg_trigger t
                         JOIN pg_class c ON t.tgrelid = c.oid
                         JOIN pg_namespace n ON c.relnamespace = n.oid
                         WHERE n.nspname = 'public') as trigger_count,
                        (SELECT COUNT(*) FROM information_schema.schemata
                         WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')) as schema_count
                """
                result = conn.execute_query(stats_query)
                if result:
                    stats = {
                        'table_count': result[0].get('table_count', 0),
                        'view_count': result[0].get('view_count', 0),
                        'trigger_count': result[0].get('trigger_count', 0),
                        'schema_count': result[0].get('schema_count', 0)
                    }

                tables_query = """
                    SELECT
                        t.schemaname,
                        t.tablename as table_name,
                        pg_size_pretty(pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename))) as size,
                        pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename)) as size_bytes,
                        (SELECT COUNT(*) FROM information_schema.columns c
                         WHERE c.table_schema = t.schemaname AND c.table_name = t.tablename) as column_count,
                        COALESCE(c.reltuples, 0)::bigint as row_count
                    FROM pg_tables t
                    JOIN pg_class c ON c.relname = t.tablename
                    JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
                    WHERE t.schemaname = 'public'
                    ORDER BY pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename)) DESC
                """
                result = conn.execute_query(tables_query)
                for row in result:
                    tables.append({
                        'database': db_name,
                        'schema': row['schemaname'],
                        'table_name': row['table_name'],
                        'size': row['size'],
                        'size_bytes': row['size_bytes'],
                        'column_count': row['column_count'],
                        'row_count': int(row['row_count']),
                        'is_backup': False
                    })

                pk_query = """
                    SELECT
                        t.schemaname,
                        t.tablename as table_name,
                        COALESCE(c.reltuples, 0)::bigint as row_count,
                        EXISTS (
                            SELECT 1 FROM information_schema.table_constraints tc
                            WHERE tc.table_schema = t.schemaname
                            AND tc.table_name = t.tablename
                            AND tc.constraint_type = 'PRIMARY KEY'
                        ) as has_primary_key,
                        EXISTS (
                            SELECT 1 FROM pg_indexes pi
                            WHERE pi.schemaname = t.schemaname
                            AND pi.tablename = t.tablename
                            AND pi.indexname IN (
                                SELECT indexname FROM pg_indexes pi2
                                WHERE pi2.schemaname = t.schemaname
                                AND pi2.tablename = t.tablename
                                AND EXISTS (
                                    SELECT 1 FROM pg_class pc
                                    JOIN pg_index pindex ON pc.oid = pindex.indexrelid
                                    WHERE pc.relname = pi2.indexname
                                    AND pindex.indisunique = true
                                )
                            )
                        ) as has_unique_index
                    FROM pg_tables t
                    JOIN pg_class c ON c.relname = t.tablename
                    JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
                    WHERE t.schemaname = 'public'
                    ORDER BY c.reltuples DESC NULLS LAST
                """
                pk_result = conn.execute_query(pk_query)
                for row in pk_result:
                    if not row['has_primary_key'] and not row['has_unique_index']:
                        size_query = """
                            SELECT
                                pg_size_pretty(pg_total_relation_size(quote_ident(%s)||'.'||quote_ident(%s))) as size,
                                pg_total_relation_size(quote_ident(%s)||'.'||quote_ident(%s)) as size_bytes,
                                (SELECT COUNT(*) FROM information_schema.columns c
                                 WHERE c.table_schema = %s AND c.table_name = %s) as column_count
                        """
                        size_result = conn.execute_query(size_query, (
                            row['schemaname'], row['table_name'],
                            row['schemaname'], row['table_name'],
                            row['schemaname'], row['table_name']
                        ))
                        if size_result:
                            tables_without_pk.append({
                                'schema': row['schemaname'],
                                'table_name': row['table_name'],
                                'size': size_result[0]['size'],
                                'size_bytes': size_result[0]['size_bytes'],
                                'column_count': size_result[0]['column_count'],
                                'row_count': int(row['row_count'])
                            })
                        else:
                            tables_without_pk.append({
                                'schema': row['schemaname'],
                                'table_name': row['table_name'],
                                'size': 'N/A',
                                'size_bytes': 0,
                                'column_count': 'N/A',
                                'row_count': int(row['row_count'])
                            })
        except Exception:
            pass

        return {'stats': stats, 'tables': tables, 'tables_without_pk': tables_without_pk}

    def _detect_backup_databases(self, databases: List[Dict]) -> List[Dict]:
        """检测疑似备份库"""
        import re
        
        all_db_names = [db['name'] for db in databases]
        
        # 强备份特征
        strong_backup_patterns = [
            r'_backup', r'_bak', r'_copy', r'_old$', r'_\d{8}$', r'_\d{6}$',
            r'_\d{4}[-_]\d{2}[-_]\d{2}', r'[_\-]test\d*$', r'^test_', r'_temp$', r'_dev$',
        ]
        
        # 弱备份特征
        weak_backup_patterns = [
            r'_new$', r'_prod$',
            r'\d{3,}$', r'\d{4}$',
        ]
        
        combo_patterns = [r'(test|dev|temp|new|prod)$']
        
        # 明确排除的正常库白名单
        exclude_db_names = ['emate_dev']
        
        for db in databases:
            db_name = db['name']
            is_backup = False
            
            # 白名单库直接跳过检测
            if db_name in exclude_db_names:
                db['is_backup'] = is_backup
                continue
            
            # 检查强备份特征
            for pattern in strong_backup_patterns:
                if re.search(pattern, db_name, re.IGNORECASE):
                    is_backup = True
                    break
            
            # 检查弱备份特征
            if not is_backup:
                for pattern in weak_backup_patterns:
                    if re.search(pattern, db_name, re.IGNORECASE):
                        base_name = self._get_db_base_name(db_name)
                        if base_name and self._has_similar_db(base_name, db_name, all_db_names):
                            is_backup = True
                            break
            
            # 检查组合命名特征
            if not is_backup:
                for pattern in combo_patterns:
                    match = re.search(pattern, db_name, re.IGNORECASE)
                    if match:
                        base_name = db_name[:match.start()]
                        if base_name and self._has_similar_db(base_name, db_name, all_db_names):
                            is_backup = True
                            break
            
            db['is_backup'] = is_backup
        
        return databases
    
    def _detect_backup_tables(self, tables: List[Dict]) -> List[Dict]:
        """检测疑似备份表"""
        import re
        
        # 排除的正常命名模式（这些结尾属于正常设定，不是备份）
        exclude_patterns = [
            r'_nod_old$', r'_lin_old$', r'_net_old$',
        ]
        
        date_patterns = [
            r'_{8}$', r'_{6}$', r'_{4}[-_]{2}[-_]{2}', r'_{4}$',
        ]
        
        backup_patterns = [
            r'_backup', r'_bak', r'_copy', r'_old$', r'_new$', r'_temp$', r'_tmp$',
        ]
        
        for table in tables:
            table_name = table['table_name']
            is_backup = False
            
            # 先检查是否在排除列表中
            excluded = False
            for pattern in exclude_patterns:
                if re.search(pattern, table_name, re.IGNORECASE):
                    excluded = True
                    break
            
            if not excluded:
                # 检查备份命名
                for pattern in backup_patterns:
                    if re.search(pattern, table_name, re.IGNORECASE):
                        is_backup = True
                        break
                
                # 检查日期命名
                if not is_backup:
                    for pattern in date_patterns:
                        if re.search(pattern, table_name, re.IGNORECASE):
                            # 检查是否有相似的基础表名
                            base_name = self._get_base_table_name(table_name)
                            if base_name:
                                # 查找是否有相同基础表名的其他表
                                similar_tables = [t for t in tables 
                                                if self._get_base_table_name(t['table_name']) == base_name 
                                                and t['table_name'] != table_name]
                                if not similar_tables:
                                    is_backup = True
                            else:
                                is_backup = True
                            break
            
            table['is_backup'] = is_backup
        
        return tables
    
    def _get_base_table_name(self, table_name: str) -> str:
        """获取基础表名"""
        import re
        patterns = [
            r'_(\d{8})$', r'_(\d{6})$', r'_(\d{4})[-_](\d{2})[-_](\d{2})$', r'_(\d{4})$',
        ]
        for pattern in patterns:
            match = re.search(pattern, table_name, re.IGNORECASE)
            if match:
                return table_name[:match.start()]
        return None
    
    def _get_db_base_name(self, db_name: str) -> str:
        """获取数据库基础名称"""
        import re
        suffix_patterns = [
            r'_(test|temp|new|dev|prod)$', r'_\d{4}$', r'\d{3,}$',
        ]
        for pattern in suffix_patterns:
            match = re.search(pattern, db_name, re.IGNORECASE)
            if match:
                return db_name[:match.start()]
        return None
    
    def _has_similar_db(self, base_name: str, current_db: str, all_db_names: list) -> bool:
        """检查是否存在相似前缀的数据库"""
        base_name_lower = base_name.lower()
        current_db_lower = current_db.lower()
        for db_name in all_db_names:
            if db_name.lower() == current_db_lower:
                continue
            db_name_lower = db_name.lower()
            if db_name_lower == base_name_lower or db_name_lower.startswith(base_name_lower + '_'):
                return True
        return False
    
    def _get_pg_settings(self) -> Dict[str, Any]:
        """获取 PG 关键配置"""
        settings = {}
        
        # 定义要查询的配置列表
        config_queries = [
            # 连接相关
            'max_connections',
            # 内存相关
            'shared_buffers', 'work_mem', 'maintenance_work_mem', 'effective_cache_size',
            # WAL相关
            'wal_level', 'max_wal_size', 'min_wal_size', 'wal_buffers',
            # 检查点相关
            'checkpoint_completion_target', 'checkpoint_timeout',
            # 自动清理相关
            'autovacuum', 'autovacuum_max_workers', 'autovacuum_naptime',
            # 查询规划器相关
            'random_page_cost', 'default_statistics_target', 'effective_io_concurrency',
            # 并行查询相关
            'max_parallel_workers_per_gather', 'max_parallel_workers',
            # 日志相关
            'logging_collector', 'log_statement', 'log_min_duration_statement',
            # 其他配置
            'timezone', 'max_locks_per_transaction',
        ]
        
        # 批量查询所有配置
        try:
            query = "SELECT name, setting FROM pg_settings WHERE name IN (" + ','.join([f"'{name}'" for name in config_queries]) + ")"
            results = self.execute_query(query)
            if results:
                for row in results:
                    name = row['name']
                    value = row['setting']
                    settings[name] = value
        except Exception as e:
            # 如果批量查询失败，尝试逐个查询
            for config_name in config_queries:
                try:
                    result = self.execute_query(f"SHOW {config_name}")
                    if result:
                        # SHOW命令返回的列名就是参数名
                        settings[config_name] = result[0].get(config_name, result[0].get('setting', 'N/A'))
                except Exception:
                    settings[config_name] = 'N/A'
        
        # 大小写敏感配置（Vastbase 特有参数）
        try:
            result = self.execute_query("SHOW enable_case_sensitive")
            settings['enable_case_sensitive'] = result[0].get('enable_case_sensitive', 'standard') if result else 'standard'
        except Exception:
            settings['enable_case_sensitive'] = 'standard'
        
        return settings
    
    # MySQL 相关方法
    def _get_mysql_version(self) -> str:
        """获取 MySQL 版本"""
        try:
            result = self.execute_query("SELECT version() as version")
            return result[0]['version'] if result else '未知'
        except Exception:
            return '未知'
    
    def _get_mysql_instance_info(self) -> Dict[str, Any]:
        """获取 MySQL 实例信息"""
        try:
            info = {}
            import re

            result = self.execute_query("SELECT version() as version")
            full_version = result[0]['version'] if result else '未知'
            info['full_version'] = full_version

            version_match = re.match(r'^([\w\-]+)\s+([\d.]+)', full_version)
            if version_match:
                info['product_name'] = version_match.group(1)
                info['product_version'] = version_match.group(2)
            else:
                info['product_name'] = 'MySQL'
                info['product_version'] = '未知'

            result = self.execute_query("SELECT DATABASE() as db")
            info['current_database'] = result[0]['db'] if result else 'N/A'

            result = self.execute_query("SELECT NOW() as db_time")
            info['db_time'] = result[0]['db_time'] if result else 'N/A'

            result = self.execute_query("SHOW VARIABLES LIKE 'port'")
            info['port'] = result[0]['Value'] if result else 'N/A'

            query = """
                SELECT SUM(data_length + index_length) as total_size
                FROM information_schema.tables
                WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
            """
            result = self.execute_query(query)
            total_size = result[0]['total_size'] if result else 0
            info['total_size'] = format_size(total_size)
            info['total_size_bytes'] = total_size

            query = """
                SELECT COUNT(*) as count FROM information_schema.schemata
                WHERE schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
            """
            result = self.execute_query(query)
            info['database_count'] = result[0]['count'] if result else 0

            result = self.execute_query("SHOW VARIABLES LIKE 'max_connections'")
            info['max_connections'] = result[0]['Value'] if result else 'N/A'

            result = self.execute_query("SHOW STATUS LIKE 'Threads_connected'")
            info['current_connections'] = int(result[0]['Value']) if result else 0

            result = self.execute_query("SHOW VARIABLES LIKE 'innodb_buffer_pool_size'")
            info['shared_buffers'] = result[0]['Value'] if result else 'N/A'

            result = self.execute_query("SHOW VARIABLES LIKE 'character_set_server'")
            info['encoding'] = result[0]['Value'] if result else 'N/A'

            info['data_directory'] = 'N/A'
            info['listen_addresses'] = 'N/A'
            info['timezone'] = 'N/A'
            info['case_sensitive'] = '由 lower_case_table_names 控制'

            return info
        except Exception as e:
            return {'error': str(e)}

    def _get_mysql_databases(self) -> List[Dict]:
        """获取 MySQL 数据库列表"""
        try:
            query = """
                SELECT
                    schema_name as name,
                    DEFAULT_CHARACTER_SET_NAME as encoding,
                    DEFAULT_COLLATION_NAME as collation
                FROM information_schema.schemata
                WHERE schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
                ORDER BY schema_name
            """
            rows = self.execute_query(query)
            databases = []
            for row in rows:
                db_name = row['name']
                size_query = """
                    SELECT SUM(data_length + index_length) as size
                    FROM information_schema.tables
                    WHERE table_schema = %s
                """
                size_result = self.execute_query(size_query, (db_name,))
                size_bytes = size_result[0]['size'] if size_result and size_result[0]['size'] else 0
                databases.append({
                    'name': db_name,
                    'size': format_size(size_bytes),
                    'size_bytes': size_bytes,
                    'encoding': row['encoding'],
                    'collation': row['collation'],
                    'schema_count': 1,
                    'table_count': 0,
                    'view_count': 0,
                    'trigger_count': 0,
                    'is_backup': False
                })
            return databases
        except Exception:
            return []

    def _get_mysql_tables(self, db_name: str) -> List[Dict]:
        """获取指定 MySQL 数据库的表信息"""
        try:
            query = """
                SELECT
                    table_name,
                    engine,
                    table_rows,
                    data_length,
                    index_length,
                    data_length + index_length as size_bytes
                FROM information_schema.tables
                WHERE table_schema = %s
                ORDER BY data_length + index_length DESC
            """
            rows = self.execute_query(query, (db_name,))
            tables = []
            for row in rows:
                size_bytes = row['size_bytes'] or 0
                tables.append({
                    'database': db_name,
                    'schema': db_name,
                    'table_name': row['table_name'],
                    'size': format_size(size_bytes),
                    'size_bytes': size_bytes,
                    'column_count': 0,
                    'row_count': row['table_rows'] or 0,
                    'is_backup': False
                })
            return tables
        except Exception:
            return []

    def _get_mysql_tables_without_pk(self, db_name: str) -> List[Dict]:
        """获取指定 MySQL 数据库中缺少主键的表"""
        try:
            query = """
                SELECT t.table_name,
                       t.table_rows,
                       t.data_length + t.index_length as size_bytes
                FROM information_schema.tables t
                WHERE t.table_schema = %s
                  AND t.table_type = 'BASE TABLE'
                  AND NOT EXISTS (
                      SELECT 1 FROM information_schema.table_constraints tc
                      WHERE tc.table_schema = t.table_schema
                        AND tc.table_name = t.table_name
                        AND tc.constraint_type = 'PRIMARY KEY'
                  )
                ORDER BY t.data_length + t.index_length DESC
            """
            rows = self.execute_query(query, (db_name,))
            tables = []
            for row in rows:
                size_bytes = row['size_bytes'] or 0
                tables.append({
                    'schema': db_name,
                    'table_name': row['table_name'],
                    'size': format_size(size_bytes),
                    'size_bytes': size_bytes,
                    'column_count': 'N/A',
                    'row_count': row['table_rows'] or 0
                })
            return tables
        except Exception:
            return []

    def _get_mysql_uptime(self) -> str:
        """获取 MySQL 运行时间"""
        try:
            result = self.execute_query("SHOW STATUS LIKE 'Uptime'")
            if result:
                uptime_seconds = int(result[0]['Value'])
                days = uptime_seconds // 86400
                hours = (uptime_seconds % 86400) // 3600
                minutes = (uptime_seconds % 3600) // 60
                if days > 0:
                    return f"{days}天 {hours}小时 {minutes}分钟"
                else:
                    return f"{hours}小时 {minutes}分钟"
        except Exception:
            pass
        return '未知'

    def _get_mysql_settings(self) -> Dict[str, Any]:
        """获取 MySQL 关键配置（使用与 PG 一致的 settings 键名）"""
        try:
            mysql_vars = {
                'max_connections': 'N/A',
                'shared_buffers': 'N/A',
                'work_mem': 'N/A',
                'maintenance_work_mem': 'N/A',
                'effective_cache_size': 'N/A',
                'wal_level': 'N/A',
                'max_wal_size': 'N/A',
                'min_wal_size': 'N/A',
                'wal_buffers': 'N/A',
                'checkpoint_completion_target': 'N/A',
                'checkpoint_timeout': 'N/A',
                'autovacuum': 'N/A',
                'autovacuum_max_workers': 'N/A',
                'autovacuum_naptime': 'N/A',
                'random_page_cost': 'N/A',
                'default_statistics_target': 'N/A',
                'effective_io_concurrency': 'N/A',
                'max_parallel_workers_per_gather': 'N/A',
                'max_parallel_workers': 'N/A',
                'logging_collector': 'N/A',
                'log_statement': 'N/A',
                'log_min_duration_statement': 'N/A',
                'timezone': 'N/A',
                'max_locks_per_transaction': 'N/A',
                'enable_case_sensitive': 'N/A',
            }

            var_mapping = {
                'max_connections': 'max_connections',
                'shared_buffers': 'innodb_buffer_pool_size',
                'work_mem': 'sort_buffer_size',
                'maintenance_work_mem': 'myisam_sort_buffer_size',
                'effective_cache_size': 'innodb_buffer_pool_size',
                'wal_level': 'binlog_format',
                'max_wal_size': 'innodb_log_file_size',
                'min_wal_size': 'innodb_log_buffer_size',
                'wal_buffers': 'innodb_log_buffer_size',
                'checkpoint_completion_target': 'innodb_io_capacity',
                'checkpoint_timeout': 'innodb_flush_log_at_timeout',
                'autovacuum': 'event_scheduler',
                'autovacuum_max_workers': 'innodb_purge_threads',
                'autovacuum_naptime': 'N/A',
                'random_page_cost': 'N/A',
                'default_statistics_target': 'N/A',
                'effective_io_concurrency': 'innodb_read_io_threads',
                'max_parallel_workers_per_gather': 'N/A',
                'max_parallel_workers': 'N/A',
                'logging_collector': 'log_error',
                'log_statement': 'general_log',
                'log_min_duration_statement': 'long_query_time',
                'timezone': 'time_zone',
                'max_locks_per_transaction': 'innodb_lock_wait_timeout',
            }

            for pg_key, mysql_var in var_mapping.items():
                if mysql_var == 'N/A':
                    continue
                try:
                    res = self.execute_query(f"SHOW VARIABLES LIKE '{mysql_var}'")
                    if res:
                        mysql_vars[pg_key] = res[0]['Value']
                except Exception:
                    pass

            try:
                res = self.execute_query("SHOW VARIABLES LIKE 'lower_case_table_names'")
                if res:
                    val = res[0]['Value']
                    mysql_vars['enable_case_sensitive'] = '忽略大小写' if val == '1' else '区分大小写'
            except Exception:
                pass

            return mysql_vars
        except Exception:
            return {}
