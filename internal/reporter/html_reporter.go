package reporter

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db-patrol/internal/models"
	"db-patrol/internal/utils"
)

//go:embed templates/report.html.tmpl
var templateFS embed.FS

// HTMLReporter HTML报告生成器
type HTMLReporter struct {
	outputDir string
}

// NewHTMLReporter 创建HTML报告生成器
func NewHTMLReporter(outputDir string) *HTMLReporter {
	return &HTMLReporter{outputDir: outputDir}
}

// Generate 生成HTML报告
func (r *HTMLReporter) Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error) {
	funcMap := template.FuncMap{
		"formatDatetime": formatDatetime,
		"formatSize":     utils.FormatSize,
		"lower":          strings.ToLower,
		"contains":       strings.Contains,
		"isPG":           isPGType,
		"formatNumber":   formatNumber,
		"add":            func(a, b int) int { return a + b },
		"safeHTML":       func(s string) template.HTML { return template.HTML(s) },
	}

	tmpl, err := template.New("report.html.tmpl").Funcs(funcMap).ParseFS(templateFS, "templates/report.html.tmpl")
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	basicInfo, _ := getMap(results, "basic_info")
	performance, _ := getMap(results, "performance")

	// 转换 basicInfo 中的 struct
	if basicInfo != nil {
		convertBasicInfoStructs(basicInfo)
	}

	// 转换 performance 中的 struct（必须在评分计算之前）
	if performance != nil {
		convertPerformanceStructs(performance)
	}

	instanceInfo, _ := getMap(basicInfo, "instance_info")
	databasesRaw, _ := getMap(basicInfo, "databases")
	tablesRaw, _ := getMap(basicInfo, "tables")

	healthScore := CalculateHealthScore(basicInfo, performance, databasesRaw, tablesRaw)
	keyFindings := GenerateKeyFindings(basicInfo, performance, databasesRaw, tablesRaw)

	// 转换健康评分为 map（模板使用小写字段名）
	healthScoreMap := map[string]interface{}{
		"score":          healthScore.Score,
		"level":          healthScore.Level,
		"label":          healthScore.Label,
		"summary":        healthScore.Summary,
		"issues":         healthScore.Issues,
		"problem_count":  healthScore.ProblemCount,
		"critical_count": healthScore.CriticalCount,
		"warning_count":  healthScore.WarningCount,
	}
	var detailsList []map[string]interface{}
	for _, d := range healthScore.Details {
		detailsList = append(detailsList, map[string]interface{}{
			"name": d.Name, "score": d.Score, "max_score": d.MaxScore,
			"status": d.Status, "detail": d.Detail,
		})
	}
	healthScoreMap["details"] = detailsList

	var findingsList []map[string]interface{}
	for _, f := range keyFindings {
		findingsList = append(findingsList, map[string]interface{}{
			"level": f.Level, "icon": f.Icon, "title": f.Title, "description": f.Description,
		})
	}

	// 连接状态
	connectionStatus := "未知"
	if cs, ok := getMap(basicInfo, "connection_status"); ok {
		if s, ok := cs["status"]; ok {
			connectionStatus = fmt.Sprintf("%v", s)
		}
	}

	databaseCount := toInt(databasesRaw["total"])
	tableCount := toInt(tablesRaw["total_count"])

	totalSize := "N/A"
	if instanceInfo != nil {
		if ts, ok := instanceInfo["total_size"]; ok && ts != nil {
			totalSize = fmt.Sprintf("%v", ts)
		}
	}

	version := fmtVal(basicInfo["version"])
	uptime := fmtVal(basicInfo["uptime"])

	settings := map[string]interface{}{}
	if s, ok := getMap(basicInfo, "settings"); ok {
		settings = s
	}

	// === 转换数据库列表（struct → map with lowercase keys）===
	databasesNormal := convertDBList(toSlice(databasesRaw["normal"]))
	databasesBackup := convertDBList(toSlice(databasesRaw["backup"]))

	// 数据库汇总
	databasesSummary := map[string]interface{}{}
	if len(databasesNormal) > 0 {
		totalSchema, totalTable, totalView, totalTrigger := 0, 0, 0, 0
		var totalSizeBytes int64
		for _, db := range databasesNormal {
			totalSchema += toInt(db["schema_count"])
			totalTable += toInt(db["table_count"])
			totalView += toInt(db["view_count"])
			totalTrigger += toInt(db["trigger_count"])
			totalSizeBytes += toInt64(db["size_bytes"])
		}
		databasesSummary["total_schema_count"] = totalSchema
		databasesSummary["total_table_count"] = totalTable
		databasesSummary["total_view_count"] = totalView
		databasesSummary["total_trigger_count"] = totalTrigger
		databasesSummary["total_size"] = utils.FormatSize(totalSizeBytes)
	}

	// 重建 databases map 供模板使用
	databases := map[string]interface{}{
		"total":                 databaseCount,
		"normal":                databasesNormal,
		"backup":                databasesBackup,
		"backup_count":          len(databasesBackup),
		"backup_total_size":     fmtVal(databasesRaw["backup_total_size"]),
		"backup_total_tables":   fmtVal(databasesRaw["backup_total_tables"]),
		"backup_total_views":    fmtVal(databasesRaw["backup_total_views"]),
		"backup_total_triggers": fmtVal(databasesRaw["backup_total_triggers"]),
	}

	// === 转换表列表（struct → map）===
	tablesNormal := convertTableList(toSlice(tablesRaw["normal"]))
	tablesBackup := convertTableList(toSlice(tablesRaw["backup"]))

	// 按数据库分组表
	sizeThreshold := int64(1 * 1024 * 1024 * 1024)
	minDisplayCount := 10
	maxDisplayCount := 50

	var tablesByDatabase []map[string]interface{}
	grouped := make(map[string][]map[string]interface{})
	for _, t := range tablesNormal {
		dbName := fmt.Sprintf("%v", t["database"])
		grouped[dbName] = append(grouped[dbName], t)
	}
	for dbName, dbTables := range grouped {
		sortTablesBySize(dbTables)
		var largeTables []map[string]interface{}
		for _, t := range dbTables {
			if toInt64(t["size_bytes"]) >= sizeThreshold {
				largeTables = append(largeTables, t)
			}
		}
		selected := dbTables
		if len(largeTables) >= minDisplayCount {
			selected = largeTables
		}
		if len(selected) > maxDisplayCount {
			selected = selected[:maxDisplayCount]
		}
		for _, t := range selected {
			if t["column_count"] == nil {
				t["column_count_display"] = "N/A"
			} else {
				t["column_count_display"] = fmt.Sprintf("%v", t["column_count"])
			}
		}
		tablesByDatabase = append(tablesByDatabase, map[string]interface{}{
			"name": dbName,
			"stats": map[string]interface{}{
				"total_count": len(dbTables), "display_count": len(selected),
				"large_count": len(largeTables), "threshold_mb": sizeThreshold / (1024 * 1024),
			},
			"tables": selected,
		})
	}

	// 重建 tables map
	tables := map[string]interface{}{
		"normal":            tablesNormal,
		"backup":            tablesBackup,
		"total_count":       toInt(tablesRaw["total_count"]),
		"backup_count":      len(tablesBackup),
		"backup_total_size": fmtVal(tablesRaw["backup_total_size"]),
	}

	backupTablesCount := len(tablesBackup)

	// === 无主键表（转为模板需要的格式）===
	var tablesWithoutPKList []map[string]interface{}
	if tablesWithoutPK, ok := getMap(basicInfo, "tables_without_pk"); ok {
		for dbName, tbls := range tablesWithoutPK {
			items := toSlice(tbls)
			if len(items) == 0 {
				continue
			}
			var names []string
			firstSchema := ""
			for _, t := range items {
				m := toMapInterface(t)
				names = append(names, fmt.Sprintf("%v", m["table_name"]))
				if firstSchema == "" {
					firstSchema = fmt.Sprintf("%v", m["schema"])
				}
			}
			tablesWithoutPKList = append(tablesWithoutPKList, map[string]interface{}{
				"name":         dbName,
				"first_schema": firstSchema,
				"count":        len(items),
				"table_names":  names,
			})
		}
	}

	// === 性能数据处理 ===
	// 添加 badge_class
	if cacheHit, ok := getMap(performance, "cache_hit_ratio"); ok {
		if ratio, ok := getFloat64(cacheHit, "ratio"); ok {
			if ratio >= 95 {
				cacheHit["badge_class"] = "success"
			} else if ratio >= 90 {
				cacheHit["badge_class"] = "warning"
			} else {
				cacheHit["badge_class"] = "error"
			}
		}
	}
	if indexHit, ok := getMap(performance, "index_hit_ratio"); ok {
		if ratio, ok := getFloat64(indexHit, "ratio"); ok {
			if ratio >= 70 {
				indexHit["badge_class"] = "success"
			} else if ratio >= 50 {
				indexHit["badge_class"] = "warning"
			} else {
				indexHit["badge_class"] = "error"
			}
		}
	}

	// 判断 PG vs MySQL 格式
	hasPID := false
	hasBlockedPID := false
	if longTxns, ok := getSlice(performance, "long_transactions"); ok && len(longTxns) > 0 {
		if m, ok := longTxns[0].(map[string]interface{}); ok {
			_, hasPID = m["pid"]
		}
	}
	if locks, ok := getSlice(performance, "locks"); ok && len(locks) > 0 {
		if m, ok := locks[0].(map[string]interface{}); ok {
			_, hasBlockedPID = m["blocked_pid"]
		}
	}
	// 死锁显示标志
	deadlockHasPID := false
	deadlockHasTrx := false
	if deadlocks, ok := getSlice(performance, "deadlocks"); ok && len(deadlocks) > 0 {
		if m, ok := deadlocks[0].(map[string]interface{}); ok {
			_, deadlockHasPID = m["pid"]
			_, deadlockHasTrx = m["trx_id"]
		}
	}

	if performance != nil {
		performance["has_pid"] = hasPID
		performance["has_blocked_pid"] = hasBlockedPID
		performance["deadlock_has_pid"] = deadlockHasPID
		performance["deadlock_has_trx"] = deadlockHasTrx
	}

	// 预计算长事务显示字段
	if longTxns, ok := getSlice(performance, "long_transactions"); ok {
		for _, t := range longTxns {
			if m, ok := t.(map[string]interface{}); ok {
				m["client_addr_display"] = firstNonEmpty(m["client_addr"], "local")
				m["wait_event_display"] = firstNonEmpty(m["wait_event"], "无")
			}
		}
	}

	// 将 map[string][]T 性能数据转换为 [{name, tables}] 格式（模板需要）
	for _, key := range []string{"dead_tuples", "vacuum_status", "io_stats", "invalid_indexes", "duplicate_indexes", "index_size_analysis"} {
		if m, ok := getMap(performance, key); ok && len(m) > 0 {
			var groups []map[string]interface{}
			for dbName, items := range m {
				// 预计算显示字段
				if arr, ok := items.([]interface{}); ok {
					for _, item := range arr {
						if tm, ok := item.(map[string]interface{}); ok {
							tm["last_vacuum_display"] = firstNonEmpty(tm["last_autovacuum"], tm["last_vacuum"], "从未")
							tm["last_analyze_display"] = firstNonEmpty(tm["last_autoanalyze"], tm["last_analyze"], "从未")
							if _, ok := tm["last_autovacuum_display"]; !ok {
								tm["last_autovacuum_display"] = firstNonEmpty(tm["last_autovacuum"], "从未")
							}
							if _, ok := tm["last_autoanalyze_display"]; !ok {
								tm["last_autoanalyze_display"] = firstNonEmpty(tm["last_autoanalyze"], "从未")
							}
						}
					}
				}
				groups = append(groups, map[string]interface{}{
					"name":   dbName,
					"tables": items,
				})
			}
			performance[key] = groups
		}
	}

	generatedAt := time.Now().Format("2006-01-02 15:04:05")

	context := map[string]interface{}{
		"report_title":           dbConfig.Name,
		"db_name":                dbConfig.Name,
		"db_type":                dbConfig.Type,
		"db_host":                dbConfig.Host,
		"db_port":                dbConfig.Port,
		"generated_at":           generatedAt,
		"connection_status":      connectionStatus,
		"database_count":         databaseCount,
		"table_count":            tableCount,
		"total_size":             totalSize,
		"instance_info":          instanceInfo,
		"version":                version,
		"uptime":                 uptime,
		"settings":               settings,
		"databases":              databases,
		"databases_summary":      databasesSummary,
		"tables":                 tables,
		"tables_by_database":     tablesByDatabase,
		"backup_tables":          tablesBackup,
		"backup_tables_count":    backupTablesCount,
		"backup_tables_total_size": fmtVal(tablesRaw["backup_total_size"]),
		"tables_without_pk":      tablesWithoutPKList,
		"performance":            performance,
		"health_score":           healthScoreMap,
		"key_findings":           findingsList,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, context); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	name := strings.ReplaceAll(dbConfig.Name, " ", "_")
	filename := fmt.Sprintf("db_inspection_%s_%s.html", name, time.Now().Format("20060102_150405"))
	fp := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(fp, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return fp, nil
}

// convertBasicInfoStructs 转换 basicInfo 中的 struct 为 map
func convertBasicInfoStructs(basicInfo map[string]interface{}) {
	// instance_info: models.InstanceInfo → map
	if v, ok := basicInfo["instance_info"]; ok {
		if s, ok := v.(models.InstanceInfo); ok {
			basicInfo["instance_info"] = map[string]interface{}{
				"full_version": s.FullVersion, "product_name": s.ProductName,
				"product_version": s.ProductVersion, "current_database": s.CurrentDatabase,
				"total_size": s.TotalSize, "total_size_bytes": s.TotalSizeBytes,
				"database_count": s.DatabaseCount, "max_connections": s.MaxConnections,
				"current_connections": s.CurrentConnections, "shared_buffers": s.SharedBuffers,
				"db_time": s.DBTime, "timezone": s.Timezone,
				"data_directory": s.DataDirectory, "listen_addresses": s.ListenAddresses,
				"port": s.Port, "case_sensitive": s.CaseSensitive, "encoding": s.Encoding,
			}
		}
	}
	// connection_status: models.ConnectionStatus → map
	if v, ok := basicInfo["connection_status"]; ok {
		if s, ok := v.(models.ConnectionStatus); ok {
			basicInfo["connection_status"] = map[string]interface{}{
				"status": s.Status, "message": s.Message,
			}
		}
	}
	// settings: map[string]string → map[string]interface{}
	if v, ok := basicInfo["settings"]; ok {
		if m, ok := v.(map[string]string); ok {
			newMap := make(map[string]interface{}, len(m))
			for k, val := range m {
				newMap[k] = val
			}
			basicInfo["settings"] = newMap
		}
	}
	// databases: 转换 internal struct slices
	if v, ok := basicInfo["databases"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			convertDatabaseMap(m)
		}
	}
	// tables: 转换 internal struct slices
	if v, ok := basicInfo["tables"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			convertTableMap(m)
		}
	}
	// tables_without_pk: 转换 internal struct slices
	if v, ok := basicInfo["tables_without_pk"]; ok {
		if m, ok := v.(map[string][]models.TableWithoutPK); ok {
			newMap := make(map[string]interface{})
			for k, items := range m {
				var newArr []interface{}
				for _, item := range items {
					newArr = append(newArr, map[string]interface{}{
						"schema": item.Schema, "table_name": item.TableName,
						"size": item.Size, "size_bytes": item.SizeBytes,
						"column_count": item.ColumnCount, "row_count": item.RowCount,
					})
				}
				newMap[k] = newArr
			}
			basicInfo["tables_without_pk"] = newMap
		}
	}
}

