from typing import Optional


def format_size(size_bytes: Optional[int]) -> str:
    if size_bytes is None or size_bytes == 0:
        return '0 B'

    units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
    unit_index = 0
    size = float(size_bytes)

    while size >= 1024 and unit_index < len(units) - 1:
        size /= 1024
        unit_index += 1

    if unit_index == 0:
        return f'{int(size)} {units[unit_index]}'
    else:
        return f'{size:.2f} {units[unit_index]}'
