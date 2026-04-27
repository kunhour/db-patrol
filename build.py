#!/usr/bin/env python3
"""
数据库巡检工具打包脚本
创建自包含的可执行 Python zipapp
"""

import subprocess
import sys
import os
import shutil
import zipfile


def clean_build_dirs():
    """清理构建目录"""
    dirs_to_clean = ['build', 'dist', '.shiv', 'db_inspector.egg-info']
    for dir_name in dirs_to_clean:
        if os.path.exists(dir_name):
            print(f"清理目录: {dir_name}")
            shutil.rmtree(dir_name)


def build_zipapp_with_deps():
    """构建包含纯 Python 依赖的 zipapp"""
    print("构建包含依赖的 zipapp...")
    
    build_dir = 'build/zipapp'
    
    # 清理并创建构建目录
    if os.path.exists(build_dir):
        shutil.rmtree(build_dir)
    os.makedirs(build_dir)
    
    # 安装纯 Python 依赖到构建目录
    print("安装纯 Python 依赖...")
    pure_python_deps = [
        'sqlalchemy>=2.0.0',
        'jinja2>=3.1.0',
        'pyyaml>=6.0',
        'click>=8.1.0',
        'tabulate>=0.9.0',
        'colorama>=0.4.6',
        'markupsafe',
        'typing-extensions',
        'greenlet',
    ]
    
    for dep in pure_python_deps:
        try:
            subprocess.check_call([
                sys.executable, '-m', 'pip', 'install',
                dep, '--target', build_dir, '--quiet'
            ])
        except Exception as e:
            print(f"  警告: 安装 {dep} 失败: {e}")
    
    # 复制项目代码
    print("复制项目代码...")
    shutil.copytree('db_inspector', os.path.join(build_dir, 'db_inspector'))
    shutil.copy('main.py', build_dir)
    
    # 复制默认配置文件
    if os.path.exists('config.yaml'):
        shutil.copy('config.yaml', build_dir)
        print("  - 包含默认配置文件: config.yaml")
    
    # 创建 __main__.py
    main_py_content = '''#!/usr/bin/env python3
import sys
import os
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# 检查依赖
def check_dependencies():
    missing = []
    try:
        import pymysql
    except ImportError:
        missing.append('pymysql>=1.1.0')
    try:
        import psycopg2
    except ImportError:
        missing.append('psycopg2-binary>=2.9.9')
    
    if missing:
        print("="*60)
        print("首次运行，需要安装数据库驱动依赖...")
        print("="*60)
        import subprocess
        try:
            subprocess.check_call([sys.executable, '-m', 'pip', 'install'] + missing)
            print("\\n依赖安装完成！重新运行程序...\\n")
            # 重新导入
            for pkg in missing:
                __import__(pkg.split('>=')[0].replace('-', '_'))
        except Exception as e:
            print(f"\\n安装失败: {e}")
            print("请手动安装:")
            for pkg in missing:
                print(f"  pip install {pkg}")
            sys.exit(1)

check_dependencies()

from main import main
if __name__ == '__main__':
    main()
'''
    with open(os.path.join(build_dir, '__main__.py'), 'w', encoding='utf-8') as f:
        f.write(main_py_content)
    
    # 清理不必要的文件以减小体积
    print("清理不必要的文件...")
    
    for root, dirs, files in os.walk(build_dir):
        for d in list(dirs):
            if d in ['__pycache__', 'tests', 'test', 'docs', '.git', 'examples', 'benchmarks']:
                path = os.path.join(root, d)
                try:
                    shutil.rmtree(path)
                    dirs.remove(d)
                except:
                    pass
        for f in files:
            if f.endswith(('.pyc', '.pyo', '.so', '.dylib', '.dll', '.c', '.h')):
                path = os.path.join(root, f)
                try:
                    os.remove(path)
                except:
                    pass
    
    # 删除 .dist-info 和 .egg-info 目录
    for item in list(os.listdir(build_dir)):
        item_path = os.path.join(build_dir, item)
        if os.path.isdir(item_path) and ('.dist-info' in item or '.egg-info' in item):
            shutil.rmtree(item_path)
    
    # 使用 zipapp 打包
    print("创建 zipapp...")
    os.makedirs('dist', exist_ok=True)
    output_file = 'dist/db-inspector.pyz'
    subprocess.check_call([
        sys.executable, '-m', 'zipapp',
        build_dir,
        '-o', output_file,
        '-p', '/usr/bin/env python3',
        '-c'  # 压缩
    ])
    
    # 获取文件大小
    size = os.path.getsize(output_file)
    size_mb = size / (1024 * 1024)
    print(f"构建完成: {output_file} ({size_mb:.2f} MB)")
    
    return output_file