// convertDatabaseMap 转换 databases map 中的 struct 切片
func convertDatabaseMap(m map[string]interface{}) {
	for _, key := range []string{"normal", "backup"} {
		if v, ok := m[key]; ok {
			if arr, ok := v.([]models.DatabaseInfo); ok {
				newArr := make([]interface{}, len(arr))
				for i, db := range arr {
					newArr[i] = map[string]interface{}{
						"name": db.Name, "size": db.Size, "size_bytes": db.SizeBytes,
						"encoding": db.Encoding, "collation": db.Collation, "ctype": db.Ctype,
						"schema_count": db.SchemaCount, "table_count": db.TableCount,
						"view_count": db.ViewCount, "trigger_count": db.TriggerCount,
						"is_backup": db.IsBackup,
					}
				}
				m[key] = newArr
			}
		}
	}
}

// convertTableMap 转换 tables map 中的 struct 切片
func convertTableMap(m map[string]interface{}) {
	for _, key := range []string{"normal", "backup", "all"} {
		if v, ok := m[key]; ok {
			if arr, ok := v.([]models.TableInfo); ok {
				newArr := make([]interface{}, len(arr))
				for i, t := range arr {
					newArr[i] = map[string]interface{}{
						"database": t.Database, "schema": t.Schema, "table_name": t.TableName,
						"size": t.Size, "size_bytes": t.SizeBytes, "column_count": t.ColumnCount,
						"row_count": t.RowCount, "is_backup": t.IsBackup, "engine": t.Engine,
					}
				}
				m[key] = newArr
			}
		}
	}
}

