package inspector

import (
	"fmt"
	"strings"
	"sync"

	"db-patrol/internal/connection"
	"db-patrol/internal/models"
	"db-patrol/internal/utils"
)

// PerformanceInspector 性能巡检器
type PerformanceInspector struct {
	conn connection.Connection
	cfg  models.InspectionConfig
}

// NewPerformanceInspector 创建性能巡检器
func NewPerformanceInspector(conn connection.Connection, cfg models.InspectionConfig) *PerformanceInspector {
	return &PerformanceInspector{conn: conn, cfg: cfg}
}

func (i *PerformanceInspector) Name() string  { return "performance" }
func (i *PerformanceInspector) Title() string { return "检查性能指标" }

func (i *PerformanceInspector) Inspect() (map[string]interface{}, error) {
	dbType := i.conn.Config().Type
	if strings.Contains(strings.ToLower(dbType), "pg") || strings.Contains(strings.ToLower(dbType), "postgres") {
		return i.inspectPG()
	}
	return i.inspectMySQL()
}

// ==================== PG ====================

func (i *PerformanceInspector) inspectPG() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"connections":          i.getPGConnections(),
		"client_connections":   i.getPGClientConnections(),
		"cache_hit_ratio":      i.getPGCacheHitRatio(),
		"index_hit_ratio":      i.getPGIndexHitRatio(),
		"activity":             i.getPGActivity(),
		"locks":                i.getPGLocks(),
		"long_transactions":    i.getPGLongTransactions(),
		"slow_queries":         i.getPGSlowQueries(),
		"table_stats":          i.getPGTableStats(),
		"index_stats":          i.getPGIndexStats(),
		"dead_tuples":          map[string]interface{}{},
		"vacuum_status":        map[string]interface{}{},
		"io_stats":             map[string]interface{}{},
		"index_size_analysis":  map[string]interface{}{},
		"invalid_indexes":      map[string]interface{}{},
		"duplicate_indexes":    map[string]interface{}{},
	}

	// 获取所有可连接的数据库，并行深度巡检
	rows, _ := i.conn.ExecuteQuery("SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true")
	var dbNames []string
	for _, row := range rows {
		dbNames = append(dbNames, toString(row["datname"]))
	}

	deepResults := i.inspectPGDatabasesParallel(dbNames)
	for dbName, data := range deepResults {
		if len(data.DeadTuples) > 0 {
			result["dead_tuples"].(map[string]interface{})[dbName] = data.DeadTuples
		}
		if len(data.VacuumStatus) > 0 {
			result["vacuum_status"].(map[string]interface{})[dbName] = data.VacuumStatus
		}
		if len(data.IOStats) > 0 {
			result["io_stats"].(map[string]interface{})[dbName] = data.IOStats
		}
		if len(data.IndexSizeAnalysis) > 0 {
			result["index_size_analysis"].(map[string]interface{})[dbName] = data.IndexSizeAnalysis
		}
		if len(data.InvalidIndexes) > 0 {
			result["invalid_indexes"].(map[string]interface{})[dbName] = data.InvalidIndexes
		}
		if len(data.DuplicateIndexes) > 0 {
			result["duplicate_indexes"].(map[string]interface{})[dbName] = data.DuplicateIndexes
		}
	}

	return result, nil
}

type pgDeepResult struct {
	DeadTuples         []models.DeadTupleInfo
	VacuumStatus       []models.VacuumStatus
	IOStats            []models.IOStats
	IndexSizeAnalysis  []models.IndexSizeAnalysis
	InvalidIndexes     []models.InvalidIndex
	DuplicateIndexes   []models.DuplicateIndex
}

func (i *PerformanceInspector) inspectPGDatabasesParallel(dbNames []string) map[string]pgDeepResult {
	results := make(map[string]pgDeepResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)

	for _, dbName := range dbNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data := i.inspectPGDatabaseDeep(name)
			mu.Lock()
			results[name] = data
			mu.Unlock()
		}(dbName)
	}
	wg.Wait()
	return results
}

