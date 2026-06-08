package reporter

import (
	"fmt"
	"sort"
	"strings"

	"db-patrol/internal/models"
)

// CalculateHealthScore 计算健康评分 (100分制)
func CalculateHealthScore(basicInfo, performance, databases, tables map[string]interface{}) models.HealthScore {
	score := 100
	var issues []string
	var details []models.ScoreDetail

	// 1. 连接使用率 (15分)
	if conn, ok := getMap(performance, "connections"); ok {
		if usage, ok := getFloat64(conn, "usage_percent"); ok {
			if usage > 90 {
				score -= 15
				issues = append(issues, "连接使用率超过90%,存在连接耗尽风险")
				details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 0, MaxScore: 15, Status: "critical", Detail: fmt.Sprintf("%.0f%%", usage)})
			} else if usage > 80 {
				score -= 10
				issues = append(issues, "连接使用率超过80%,需要关注")
				details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 5, MaxScore: 15, Status: "warning", Detail: fmt.Sprintf("%.0f%%", usage)})
			} else if usage > 60 {
				score -= 5
				details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 10, MaxScore: 15, Status: "good", Detail: fmt.Sprintf("%.0f%%", usage)})
			} else {
				details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 15, MaxScore: 15, Status: "excellent", Detail: fmt.Sprintf("%.0f%%", usage)})
			}
		} else {
			details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 15, MaxScore: 15, Status: "excellent", Detail: "数据不足"})
		}
	} else {
		details = append(details, models.ScoreDetail{Name: "连接使用率", Score: 15, MaxScore: 15, Status: "excellent", Detail: "数据不足"})
	}

	// 2. 缓存命中率 (20分)
	if cacheHit, ok := getMap(performance, "cache_hit_ratio"); ok {
		if ratio, ok := getFloat64(cacheHit, "ratio"); ok {
			if ratio < 90 {
				score -= 20
				issues = append(issues, "缓存命中率低于90%,严重影响性能")
				details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 0, MaxScore: 20, Status: "critical", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else if ratio < 95 {
				score -= 15
				issues = append(issues, "缓存命中率低于95%,建议增加shared_buffers")
				details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 5, MaxScore: 20, Status: "warning", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else if ratio < 99 {
				score -= 5
				details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 15, MaxScore: 20, Status: "good", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else {
				details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 20, MaxScore: 20, Status: "excellent", Detail: fmt.Sprintf("%.2f%%", ratio)})
			}
		} else {
			details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 20, MaxScore: 20, Status: "excellent", Detail: "数据不足"})
		}
	} else {
		details = append(details, models.ScoreDetail{Name: "缓存命中率", Score: 20, MaxScore: 20, Status: "excellent", Detail: "数据不足"})
	}

	// 3. 索引命中率 (15分)
	if indexHit, ok := getMap(performance, "index_hit_ratio"); ok {
		if ratio, ok := getFloat64(indexHit, "ratio"); ok {
			if ratio < 50 {
				score -= 15
				issues = append(issues, "索引命中率低于50%,大量查询未使用索引")
				details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 0, MaxScore: 15, Status: "critical", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else if ratio < 70 {
				score -= 10
				issues = append(issues, "索引命中率低于70%,建议检查缺失索引")
				details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 5, MaxScore: 15, Status: "warning", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else if ratio < 90 {
				score -= 5
				details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 10, MaxScore: 15, Status: "good", Detail: fmt.Sprintf("%.2f%%", ratio)})
			} else {
				details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 15, MaxScore: 15, Status: "excellent", Detail: fmt.Sprintf("%.2f%%", ratio)})
			}
		} else {
			details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 15, MaxScore: 15, Status: "excellent", Detail: "数据不足"})
		}
	} else {
		details = append(details, models.ScoreDetail{Name: "索引命中率", Score: 15, MaxScore: 15, Status: "excellent", Detail: "数据不足"})
	}

	// 4. 主键完整性 (10分)
	totalWithoutPK := 0
	if tablesWithoutPK, ok := getMap(basicInfo, "tables_without_pk"); ok {
		for _, v := range tablesWithoutPK {
			if arr, ok := v.([]interface{}); ok {
				totalWithoutPK += len(arr)
			}
		}
	}
	if totalWithoutPK > 20 {
		score -= 10
		issues = append(issues, fmt.Sprintf("发现%d个表缺少主键,影响数据完整性", totalWithoutPK))
		details = append(details, models.ScoreDetail{Name: "主键完整性", Score: 0, MaxScore: 10, Status: "critical", Detail: fmt.Sprintf("%d个表无主键", totalWithoutPK)})
	} else if totalWithoutPK > 10 {
		score -= 7
		issues = append(issues, fmt.Sprintf("发现%d个表缺少主键", totalWithoutPK))
		details = append(details, models.ScoreDetail{Name: "主键完整性", Score: 3, MaxScore: 10, Status: "warning", Detail: fmt.Sprintf("%d个表无主键", totalWithoutPK)})
	} else if totalWithoutPK > 0 {
		score -= 3
		details = append(details, models.ScoreDetail{Name: "主键完整性", Score: 7, MaxScore: 10, Status: "good", Detail: fmt.Sprintf("%d个表无主键", totalWithoutPK)})
	} else {
		details = append(details, models.ScoreDetail{Name: "主键完整性", Score: 10, MaxScore: 10, Status: "excellent", Detail: "全部表都有主键"})
	}

	// 5. 备份数据清理 (10分)
	backupDBCount := 0
	backupTableCount := 0
	if dbs, ok := getSlice(databases, "backup"); ok {
		backupDBCount = len(dbs)
	}
	if tbs, ok := getSlice(tables, "backup"); ok {
		backupTableCount = len(tbs)
	}
	if backupDBCount > 0 || backupTableCount > 0 {
		if backupDBCount > 5 || backupTableCount > 20 {
			score -= 10
			issues = append(issues, fmt.Sprintf("发现%d个疑似备份库和%d个备份表,建议清理", backupDBCount, backupTableCount))
			details = append(details, models.ScoreDetail{Name: "备份数据清理", Score: 0, MaxScore: 10, Status: "critical", Detail: fmt.Sprintf("%d个备份库, %d个备份表", backupDBCount, backupTableCount)})
		} else {
			score -= 5
			details = append(details, models.ScoreDetail{Name: "备份数据清理", Score: 5, MaxScore: 10, Status: "warning", Detail: fmt.Sprintf("%d个备份库, %d个备份表", backupDBCount, backupTableCount)})
		}
	} else {
		details = append(details, models.ScoreDetail{Name: "备份数据清理", Score: 10, MaxScore: 10, Status: "excellent", Detail: "无备份数据"})
	}

	// 6. 索引大小占比 (15分)
	criticalIndexes := 0
	if indexAnalysis, ok := getMap(performance, "index_size_analysis"); ok {
		for _, v := range indexAnalysis {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						if m["attention"] == "严重" {
							criticalIndexes++
						}
					}
				}
			}
		}
	}
	if criticalIndexes > 10 {
		score -= 15
		issues = append(issues, fmt.Sprintf("发现%d个表索引占比严重超标", criticalIndexes))
		details = append(details, models.ScoreDetail{Name: "索引大小占比", Score: 0, MaxScore: 15, Status: "critical", Detail: fmt.Sprintf("%d个表严重超标", criticalIndexes)})
	} else if criticalIndexes > 5 {
		score -= 10
		details = append(details, models.ScoreDetail{Name: "索引大小占比", Score: 5, MaxScore: 15, Status: "warning", Detail: fmt.Sprintf("%d个表严重超标", criticalIndexes)})
	} else if criticalIndexes > 0 {
		score -= 5
		details = append(details, models.ScoreDetail{Name: "索引大小占比", Score: 10, MaxScore: 15, Status: "good", Detail: fmt.Sprintf("%d个表严重超标", criticalIndexes)})
	} else {
		details = append(details, models.ScoreDetail{Name: "索引大小占比", Score: 15, MaxScore: 15, Status: "excellent", Detail: "索引占比正常"})
	}

	// 7. 无效索引 (10分)
	totalInvalid := 0
	if invalidIndexes, ok := getMap(performance, "invalid_indexes"); ok {
		for _, v := range invalidIndexes {
			if arr, ok := v.([]interface{}); ok {
				totalInvalid += len(arr)
			}
		}
	}
	if totalInvalid > 0 {
		score -= 10
		issues = append(issues, fmt.Sprintf("发现%d个无效索引,浪费存储空间", totalInvalid))
		details = append(details, models.ScoreDetail{Name: "无效索引", Score: 0, MaxScore: 10, Status: "critical", Detail: fmt.Sprintf("%d个无效索引", totalInvalid)})
	} else {
		details = append(details, models.ScoreDetail{Name: "无效索引", Score: 10, MaxScore: 10, Status: "excellent", Detail: "无无效索引"})
	}

	// 8. 重复索引 (5分)
	totalDuplicate := 0
	if duplicateIndexes, ok := getMap(performance, "duplicate_indexes"); ok {
		for _, v := range duplicateIndexes {
			if arr, ok := v.([]interface{}); ok {
				totalDuplicate += len(arr)
			}
		}
	}
	if totalDuplicate > 0 {
		score -= 5
		issues = append(issues, fmt.Sprintf("发现%d个重复索引", totalDuplicate))
		details = append(details, models.ScoreDetail{Name: "重复索引", Score: 0, MaxScore: 5, Status: "warning", Detail: fmt.Sprintf("%d个重复索引", totalDuplicate)})
	} else {
		details = append(details, models.ScoreDetail{Name: "重复索引", Score: 5, MaxScore: 5, Status: "excellent", Detail: "无重复索引"})
	}

	// 限制分数范围
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// 统计问题
	problemCount := 0
	criticalCount := 0
	warningCount := 0
	for _, d := range details {
		if d.Status == "warning" || d.Status == "critical" {
			problemCount++
		}
		if d.Status == "critical" {
			criticalCount++
		}
		if d.Status == "warning" {
			warningCount++
		}
	}

	// 判定等级
	var level, label, summary string
	if score >= 90 {
		level = "excellent"
		label = "优秀"
		summary = "数据库运行状态良好,各项指标正常"
	} else if score >= 75 {
		level = "good"
		label = "良好"
		summary = "数据库运行状态较好,部分指标需关注"
	} else if score >= 60 {
		level = "average"
		label = "一般"
		summary = "数据库存在一些问题,建议优化"
	} else {
		level = "poor"
		label = "较差"
		summary = "数据库存在较多问题,需要立即优化"
	}

	if problemCount > 0 {
		summary += fmt.Sprintf("。发现%d个异常项(", problemCount)
		var parts []string
		if criticalCount > 0 {
			parts = append(parts, fmt.Sprintf("%d个严重问题", criticalCount))
		}
		if warningCount > 0 {
			parts = append(parts, fmt.Sprintf("%d个需关注项", warningCount))
		}
		for i, p := range parts {
			if i > 0 {
				summary += ","
			}
			summary += p
		}
		summary += ")"
	}

	return models.HealthScore{
		Score:        score,
		Level:        level,
		Label:        label,
		Summary:      summary,
		Issues:       issues,
		Details:      details,
		ProblemCount: problemCount,
		CriticalCount: criticalCount,
		WarningCount: warningCount,
	}
}