// convertPerformanceStructs 将性能 map 中的 struct 值转为 map[string]interface{}
func convertPerformanceStructs(perf map[string]interface{}) {
	// 简单 struct → map 转换
	structToMap := func(v interface{}) interface{} {
		switch s := v.(type) {
		case models.ConnectionStats:
			return map[string]interface{}{
				"current": s.Current, "max": s.Max, "active": s.Active, "idle": s.Idle,
				"usage_percent": s.UsagePercent, "status": s.Status,
			}
		case models.CacheHitRatio:
			return map[string]interface{}{
				"ratio": s.Ratio, "heap_read": s.HeapRead, "heap_hit": s.HeapHit,
				"status": s.Status, "suggestion": s.Suggestion,
			}
		case models.IndexHitRatio:
			return map[string]interface{}{
				"ratio": s.Ratio, "idx_scan": s.IdxScan, "seq_scan": s.SeqScan,
				"status": s.Status, "suggestion": s.Suggestion,
			}
		case models.ClientConnection:
			return map[string]interface{}{
				"client_ip": s.ClientIP, "total_connections": s.TotalConnections,
				"database_count": s.DatabaseCount, "databases": s.Databases,
				"user_count": s.UserCount, "users": s.Users,
				"application_count": s.ApplicationCount, "applications": s.Applications,
				"active": s.Active, "idle": s.Idle,
				"idle_in_transaction": s.IdleInTransaction,
			}
		case models.LockInfo:
			return map[string]interface{}{
				"blocked_pid": s.BlockedPID, "blocked_user": s.BlockedUser,
				"blocking_pid": s.BlockingPID, "blocking_user": s.BlockingUser,
				"database": s.Database, "application_name": s.ApplicationName,
				"client_addr": s.ClientAddr, "locktype": s.LockType,
				"blocked_mode": s.BlockedMode, "blocking_mode": s.BlockingMode,
				"blocked_state": s.BlockedState, "wait_seconds": s.WaitSeconds,
				"blocked_query": s.BlockedQuery, "blocking_query": s.BlockingQuery,
				"severity": s.Severity, "severity_label": s.SeverityLabel,
				"wait_display": s.WaitDisplay,
				"waiting_trx_id": s.WaitingTrxID, "waiting_thread": s.WaitingThread,
				"blocking_trx_id": s.BlockingTrxID, "blocking_thread": s.BlockingThread,
				"waiting_lock_mode": s.WaitingLockMode, "waiting_lock_type": s.WaitingLockType,
			}
		case models.LongTransaction:
			return map[string]interface{}{
				"pid": s.PID, "database": s.Database, "username": s.Username,
				"client_addr": s.ClientAddr, "application_name": s.ApplicationName,
				"state": s.State, "query_start": s.QueryStart,
				"duration_seconds": s.DurationSeconds, "severity": s.Severity,
				"severity_label": s.SeverityLabel, "duration_display": s.DurationDisplay,
				"query": s.Query, "trx_id": s.TrxID, "thread_id": s.ThreadID,
				"tables_locked": s.TablesLocked, "rows_locked": s.RowsLocked,
			}
		case models.DeadlockInfo:
			return map[string]interface{}{
				"pid": s.PID, "database": s.Database, "username": s.Username,
				"application_name": s.ApplicationName, "client_addr": s.ClientAddr,
				"state": s.State, "wait_event": s.WaitEvent, "query": s.Query,
				"duration_seconds": s.DurationSeconds, "trx_id": s.TrxID,
				"thread_id": s.ThreadID, "state2": s.State2,
				"tables_locked": s.TablesLocked, "rows_locked": s.RowsLocked,
				"deadlock_count": s.DeadlockCount, "severity": s.Severity,
				"severity_label": s.SeverityLabel, "duration_display": s.DurationDisplay,
				"suggestion": s.Suggestion,
			}
		case models.DeadTupleInfo:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"live_tuples": s.LiveTuples, "dead_tuples": s.DeadTuples,
				"dead_tuple_ratio": s.DeadTupleRatio,
				"last_vacuum": s.LastVacuum, "last_autovacuum": s.LastAutovacuum,
				"last_analyze": s.LastAnalyze, "last_autoanalyze": s.LastAutoanalyze,
				"table_size": s.TableSize, "severity": s.Severity,
				"severity_label": s.SeverityLabel, "suggestion": s.Suggestion,
			}
		case models.VacuumStatus:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"live_tuples": s.LiveTuples,
				"last_vacuum": s.LastVacuum, "last_autovacuum": s.LastAutovacuum,
				"last_analyze": s.LastAnalyze, "last_autoanalyze": s.LastAutoanalyze,
				"vacuum_status": s.VacuumStatus, "severity": s.Severity,
				"suggestion": s.Suggestion,
			}
		case models.IOStats:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"disk_reads": s.DiskReads, "buffer_hits": s.BufferHits,
				"index_disk_reads": s.IndexDiskReads, "index_buffer_hits": s.IndexBufferHits,
				"toast_disk_reads": s.ToastDiskReads, "toast_buffer_hits": s.ToastBufferHits,
				"cache_hit_ratio": s.CacheHitRatio, "table_size": s.TableSize,
				"io_level": s.IOLevel, "io_label": s.IOLabel, "suggestion": s.Suggestion,
			}
		case models.IndexSizeAnalysis:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"row_count": s.RowCount, "table_size": s.TableSize,
				"index_size": s.IndexSize, "total_size": s.TotalSize,
				"table_size_bytes": s.TableSizeBytes, "index_size_bytes": s.IndexSizeBytes,
				"total_size_bytes": s.TotalSizeBytes, "index_ratio": s.IndexRatio,
				"index_count": s.IndexCount, "attention": s.Attention,
				"suggestion": s.Suggestion,
			}
		case models.InvalidIndex:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"index_name": s.IndexName, "index_size": s.IndexSize,
				"index_scans": s.IndexScans, "issue_type": s.IssueType,
				"suggestion": s.Suggestion, "database": s.Database,
			}
		case models.DuplicateIndex:
			return map[string]interface{}{
				"schemaname": s.Schema, "table_name": s.TableName,
				"index_name": s.IndexName, "index_definition": s.IndexDefinition,
				"index_size": s.IndexSize, "index_scans": s.IndexScans,
				"suggestion": s.Suggestion, "database": s.Database,
			}
		case models.InstanceInfo:
			return map[string]interface{}{
				"full_version": s.FullVersion, "product_name": s.ProductName,
				"product_version": s.ProductVersion, "current_database": s.CurrentDatabase,
				"total_size": s.TotalSize, "total_size_bytes": s.TotalSizeBytes,
				"database_count": s.DatabaseCount, "max_connections": s.MaxConnections,
				"current_connections": s.CurrentConnections, "shared_buffers": s.SharedBuffers,
				"db_time": s.DBTime, "timezone": s.Timezone,
				"data_directory": s.DataDirectory, "listen_addresses": s.ListenAddresses,
				"port": s.Port, "case_sensitive": s.CaseSensitive, "encoding": s.Encoding,
			}
		}
		return v
	}

	// 转换顶层 struct 值
	simpleKeys := []string{"connections", "cache_hit_ratio", "index_hit_ratio", "instance_info"}
	for _, key := range simpleKeys {
		if v, ok := perf[key]; ok {
			perf[key] = structToMap(v)
		}
	}

	// 转换 slice 中的 struct
	sliceKeys := []string{"client_connections", "locks", "long_transactions", "deadlocks"}
	for _, key := range sliceKeys {
		if v, ok := perf[key]; ok {
			if arr, ok := v.([]interface{}); ok {
				for i, item := range arr {
					arr[i] = structToMap(item)
				}
			}
			// 也处理具体类型的 slice
			if arr, ok := v.([]models.ClientConnection); ok {
				newArr := make([]interface{}, len(arr))
				for i, item := range arr {
					newArr[i] = structToMap(item)
				}
				perf[key] = newArr
			}
			if arr, ok := v.([]models.LockInfo); ok {
				newArr := make([]interface{}, len(arr))
				for i, item := range arr {
					newArr[i] = structToMap(item)
				}
				perf[key] = newArr
			}
			if arr, ok := v.([]models.LongTransaction); ok {
				newArr := make([]interface{}, len(arr))
				for i, item := range arr {
					newArr[i] = structToMap(item)
				}
				perf[key] = newArr
			}
			if arr, ok := v.([]models.DeadlockInfo); ok {
				newArr := make([]interface{}, len(arr))
				for i, item := range arr {
					newArr[i] = structToMap(item)
				}
				perf[key] = newArr
			}
		}
	}

	// 转换 map[string][]T 类型的值
	mapKeys := []string{"dead_tuples", "vacuum_status", "io_stats", "invalid_indexes", "duplicate_indexes", "index_size_analysis"}
	for _, key := range mapKeys {
		if v, ok := perf[key]; ok {
			if m, ok := v.(map[string]interface{}); ok {
				for dbName, items := range m {
					m[dbName] = convertSliceItems(items, structToMap)
				}
			}
			// 处理具体类型的 map
			if m, ok := v.(map[string][]models.DeadTupleInfo); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			if m, ok := v.(map[string][]models.VacuumStatus); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			if m, ok := v.(map[string][]models.IOStats); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			if m, ok := v.(map[string][]models.InvalidIndex); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			if m, ok := v.(map[string][]models.DuplicateIndex); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			if m, ok := v.(map[string][]models.IndexSizeAnalysis); ok {
				newMap := make(map[string]interface{})
				for k, items := range m {
					var newArr []interface{}
					for _, item := range items {
						newArr = append(newArr, structToMap(item))
					}
					newMap[k] = newArr
				}
				perf[key] = newMap
			}
			// 处理 map[string]interface{} 其中 value 是具体类型的 slice
			if m, ok := v.(map[string]interface{}); ok {
				for dbName, items := range m {
					switch arr := items.(type) {
					case []models.IndexSizeAnalysis:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					case []models.DeadTupleInfo:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					case []models.VacuumStatus:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					case []models.IOStats:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					case []models.InvalidIndex:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					case []models.DuplicateIndex:
						newArr := make([]interface{}, len(arr))
						for i, item := range arr {
							newArr[i] = structToMap(item)
						}
						m[dbName] = newArr
					}
				}
			}
		}
	}
}