func (i *PerformanceInspector) inspectPGDatabaseDeep(dbName string) pgDeepResult {
	cfg := i.conn.Config()
	cfg.Database = dbName
	result := pgDeepResult{}

	conn, err := connection.CreateConnection(cfg)
	if err != nil {
		return result
	}
	defer conn.Close()

	// 死元组
	rows, _ := conn.ExecuteQuery(`
		SELECT schemaname, relname as table_name, n_live_tup as live_tuples, n_dead_tup as dead_tuples,
			CASE WHEN n_live_tup + n_dead_tup > 0
				THEN ROUND(n_dead_tup::numeric / (n_live_tup + n_dead_tup)::numeric * 100, 2)
				ELSE 0 END as dead_tuple_ratio,
			last_vacuum, last_autovacuum, last_analyze, last_autoanalyze,
			pg_size_pretty(pg_total_relation_size(relid)) as table_size
		FROM pg_stat_user_tables
		WHERE n_dead_tup > 1000
		   OR (n_live_tup + n_dead_tup > 0 AND n_dead_tup::numeric / (n_live_tup + n_dead_tup)::numeric > 0.1)
		ORDER BY n_dead_tup DESC LIMIT 30
	`)
	for _, row := range rows {
		ratio := toFloat64(row["dead_tuple_ratio"])
		severity := "normal"
		severityLabel := "正常"
		suggestion := ""
		if ratio > 50 {
			severity = "critical"
			severityLabel = "严重(>50%)"
			suggestion = "立即执行VACUUM,表膨胀严重"
		} else if ratio > 30 {
			severity = "warning"
			severityLabel = "警告(>30%)"
			suggestion = "建议执行VACUUM FULL或VACUUM"
		} else if ratio > 10 {
			severity = "info"
			severityLabel = "关注(>10%)"
			suggestion = "监控自动VACUUM是否正常工作"
		}
		result.DeadTuples = append(result.DeadTuples, models.DeadTupleInfo{
			Schema:        toString(row["schemaname"]),
			TableName:     toString(row["table_name"]),
			LiveTuples:    toInt64(row["live_tuples"]),
			DeadTuples:    toInt64(row["dead_tuples"]),
			DeadTupleRatio: ratio,
			TableSize:     toString(row["table_size"]),
			Severity:      severity,
			SeverityLabel: severityLabel,
			Suggestion:    suggestion,
		})
	}

	// VACUUM状态
	rows, _ = conn.ExecuteQuery(`
		SELECT schemaname, relname as table_name, n_live_tup as live_tuples,
			last_vacuum, last_autovacuum, last_analyze, last_autoanalyze,
			CASE
				WHEN last_autovacuum IS NULL AND last_vacuum IS NULL THEN '从未执行'
				WHEN last_autovacuum < NOW() - INTERVAL '7 days' THEN '超过7天'
				WHEN last_autoanalyze IS NULL AND last_analyze IS NULL THEN '从未执行'
				WHEN last_autoanalyze < NOW() - INTERVAL '7 days' THEN '超过7天'
				ELSE '正常'
			END as vacuum_status
		FROM pg_stat_user_tables
		WHERE last_autovacuum IS NULL OR last_vacuum IS NULL OR last_autoanalyze IS NULL OR last_analyze IS NULL
		   OR last_autovacuum < NOW() - INTERVAL '7 days' OR last_autoanalyze < NOW() - INTERVAL '7 days'
		ORDER BY n_live_tup DESC LIMIT 20
	`)
	for _, row := range rows {
		status := toString(row["vacuum_status"])
		severity := "info"
		suggestion := ""
		if strings.Contains(status, "从未执行") {
			severity = "critical"
			suggestion = "检查autovacuum是否启用,或手动执行VACUUM ANALYZE"
		} else if strings.Contains(status, "超过7天") {
			severity = "warning"
			suggestion = "检查autovacuum配置,可能需要调整阈值"
		}
		result.VacuumStatus = append(result.VacuumStatus, models.VacuumStatus{
			Schema:       toString(row["schemaname"]),
			TableName:    toString(row["table_name"]),
			LiveTuples:   toInt64(row["live_tuples"]),
			VacuumStatus: status,
			Severity:     severity,
			Suggestion:   suggestion,
		})
	}

	// IO统计
	rows, _ = conn.ExecuteQuery(`
		SELECT schemaname, relname as table_name, heap_blks_read as disk_reads, heap_blks_hit as buffer_hits,
			idx_blks_read as index_disk_reads, idx_blks_hit as index_buffer_hits,
			CASE WHEN heap_blks_read + heap_blks_hit > 0
				THEN ROUND(heap_blks_hit::numeric / (heap_blks_read + heap_blks_hit)::numeric * 100, 2)
				ELSE 0 END as cache_hit_ratio,
			pg_size_pretty(pg_total_relation_size(relid)) as table_size
		FROM pg_statio_user_tables
		WHERE heap_blks_read > 0 OR idx_blks_read > 0
		ORDER BY (heap_blks_read + idx_blks_read) DESC LIMIT 20
	`)
	for _, row := range rows {
		totalReads := toInt64(row["disk_reads"]) + toInt64(row["index_disk_reads"])
		cacheRatio := toFloat64(row["cache_hit_ratio"])
		ioLevel := "low"
		ioLabel := "低IO"
		suggestion := ""
		if totalReads > 100000 && cacheRatio < 90 {
			ioLevel = "high"
			ioLabel = "高IO"
			suggestion = "该表IO密集且缓存命中率低,建议优化查询或增加shared_buffers"
		} else if totalReads > 50000 {
			ioLevel = "medium"
			ioLabel = "中IO"
			suggestion = "该表IO较密集,建议检查是否有缺失索引"
		}
		result.IOStats = append(result.IOStats, models.IOStats{
			Schema:          toString(row["schemaname"]),
			TableName:       toString(row["table_name"]),
			DiskReads:       toInt64(row["disk_reads"]),
			BufferHits:      toInt64(row["buffer_hits"]),
			IndexDiskReads:  toInt64(row["index_disk_reads"]),
			IndexBufferHits: toInt64(row["index_buffer_hits"]),
			CacheHitRatio:   cacheRatio,
			TableSize:       toString(row["table_size"]),
			IOLevel:         ioLevel,
			IOLabel:         ioLabel,
			Suggestion:      suggestion,
		})
	}

	// 索引大小分析
	rows, _ = conn.ExecuteQuery(`
		SELECT t.schemaname, t.relname as table_name, t.n_live_tup as row_count,
			pg_size_pretty(pg_relation_size(t.relid)) as table_size,
			pg_size_pretty(pg_indexes_size(t.relid)) as index_size,
			pg_size_pretty(pg_total_relation_size(t.relid)) as total_size,
			pg_relation_size(t.relid) as table_size_bytes,
			pg_indexes_size(t.relid) as index_size_bytes,
			pg_total_relation_size(t.relid) as total_size_bytes,
			CASE WHEN pg_relation_size(t.relid) > 0
				THEN ROUND(pg_indexes_size(t.relid)::numeric / pg_relation_size(t.relid)::numeric * 100, 2)
				ELSE 0 END as index_ratio,
			(SELECT COUNT(*) FROM pg_indexes WHERE tablename = t.relname AND schemaname = t.schemaname) as index_count
		FROM pg_stat_user_tables t
		WHERE pg_relation_size(t.relid) > 0 AND t.n_live_tup >= 10000
		ORDER BY CASE WHEN pg_relation_size(t.relid) > 0
			THEN pg_indexes_size(t.relid)::numeric / pg_relation_size(t.relid)::numeric ELSE 0 END DESC
		LIMIT 30
	`)
	for _, row := range rows {
		ratio := toFloat64(row["index_ratio"])
		attention := "正常"
		suggestion := ""
		if ratio > 100 {
			attention = "严重"
			suggestion = fmt.Sprintf("索引大小(%s)超过数据大小(%s),请检查冗余索引", toString(row["index_size"]), toString(row["table_size"]))
		} else if ratio > 50 {
			attention = "关注"
			suggestion = fmt.Sprintf("索引占比%.2f%%,建议检查索引是否合理", ratio)
		}
		result.IndexSizeAnalysis = append(result.IndexSizeAnalysis, models.IndexSizeAnalysis{
			Schema:         toString(row["schemaname"]),
			TableName:      toString(row["table_name"]),
			RowCount:       toInt64(row["row_count"]),
			TableSize:      toString(row["table_size"]),
			IndexSize:      toString(row["index_size"]),
			TotalSize:      toString(row["total_size"]),
			TableSizeBytes: toInt64(row["table_size_bytes"]),
			IndexSizeBytes: toInt64(row["index_size_bytes"]),
			TotalSizeBytes: toInt64(row["total_size_bytes"]),
			IndexRatio:     ratio,
			IndexCount:     toInt(row["index_count"]),
			Attention:      attention,
			Suggestion:     suggestion,
		})
	}

	// 无效索引
	rows, _ = conn.ExecuteQuery(`
		SELECT schemaname, tablename as table_name, indexname as index_name,
			pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
			idx_scan as index_scans,
			CASE WHEN NOT indisvalid THEN '索引失效'
				 WHEN idx_scan = 0 THEN '从未使用'
				 ELSE '疑似无效' END as issue_type
		FROM pg_stat_user_indexes sui
		JOIN pg_index pi ON sui.indexrelid = pi.indexrelid
		WHERE NOT indisvalid
		   OR (idx_scan = 0 AND pg_relation_size(sui.indexrelid) > 1048576)
		ORDER BY pg_relation_size(sui.indexrelid) DESC
	`)
	for _, row := range rows {
		issueType := toString(row["issue_type"])
		suggestion := "建议评估该索引是否仍然需要"
		if issueType == "索引失效" {
			suggestion = "建议删除无效索引以释放空间和提升写入性能"
		}
		result.InvalidIndexes = append(result.InvalidIndexes, models.InvalidIndex{
			Schema:     toString(row["schemaname"]),
			TableName:  toString(row["table_name"]),
			IndexName:  toString(row["index_name"]),
			IndexSize:  toString(row["index_size"]),
			IndexScans: toInt64(row["index_scans"]),
			IssueType:  issueType,
			Suggestion: suggestion,
			Database:   dbName,
		})
	}

	// 重复索引
	rows, _ = conn.ExecuteQuery(`
		SELECT a.schemaname, a.tablename as table_name, a.indexname as index_name,
			a.indexdef as index_definition,
			pg_size_pretty(pg_relation_size(b.indexrelid)) as index_size,
			a.idx_scan as index_scans
		FROM pg_stat_user_indexes a
		JOIN pg_stat_user_indexes b
			ON a.schemaname = b.schemaname AND a.tablename = b.tablename
			AND a.indexrelid < b.indexrelid
		WHERE EXISTS (
			SELECT 1 FROM pg_index i1
			JOIN pg_index i2 ON i1.indrelid = i2.indrelid
			WHERE i1.indexrelid = a.indexrelid
			  AND i2.indexrelid = b.indexrelid
			  AND i1.indkey::text = i2.indkey::text
			  AND i1.indexrelid != i2.indexrelid
		)
		ORDER BY a.schemaname, a.tablename, pg_relation_size(b.indexrelid) DESC
	`)
	for _, row := range rows {
		result.DuplicateIndexes = append(result.DuplicateIndexes, models.DuplicateIndex{
			Schema:          toString(row["schemaname"]),
			TableName:       toString(row["table_name"]),
			IndexName:       toString(row["index_name"]),
			IndexDefinition: toString(row["index_definition"]),
			IndexSize:       toString(row["index_size"]),
			IndexScans:      toInt64(row["index_scans"]),
			Suggestion:      fmt.Sprintf("表 %s 存在重复索引,建议保留使用频率较高的索引,删除其他重复索引", toString(row["table_name"])),
			Database:        dbName,
		})
	}

	return result
}

