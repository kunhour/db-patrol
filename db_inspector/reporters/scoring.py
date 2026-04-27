from typing import Dict, Any, List


def calculate_health_score(basic_info: Dict, performance: Dict, databases: Dict, tables: Dict) -> Dict[str, Any]:
    score = 100
    issues = []
    details = []

    connections = performance.get('connections', {})
    if connections and 'usage_percent' in connections:
        usage = connections['usage_percent']
        if usage > 90:
            score -= 15
            issues.append('连接使用率超过90%,存在连接耗尽风险')
            details.append({'name': '连接使用率', 'score': 0, 'max_score': 15, 'status': 'critical', 'detail': f'{usage}%'})
        elif usage > 80:
            score -= 10
            issues.append('连接使用率超过80%,需要关注')
            details.append({'name': '连接使用率', 'score': 5, 'max_score': 15, 'status': 'warning', 'detail': f'{usage}%'})
        elif usage > 60:
            score -= 5
            details.append({'name': '连接使用率', 'score': 10, 'max_score': 15, 'status': 'good', 'detail': f'{usage}%'})
        else:
            details.append({'name': '连接使用率', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': f'{usage}%'})
    else:
        details.append({'name': '连接使用率', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': '数据不足'})

    cache_hit = performance.get('cache_hit_ratio', {})
    if cache_hit and 'ratio' in cache_hit:
        ratio = cache_hit['ratio']
        if ratio < 90:
            score -= 20
            issues.append('缓存命中率低于90%,严重影响性能')
            details.append({'name': '缓存命中率', 'score': 0, 'max_score': 20, 'status': 'critical', 'detail': f'{ratio}%'})
        elif ratio < 95:
            score -= 15
            issues.append('缓存命中率低于95%,建议增加shared_buffers')
            details.append({'name': '缓存命中率', 'score': 5, 'max_score': 20, 'status': 'warning', 'detail': f'{ratio}%'})
        elif ratio < 99:
            score -= 5
            details.append({'name': '缓存命中率', 'score': 15, 'max_score': 20, 'status': 'good', 'detail': f'{ratio}%'})
        else:
            details.append({'name': '缓存命中率', 'score': 20, 'max_score': 20, 'status': 'excellent', 'detail': f'{ratio}%'})
    else:
        details.append({'name': '缓存命中率', 'score': 20, 'max_score': 20, 'status': 'excellent', 'detail': '数据不足'})

    index_hit = performance.get('index_hit_ratio', {})
    if index_hit and 'ratio' in index_hit:
        ratio = index_hit['ratio']
        if ratio < 50:
            score -= 15
            issues.append('索引命中率低于50%,大量查询未使用索引')
            details.append({'name': '索引命中率', 'score': 0, 'max_score': 15, 'status': 'critical', 'detail': f'{ratio}%'})
        elif ratio < 70:
            score -= 10
            issues.append('索引命中率低于70%,建议检查缺失索引')
            details.append({'name': '索引命中率', 'score': 5, 'max_score': 15, 'status': 'warning', 'detail': f'{ratio}%'})
        elif ratio < 90:
            score -= 5
            details.append({'name': '索引命中率', 'score': 10, 'max_score': 15, 'status': 'good', 'detail': f'{ratio}%'})
        else:
            details.append({'name': '索引命中率', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': f'{ratio}%'})
    else:
        details.append({'name': '索引命中率', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': '数据不足'})

    tables_without_pk = basic_info.get('tables_without_pk', {})
    if tables_without_pk:
        total_tables_without_pk = sum(len(tables) for tables in tables_without_pk.values())
        if total_tables_without_pk > 20:
            score -= 10
            issues.append(f'发现{total_tables_without_pk}个表缺少主键,影响数据完整性')
            details.append({'name': '主键完整性', 'score': 0, 'max_score': 10, 'status': 'critical', 'detail': f'{total_tables_without_pk}个表无主键'})
        elif total_tables_without_pk > 10:
            score -= 7
            issues.append(f'发现{total_tables_without_pk}个表缺少主键')
            details.append({'name': '主键完整性', 'score': 3, 'max_score': 10, 'status': 'warning', 'detail': f'{total_tables_without_pk}个表无主键'})
        elif total_tables_without_pk > 0:
            score -= 3
            details.append({'name': '主键完整性', 'score': 7, 'max_score': 10, 'status': 'good', 'detail': f'{total_tables_without_pk}个表无主键'})
        else:
            details.append({'name': '主键完整性', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '全部表都有主键'})
    else:
        details.append({'name': '主键完整性', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '全部表都有主键'})

    backup_dbs = databases.get('backup', [])
    backup_tables = tables.get('backup', [])
    if backup_dbs or backup_tables:
        if len(backup_dbs) > 5 or len(backup_tables) > 20:
            score -= 10
            issues.append(f'发现{len(backup_dbs)}个疑似备份库和{len(backup_tables)}个备份表,建议清理')
            details.append({'name': '备份数据清理', 'score': 0, 'max_score': 10, 'status': 'critical', 'detail': f'{len(backup_dbs)}个备份库, {len(backup_tables)}个备份表'})
        elif len(backup_dbs) > 0 or len(backup_tables) > 0:
            score -= 5
            details.append({'name': '备份数据清理', 'score': 5, 'max_score': 10, 'status': 'warning', 'detail': f'{len(backup_dbs)}个备份库, {len(backup_tables)}个备份表'})
        else:
            details.append({'name': '备份数据清理', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '无备份数据'})
    else:
        details.append({'name': '备份数据清理', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '无备份数据'})

    index_analysis = performance.get('index_size_analysis', {})
    if index_analysis:
        critical_indexes = 0
        for db_tables in index_analysis.values():
            if isinstance(db_tables, list):
                critical_indexes += sum(1 for t in db_tables if t.get('attention') == '严重')

        if critical_indexes > 10:
            score -= 15
            issues.append(f'发现{critical_indexes}个表索引占比严重超标')
            details.append({'name': '索引大小占比', 'score': 0, 'max_score': 15, 'status': 'critical', 'detail': f'{critical_indexes}个表严重超标'})
        elif critical_indexes > 5:
            score -= 10
            details.append({'name': '索引大小占比', 'score': 5, 'max_score': 15, 'status': 'warning', 'detail': f'{critical_indexes}个表严重超标'})
        elif critical_indexes > 0:
            score -= 5
            details.append({'name': '索引大小占比', 'score': 10, 'max_score': 15, 'status': 'good', 'detail': f'{critical_indexes}个表严重超标'})
        else:
            details.append({'name': '索引大小占比', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': '索引占比正常'})
    else:
        details.append({'name': '索引大小占比', 'score': 15, 'max_score': 15, 'status': 'excellent', 'detail': '数据不足'})

    invalid_indexes = performance.get('invalid_indexes', {})
    if invalid_indexes:
        total_invalid = sum(len(indexes) for indexes in invalid_indexes.values() if isinstance(indexes, list))
        if total_invalid > 0:
            score -= 10
            issues.append(f'发现{total_invalid}个无效索引,浪费存储空间')
            details.append({'name': '无效索引', 'score': 0, 'max_score': 10, 'status': 'critical', 'detail': f'{total_invalid}个无效索引'})
        else:
            details.append({'name': '无效索引', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '无无效索引'})
    else:
        details.append({'name': '无效索引', 'score': 10, 'max_score': 10, 'status': 'excellent', 'detail': '无无效索引'})

    duplicate_indexes = performance.get('duplicate_indexes', {})
    if duplicate_indexes:
        total_duplicate = sum(len(indexes) for indexes in duplicate_indexes.values() if isinstance(indexes, list))
        if total_duplicate > 0:
            score -= 5
            issues.append(f'发现{total_duplicate}个重复索引')
            details.append({'name': '重复索引', 'score': 0, 'max_score': 5, 'status': 'warning', 'detail': f'{total_duplicate}个重复索引'})
        else:
            details.append({'name': '重复索引', 'score': 5, 'max_score': 5, 'status': 'excellent', 'detail': '无重复索引'})
    else:
        details.append({'name': '重复索引', 'score': 5, 'max_score': 5, 'status': 'excellent', 'detail': '无重复索引'})

    score = max(0, min(100, score))

    non_perfect_items = [d for d in details if d['status'] in ('warning', 'critical')]
    critical_items = [d for d in details if d['status'] == 'critical']
    warning_items = [d for d in details if d['status'] == 'warning']

    if score >= 90:
        level = 'excellent'
        label = '优秀'
        summary = '数据库运行状态良好,各项指标正常'
    elif score >= 75:
        level = 'good'
        label = '良好'
        summary = '数据库运行状态较好,部分指标需关注'
    elif score >= 60:
        level = 'average'
        label = '一般'
        summary = '数据库存在一些问题,建议优化'
    else:
        level = 'poor'
        label = '较差'
        summary = '数据库存在较多问题,需要立即优化'

    if non_perfect_items:
        problem_parts = []
        if critical_items:
            problem_parts.append(f'{len(critical_items)}个严重问题')
        if warning_items:
            problem_parts.append(f'{len(warning_items)}个需关注项')
        summary += f'。发现{len(non_perfect_items)}个异常项' + '(' + ','.join(problem_parts) + ')'

    return {
        'score': score,
        'level': level,
        'label': label,
        'summary': summary,
        'issues': issues,
        'details': details,
        'problem_count': len(non_perfect_items),
        'critical_count': len(critical_items),
        'warning_count': len(warning_items)
    }


def generate_key_findings(basic_info: Dict, performance: Dict, databases: Dict, tables: Dict) -> List[Dict[str, str]]:
    findings = []

    connections = performance.get('connections', {})
    if connections and connections.get('usage_percent', 0) > 80:
        findings.append({
            'level': 'critical' if connections['usage_percent'] > 90 else 'warning',
            'icon': '🔴' if connections['usage_percent'] > 90 else '🟡',
            'title': '连接使用率过高',
            'description': f"当前连接使用率为{connections['usage_percent']}%,已达到{connections.get('current')}/{connections.get('max')}。建议: 1) 增加max_connections; 2) 使用连接池; 3) 排查异常连接"
        })

    cache_hit = performance.get('cache_hit_ratio', {})
    if cache_hit and cache_hit.get('ratio', 100) < 95:
        findings.append({
            'level': 'critical' if cache_hit['ratio'] < 90 else 'warning',
            'icon': '🔴' if cache_hit['ratio'] < 90 else '🟡',
            'title': '缓存命中率偏低',
            'description': f"当前缓存命中率为{cache_hit['ratio']}%。建议: 1) 增加shared_buffers配置; 2) 优化查询减少磁盘IO; 3) 检查是否有大表全表扫描"
        })

    index_hit = performance.get('index_hit_ratio', {})
    if index_hit and index_hit.get('ratio', 100) < 70:
        findings.append({
            'level': 'warning',
            'icon': '🟡',
            'title': '索引命中率较低',
            'description': f"当前索引命中率为{index_hit['ratio']}%,说明大量查询使用顺序扫描。建议: 1) 分析慢查询添加索引; 2) 检查现有索引是否合理; 3) 使用EXPLAIN分析查询计划"
        })

    long_transactions = performance.get('long_transactions', [])
    if long_transactions and len(long_transactions) > 0:
        critical_txns = [t for t in long_transactions if t.get('severity') == 'critical']
        warning_txns = [t for t in long_transactions if t.get('severity') == 'warning']

        if critical_txns:
            findings.append({
                'level': 'critical',
                'icon': '🔴',
                'title': '存在严重长事务',
                'description': f"发现{len(critical_txns)}个运行超过1小时的长事务。长事务会阻塞VACUUM导致表膨胀,建议: 1) 立即排查这些事务是否可以终止; 2) 优化事务逻辑; 3) 设置statement_timeout限制"
            })
        elif warning_txns:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '存在长事务',
                'description': f"发现{len(warning_txns)}个运行超过30分钟的事务。建议监控这些事务,避免演变为严重长事务"
            })

    locks = performance.get('locks', [])
    if locks and len(locks) > 0:
        critical_locks = [l for l in locks if l.get('severity') == 'critical']
        if critical_locks:
            findings.append({
                'level': 'critical',
                'icon': '🔴',
                'title': '存在严重锁等待',
                'description': f"发现{len(critical_locks)}个锁等待超过60秒。锁等待会导致业务响应缓慢,建议: 1) 排查阻塞源头会话; 2) 优化事务隔离级别; 3) 减少事务持有锁的时间"
            })
        elif len(locks) > 5:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '存在较多锁等待',
                'description': f"发现{len(locks)}个锁等待。建议关注并发事务的锁竞争情况"
            })

    tables_without_pk = basic_info.get('tables_without_pk', {})
    if tables_without_pk:
        total = sum(len(t) for t in tables_without_pk.values())
        if total > 0:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '部分表缺少主键',
                'description': f"发现{total}个表缺少主键或唯一索引。建议为这些表添加主键以保证数据完整性和提升查询性能"
            })

    dead_tuples = performance.get('dead_tuples', {})
    if dead_tuples:
        total_tables = 0
        critical_tables = 0
        warning_tables = 0
        for db_tables in dead_tuples.values():
            if isinstance(db_tables, list):
                for table in db_tables:
                    total_tables += 1
                    severity = table.get('severity', '')
                    if severity == 'critical':
                        critical_tables += 1
                    elif severity == 'warning':
                        warning_tables += 1

        if critical_tables > 0:
            findings.append({
                'level': 'critical',
                'icon': '🔴',
                'title': '部分表死元组比例严重',
                'description': f"发现{critical_tables}个表死元组比例超过50%,表膨胀严重。建议立即执行VACUUM FULL或VACUUM清理"
            })
        elif warning_tables > 0:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '部分表死元组比例偏高',
                'description': f"发现{warning_tables}个表死元组比例超过30%。建议执行VACUUM清理,检查autovacuum配置是否合理"
            })

    vacuum_status = performance.get('vacuum_status', {})
    if vacuum_status:
        total_issues = 0
        never_vacuumed = 0
        for db_tables in vacuum_status.values():
            if isinstance(db_tables, list):
                for table in db_tables:
                    total_issues += 1
                    if '从未执行' in table.get('vacuum_status', ''):
                        never_vacuumed += 1

        if never_vacuumed > 0:
            findings.append({
                'level': 'critical',
                'icon': '🔴',
                'title': '部分表从未执行VACUUM',
                'description': f"发现{never_vacuumed}个表从未执行过VACUUM或ANALYZE。请检查autovacuum是否启用,或手动执行VACUUM ANALYZE"
            })
        elif total_issues > 10:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': 'VACUUM执行不及时',
                'description': f"发现{total_issues}个表的VACUUM/ANALYZE超过7天未执行。建议调整autovacuum阈值参数"
            })

    invalid_indexes = performance.get('invalid_indexes', {})
    if invalid_indexes:
        total = sum(len(indexes) for indexes in invalid_indexes.values() if isinstance(indexes, list))
        if total > 0:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '存在无效索引',
                'description': f"发现{total}个无效或未使用的索引。建议删除这些索引以释放存储空间并提升写入性能"
            })

    duplicate_indexes = performance.get('duplicate_indexes', {})
    if duplicate_indexes:
        total = sum(len(indexes) for indexes in duplicate_indexes.values() if isinstance(indexes, list))
        if total > 0:
            findings.append({
                'level': 'info',
                'icon': '🔵',
                'title': '存在重复索引',
                'description': f"发现{total}个重复索引(相同列上多个索引)。建议保留使用频率高的索引,删除冗余索引"
            })

    io_stats = performance.get('io_stats', {})
    if io_stats:
        high_io_tables = 0
        for db_tables in io_stats.values():
            if isinstance(db_tables, list):
                for table in db_tables:
                    if table.get('io_level') == 'high':
                        high_io_tables += 1

        if high_io_tables > 0:
            findings.append({
                'level': 'warning',
                'icon': '🟡',
                'title': '存在高IO负载表',
                'description': f"发现{high_io_tables}个表存在大量磁盘IO且缓存命中率低。建议: 1) 优化查询添加索引; 2) 增加shared_buffers; 3) 检查是否有不必要的全表扫描"
            })

    backup_dbs = databases.get('backup', [])
    backup_tables = tables.get('backup', [])
    if len(backup_dbs) > 3 or len(backup_tables) > 10:
        findings.append({
            'level': 'warning',
            'icon': '🟡',
            'title': '疑似备份数据过多',
            'description': f"发现{len(backup_dbs)}个疑似备份库和{len(backup_tables)}个备份表。建议定期清理历史备份以释放存储空间"
        })

    level_order = {'critical': 0, 'warning': 1, 'info': 2}
    findings.sort(key=lambda x: level_order.get(x['level'], 3))

    return findings