func convertSliceItems(v interface{}, fn func(interface{}) interface{}) interface{} {
	if arr, ok := v.([]interface{}); ok {
		for i, item := range arr {
			arr[i] = fn(item)
		}
		return arr
	}
	return v
}

// ==================== 数据转换函数 ====================

// convertDBList 将 []models.DatabaseInfo 或 []interface{} 转换为 []map[string]interface{}
func convertDBList(items []interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, item := range items {
		m := toMapInterface(item)
		entry := map[string]interface{}{
			"name":           m["name"],
			"size":           m["size"],
			"size_bytes":     m["size_bytes"],
			"encoding":       m["encoding"],
			"collation":      m["collation"],
			"ctype":          m["ctype"],
			"schema_count":   m["schema_count"],
			"table_count":    m["table_count"],
			"view_count":     m["view_count"],
			"trigger_count":  m["trigger_count"],
			"is_backup":      m["is_backup"],
		}
		if entry["schema_count"] == nil {
			entry["schema_count"] = 0
		}
		entry["schema_count_display"] = fmt.Sprintf("%v", entry["schema_count"])
		result = append(result, entry)
	}
	return result
}

// convertTableList 将 []models.TableInfo 或 []interface{} 转换为 []map[string]interface{}
func convertTableList(items []interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	for _, item := range items {
		m := toMapInterface(item)
		entry := map[string]interface{}{
			"database":     m["database"],
			"schema":       m["schema"],
			"table_name":   m["table_name"],
			"size":         m["size"],
			"size_bytes":   m["size_bytes"],
			"column_count": m["column_count"],
			"row_count":    m["row_count"],
			"is_backup":    m["is_backup"],
			"engine":       m["engine"],
		}
		result = append(result, entry)
	}
	return result
}