func (i *PerformanceInspector) getPGConnections() models.ConnectionStats {
	current := 0
	max := 100
	active := 0
	idle := 0
	rows, _ := i.conn.ExecuteQuery("SELECT count(*) as count FROM pg_stat_activity")
	if len(rows) > 0 {
		current = toInt(rows[0]["count"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW max_connections")
	if len(rows) > 0 {
		max = toInt(rows[0]["max_connections"])
	}
	rows, _ = i.conn.ExecuteQuery("SELECT count(*) as count FROM pg_stat_activity WHERE state = 'active'")
	if len(rows) > 0 {
		active = toInt(rows[0]["count"])
	}
	rows, _ = i.conn.ExecuteQuery("SELECT count(*) as count FROM pg_stat_activity WHERE state = 'idle'")
	if len(rows) > 0 {
		idle = toInt(rows[0]["count"])
	}
	usage := 0.0
	if max > 0 {
		usage = float64(current) / float64(max) * 100
	}
	status := "正常"
	if usage > float64(i.cfg.MaxConnectionsThreshold) {
		status = "警告"
	}
	return models.ConnectionStats{Current: current, Max: max, Active: active, Idle: idle, UsagePercent: round(usage), Status: status}
}

func (i *PerformanceInspector) getPGCacheHitRatio() models.CacheHitRatio {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT SUM(heap_blks_read) as heap_read, SUM(heap_blks_hit) as heap_hit,
			CASE WHEN SUM(heap_blks_read) + SUM(heap_blks_hit) > 0
				THEN ROUND(SUM(heap_blks_hit)::numeric / (SUM(heap_blks_read) + SUM(heap_blks_hit))::numeric * 100, 2)
				ELSE 0 END as cache_hit_ratio
		FROM pg_statio_user_tables
	`)
	if len(rows) == 0 {
		return models.CacheHitRatio{Ratio: 0, Status: "未知"}
	}
	ratio := toFloat64(rows[0]["cache_hit_ratio"])
	status := "较差"
	suggestion := "缓存命中率低于95%,建议增加shared_buffers"
	if ratio >= 99 {
		status = "优秀"
		suggestion = "缓存命中率正常"
	} else if ratio >= 95 {
		status = "良好"
		suggestion = "缓存命中率正常"
	} else if ratio >= 90 {
		status = "一般"
	}
	return models.CacheHitRatio{
		Ratio:      ratio,
		HeapRead:   toInt64(rows[0]["heap_read"]),
		HeapHit:    toInt64(rows[0]["heap_hit"]),
		Status:     status,
		Suggestion: suggestion,
	}
}

func (i *PerformanceInspector) getPGIndexHitRatio() models.IndexHitRatio {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT SUM(idx_scan) as idx_scan, SUM(seq_scan) as seq_scan,
			CASE WHEN SUM(idx_scan) + SUM(seq_scan) > 0
				THEN ROUND(SUM(idx_scan)::numeric / (SUM(idx_scan) + SUM(seq_scan))::numeric * 100, 2)
				ELSE 0 END as index_hit_ratio
		FROM pg_stat_user_tables
	`)
	if len(rows) == 0 {
		return models.IndexHitRatio{Ratio: 0, Status: "未知"}
	}
	ratio := toFloat64(rows[0]["index_hit_ratio"])
	status := "较差"
	suggestion := "索引命中率低于70%,建议检查是否有缺失索引或查询未使用索引"
	if ratio >= 90 {
		status = "优秀"
		suggestion = "索引命中率正常"
	} else if ratio >= 70 {
		status = "良好"
		suggestion = "索引命中率正常"
	} else if ratio >= 50 {
		status = "一般"
	}
	return models.IndexHitRatio{
		Ratio:      ratio,
		IdxScan:    toInt64(rows[0]["idx_scan"]),
		SeqScan:    toInt64(rows[0]["seq_scan"]),
		Status:     status,
		Suggestion: suggestion,
	}
}

func (i *PerformanceInspector) getPGClientConnections() []models.ClientConnection {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT datname as database, usename as username,
			COALESCE(client_addr::text, 'local') as client_ip,
			application_name, state, COUNT(*) as connection_count
		FROM pg_stat_activity
		WHERE pid IS NOT NULL
		GROUP BY datname, usename, client_addr, application_name, state
		ORDER BY connection_count DESC
	`)
	ipStats := map[string]*models.ClientConnection{}
	for _, row := range rows {
		ip := toString(row["client_ip"])
		if _, ok := ipStats[ip]; !ok {
			ipStats[ip] = &models.ClientConnection{ClientIP: ip}
		}
		stats := ipStats[ip]
		stats.TotalConnections += toInt(row["connection_count"])
		if state := toString(row["state"]); state == "active" {
			stats.Active += toInt(row["connection_count"])
		} else if state == "idle" {
			stats.Idle += toInt(row["connection_count"])
		}
	}
	var result []models.ClientConnection
	for _, v := range ipStats {
		result = append(result, *v)
	}
	return result
}

func (i *PerformanceInspector) getPGActivity() []map[string]interface{} {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT datname, usename, application_name, client_addr, state, query_start, state_change, query
		FROM pg_stat_activity WHERE state IS NOT NULL ORDER BY query_start DESC LIMIT 20
	`)
	return rows
}

func (i *PerformanceInspector) getPGLocks() []models.LockInfo {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT blocked_locks.pid AS blocked_pid, blocked_activity.usename AS blocked_user,
			blocking_locks.pid AS blocking_pid, blocking_activity.usename AS blocking_user,
			blocked_activity.datname AS database, blocked_activity.application_name,
			blocked_activity.client_addr, blocked_locks.locktype,
			blocked_locks.mode AS blocked_mode, blocking_locks.mode AS blocking_mode,
			blocked_activity.state AS blocked_state,
			EXTRACT(EPOCH FROM (NOW() - blocked_activity.query_start)) AS wait_seconds,
			LEFT(blocked_activity.query, 200) AS blocked_query,
			LEFT(blocking_activity.query, 200) AS blocking_query
		FROM pg_catalog.pg_locks blocked_locks
		JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
		JOIN pg_catalog.pg_locks blocking_locks
			ON blocking_locks.locktype = blocked_locks.locktype
			AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
			AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
			AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
			AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
			AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
			AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
			AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
			AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
			AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
			AND blocking_locks.pid != blocked_locks.pid
		JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
		WHERE NOT blocked_locks.granted
		ORDER BY wait_seconds DESC LIMIT 20
	`)
	var result []models.LockInfo
	for _, row := range rows {
		waitSec := toFloat64(row["wait_seconds"])
		severity := "info"
		severityLabel := "关注"
		if waitSec > 60 {
			severity = "critical"
			severityLabel = "严重(>60秒)"
		} else if waitSec > 10 {
			severity = "warning"
			severityLabel = "警告(>10秒)"
		}
		result = append(result, models.LockInfo{
			BlockedPID:      toInt(row["blocked_pid"]),
			BlockedUser:     toString(row["blocked_user"]),
			BlockingPID:     toInt(row["blocking_pid"]),
			BlockingUser:    toString(row["blocking_user"]),
			Database:        toString(row["database"]),
			ApplicationName: toString(row["application_name"]),
			ClientAddr:      toString(row["client_addr"]),
			LockType:        toString(row["locktype"]),
			BlockedMode:     toString(row["blocked_mode"]),
			BlockingMode:    toString(row["blocking_mode"]),
			BlockedState:    toString(row["blocked_state"]),
			WaitSeconds:     waitSec,
			BlockedQuery:    toString(row["blocked_query"]),
			BlockingQuery:   toString(row["blocking_query"]),
			Severity:        severity,
			SeverityLabel:   severityLabel,
			WaitDisplay:     fmt.Sprintf("%.1f秒", waitSec),
		})
	}
	return result
}

func (i *PerformanceInspector) getPGLongTransactions() []models.LongTransaction {
	threshold := i.cfg.LongTransactionThreshold
	if threshold == 0 {
		threshold = 300
	}
	rows, _ := i.conn.ExecuteQuery(fmt.Sprintf(`
		SELECT pid, datname as database, usename as username, client_addr, application_name, state,
			query_start, NOW() - query_start as duration,
			EXTRACT(EPOCH FROM (NOW() - query_start)) as duration_seconds,
			LEFT(query, 200) as query
		FROM pg_stat_activity
		WHERE state != 'idle'
		  AND query NOT LIKE '%%pg_stat_activity%%'
		  AND NOW() - query_start > INTERVAL '%d seconds'
		ORDER BY query_start ASC
	`, threshold))
	var result []models.LongTransaction
	for _, row := range rows {
		durationSec := toFloat64(row["duration_seconds"])
		severity := "info"
		severityLabel := "关注(>5分钟)"
		if durationSec > 3600 {
			severity = "critical"
			severityLabel = "严重(>1小时)"
		} else if durationSec > 1800 {
			severity = "warning"
			severityLabel = "警告(>30分钟)"
		}
		result = append(result, models.LongTransaction{
			PID:             toInt(row["pid"]),
			Database:        toString(row["database"]),
			Username:        toString(row["username"]),
			ClientAddr:      toString(row["client_addr"]),
			ApplicationName: toString(row["application_name"]),
			State:           toString(row["state"]),
			DurationSeconds: durationSec,
			Severity:        severity,
			SeverityLabel:   severityLabel,
			DurationDisplay: fmt.Sprintf("%v", row["duration"]),
			Query:           toString(row["query"]),
		})
	}
	return result
}

func (i *PerformanceInspector) getPGSlowQueries() []map[string]interface{} {
	threshold := i.cfg.SlowQueryThreshold
	if threshold == 0 {
		threshold = 1.0
	}
	rows, _ := i.conn.ExecuteQuery(`
		SELECT query, calls, total_time, mean_time, max_time, rows
		FROM pg_stat_statements WHERE mean_time > $1 ORDER BY mean_time DESC LIMIT 10
	`, threshold*1000)
	return rows
}

func (i *PerformanceInspector) getPGTableStats() []map[string]interface{} {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT schemaname, relname as table_name, n_live_tup as live_tuples,
			n_dead_tup as dead_tuples, last_vacuum, last_autovacuum, last_analyze, last_autoanalyze
		FROM pg_stat_user_tables ORDER BY n_live_tup DESC LIMIT 20
	`)
	return rows
}

