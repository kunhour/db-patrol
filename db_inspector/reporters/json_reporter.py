import json
from datetime import datetime
from typing import Dict, Any

from .base import BaseReporter


class JSONReporter(BaseReporter):

    def generate(self, db_config: Dict[str, Any], results: Dict[str, Any]) -> str:
        report = {
            'metadata': {
                'db_name': db_config.get('name', 'Unknown'),
                'db_type': db_config.get('type', 'Unknown'),
                'generated_at': datetime.now().isoformat(),
                'host': db_config.get('host', 'Unknown'),
                'port': db_config.get('port', 'Unknown')
            },
            'results': results
        }

        filename = f"db_inspection_{db_config.get('name', 'report').replace(' ', '_')}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
        filepath = self._write_report(filename, json.dumps(report, ensure_ascii=False, indent=2, default=str))

        return filepath

    def _write_report(self, filename: str, content: str) -> str:
        import os
        filepath = os.path.join(self.output_dir, filename)
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)
        return filepath