// GenerateKeyFindings 生成关键发现与建议
func GenerateKeyFindings(basicInfo, performance, databases, tables map[string]interface{}) []models.KeyFinding {
	var findings []models.KeyFinding

	// 连接使用率
	if conn, ok := getMap(performance, "connections"); ok {
		if usage, ok := getFloat64(conn, "usage_percent"); ok && usage > 80 {
			level := "warning"
			icon := "\U0001f7e1" // 🟡
			if usage > 90 {
				level = "critical"
				icon = "\U0001f534" // 🔴
			}
			current := toInt(conn["current"])
			max := toInt(conn["max"])
			findings = append(findings, models.KeyFinding{
				Level:       level,
				Icon:        icon,
				Title:       "连接使用率过高",
				Description: fmt.Sprintf("当前连接使用率为%.0f%%,已达到%d/%d。建议: 1) 增加max_connections; 2) 使用连接池; 3) 排查异常连接", usage, current, max),
			})
		}
	}

	// 缓存命中率
	if cacheHit, ok := getMap(performance, "cache_hit_ratio"); ok {
		if ratio, ok := getFloat64(cacheHit, "ratio"); ok && ratio < 95 {
			level := "warning"
			icon := "\U0001f7e1"
			if ratio < 90 {
				level = "critical"
				icon = "\U0001f534"
			}
			findings = append(findings, models.KeyFinding{
				Level:       level,
				Icon:        icon,
				Title:       "缓存命中率偏低",
				Description: fmt.Sprintf("当前缓存命中率为%.2f%%。建议: 1) 增加shared_buffers配置; 2) 优化查询减少磁盘IO; 3) 检查是否有大表全表扫描", ratio),
			})
		}
	}

	// 索引命中率
	if indexHit, ok := getMap(performance, "index_hit_ratio"); ok {
		if ratio, ok := getFloat64(indexHit, "ratio"); ok && ratio < 70 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "索引命中率较低",
				Description: fmt.Sprintf("当前索引命中率为%.2f%%,说明大量查询使用顺序扫描。建议: 1) 分析慢查询添加索引; 2) 检查现有索引是否合理; 3) 使用EXPLAIN分析查询计划", ratio),
			})
		}
	}

	// 长事务
	if longTxns, ok := getSlice(performance, "long_transactions"); ok && len(longTxns) > 0 {
		criticalTxns := 0
		warningTxns := 0
		for _, t := range longTxns {
			if m, ok := t.(map[string]interface{}); ok {
				switch m["severity"] {
				case "critical":
					criticalTxns++
				case "warning":
					warningTxns++
				}
			}
		}
		if criticalTxns > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "critical",
				Icon:        "\U0001f534",
				Title:       "存在严重长事务",
				Description: fmt.Sprintf("发现%d个运行超过1小时的长事务。长事务会阻塞VACUUM导致表膨胀,建议: 1) 立即排查这些事务是否可以终止; 2) 优化事务逻辑; 3) 设置statement_timeout限制", criticalTxns),
			})
		} else if warningTxns > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "存在长事务",
				Description: fmt.Sprintf("发现%d个运行超过30分钟的事务。建议监控这些事务,避免演变为严重长事务", warningTxns),
			})
		}
	}

	// 锁等待
	if locks, ok := getSlice(performance, "locks"); ok && len(locks) > 0 {
		criticalLocks := 0
		for _, l := range locks {
			if m, ok := l.(map[string]interface{}); ok && m["severity"] == "critical" {
				criticalLocks++
			}
		}
		if criticalLocks > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "critical",
				Icon:        "\U0001f534",
				Title:       "存在严重锁等待",
				Description: fmt.Sprintf("发现%d个锁等待超过60秒。锁等待会导致业务响应缓慢,建议: 1) 排查阻塞源头会话; 2) 优化事务隔离级别; 3) 减少事务持有锁的时间", criticalLocks),
			})
		} else if len(locks) > 5 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "存在较多锁等待",
				Description: fmt.Sprintf("发现%d个锁等待。建议关注并发事务的锁竞争情况", len(locks)),
			})
		}
	}

	// 无主键表
	if tablesWithoutPK, ok := getMap(basicInfo, "tables_without_pk"); ok {
		total := 0
		for _, v := range tablesWithoutPK {
			if arr, ok := v.([]interface{}); ok {
				total += len(arr)
			}
		}
		if total > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "部分表缺少主键",
				Description: fmt.Sprintf("发现%d个表缺少主键或唯一索引。建议为这些表添加主键以保证数据完整性和提升查询性能", total),
			})
		}
	}

	// 死元组
	if deadTuples, ok := getMap(performance, "dead_tuples"); ok {
		criticalTables := 0
		warningTables := 0
		for _, v := range deadTuples {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						switch m["severity"] {
						case "critical":
							criticalTables++
						case "warning":
							warningTables++
						}
					}
				}
			}
		}
		if criticalTables > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "critical",
				Icon:        "\U0001f534",
				Title:       "部分表死元组比例严重",
				Description: fmt.Sprintf("发现%d个表死元组比例超过50%%,表膨胀严重。建议立即执行VACUUM FULL或VACUUM清理", criticalTables),
			})
		} else if warningTables > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "部分表死元组比例偏高",
				Description: fmt.Sprintf("发现%d个表死元组比例超过30%%。建议执行VACUUM清理,检查autovacuum配置是否合理", warningTables),
			})
		}
	}

	// VACUUM状态
	if vacuumStatus, ok := getMap(performance, "vacuum_status"); ok {
		totalIssues := 0
		neverVacuumed := 0
		for _, v := range vacuumStatus {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						totalIssues++
						if status, ok := m["vacuum_status"].(string); ok && strings.Contains(status, "从未执行") {
							neverVacuumed++
						}
					}
				}
			}
		}
		if neverVacuumed > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "critical",
				Icon:        "\U0001f534",
				Title:       "部分表从未执行VACUUM",
				Description: fmt.Sprintf("发现%d个表从未执行过VACUUM或ANALYZE。请检查autovacuum是否启用,或手动执行VACUUM ANALYZE", neverVacuumed),
			})
		} else if totalIssues > 10 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "VACUUM执行不及时",
				Description: fmt.Sprintf("发现%d个表的VACUUM/ANALYZE超过7天未执行。建议调整autovacuum阈值参数", totalIssues),
			})
		}
	}

	// 无效索引
	if invalidIndexes, ok := getMap(performance, "invalid_indexes"); ok {
		total := 0
		for _, v := range invalidIndexes {
			if arr, ok := v.([]interface{}); ok {
				total += len(arr)
			}
		}
		if total > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "存在无效索引",
				Description: fmt.Sprintf("发现%d个无效或未使用的索引。建议删除这些索引以释放存储空间并提升写入性能", total),
			})
		}
	}

	// 重复索引
	if duplicateIndexes, ok := getMap(performance, "duplicate_indexes"); ok {
		total := 0
		for _, v := range duplicateIndexes {
			if arr, ok := v.([]interface{}); ok {
				total += len(arr)
			}
		}
		if total > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "info",
				Icon:        "\U0001f535", // 🔵
				Title:       "存在重复索引",
				Description: fmt.Sprintf("发现%d个重复索引(相同列上多个索引)。建议保留使用频率高的索引,删除冗余索引", total),
			})
		}
	}

	// IO负载
	if ioStats, ok := getMap(performance, "io_stats"); ok {
		highIOTables := 0
		for _, v := range ioStats {
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok && m["io_level"] == "high" {
						highIOTables++
					}
				}
			}
		}
		if highIOTables > 0 {
			findings = append(findings, models.KeyFinding{
				Level:       "warning",
				Icon:        "\U0001f7e1",
				Title:       "存在高IO负载表",
				Description: fmt.Sprintf("发现%d个表存在大量磁盘IO且缓存命中率低。建议: 1) 优化查询添加索引; 2) 增加shared_buffers; 3) 检查是否有不必要的全表扫描", highIOTables),
			})
		}
	}

	// 备份数据
	backupDBs, _ := getSlice(databases, "backup")
	backupTables, _ := getSlice(tables, "backup")
	if len(backupDBs) > 3 || len(backupTables) > 10 {
		findings = append(findings, models.KeyFinding{
			Level:       "warning",
			Icon:        "\U0001f7e1",
			Title:       "疑似备份数据过多",
			Description: fmt.Sprintf("发现%d个疑似备份库和%d个备份表。建议定期清理历史备份以释放存储空间", len(backupDBs), len(backupTables)),
		})
	}

	// 按严重程度排序
	levelOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
	sort.Slice(findings, func(i, j int) bool {
		return levelOrder[findings[i].Level] < levelOrder[findings[j].Level]
	})

	return findings
}

// ==================== 辅助函数 ====================

func getMap(m map[string]interface{}, key string) (map[string]interface{}, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	result, ok := v.(map[string]interface{})
	return result, ok
}

func getSlice(m map[string]interface{}, key string) ([]interface{}, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	result, ok := v.([]interface{})
	return result, ok
}

func getFloat64(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	return 0
}