func (i *PerformanceInspector) getPGIndexStats() []map[string]interface{} {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT schemaname, relname as table_name, indexrelname as index_name,
			idx_scan, idx_tup_read, idx_tup_fetch
		FROM pg_stat_user_indexes ORDER BY idx_scan DESC LIMIT 20
	`)
	return rows
}

// ==================== MySQL ====================

func (i *PerformanceInspector) inspectMySQL() (map[string]interface{}, error) {
	return map[string]interface{}{
		"connections":          i.getMySQLConnections(),
		"client_connections":   i.getMySQLClientConnections(),
		"cache_hit_ratio":      i.getMySQLCacheHitRatio(),
		"index_hit_ratio":      i.getMySQLIndexHitRatio(),
		"status":               i.getMySQLStatus(),
		"slow_queries":         i.getMySQLSlowQueries(),
		"table_stats":          i.getMySQLTableStats(),
		"index_stats":          i.getMySQLIndexStats(),
		"processlist":          i.getMySQLProcesslist(),
		"long_transactions":    i.getMySQLLongTransactions(),
		"locks":                i.getMySQLLocks(),
		"index_size_analysis":  i.getMySQLIndexSizeAnalysis(),
		"invalid_indexes":      i.getMySQLInvalidIndexes(),
		"duplicate_indexes":    i.getMySQLDuplicateIndexes(),
	}, nil
}

func (i *PerformanceInspector) getMySQLConnections() models.ConnectionStats {
	current := 0
	max := 151
	rows, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Threads_connected'")
	if len(rows) > 0 {
		current = toInt(rows[0]["Value"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'max_connections'")
	if len(rows) > 0 {
		max = toInt(rows[0]["Value"])
	}
	usage := 0.0
	if max > 0 {
		usage = float64(current) / float64(max) * 100
	}
	status := "正常"
	if usage > float64(i.cfg.MaxConnectionsThreshold) {
		status = "警告"
	}
	return models.ConnectionStats{Current: current, Max: max, UsagePercent: round(usage), Status: status}
}

func (i *PerformanceInspector) getMySQLClientConnections() []models.ClientConnection {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT db as database, user as username, host as client_host, command, state, COUNT(*) as connection_count
		FROM information_schema.processlist WHERE command != 'Daemon'
		GROUP BY db, user, host, command, state ORDER BY connection_count DESC
	`)
	ipStats := map[string]*models.ClientConnection{}
	for _, row := range rows {
		host := toString(row["client_host"])
		ip := host
		if idx := strings.Index(host, ":"); idx >= 0 {
			ip = host[:idx]
		}
		if _, ok := ipStats[ip]; !ok {
			ipStats[ip] = &models.ClientConnection{ClientIP: ip}
		}
		stats := ipStats[ip]
		stats.TotalConnections += toInt(row["connection_count"])
		if toString(row["command"]) == "Sleep" {
			stats.Idle += toInt(row["connection_count"])
		} else {
			stats.Active += toInt(row["connection_count"])
		}
	}
	var result []models.ClientConnection
	for _, v := range ipStats {
		result = append(result, *v)
	}
	return result
}

