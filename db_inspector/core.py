import yaml
import sys
import os
from typing import Dict, Any, List, Optional
from datetime import datetime
from colorama import init, Fore, Style

from .connection import create_connection
from .inspectors import get_inspectors
from .reporters import create_reporter


def _ensure_stdout_encoding():
    if hasattr(sys.stdout, 'reconfigure'):
        try:
            sys.stdout.reconfigure(encoding='utf-8')
        except Exception:
            pass


_ensure_stdout_encoding()
init()


def load_config_file(path: str) -> str:
    """加载配置文件内容，支持从 zipapp 内部读取"""
    # 首先尝试作为普通文件打开
    if os.path.exists(path):
        with open(path, 'r', encoding='utf-8') as f:
            return f.read()
    
    # 如果在 zipapp 中运行，尝试从 zipapp 内部读取
    try:
        # 获取当前文件所在目录
        current_file = os.path.abspath(__file__)
        current_dir = os.path.dirname(current_file)
        
        # 检查是否在 zipapp 中（路径包含 .pyz）
        if '.pyz' in current_dir:
            # 找到 zipapp 文件路径
            zipapp_path = current_dir
            while '.pyz' in zipapp_path and not zipapp_path.endswith('.pyz'):
                zipapp_path = os.path.dirname(zipapp_path)
            
            # 使用 zipfile 从 zipapp 中读取文件
            import zipfile
            with zipfile.ZipFile(zipapp_path, 'r') as zf:
                if path in zf.namelist():
                    return zf.read(path).decode('utf-8')
    except Exception:
        pass
    
    # 如果都失败了，抛出文件不存在错误
    raise FileNotFoundError(f"配置文件不存在: {path}")