// toMapInterface 将 struct 或 map 转换为 map[string]interface{}
func toMapInterface(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	// 处理 models.DatabaseInfo
	if db, ok := v.(models.DatabaseInfo); ok {
		return map[string]interface{}{
			"name": db.Name, "size": db.Size, "size_bytes": db.SizeBytes,
			"encoding": db.Encoding, "collation": db.Collation, "ctype": db.Ctype,
			"schema_count": db.SchemaCount, "table_count": db.TableCount,
			"view_count": db.ViewCount, "trigger_count": db.TriggerCount,
			"is_backup": db.IsBackup,
		}
	}
	// 处理 models.TableInfo
	if t, ok := v.(models.TableInfo); ok {
		return map[string]interface{}{
			"database": t.Database, "schema": t.Schema, "table_name": t.TableName,
			"size": t.Size, "size_bytes": t.SizeBytes, "column_count": t.ColumnCount,
			"row_count": t.RowCount, "is_backup": t.IsBackup, "engine": t.Engine,
		}
	}
	// 处理 models.TableWithoutPK
	if t, ok := v.(models.TableWithoutPK); ok {
		return map[string]interface{}{
			"schema": t.Schema, "table_name": t.TableName, "size": t.Size,
			"size_bytes": t.SizeBytes, "column_count": t.ColumnCount,
			"row_count": t.RowCount,
		}
	}
	// fallback: 尝试 fmt
	return map[string]interface{}{}
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	// 处理具体类型的 slice
	if arr, ok := v.([]models.DatabaseInfo); ok {
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result
	}
	if arr, ok := v.([]models.TableInfo); ok {
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result
	}
	if arr, ok := v.([]models.TableWithoutPK); ok {
		result := make([]interface{}, len(arr))
		for i, item := range arr {
			result[i] = item
		}
		return result
	}
	return nil
}