func (i *PerformanceInspector) getMySQLStatus() map[string]string {
	vars := []string{"Queries", "Questions", "Slow_queries", "Uptime"}
	result := map[string]string{}
	for _, v := range vars {
		rows, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE ?", v)
		if len(rows) > 0 {
			result[v] = toString(rows[0]["Value"])
		}
	}
	return result
}

func (i *PerformanceInspector) getMySQLSlowQueries() []map[string]interface{} {
	rows, _ := i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'slow_query_log'")
	if len(rows) > 0 && toString(rows[0]["Value"]) != "ON" {
		return []map[string]interface{}{{"message": "慢查询日志未开启"}}
	}
	return []map[string]interface{}{{"message": "请检查慢查询日志文件"}}
}

func (i *PerformanceInspector) getMySQLTableStats() []map[string]interface{} {
	dbName := i.conn.Config().Database
	rows, _ := i.conn.ExecuteQuery(`
		SELECT table_name, engine, table_rows, data_length, index_length, data_free
		FROM information_schema.tables WHERE table_schema = ? ORDER BY data_length + index_length DESC LIMIT 20
	`, dbName)
	return rows
}

func (i *PerformanceInspector) getMySQLIndexStats() []map[string]interface{} {
	dbName := i.conn.Config().Database
	rows, _ := i.conn.ExecuteQuery(`
		SELECT table_name, index_name, cardinality
		FROM information_schema.statistics WHERE table_schema = ? ORDER BY cardinality DESC LIMIT 20
	`, dbName)
	return rows
}

