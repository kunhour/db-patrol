import pymysql
import psycopg2
from abc import ABC, abstractmethod
from typing import Dict, List, Any, Optional
from contextlib import contextmanager


class DatabaseConnection(ABC):
    """数据库连接抽象基类"""
    
    def __init__(self, config: Dict[str, Any]):
        self.config = config
        self.connection = None
        self.cursor = None
    
    @abstractmethod
    def connect(self):
        """建立数据库连接"""
        pass
    
    @abstractmethod
    def execute_query(self, query: str, params: tuple = ()) -> List[Dict]:
        """执行查询并返回结果"""
        pass
    
    @abstractmethod
    def execute(self, query: str, params: tuple = ()):
        """执行SQL语句"""
        pass
    
    def close(self):
        """关闭连接"""
        if self.cursor:
            self.cursor.close()
        if self.connection:
            self.connection.close()
    
    def __enter__(self):
        self.connect()
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()


class VastbasePGConnection(DatabaseConnection):
    """Vastbase PG 模式连接"""
    
    def connect(self):
        self.connection = psycopg2.connect(
            host=self.config['host'],
            port=self.config.get('port', 5432),
            user=self.config['user'],
            password=self.config['password'],
            database=self.config['database']
        )
        self.connection.autocommit = True
        self.cursor = self.connection.cursor()
    
    def execute_query(self, query: str, params: tuple = ()) -> List[Dict]:
        self.cursor.execute(query, params)
        columns = [desc[0] for desc in self.cursor.description] if self.cursor.description else []
        rows = self.cursor.fetchall()
        return [dict(zip(columns, row)) for row in rows]
    
    def execute(self, query: str, params: tuple = ()):
        self.cursor.execute(query, params)


class VastbaseMySQLConnection(DatabaseConnection):
    """Vastbase MySQL 模式连接"""
    
    def connect(self):
        self.connection = pymysql.connect(
            host=self.config['host'],
            port=self.config.get('port', 3306),
            user=self.config['user'],
            password=self.config['password'],
            database=self.config['database'],
            charset=self.config.get('charset', 'utf8mb4'),
            cursorclass=pymysql.cursors.DictCursor
        )
        self.cursor = self.connection.cursor()
    
    def execute_query(self, query: str, params: tuple = ()) -> List[Dict]:
        self.cursor.execute(query, params)
        return self.cursor.fetchall()
    
    def execute(self, query: str, params: tuple = ()):
        self.cursor.execute(query, params)


def create_connection(config: Dict[str, Any]) -> DatabaseConnection:
    """根据配置创建对应的数据库连接"""
    db_type = config['type'].lower()
    
    if db_type == 'vastbase_pg':
        return VastbasePGConnection(config)
    elif db_type == 'vastbase_mysql':
        return VastbaseMySQLConnection(config)
    elif db_type == 'mysql':
        return VastbaseMySQLConnection(config)
    elif db_type == 'postgresql':
        return VastbasePGConnection(config)
    else:
        raise ValueError(f"不支持的数据库类型: {db_type}")