// ==================== 辅助函数 ====================

func formatDatetime(v interface{}) string {
	if v == nil {
		return "N/A"
	}
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case *time.Time:
		if t != nil {
			return t.Format("2006-01-02 15:04:05")
		}
	case string:
		return t
	}
	return fmt.Sprintf("%v", v)
}

func isPGType(dbType string) bool {
	lower := strings.ToLower(dbType)
	return strings.Contains(lower, "pg") || strings.Contains(lower, "postgres")
}

func formatNumber(v interface{}) string {
	var n int64
	switch val := v.(type) {
	case int:
		n = int64(val)
	case int64:
		n = val
	case float64:
		n = int64(val)
	case float32:
		n = int64(val)
	default:
		return "0"
	}
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func fmtVal(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func firstNonEmpty(values ...interface{}) string {
	for _, v := range values {
		if v != nil {
			s := fmt.Sprintf("%v", v)
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return "从未"
}

func sortTablesBySize(tables []map[string]interface{}) {
	for i := 0; i < len(tables); i++ {
		for j := i + 1; j < len(tables); j++ {
			if toInt64(tables[i]["size_bytes"]) < toInt64(tables[j]["size_bytes"]) {
				tables[i], tables[j] = tables[j], tables[i]
			}
		}
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	}
	return 0
}
