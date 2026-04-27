from typing import Dict, Any
from .base import BaseReporter


_REPORTER_REGISTRY: Dict[str, type] = {}


def register_reporter(format_type: str, reporter_class: type = None):

    def decorator(cls):
        _REPORTER_REGISTRY[format_type] = cls
        return cls

    if reporter_class is not None:
        _REPORTER_REGISTRY[format_type] = reporter_class
        return reporter_class

    return decorator


def create_reporter(format_type: str, output_dir: str = './reports') -> BaseReporter:
    from .html_reporter import HTMLReporter
    from .markdown_reporter import MarkdownReporter
    from .json_reporter import JSONReporter

    if format_type not in _REPORTER_REGISTRY:
        _REPORTER_REGISTRY['html'] = HTMLReporter
        _REPORTER_REGISTRY['markdown'] = MarkdownReporter
        _REPORTER_REGISTRY['json'] = JSONReporter

    reporter_class = _REPORTER_REGISTRY.get(format_type)
    if reporter_class is None:
        raise ValueError(f"Unsupported report format: {format_type}. Supported: {list(_REPORTER_REGISTRY.keys())}")

    return reporter_class(output_dir)