class DBInspector:
    """数据库巡检核心类"""
    
    def __init__(self, config_path: str = 'config.yaml', databases_config: Optional[List[Dict[str, Any]]] = None):
        self.config = self._load_config(config_path)
        
        # 如果通过参数传入了数据库配置，使用传入的配置
        if databases_config is not None:
            self.config['databases'] = databases_config
        else:
            # 从配置文件读取数据库配置，但给出安全提示
            if 'databases' in self.config and self.config['databases']:
                print("警告: 从配置文件读取数据库配置，不建议在生产环境使用！")
                print("建议通过命令行参数传递数据库配置以提高安全性。")
        
        self.results = {}
    
    def _load_config(self, path: str) -> Dict[str, Any]:
        """加载配置文件"""
        content = load_config_file(path)
        return yaml.safe_load(content)
    
    def inspect_all(self):
        """巡检所有配置的数据库"""
        databases = self.config.get('databases', [])
        inspection_config = self.config.get('inspection', {})
        
        if not databases:
            print("错误: 未配置任何数据库！")
            print("请通过命令行参数传递数据库配置，或在配置文件中配置。")
            sys.exit(1)
        
        total = len(databases)
        print(f"\n{'#'*60}")
        print(f"# 数据库巡检开始")
        print(f"# 数据库数量: {total}")
        print(f"# 开始时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"{'#'*60}\n")
        sys.stdout.flush()
        
        for idx, db_config in enumerate(databases, 1):
            print(f"\n{'='*60}")
            print(f"📊 [{idx}/{total}] 开始巡检: {db_config.get('name', 'Unknown')}")
            print(f"   类型: {db_config.get('type', 'Unknown')}")
            print(f"   地址: {db_config.get('host', 'Unknown')}:{db_config.get('port', 'Unknown')}")
            print(f"{'='*60}")
            sys.stdout.flush()
            
            start_time = datetime.now()
            
            try:
                result = self.inspect_database(db_config, inspection_config)
                self.results[db_config['name']] = result
                
                # 生成报告
                self._generate_report(db_config, result)
                
                elapsed = (datetime.now() - start_time).total_seconds()
                print(f"\n  ✓ 巡检成功完成,耗时: {elapsed:.1f}秒")
                
            except Exception as e:
                elapsed = (datetime.now() - start_time).total_seconds()
                print(f"\n  ✗ 巡检失败,耗时: {elapsed:.1f}秒")
                print(f"  错误: {str(e)}")
                import traceback
                traceback.print_exc()
                self.results[db_config['name']] = {'error': str(e)}
            
            sys.stdout.flush()
        
        # 打印总结
        end_time = datetime.now()
        total_time = (end_time - start_time).total_seconds() if 'start_time' in locals() else 0
        
        print(f"\n{'#'*60}")
        print(f"# 巡检完成摘要")
        print(f"# 结束时间: {end_time.strftime('%Y-%m-%d %H:%M:%S')}")
        print(f"# 总耗时: {total_time:.1f}秒")
        print(f"{'#'*60}\n")
        sys.stdout.flush()
    
    def inspect_database(self, db_config: Dict[str, Any], inspection_config: Dict[str, Any]) -> Dict[str, Any]:
        """巡检单个数据库"""
        result = {}
        checks = inspection_config.get('checks', {})
        
        config = {**inspection_config, **db_config}

        with create_connection(db_config) as conn:
            inspectors = get_inspectors(conn, config)
            enabled = [ins for ins in inspectors if checks.get(ins.name, True)]
            step = 1
            total = len(enabled)

            for inspector in inspectors:
                key = inspector.name
                if not checks.get(key, True):
                    continue

                print(f"\n  {'='*50}")
                print(f"  [{step}/{total}] {inspector.title}...")
                print(f"  {'='*50}")
                sys.stdout.flush()

                result[key] = inspector.inspect()
                step += 1
        
        print(f"\n  {'='*50}")
        print(f"  [OK] 数据库巡检完成")
        print(f"  {'='*50}\n")
        sys.stdout.flush()
        return result
    
    def _generate_report(self, db_config: Dict[str, Any], result: Dict[str, Any]):
        """生成报告"""
        report_config = self.config.get('report', {})
        output_dir = report_config.get('output_dir', './reports')
        format_type = report_config.get('format', 'html')
        
        print(f"\n  {'─'*50}")
        print(f"  生成报告...")
        print(f"  {'─'*50}")
        print(f"    → 格式: {format_type.upper()}")
        print(f"    → 目录: {output_dir}")
        sys.stdout.flush()
        
        start_time = datetime.now()
        
        reporter = create_reporter(format_type, output_dir)
        
        print(f"    → 计算健康评分和关键发现...")
        sys.stdout.flush()
        
        filepath = reporter.generate(db_config, result)
        
        elapsed = (datetime.now() - start_time).total_seconds()
        file_size = os.path.getsize(filepath) if os.path.exists(filepath) else 0
        file_size_str = f"{file_size/1024:.1f}KB" if file_size > 0 else "未知"
        
        abs_filepath = os.path.abspath(filepath)
        print(f"    ✓ 报告已生成: {abs_filepath}")
        print(f"    ✓ 文件大小: {file_size_str}, 耗时: {elapsed:.1f}秒")
        sys.stdout.flush()
    
    def print_summary(self):
        """打印巡检摘要"""
        print(f"\n{'='*60}")
        print("巡检摘要")
        print(f"{'='*60}")
        
        for db_name, result in self.results.items():
            if 'error' in result:
                status = "[失败]"
            else:
                status = "[成功]"
            
            print(f"{db_name}: {status}")
            
            # 显示数据库统计信息
            if 'basic_info' in result:
                basic_info = result['basic_info']
                if 'databases' in basic_info:
                    db_stats = basic_info['databases']
                    print(f"  - 数据库总数: {db_stats.get('total', 0)}")
                    print(f"  - 疑似备份库: {db_stats.get('backup_count', 0)}")
                if 'tables' in basic_info:
                    table_stats = basic_info['tables']
                    print(f"  - 表总数: {table_stats.get('total_count', 0)}")
                    print(f"  - 疑似备份表: {table_stats.get('backup_count', 0)}")