def create_standalone_script():
    """创建独立的启动脚本（备选方案）"""
    print("创建独立启动脚本...")
    
    script_content = '''#!/usr/bin/env python3
"""
数据库巡检工具 - 自包含启动脚本
自动安装依赖并运行
"""

import subprocess
import sys
import os

# 当前脚本所在目录
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

def ensure_dependencies():
    """确保依赖已安装"""
    required = {
        'pymysql': 'pymysql>=1.1.0',
        'psycopg2': 'psycopg2-binary>=2.9.9',
        'sqlalchemy': 'sqlalchemy>=2.0.0',
        'jinja2': 'jinja2>=3.1.0',
        'yaml': 'pyyaml>=6.0',
        'click': 'click>=8.1.0',
        'tabulate': 'tabulate>=0.9.0',
        'colorama': 'colorama>=0.4.6',
    }
    
    missing = []
    for module, package in required.items():
        try:
            __import__(module)
        except ImportError:
            missing.append(package)
    
    if missing:
        print("="*60)
        print("首次运行，正在安装依赖...")
        print("="*60)
        subprocess.check_call([sys.executable, '-m', 'pip', 'install'] + missing)
        print("\\n依赖安装完成！\\n")

def main():
    ensure_dependencies()
    
    # 添加项目路径
    sys.path.insert(0, SCRIPT_DIR)
    
    # 导入并运行
    from main import main as db_main
    db_main()

if __name__ == '__main__':
    main()
'''
    
    with open('dist/db-inspector.py', 'w', encoding='utf-8') as f:
        f.write(script_content)
    
    print("创建: dist/db-inspector.py")


def copy_project_files():
    """复制项目文件到 dist 目录"""
    print("复制项目文件...")
    
    # 复制主程序
    shutil.copy('main.py', 'dist/')
    
    # 复制 db_inspector 包
    if os.path.exists('dist/db_inspector'):
        shutil.rmtree('dist/db_inspector')
    shutil.copytree('db_inspector', 'dist/db_inspector')
    
    # 复制配置文件和文档
    files_to_copy = ['config.yaml', 'README.md', 'README_USER.md', 'requirements.txt']
    for file_name in files_to_copy:
        if os.path.exists(file_name):
            shutil.copy(file_name, 'dist/')
            print(f"  - {file_name}")


def create_bat_wrapper():
    """创建 Windows bat 包装器"""
    bat_content = '''@echo off
chcp 65001 >nul
python "%~dp0db-inspector.py" %*
'''
    with open('dist/db-inspector.bat', 'w', encoding='utf-8') as f:
        f.write(bat_content)
    print("创建: dist/db-inspector.bat")


def create_sh_wrapper():
    """创建 Linux/Mac shell 包装器"""
    sh_content = '''#!/bin/bash
cd "$(dirname "$0")"
python3 db-inspector.py "$@"
'''
    sh_path = 'dist/db-inspector.sh'
    with open(sh_path, 'w', encoding='utf-8') as f:
        f.write(sh_content)
    
    # 尝试设置可执行权限（在 Unix 系统上）
    try:
        os.chmod(sh_path, 0o755)
    except:
        pass
    
    print("创建: dist/db-inspector.sh")


def create_zip_package(zipapp_file):
    """创建最终分发 zip 包"""
    print("创建分发包...")
    
    zip_path = 'dist/db-inspector-package.zip'
    
    with zipfile.ZipFile(zip_path, 'w', zipfile.ZIP_DEFLATED) as zf:
        # 添加 zipapp
        zf.write(zipapp_file, 'db-inspector.pyz')
        
        # 添加包装器
        zf.write('dist/db-inspector.bat', 'db-inspector.bat')
        zf.write('dist/db-inspector.sh', 'db-inspector.sh')
        
        # 添加配置文件
        if os.path.exists('config.yaml'):
            zf.write('config.yaml', 'config.yaml')
            print("  - 包含配置文件: config.yaml")
        
        # 添加用户使用手册
        if os.path.exists('USER_GUIDE.md'):
            zf.write('USER_GUIDE.md', 'USER_GUIDE.md')
            print("  - 包含用户使用手册: USER_GUIDE.md")
    
    size = os.path.getsize(zip_path)
    size_mb = size / (1024 * 1024)
    print(f"分发包: {zip_path} ({size_mb:.2f} MB)")
    
    return zip_path


def show_result():
    """显示打包结果"""
    print("\n" + "="*60)
    print("打包完成!")
    print("="*60)
    
    print("\n分发文件:")
    total_size = 0
    for f in sorted(os.listdir('dist')):
        fpath = os.path.join('dist', f)
        if os.path.isfile(fpath):
            fsize = os.path.getsize(fpath)
            total_size += fsize
            size_str = f"{fsize/1024:.1f} KB" if fsize < 1024*1024 else f"{fsize/(1024*1024):.2f} MB"
            print(f"  - {f} ({size_str})")
    
    print(f"\n总计: {total_size/(1024*1024):.2f} MB")
    
    print("\n使用方法:")
    print("  1. 直接使用 zipapp:")
    print("     python db-inspector.pyz --help")
    print("\n  2. 使用包装器脚本:")
    print("     Windows: db-inspector.bat --help")
    print("     Linux/Mac: ./db-inspector.sh --help")
    
    print("\n注意事项:")
    print("  - 需要 Python 3.8+")
    print("  - 首次运行会自动安装数据库驱动 (pymysql, psycopg2-binary)")
    print("  - 或者提前运行: pip install pymysql psycopg2-binary")


def main_build():
    """主构建流程"""
    print("="*60)
    print("数据库巡检工具打包脚本")
    print("="*60)
    
    # 清理
    clean_build_dirs()
    os.makedirs('dist', exist_ok=True)
    
    # 构建 zipapp（包含纯 Python 依赖）
    zipapp_file = build_zipapp_with_deps()
    
    # 创建包装器
    create_bat_wrapper()
    create_sh_wrapper()
    
    # 创建分发包
    create_zip_package(zipapp_file)
    
    # 显示结果
    show_result()
    
    # 清理中间产物
    if os.path.exists('build'):
        shutil.rmtree('build')
        print("已清理中间产物: build/")


if __name__ == '__main__':
    main_build()
