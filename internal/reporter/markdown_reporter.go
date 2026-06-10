package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"db-patrol/internal/models"
	"db-patrol/internal/utils"
)

// MarkdownReporter Markdown报告生成器
type MarkdownReporter struct {
	outputDir string
}

// NewMarkdownReporter 创建Markdown报告生成器
func NewMarkdownReporter(outputDir string) *MarkdownReporter {
	return &MarkdownReporter{outputDir: outputDir}
}

// Generate 生成Markdown报告
func (r *MarkdownReporter) Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error) {
	basicInfo, _ := getMap(results, "basic_info")
	performance, _ := getMap(results, "performance")

	// 转换 struct 为 map（必须在评分计算之前）
	if basicInfo != nil {
		convertBasicInfoStructs(basicInfo)
	}
	if performance != nil {
		convertPerformanceStructs(performance)
	}

	databases, _ := getMap(basicInfo, "databases")
	tables, _ := getMap(basicInfo, "tables")

	healthScore := CalculateHealthScore(basicInfo, performance, databases, tables)
	keyFindings := GenerateKeyFindings(basicInfo, performance, databases, tables)

	var sb strings.Builder

	// ====== 1. 标题 ======
	sb.WriteString(fmt.Sprintf("# 数据库巡检报告 - %s\n\n", dbConfig.Name))
	sb.WriteString(fmt.Sprintf("- **实例名称**: %s\n", dbConfig.Name))
	sb.WriteString(fmt.Sprintf("- **数据库类型**: %s\n", dbConfig.Type))
	sb.WriteString(fmt.Sprintf("- **连接地址**: %s:%d\n", dbConfig.Host, dbConfig.Port))
	sb.WriteString(fmt.Sprintf("- **生成时间**: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// ====== 2. 概览卡片 ======
	instanceInfo, _ := getMap(basicInfo, "instance_info")
	connectionStatus := "未知"
	if cs, ok := getMap(basicInfo, "connection_status"); ok {
		if s, ok := cs["status"]; ok {
			connectionStatus = fmt.Sprintf("%v", s)
		}
	}
	databaseCount := toInt(databases["total"])
	tableCount := toInt(tables["total_count"])
	totalSize := "N/A"
	if instanceInfo != nil {
		if ts, ok := instanceInfo["total_size"]; ok && ts != nil {
			totalSize = fmt.Sprintf("%v", ts)
		}
	}

	sb.WriteString("## 概览\n\n")
	sb.WriteString(fmt.Sprintf("| 指标 | 值 |\n"))
	sb.WriteString(fmt.Sprintf("|------|-----|\n"))
	sb.WriteString(fmt.Sprintf("| 连接状态 | %s |\n", connectionStatus))
	sb.WriteString(fmt.Sprintf("| 数据库总数 | %d |\n", databaseCount))
	sb.WriteString(fmt.Sprintf("| 表总数 | %d |\n", tableCount))
	sb.WriteString(fmt.Sprintf("| 实例总大小 | %s |\n\n", totalSize))

	// ====== 3. 健康评分 ======
	sb.WriteString(fmt.Sprintf("## 健康评分: %d分 (%s)\n\n", healthScore.Score, healthScore.Label))
	sb.WriteString(fmt.Sprintf("%s\n\n", healthScore.Summary))

	if len(healthScore.Details) > 0 {
		sb.WriteString("### 评分明细\n\n")
		sb.WriteString("| 检查项 | 得分 | 状态 | 详情 |\n")
		sb.WriteString("|--------|------|------|------|\n")
		for _, d := range healthScore.Details {
			statusLabel := d.Status
			switch d.Status {
			case "excellent":
				statusLabel = "优秀"
			case "good":
				statusLabel = "良好"
			case "warning":
				statusLabel = "警告"
			case "critical":
				statusLabel = "严重"
			}
			sb.WriteString(fmt.Sprintf("| %s | %d/%d | %s | %s |\n", d.Name, d.Score, d.MaxScore, statusLabel, d.Detail))
		}
		sb.WriteString("\n")
	}

	// ====== 4. 关键发现 ======
	if len(keyFindings) > 0 {
		sb.WriteString("## 关键发现与建议\n\n")
		for _, f := range keyFindings {
			sb.WriteString(fmt.Sprintf("### %s %s\n\n", f.Icon, f.Title))
			sb.WriteString(fmt.Sprintf("%s\n\n", f.Description))
		}
	}

	// ====== 5. 实例基本信息 ======
	if instanceInfo != nil {
		sb.WriteString("## 实例基本信息\n\n")
		fields := []struct{ key, label string }{
			{"product_name", "产品名称"},
			{"full_version", "完整版本"},
			{"product_version", "产品版本"},
			{"current_database", "当前数据库"},
			{"total_size", "实例总大小"},
			{"database_count", "数据库数量"},
			{"max_connections", "最大连接数"},
			{"current_connections", "当前连接数"},
			{"shared_buffers", "共享缓冲区"},
			{"data_directory", "数据目录"},
			{"listen_addresses", "监听地址"},
			{"port", "端口"},
			{"timezone", "时区"},
			{"encoding", "编码"},
			{"case_sensitive", "大小写敏感"},
		}
		for _, f := range fields {
			if v, ok := instanceInfo[f.key]; ok && v != nil {
				val := fmt.Sprintf("%v", v)
				if val != "" && val != "<nil>" {
					sb.WriteString(fmt.Sprintf("- **%s**: %s\n", f.label, val))
				}
			}
		}
		sb.WriteString("\n")
	}

	// ====== 6. 版本和运行时间 ======
	version := fmtVal(basicInfo["version"])
	uptime := fmtVal(basicInfo["uptime"])
	if version != "" || uptime != "" {
		sb.WriteString("## 运行信息\n\n")
		if version != "" {
			sb.WriteString(fmt.Sprintf("- **版本**: %s\n", version))
		}
		if uptime != "" {
			sb.WriteString(fmt.Sprintf("- **运行时间**: %s\n", uptime))
		}
		sb.WriteString("\n")
	}

	// ====== 7. 实例配置 ======
	if settings, ok := getMap(basicInfo, "settings"); ok && len(settings) > 0 {
		sb.WriteString("## 实例配置\n\n")
		sb.WriteString("| 配置项 | 值 |\n")
		sb.WriteString("|--------|-----|\n")
		// 按 key 排序输出
		keys := make([]string, 0, len(settings))
		for k := range settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("| %s | %v |\n", k, settings[k]))
		}
		sb.WriteString("\n")
	}

	// ====== 8. 数据库列表 ======
	if dbs, ok := getSlice(databases, "normal"); ok && len(dbs) > 0 {
		sb.WriteString("## 数据库列表\n\n")
		sb.WriteString("| 序号 | 数据库名 | 大小 | Schema数 | 表数 | 视图数 | 触发器数 |\n")
		sb.WriteString("|------|----------|------|----------|------|--------|----------|\n")
		for i, db := range dbs {
			m := toMapInterface(db)
			sb.WriteString(fmt.Sprintf("| %d | %v | %v | %v | %v | %v | %v |\n",
				i+1, m["name"], m["size"], m["schema_count"], m["table_count"], m["view_count"], m["trigger_count"]))
		}
		// 汇总行
		totalSchema, totalTable, totalView, totalTrigger := 0, 0, 0, 0
		var totalSizeBytes int64
		for _, db := range dbs {
			m := toMapInterface(db)
			totalSchema += toInt(m["schema_count"])
			totalTable += toInt(m["table_count"])
			totalView += toInt(m["view_count"])
			totalTrigger += toInt(m["trigger_count"])
			totalSizeBytes += toInt64(m["size_bytes"])
		}
		sb.WriteString(fmt.Sprintf("| **合计** | - | **%s** | **%d** | **%d** | **%d** | **%d** |\n\n",
			utils.FormatSize(totalSizeBytes), totalSchema, totalTable, totalView, totalTrigger))
	}

	// ====== 9. 疑似备份库 ======
	if dbs, ok := getSlice(databases, "backup"); ok && len(dbs) > 0 {
		sb.WriteString("## ⚠️ 疑似备份库 (共 " + fmt.Sprintf("%d", len(dbs)) + " 个)\n\n")
		sb.WriteString("以下数据库疑似为备份库，建议定期清理以释放存储空间。\n\n")
		sb.WriteString("| 数据库名 | 大小 | 表数 | 视图数 |\n")
		sb.WriteString("|----------|------|------|--------|\n")
		for _, db := range dbs {
			m := toMapInterface(db)
			sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v |\n", m["name"], m["size"], m["table_count"], m["view_count"]))
		}
		sb.WriteString("\n")
	}

	// ====== 10. 数据表清单 (按库分组，只显示大表) ======
	if tbls, ok := getSlice(tables, "normal"); ok && len(tbls) > 0 {
		// 按数据库分组
		grouped := make(map[string][]map[string]interface{})
		for _, t := range tbls {
			m := toMapInterface(t)
			dbName := fmt.Sprintf("%v", m["database"])
			grouped[dbName] = append(grouped[dbName], m)
		}

		sizeThreshold := int64(1 * 1024 * 1024 * 1024) // 1GB
		sb.WriteString("## 数据表清单 (≥1GB)\n\n")
		for dbName, dbTables := range grouped {
			// 筛选大表并排序
			var largeTables []map[string]interface{}
			for _, t := range dbTables {
				if toInt64(t["size_bytes"]) >= sizeThreshold {
					largeTables = append(largeTables, t)
				}
			}
			if len(largeTables) == 0 {
				continue
			}
			// 按大小排序
			sort.Slice(largeTables, func(i, j int) bool {
				return toInt64(largeTables[i]["size_bytes"]) > toInt64(largeTables[j]["size_bytes"])
			})
			// 限制显示
			display := largeTables
			if len(display) > 50 {
				display = display[:50]
			}

			sb.WriteString(fmt.Sprintf("### %s (%d 个大表，共 %d 张表)\n\n", dbName, len(largeTables), len(dbTables)))
			sb.WriteString("| 序号 | 模式 | 表名 | 大小 | 行数 | 列数 |\n")
			sb.WriteString("|------|------|------|------|------|------|\n")
			for i, t := range display {
				sb.WriteString(fmt.Sprintf("| %d | %v | %v | %v | %v | %v |\n",
					i+1, t["schema"], t["table_name"], t["size"], t["row_count"], t["column_count"]))
			}
			if len(largeTables) > 50 {
				sb.WriteString(fmt.Sprintf("\n> ... 还有 %d 个大表未显示\n\n", len(largeTables)-50))
			} else {
				sb.WriteString("\n")
			}
		}
	}

	// ====== 11. 疑似备份表 ======
	if tbls, ok := getSlice(tables, "backup"); ok && len(tbls) > 0 {
		sb.WriteString("## ⚠️ 疑似备份表 (共 " + fmt.Sprintf("%d", len(tbls)) + " 个)\n\n")
		sb.WriteString("以下表疑似为备份表，建议定期清理以释放存储空间。\n\n")
		// 按数据库分组
		grouped := make(map[string][]map[string]interface{})
		for _, t := range tbls {
			m := toMapInterface(t)
			dbName := fmt.Sprintf("%v", m["database"])
			grouped[dbName] = append(grouped[dbName], m)
		}
		for dbName, dbTables := range grouped {
			sb.WriteString(fmt.Sprintf("### %s (%d 个)\n\n", dbName, len(dbTables)))
			sb.WriteString("| 模式 | 表名 | 大小 | 行数 |\n")
			sb.WriteString("|------|------|------|------|\n")
			display := dbTables
			if len(display) > 100 {
				display = display[:100]
			}
			for _, t := range display {
				sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v |\n", t["schema"], t["table_name"], t["size"], t["row_count"]))
			}
			if len(dbTables) > 100 {
				sb.WriteString(fmt.Sprintf("\n> ... 还有 %d 个备份表未显示\n", len(dbTables)-100))
			}
			sb.WriteString("\n")
		}
	}

	// ====== 12. 缺少主键的表 ======
	if tablesWithoutPK, ok := getMap(basicInfo, "tables_without_pk"); ok {
		totalWithoutPK := 0
		var pkList []map[string]interface{}
		for dbName, tbls := range tablesWithoutPK {
			items := toSlice(tbls)
			if len(items) == 0 {
				continue
			}
			totalWithoutPK += len(items)
			var names []string
			firstSchema := ""
			for _, t := range items {
				m := toMapInterface(t)
				names = append(names, fmt.Sprintf("%v", m["table_name"]))
				if firstSchema == "" {
					firstSchema = fmt.Sprintf("%v", m["schema"])
				}
			}
			pkList = append(pkList, map[string]interface{}{
				"database": dbName, "schema": firstSchema,
				"count": len(items), "tables": names,
			})
		}
		if totalWithoutPK > 0 {
			sb.WriteString(fmt.Sprintf("## ️ 缺少主键或唯一索引的表 (共 %d 个)\n\n", totalWithoutPK))
			sb.WriteString("以下表缺少主键或唯一索引，建议添加以保证数据完整性和查询性能。\n\n")
			sb.WriteString("| 数据库 | 模式 | 数量 | 表名 |\n")
			sb.WriteString("|--------|------|------|------|\n")
			for _, p := range pkList {
				names := p["tables"].([]string)
				nameStr := strings.Join(names, "、")
				sb.WriteString(fmt.Sprintf("| %v | %v | %d | %s |\n", p["database"], p["schema"], p["count"], nameStr))
			}
			sb.WriteString("\n")
		}
	}

	// ====== 13. 性能指标 ======
	sb.WriteString("## 性能指标\n\n")

	// 连接使用率
	if conn, ok := getMap(performance, "connections"); ok {
		sb.WriteString("### 连接统计\n\n")
		sb.WriteString(fmt.Sprintf("- **当前连接数**: %v\n", conn["current"]))
		sb.WriteString(fmt.Sprintf("- **最大连接数**: %v\n", conn["max"]))
		sb.WriteString(fmt.Sprintf("- **活跃连接**: %v\n", conn["active"]))
		sb.WriteString(fmt.Sprintf("- **空闲连接**: %v\n", conn["idle"]))
		if usage, ok := getFloat64(conn, "usage_percent"); ok {
			sb.WriteString(fmt.Sprintf("- **连接使用率**: %.1f%%\n", usage))
		}
		sb.WriteString("\n")
	}

	// 缓存命中率
	if cacheHit, ok := getMap(performance, "cache_hit_ratio"); ok {
		sb.WriteString("### 缓存命中率\n\n")
		if ratio, ok := getFloat64(cacheHit, "ratio"); ok {
			sb.WriteString(fmt.Sprintf("- **命中率**: %.2f%%\n", ratio))
		}
		sb.WriteString(fmt.Sprintf("- **Heap Read**: %v\n", cacheHit["heap_read"]))
		sb.WriteString(fmt.Sprintf("- **Heap Hit**: %v\n", cacheHit["heap_hit"]))
		if s, ok := cacheHit["status"]; ok {
			sb.WriteString(fmt.Sprintf("- **状态**: %v\n", s))
		}
		if s, ok := cacheHit["suggestion"]; ok {
			sb.WriteString(fmt.Sprintf("- **建议**: %v\n", s))
		}
		sb.WriteString("\n")
	}

	// 索引命中率
	if indexHit, ok := getMap(performance, "index_hit_ratio"); ok {
		sb.WriteString("### 索引命中率\n\n")
		if ratio, ok := getFloat64(indexHit, "ratio"); ok {
			sb.WriteString(fmt.Sprintf("- **命中率**: %.2f%%\n", ratio))
		}
		sb.WriteString(fmt.Sprintf("- **索引扫描**: %v\n", indexHit["idx_scan"]))
		sb.WriteString(fmt.Sprintf("- **顺序扫描**: %v\n", indexHit["seq_scan"]))
		if s, ok := indexHit["status"]; ok {
			sb.WriteString(fmt.Sprintf("- **状态**: %v\n", s))
		}
		if s, ok := indexHit["suggestion"]; ok {
			sb.WriteString(fmt.Sprintf("- **建议**: %v\n", s))
		}
		sb.WriteString("\n")
	}

	// 客户端连接详情
	if clients, ok := getSlice(performance, "client_connections"); ok && len(clients) > 0 {
		sb.WriteString("### 客户端连接详情\n\n")
		sb.WriteString("| 客户端IP | 总连接 | 数据库数 | 用户数 | 应用数 | 活跃 | 空闲 | 空闲(事务中) |\n")
		sb.WriteString("|----------|--------|----------|--------|--------|------|------|-------------|\n")
		for _, c := range clients {
			m := toMapInterface(c)
			sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %v | %v | %v |\n",
				m["client_ip"], m["total_connections"], m["database_count"],
				m["user_count"], m["application_count"], m["active"], m["idle"], m["idle_in_transaction"]))
		}
		sb.WriteString("\n")
	}

	// 长事务
	sb.WriteString("### 🕒 长事务检测\n\n")
	if longTxns, ok := getSlice(performance, "long_transactions"); ok && len(longTxns) > 0 {
		sb.WriteString("| 数据库 | 用户 | 客户端 | 状态 | 持续时间 | 查询 |\n")
		sb.WriteString("|--------|------|--------|------|----------|------|\n")
		for _, t := range longTxns {
			m := toMapInterface(t)
			sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %s |\n",
				m["database"], m["username"], m["client_addr"], m["state"],
				m["duration_display"], truncateStr(fmt.Sprintf("%v", m["query"]), 80)))
		}
	} else {
		sb.WriteString("✅ 未发现长事务,所有事务运行时间正常\n")
	}
	sb.WriteString("\n")

	// 锁等待
	sb.WriteString("### 🔒 锁等待分析\n\n")
	if locks, ok := getSlice(performance, "locks"); ok && len(locks) > 0 {
		sb.WriteString("| 数据库 | 阻塞用户 | 阻塞PID | 等待用户 | 等待PID | 等待时间 | 阻塞模式 | 等待模式 |\n")
		sb.WriteString("|--------|----------|---------|----------|---------|----------|----------|----------|\n")
		for _, l := range locks {
			m := toMapInterface(l)
			sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %v | %v | %v |\n",
				m["database"], m["blocking_user"], m["blocking_pid"],
				m["blocked_user"], m["blocked_pid"], m["wait_display"],
				m["blocking_mode"], m["blocked_mode"]))
		}
	} else {
		sb.WriteString("✅ 未发现锁等待,无会话被阻塞\n")
	}
	sb.WriteString("\n")

	// 死锁检测
	sb.WriteString("### 💀 死锁检测\n\n")
	if deadlocks, ok := getSlice(performance, "deadlocks"); ok && len(deadlocks) > 0 {
		sb.WriteString("| 数据库 | 死锁次数 | 严重程度 | 建议 |\n")
		sb.WriteString("|--------|----------|----------|------|\n")
		for _, d := range deadlocks {
			m := toMapInterface(d)
			if m["deadlock_count"] != nil && m["deadlock_count"].(int64) > 0 {
				sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v |\n",
					m["database"], m["deadlock_count"], m["severity_label"], m["suggestion"]))
			}
		}
		// 显示潜在死锁(长时间锁等待)
		hasPotential := false
		for _, d := range deadlocks {
			m := toMapInterface(d)
			if m["pid"] != nil || m["trx_id"] != nil {
				if !hasPotential {
					sb.WriteString("\n**潜在死锁(长时间锁等待/长事务):**\n\n")
					sb.WriteString("| 类型 | 标识 | 数据库 | 持续时间 | 建议 |\n")
					sb.WriteString("|------|------|--------|----------|------|\n")
					hasPotential = true
				}
				id := ""
				typ := "PG"
				if m["pid"] != nil {
					id = fmt.Sprintf("PID:%v", m["pid"])
				} else {
					id = fmt.Sprintf("TrxID:%v", m["trx_id"])
					typ = "MySQL"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %v | %v | %v |\n",
					typ, id, m["database"], m["duration_display"], m["suggestion"]))
			}
		}
	} else {
		sb.WriteString("✅ 未发现死锁记录,数据库事务运行正常\n")
	}
	sb.WriteString("\n")

	// 索引大小分析
	if groups, ok := performance["index_size_analysis"]; ok {
		sb.WriteString("### 📊 索引大小占比分析\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 行数 | 表大小 | 索引大小 | 索引占比 | 状态 |\n")
				sb.WriteString("|------|------|------|--------|----------|----------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %.1f%% | %v |\n",
						im["schemaname"], im["table_name"], im["row_count"],
						im["table_size"], im["index_size"], im["index_ratio"], im["attention"]))
				}
				sb.WriteString("\n")
			}
		}
	}

	// 死元组分析
	if groups, ok := performance["dead_tuples"]; ok {
		sb.WriteString("### 📊 死元组与表膨胀分析\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 活元组 | 死元组 | 死元组占比 | 状态 | 建议 |\n")
				sb.WriteString("|------|------|--------|--------|------------|------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %.2f%% | %v | %s |\n",
						im["schemaname"], im["table_name"], im["live_tuples"], im["dead_tuples"],
						im["dead_tuple_ratio"], im["severity_label"], truncateStr(fmt.Sprintf("%v", im["suggestion"]), 60)))
				}
				sb.WriteString("\n")
			}
		}
	}

	// VACUUM 状态
	if groups, ok := performance["vacuum_status"]; ok {
		sb.WriteString("### 🧹 自动VACUUM/ANALYZE执行情况\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 活元组 | 最后VACUUM | 最后AUTOVACUUM | 最后ANALYZE | 状态 | 建议 |\n")
				sb.WriteString("|------|------|--------|------------|----------------|-------------|------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %v | %v | %s |\n",
						im["schemaname"], im["table_name"], im["live_tuples"],
						im["last_vacuum_display"], im["last_autovacuum_display"],
						im["last_autoanalyze_display"], im["vacuum_status"],
						truncateStr(fmt.Sprintf("%v", im["suggestion"]), 60)))
				}
				sb.WriteString("\n")
			}
		}
	}

	// IO 统计
	if groups, ok := performance["io_stats"]; ok {
		sb.WriteString("### 💾 Top IO密集表\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 磁盘读 | 缓存命中 | 缓存命中率 | 表大小 | IO级别 | 建议 |\n")
				sb.WriteString("|------|------|--------|----------|------------|--------|--------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %.2f%% | %v | %v | %s |\n",
						im["schemaname"], im["table_name"], im["disk_reads"], im["buffer_hits"],
						im["cache_hit_ratio"], im["table_size"], im["io_label"],
						truncateStr(fmt.Sprintf("%v", im["suggestion"]), 60)))
				}
				sb.WriteString("\n")
			}
		}
	}

	// 无效索引
	if groups, ok := performance["invalid_indexes"]; ok {
		sb.WriteString("### ⚠️ 无效索引检测\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 索引名 | 索引大小 | 扫描次数 | 问题类型 | 建议 |\n")
				sb.WriteString("|------|------|--------|----------|----------|----------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %v | %s |\n",
						im["schemaname"], im["table_name"], im["index_name"],
						im["index_size"], im["index_scans"], im["issue_type"],
						truncateStr(fmt.Sprintf("%v", im["suggestion"]), 60)))
				}
				sb.WriteString("\n")
			}
		}
	}

	// 重复索引
	if groups, ok := performance["duplicate_indexes"]; ok {
		sb.WriteString("### ⚠️ 重复索引检测\n\n")
		if arr, ok := groups.([]interface{}); ok {
			for _, g := range arr {
				gm := toMapInterface(g)
				name := fmt.Sprintf("%v", gm["name"])
				items, _ := gm["tables"].([]interface{})
				if len(items) == 0 {
					continue
				}
				sb.WriteString(fmt.Sprintf("#### %s\n\n", name))
				sb.WriteString("| 模式 | 表名 | 索引名 | 索引定义 | 索引大小 | 扫描次数 | 建议 |\n")
				sb.WriteString("|------|------|--------|----------|----------|----------|------|\n")
				for _, item := range items {
					im := toMapInterface(item)
					sb.WriteString(fmt.Sprintf("| %v | %v | %v | %v | %v | %v | %s |\n",
						im["schemaname"], im["table_name"], im["index_name"],
						truncateStr(fmt.Sprintf("%v", im["index_definition"]), 60),
						im["index_size"], im["index_scans"],
						truncateStr(fmt.Sprintf("%v", im["suggestion"]), 60)))
				}
				sb.WriteString("\n")
			}
		}
	}

	// ====== 页脚 ======
	sb.WriteString("---\n")
	sb.WriteString("*由 db-patrol 自动生成*\n")

	// 保存文件
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	name := strings.ReplaceAll(dbConfig.Name, " ", "_")
	filename := fmt.Sprintf("db_inspection_%s_%s.md", name, time.Now().Format("20060102_150405"))
	fp := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(fp, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return fp, nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