func (i *PerformanceInspector) getMySQLProcesslist() []map[string]interface{} {
	rows, _ := i.conn.ExecuteQuery("SELECT * FROM information_schema.processlist WHERE command != 'Daemon' LIMIT 20")
	return rows
}

func (i *PerformanceInspector) getMySQLCacheHitRatio() models.CacheHitRatio {
	readReq, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Innodb_buffer_pool_read_requests'")
	reads, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Innodb_buffer_pool_reads'")
	var totalRequests, totalReads int64
	if len(readReq) > 0 {
		totalRequests = toInt64(readReq[0]["Value"])
	}
	if len(reads) > 0 {
		totalReads = toInt64(reads[0]["Value"])
	}
	total := totalRequests + totalReads
	ratio := 100.0
	if total > 0 {
		ratio = float64(totalRequests) / float64(total) * 100
	}
	status := "较差"
	suggestion := "InnoDB缓存命中率低于95%,建议增加innodb_buffer_pool_size"
	if ratio >= 99 {
		status = "优秀"
		suggestion = "缓存命中率正常"
	} else if ratio >= 95 {
		status = "良好"
		suggestion = "缓存命中率正常"
	} else if ratio >= 90 {
		status = "一般"
	}
	return models.CacheHitRatio{Ratio: round(ratio), HeapRead: totalReads, HeapHit: totalRequests, Status: status, Suggestion: suggestion}
}

