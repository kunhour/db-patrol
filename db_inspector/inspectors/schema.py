from typing import Dict, Any, List, Optional, Callable
from .base import BaseInspector


class SchemaInspector(BaseInspector):

    name = 'schema'
    title = '检查设计规范'

    def inspect(self) -> Dict[str, Any]:
        """检查数据库设计规范"""
        db_type = self.connection.config['type']
        
        if 'pg' in db_type.lower() or 'postgres' in db_type.lower():
            return self._inspect_pg()
        else:
            return self._inspect_mysql()
    
    def _inspect_pg(self) -> Dict[str, Any]:
        """Vastbase PG 模式设计规范检查"""
        return {
            'table_naming': self._check_pg_table_naming(),
            'column_naming': self._check_pg_column_naming(),
            'primary_keys': self._check_pg_primary_keys(),
            'indexes': self._check_pg_indexes(),
            'constraints': self._check_pg_constraints(),
            'data_types': self._check_pg_data_types(),
            'comments': self._check_pg_comments(),
            'large_tables': self._check_pg_large_tables()
        }
    
    def _inspect_mysql(self) -> Dict[str, Any]:
        """Vastbase MySQL 模式设计规范检查"""
        return {
            'table_naming': self._check_mysql_table_naming(),
            'column_naming': self._check_mysql_column_naming(),
            'primary_keys': self._check_mysql_primary_keys(),
            'indexes': self._check_mysql_indexes(),
            'constraints': self._check_mysql_constraints(),
            'data_types': self._check_mysql_data_types(),
            'comments': self._check_mysql_comments(),
            'large_tables': self._check_mysql_large_tables(),
            'engine_charset': self._check_mysql_engine_charset()
        }
    
    def _check_pg_table_naming(self) -> List[Dict]:
        """检查 PG 表命名规范"""
        issues = []
        try:
            query = """
                SELECT schemaname, tablename 
                FROM pg_tables 
                WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
            """
            tables = self.execute_query(query)
            
            for table in tables:
                name = table['tablename']
                # 检查是否小写
                if name != name.lower():
                    issues.append({
                        'table': name,
                        'issue': '表名包含大写字母',
                        'suggestion': '建议使用小写字母和下划线'
                    })
                # 检查是否包含非法字符
                if '-' in name or ' ' in name:
                    issues.append({
                        'table': name,
                        'issue': '表名包含非法字符',
                        'suggestion': '建议使用小写字母和下划线'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_column_naming(self) -> List[Dict]:
        """检查 PG 列命名规范"""
        issues = []
        try:
            query = """
                SELECT table_name, column_name, data_type
                FROM information_schema.columns
                WHERE table_schema = 'public'
            """
            columns = self.execute_query(query)
            
            for col in columns:
                name = col['column_name']
                # 检查是否小写
                if name != name.lower():
                    issues.append({
                        'table': col['table_name'],
                        'column': name,
                        'issue': '列名包含大写字母',
                        'suggestion': '建议使用小写字母和下划线'
                    })
                # 检查是否以数字开头
                if name[0].isdigit():
                    issues.append({
                        'table': col['table_name'],
                        'column': name,
                        'issue': '列名以数字开头',
                        'suggestion': '列名应以字母开头'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_primary_keys(self) -> List[Dict]:
        """检查 PG 主键规范"""
        issues = []
        try:
            query = """
                SELECT tc.table_name, kcu.column_name
                FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu
                    ON tc.constraint_name = kcu.constraint_name
                WHERE tc.constraint_type = 'PRIMARY KEY'
                    AND tc.table_schema = 'public'
            """
            pks = self.execute_query(query)
            
            # 检查没有主键的表
            tables_query = """
                SELECT tablename FROM pg_tables 
                WHERE schemaname = 'public'
            """
            tables = self.execute_query(tables_query)
            tables_with_pk = {pk['table_name'] for pk in pks}
            
            for table in tables:
                if table['tablename'] not in tables_with_pk:
                    issues.append({
                        'table': table['tablename'],
                        'issue': '表缺少主键',
                        'suggestion': '建议为每个表添加主键'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_indexes(self) -> List[Dict]:
        """检查 PG 索引规范"""
        issues = []
        try:
            query = """
                SELECT schemaname, tablename, indexname
                FROM pg_indexes
                WHERE schemaname = 'public'
            """
            indexes = self.execute_query(query)
            
            for idx in indexes:
                name = idx['indexname']
                # 检查索引命名规范
                if not (name.startswith('idx_') or name.startswith('pk_') or 
                        name.startswith('fk_') or name.startswith('uq_')):
                    if 'pkey' not in name:  # 排除主键索引
                        issues.append({
                            'table': idx['tablename'],
                            'index': name,
                            'issue': '索引命名不规范',
                            'suggestion': '建议以 idx_, pk_, fk_, uq_ 开头'
                        })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_constraints(self) -> List[Dict]:
        """检查 PG 约束规范"""
        issues = []
        try:
            # 检查外键约束
            query = """
                SELECT tc.table_name, tc.constraint_name, kcu.column_name,
                       ccu.table_name AS foreign_table_name,
                       ccu.column_name AS foreign_column_name
                FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu
                    ON tc.constraint_name = kcu.constraint_name
                JOIN information_schema.constraint_column_usage ccu
                    ON ccu.constraint_name = tc.constraint_name
                WHERE tc.constraint_type = 'FOREIGN KEY'
                    AND tc.table_schema = 'public'
            """
            fks = self.execute_query(query)
            
            # 检查是否有外键索引
            for fk in fks:
                issues.append({
                    'table': fk['table_name'],
                    'column': fk['column_name'],
                    'constraint': fk['constraint_name'],
                    'issue': '存在外键约束',
                    'suggestion': '确保外键列上有索引以提高性能'
                })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_data_types(self) -> List[Dict]:
        """检查 PG 数据类型规范"""
        issues = []
        try:
            query = """
                SELECT table_name, column_name, data_type, character_maximum_length
                FROM information_schema.columns
                WHERE table_schema = 'public'
            """
            columns = self.execute_query(query)
            
            for col in columns:
                data_type = col['data_type']
                # 检查是否使用 varchar 而没有指定长度
                if data_type == 'character varying' and col['character_maximum_length'] is None:
                    issues.append({
                        'table': col['table_name'],
                        'column': col['column_name'],
                        'issue': '使用 varchar 未指定长度',
                        'suggestion': '建议指定 varchar 长度或使用 text'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_comments(self) -> List[Dict]:
        """检查 PG 注释规范"""
        issues = []
        try:
            # 检查表注释
            query = """
                SELECT c.relname as table_name, obj_description(c.oid) as comment
                FROM pg_class c
                JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE c.relkind = 'r' AND n.nspname = 'public'
            """
            tables = self.execute_query(query)
            
            for table in tables:
                if not table['comment']:
                    issues.append({
                        'table': table['table_name'],
                        'issue': '表缺少注释',
                        'suggestion': '建议为表添加注释说明'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_pg_large_tables(self) -> List[Dict]:
        """检查 PG 大表"""
        issues = []
        try:
            threshold = self.config.get('table_size_threshold', 1024) * 1024 * 1024  # MB to bytes
            query = """
                SELECT schemaname, relname as table_name,
                       pg_total_relation_size(relid) as total_size
                FROM pg_stat_user_tables
                WHERE pg_total_relation_size(relid) > %s
                ORDER BY pg_total_relation_size(relid) DESC
            """
            tables = self.execute_query(query, (threshold,))
            
            for table in tables:
                size_mb = table['total_size'] / (1024 * 1024)
                issues.append({
                    'table': table['table_name'],
                    'issue': f'表过大 ({size_mb:.2f} MB)',
                    'suggestion': '考虑分区或归档历史数据'
                })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_table_naming(self) -> List[Dict]:
        """检查 MySQL 表命名规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name FROM information_schema.tables
                WHERE table_schema = %s
            """
            tables = self.execute_query(query, (db_name,))
            
            for table in tables:
                name = table['table_name']
                # 检查是否小写
                if name != name.lower():
                    issues.append({
                        'table': name,
                        'issue': '表名包含大写字母',
                        'suggestion': '建议使用小写字母和下划线'
                    })
                # 检查前缀
                if name.startswith('tb_') or name.startswith('t_'):
                    issues.append({
                        'table': name,
                        'issue': '表名使用冗余前缀',
                        'suggestion': '建议直接使用名词，不加 tb_ 或 t_ 前缀'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_column_naming(self) -> List[Dict]:
        """检查 MySQL 列命名规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name, column_name, data_type
                FROM information_schema.columns
                WHERE table_schema = %s
            """
            columns = self.execute_query(query, (db_name,))
            
            for col in columns:
                name = col['column_name']
                # 检查是否小写
                if name != name.lower():
                    issues.append({
                        'table': col['table_name'],
                        'column': name,
                        'issue': '列名包含大写字母',
                        'suggestion': '建议使用小写字母和下划线'
                    })
                # 检查保留字
                reserved_words = ['select', 'insert', 'update', 'delete', 'order', 'group']
                if name.lower() in reserved_words:
                    issues.append({
                        'table': col['table_name'],
                        'column': name,
                        'issue': '列名使用保留字',
                        'suggestion': f'避免使用 {name} 作为列名'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_primary_keys(self) -> List[Dict]:
        """检查 MySQL 主键规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name FROM information_schema.tables
                WHERE table_schema = %s
            """
            tables = self.execute_query(query, (db_name,))
            
            for table in tables:
                table_name = table['table_name']
                pk_query = """
                    SELECT column_name FROM information_schema.key_column_usage
                    WHERE table_schema = %s AND table_name = %s
                    AND constraint_name = 'PRIMARY'
                """
                pk = self.execute_query(pk_query, (db_name, table_name))
                
                if not pk:
                    issues.append({
                        'table': table_name,
                        'issue': '表缺少主键',
                        'suggestion': '建议为每个表添加主键'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_indexes(self) -> List[Dict]:
        """检查 MySQL 索引规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name, index_name, column_name
                FROM information_schema.statistics
                WHERE table_schema = %s
            """
            indexes = self.execute_query(query, (db_name,))
            
            # 检查重复索引
            index_dict = {}
            for idx in indexes:
                key = (idx['table_name'], idx['index_name'])
                if key not in index_dict:
                    index_dict[key] = []
                index_dict[key].append(idx['column_name'])
            
            # 简化检查：查找可能的重复索引
            seen = {}
            for (table, index), columns in index_dict.items():
                cols_str = ','.join(columns)
                if (table, cols_str) in seen:
                    issues.append({
                        'table': table,
                        'index': index,
                        'issue': f'可能存在重复索引: {seen[(table, cols_str)]}',
                        'suggestion': '检查并删除重复索引'
                    })
                else:
                    seen[(table, cols_str)] = index
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_constraints(self) -> List[Dict]:
        """检查 MySQL 约束规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            # 检查外键
            query = """
                SELECT table_name, constraint_name
                FROM information_schema.table_constraints
                WHERE table_schema = %s AND constraint_type = 'FOREIGN KEY'
            """
            fks = self.execute_query(query, (db_name,))
            
            for fk in fks:
                issues.append({
                    'table': fk['table_name'],
                    'constraint': fk['constraint_name'],
                    'issue': '存在外键约束',
                    'suggestion': '确保外键列上有索引以提高性能'
                })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_data_types(self) -> List[Dict]:
        """检查 MySQL 数据类型规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name, column_name, data_type, column_type
                FROM information_schema.columns
                WHERE table_schema = %s
            """
            columns = self.execute_query(query, (db_name,))
            
            for col in columns:
                data_type = col['data_type']
                # 检查是否使用 float/double 存储金额
                if data_type in ['float', 'double']:
                    issues.append({
                        'table': col['table_name'],
                        'column': col['column_name'],
                        'issue': f'使用 {data_type} 存储浮点数',
                        'suggestion': '金额类数据建议使用 DECIMAL'
                    })
                # 检查是否使用 timestamp
                if data_type == 'timestamp':
                    issues.append({
                        'table': col['table_name'],
                        'column': col['column_name'],
                        'issue': '使用 TIMESTAMP 类型',
                        'suggestion': '注意 TIMESTAMP 的时间范围限制 (1970-2038)'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_comments(self) -> List[Dict]:
        """检查 MySQL 注释规范"""
        issues = []
        try:
            db_name = self.connection.config['database']
            # 检查表注释
            query = """
                SELECT table_name, table_comment
                FROM information_schema.tables
                WHERE table_schema = %s
            """
            tables = self.execute_query(query, (db_name,))
            
            for table in tables:
                if not table['table_comment']:
                    issues.append({
                        'table': table['table_name'],
                        'issue': '表缺少注释',
                        'suggestion': '建议为表添加注释说明'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_large_tables(self) -> List[Dict]:
        """检查 MySQL 大表"""
        issues = []
        try:
            db_name = self.connection.config['database']
            threshold = self.config.get('table_size_threshold', 1024) * 1024 * 1024  # MB to bytes
            query = """
                SELECT table_name, 
                       data_length + index_length as total_size,
                       table_rows
                FROM information_schema.tables
                WHERE table_schema = %s
                AND data_length + index_length > %s
                ORDER BY data_length + index_length DESC
            """
            tables = self.execute_query(query, (db_name, threshold))
            
            for table in tables:
                size_mb = table['total_size'] / (1024 * 1024)
                issues.append({
                    'table': table['table_name'],
                    'issue': f'表过大 ({size_mb:.2f} MB, {table["table_rows"]} 行)',
                    'suggestion': '考虑分区或归档历史数据'
                })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
    
    def _check_mysql_engine_charset(self) -> List[Dict]:
        """检查 MySQL 存储引擎和字符集"""
        issues = []
        try:
            db_name = self.connection.config['database']
            query = """
                SELECT table_name, engine, table_collation
                FROM information_schema.tables
                WHERE table_schema = %s
            """
            tables = self.execute_query(query, (db_name,))
            
            for table in tables:
                engine = table['engine']
                collation = table['table_collation']
                
                # 检查存储引擎
                if engine and engine.lower() != 'innodb':
                    issues.append({
                        'table': table['table_name'],
                        'issue': f'使用 {engine} 引擎',
                        'suggestion': '建议使用 InnoDB 引擎以获得更好的事务支持'
                    })
                
                # 检查字符集
                if collation and 'utf8mb4' not in collation:
                    issues.append({
                        'table': table['table_name'],
                        'issue': f'字符集为 {collation}',
                        'suggestion': '建议使用 utf8mb4 以支持完整的 Unicode'
                    })
        except Exception as e:
            issues.append({'error': str(e)})
        
        return issues
