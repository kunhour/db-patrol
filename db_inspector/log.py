import logging
import sys


class ColoramaStreamHandler(logging.StreamHandler):
    _LEVEL_STYLES = {
        logging.CRITICAL: '\033[91m',
        logging.ERROR: '\033[91m',
        logging.WARNING: '\033[93m',
        logging.INFO: '',
        logging.DEBUG: '\033[90m',
    }
    _RESET = '\033[0m'

    def format(self, record: logging.LogRecord) -> str:
        msg = super().format(record)
        style = self._LEVEL_STYLES.get(record.levelno, '')
        if style:
            return f"{style}{msg}{self._RESET}"
        return msg


def get_logger(name: str = 'db_patrol', level: int = logging.INFO) -> logging.Logger:
    logger = logging.getLogger(name)
    if not logger.handlers:
        handler = ColoramaStreamHandler(sys.stdout)
        handler.setLevel(level)
        handler.setFormatter(logging.Formatter('%(message)s'))
        logger.setLevel(level)
        logger.addHandler(handler)
    return logger