func (i *PerformanceInspector) getMySQLIndexHitRatio() models.IndexHitRatio {
	readKey, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Handler_read_key'")
	readNext, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Handler_read_rnd_next'")
	readFirst, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Handler_read_first'")
	readPrev, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Handler_read_prev'")
	readRnd, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Handler_read_rnd'")
	var idxReads, seqReads int64
	if len(readKey) > 0 {
		idxReads = toInt64(readKey[0]["Value"])
	}
	if len(readFirst) > 0 {
		idxReads += toInt64(readFirst[0]["Value"])
	}
	if len(readPrev) > 0 {
		idxReads += toInt64(readPrev[0]["Value"])
	}
	if len(readNext) > 0 {
		seqReads = toInt64(readNext[0]["Value"])
	}
	if len(readRnd) > 0 {
		seqReads += toInt64(readRnd[0]["Value"])
	}
	total := idxReads + seqReads
	ratio := 100.0
	if total > 0 {
		ratio = float64(idxReads) / float64(total) * 100
	}
	status := "较差"
	suggestion := "索引使用率低于70%,建议检查是否有缺失索引或查询未使用索引"
	if ratio >= 90 {
		status = "优秀"
		suggestion = "索引使用率正常"
	} else if ratio >= 70 {
		status = "良好"
		suggestion = "索引使用率正常"
	} else if ratio >= 50 {
		status = "一般"
	}
	return models.IndexHitRatio{Ratio: round(ratio), IdxScan: idxReads, SeqScan: seqReads, Status: status, Suggestion: suggestion}
}

func (i *PerformanceInspector) getMySQLLongTransactions() []models.LongTransaction {
	threshold := i.cfg.LongTransactionThreshold
	if threshold == 0 {
		threshold = 300
	}
	rows, _ := i.conn.ExecuteQuery(`
		SELECT trx_id, trx_mysql_thread_id as thread_id, trx_state as state,
			trx_tables_locked as tables_locked, trx_rows_locked as rows_locked,
			TIMESTAMPDIFF(SECOND, trx_started, NOW()) as duration_seconds
		FROM information_schema.innodb_trx
		WHERE TIMESTAMPDIFF(SECOND, trx_started, NOW()) > ?
		ORDER BY trx_started ASC
	`, threshold)
	var result []models.LongTransaction
	for _, row := range rows {
		durationSec := toInt64(row["duration_seconds"])
		severity := "info"
		severityLabel := "关注(>5分钟)"
		if durationSec > 3600 {
			severity = "critical"
			severityLabel = "严重(>1小时)"
		} else if durationSec > 1800 {
			severity = "warning"
			severityLabel = "警告(>30分钟)"
		}
		result = append(result, models.LongTransaction{
			TrxID:           toString(row["trx_id"]),
			ThreadID:        toInt(row["thread_id"]),
			State:           toString(row["state"]),
			TablesLocked:    toInt(row["tables_locked"]),
			RowsLocked:      toInt(row["rows_locked"]),
			DurationSeconds: float64(durationSec),
			Severity:        severity,
			SeverityLabel:   severityLabel,
			DurationDisplay: fmt.Sprintf("%d分%d秒", durationSec/60, durationSec%60),
		})
	}
	return result
}

