from abc import ABC, abstractmethod
from typing import Dict, List, Any, Callable, Optional
from ..connection import DatabaseConnection


class BaseInspector(ABC):

    name: str = ''
    title: str = ''

    def __init__(self, connection: DatabaseConnection, config: Dict[str, Any],
                 connection_factory: Optional[Callable] = None):
        self.connection = connection
        self.config = config
        self.results = {}
        self._connection_factory = connection_factory

    def _create_connection(self, db_config: Dict[str, Any]) -> DatabaseConnection:
        if self._connection_factory is not None:
            return self._connection_factory(db_config)
        from ..connection import create_connection
        return create_connection(db_config)

    @abstractmethod
    def inspect(self) -> Dict[str, Any]:
        pass

    def execute_query(self, query: str, params: tuple = ()) -> List[Dict]:
        return self.connection.execute_query(query, params)
