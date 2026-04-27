import os
from abc import ABC, abstractmethod
from typing import Dict, Any


class BaseReporter(ABC):

    def __init__(self, output_dir: str = './reports'):
        self.output_dir = output_dir
        os.makedirs(output_dir, exist_ok=True)

    @abstractmethod
    def generate(self, db_config: Dict[str, Any], results: Dict[str, Any]) -> str:
        ...