func (i *PerformanceInspector) getMySQLLocks() []models.LockInfo {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT r.trx_id as waiting_trx_id, r.trx_mysql_thread_id as waiting_thread,
			b.trx_id as blocking_trx_id, b.trx_mysql_thread_id as blocking_thread,
			w.lock_mode as waiting_lock_mode, w.lock_type as waiting_lock_type,
			b_lock.lock_mode as blocking_lock_mode,
			TIMESTAMPDIFF(SECOND, r.trx_started, NOW()) as wait_seconds
		FROM information_schema.innodb_lock_waits w
		INNER JOIN information_schema.innodb_trx b ON b.trx_id = w.blocking_trx_id
		INNER JOIN information_schema.innodb_trx r ON r.trx_id = w.requesting_trx_id
		INNER JOIN information_schema.innodb_locks b_lock ON b_lock.lock_id = w.blocking_lock_id
		ORDER BY wait_seconds DESC
	`)
	var result []models.LockInfo
	for _, row := range rows {
		waitSec := toInt64(row["wait_seconds"])
		severity := "info"
		severityLabel := "关注"
		if waitSec > 60 {
			severity = "critical"
			severityLabel = "严重(>60秒)"
		} else if waitSec > 10 {
			severity = "warning"
			severityLabel = "警告(>10秒)"
		}
		result = append(result, models.LockInfo{
			WaitingTrxID:     toString(row["waiting_trx_id"]),
			WaitingThread:    toInt(row["waiting_thread"]),
			BlockingTrxID:    toString(row["blocking_trx_id"]),
			BlockingThread:   toInt(row["blocking_thread"]),
			WaitingLockMode:  toString(row["waiting_lock_mode"]),
			WaitingLockType:  toString(row["waiting_lock_type"]),
			BlockingMode:     toString(row["blocking_lock_mode"]),
			WaitSeconds:      float64(waitSec),
			Severity:         severity,
			SeverityLabel:    severityLabel,
			WaitDisplay:      fmt.Sprintf("%.1f秒", float64(waitSec)),
		})
	}
	return result
}

func (i *PerformanceInspector) getMySQLIndexSizeAnalysis() map[string][]models.IndexSizeAnalysis {
	dbName := i.conn.Config().Database
	rows, _ := i.conn.ExecuteQuery(`
		SELECT table_name, table_rows, data_length, index_length, data_length + index_length as total_size,
			CASE WHEN data_length > 0 THEN ROUND(index_length / data_length * 100, 2) ELSE 0 END as index_ratio
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE' AND table_rows >= 10000 AND data_length > 0
		ORDER BY index_length / data_length DESC LIMIT 30
	`, dbName)
	var analysis []models.IndexSizeAnalysis
	for _, row := range rows {
		ratio := toFloat64(row["index_ratio"])
		attention := "正常"
		suggestion := ""
		if ratio > 100 {
			attention = "严重"
			suggestion = fmt.Sprintf("索引大小(%s)超过数据大小(%s),请检查冗余索引", utils.FormatSize(toInt64(row["index_length"])), utils.FormatSize(toInt64(row["data_length"])))
		} else if ratio > 50 {
			attention = "关注"
			suggestion = fmt.Sprintf("索引占比%.2f%%,建议检查索引是否合理", ratio)
		}
		analysis = append(analysis, models.IndexSizeAnalysis{
			Schema:         dbName,
			TableName:      toString(row["table_name"]),
			RowCount:       toInt64(row["table_rows"]),
			TableSize:      utils.FormatSize(toInt64(row["data_length"])),
			IndexSize:      utils.FormatSize(toInt64(row["index_length"])),
			TotalSize:      utils.FormatSize(toInt64(row["total_size"])),
			TableSizeBytes: toInt64(row["data_length"]),
			IndexSizeBytes: toInt64(row["index_length"]),
			TotalSizeBytes: toInt64(row["total_size"]),
			IndexRatio:     ratio,
			Attention:      attention,
			Suggestion:     suggestion,
		})
	}
	if len(analysis) > 0 {
		return map[string][]models.IndexSizeAnalysis{dbName: analysis}
	}
	return map[string][]models.IndexSizeAnalysis{}
}

func (i *PerformanceInspector) getMySQLInvalidIndexes() map[string][]models.InvalidIndex {
	dbName := i.conn.Config().Database
	rows, _ := i.conn.ExecuteQuery(`
		SELECT table_name, index_name, cardinality, seq_in_index
		FROM information_schema.statistics
		WHERE table_schema = ? AND cardinality = 0 AND index_name != 'PRIMARY'
		ORDER BY table_name, index_name
	`, dbName)
	var indexes []models.InvalidIndex
	for _, row := range rows {
		indexes = append(indexes, models.InvalidIndex{
			Schema:     dbName,
			TableName:  toString(row["table_name"]),
			IndexName:  toString(row["index_name"]),
			IndexSize:  "N/A",
			IndexScans: 0,
			IssueType:  "基数为0",
			Suggestion: "该索引基数为0,建议评估是否需要删除",
			Database:   dbName,
		})
	}
	if len(indexes) > 0 {
		return map[string][]models.InvalidIndex{dbName: indexes}
	}
	return map[string][]models.InvalidIndex{}
}

func (i *PerformanceInspector) getMySQLDuplicateIndexes() map[string][]models.DuplicateIndex {
	dbName := i.conn.Config().Database
	rows, _ := i.conn.ExecuteQuery(`
		SELECT t1.table_name, t1.index_name as index_name_a, t2.index_name as index_name_b,
			GROUP_CONCAT(t1.column_name ORDER BY t1.seq_in_index) as columns_a,
			GROUP_CONCAT(t2.column_name ORDER BY t2.seq_in_index) as columns_b
		FROM information_schema.statistics t1
		JOIN information_schema.statistics t2
			ON t1.table_schema = t2.table_schema AND t1.table_name = t2.table_name
			AND t1.index_name < t2.index_name AND t1.seq_in_index = t2.seq_in_index
			AND t1.column_name = t2.column_name
		WHERE t1.table_schema = ?
		GROUP BY t1.table_name, t1.index_name, t2.index_name
		HAVING columns_a = columns_b LIMIT 50
	`, dbName)
	var indexes []models.DuplicateIndex
	for _, row := range rows {
		indexes = append(indexes, models.DuplicateIndex{
			Schema:          dbName,
			TableName:       toString(row["table_name"]),
			IndexName:       toString(row["index_name_b"]),
			IndexDefinition: fmt.Sprintf("%s 与 %s 列相同: (%s)", toString(row["index_name_a"]), toString(row["index_name_b"]), toString(row["columns_a"])),
			IndexSize:       "N/A",
			IndexScans:      0,
			Suggestion:      fmt.Sprintf("表 %s 存在重复索引,建议保留使用频率较高的索引,删除其他重复索引", toString(row["table_name"])),
			Database:        dbName,
		})
	}
	if len(indexes) > 0 {
		return map[string][]models.DuplicateIndex{dbName: indexes}
	}
	return map[string][]models.DuplicateIndex{}
}

func round(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
