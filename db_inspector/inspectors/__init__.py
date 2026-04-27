from typing import Dict, Any, List, Type, Optional
from .base import BaseInspector


_INSPECTOR_REGISTRY: List[Type[BaseInspector]] = []


def register_inspector(inspector_class: Type[BaseInspector]) -> Type[BaseInspector]:
    _INSPECTOR_REGISTRY.append(inspector_class)
    return inspector_class


def get_inspectors(connection, config: Dict[str, Any]) -> List[BaseInspector]:
    if not _INSPECTOR_REGISTRY:
        from .basic_info import BasicInfoInspector
        from .performance import PerformanceInspector
        from .schema import SchemaInspector
        _INSPECTOR_REGISTRY.append(BasicInfoInspector)
        _INSPECTOR_REGISTRY.append(PerformanceInspector)
        _INSPECTOR_REGISTRY.append(SchemaInspector)

    return [cls(connection, config) for cls in _INSPECTOR_REGISTRY]
