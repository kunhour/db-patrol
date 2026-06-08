package core

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"db-patrol/internal/connection"
	"db-patrol/internal/inspector"
	"db-patrol/internal/models"
	"db-patrol/internal/reporter"
)

// DBInspector 数据库巡检核心控制器
type DBInspector struct {
	appConfig *models.AppConfig
	results   map[string]map[string]interface{}
}

// NewDBInspector 创建巡检控制器实例
func NewDBInspector(appConfig *models.AppConfig) *DBInspector {
	return &DBInspector{
		appConfig: appConfig,
		results:   make(map[string]map[string]interface{}),
	}
}

// InspectDatabase 巡检单个数据库
func (i *DBInspector) InspectDatabase(dbConfig models.DBConfig) (map[string]interface{}, error) {
	conn, err := connection.CreateConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}
	defer conn.Close()

	result := make(map[string]interface{})

	inspectors := inspector.GetInspectors(conn, i.appConfig.Inspection)
	enabledCount := len(inspectors)

	step := 1
	for _, insp := range inspectors {

		fmt.Printf("\n  %s\n", "==================================================")
		color.New(color.FgCyan).Printf("  [%d/%d] %s...\n", step, enabledCount, insp.Title())
		fmt.Printf("  %s\n", "==================================================")

		inspResult, err := insp.Inspect()
		if err != nil {
			color.New(color.FgYellow).Printf("  ⚠ 巡检项 %s 执行失败: %v\n", insp.Title(), err)
			inspResult = map[string]interface{}{"error": err.Error()}
		}

		result[insp.Name()] = inspResult
		step++
	}

	fmt.Printf("\n  %s\n", "==================================================")
	color.New(color.FgGreen).Println("  [OK] 数据库巡检完成")
	fmt.Printf("  %s\n\n", "==================================================")

	return result, nil
}

// InspectAll 巡检所有配置的数据库
func (i *DBInspector) InspectAll(databases []models.DBConfig) {
	total := len(databases)

	fmt.Printf("\n%s\n", "############################################################")
	color.New(color.FgGreen, color.Bold).Println("# 数据库巡检开始")
	fmt.Printf("# 数据库数量: %d\n", total)
	fmt.Printf("# 开始时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("%s\n\n", "############################################################")

	overallStart := time.Now()

	for idx, dbConfig := range databases {
		fmt.Printf("\n%s\n", "============================================================")
		color.New(color.FgYellow, color.Bold).Printf("📊 [%d/%d] 开始巡检: %s\n", idx+1, total, dbConfig.Name)
		fmt.Printf("   类型: %s\n", dbConfig.Type)
		fmt.Printf("   地址: %s:%d\n", dbConfig.Host, dbConfig.Port)
		fmt.Printf("%s\n", "============================================================")

		startTime := time.Now()

		result, err := i.InspectDatabase(dbConfig)
		if err != nil {
			elapsed := time.Since(startTime).Seconds()
			color.New(color.FgRed).Printf("\n  ✗ 巡检失败, 耗时: %.1f秒\n", elapsed)
			color.New(color.FgRed).Printf("  错误: %v\n", err)
			i.results[dbConfig.Name] = map[string]interface{}{"error": err.Error()}
			continue
		}

		i.results[dbConfig.Name] = result

		// 生成报告
		i.generateReport(dbConfig, result)

		elapsed := time.Since(startTime).Seconds()
		color.New(color.FgGreen).Printf("\n  ✓ 巡检成功完成, 耗时: %.1f秒\n", elapsed)
	}

	// 打印总结
	overallElapsed := time.Since(overallStart).Seconds()
	fmt.Printf("\n%s\n", "############################################################")
	color.New(color.FgGreen, color.Bold).Println("# 巡检完成摘要")
	fmt.Printf("# 结束时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("# 总耗时: %.1f秒\n", overallElapsed)
	fmt.Printf("%s\n\n", "############################################################")
}

// generateReport 生成巡检报告
func (i *DBInspector) generateReport(dbConfig models.DBConfig, result map[string]interface{}) {
	reportConfig := i.appConfig.Report
	outputDir := reportConfig.OutputDir
	formatType := reportConfig.Format

	fmt.Printf("\n  %s\n", "──────────────────────────────────────────────────────────")
	color.New(color.FgCyan).Println("  生成报告...")
	fmt.Printf("  %s\n", "──────────────────────────────────────────────────────────")
	fmt.Printf("    → 格式: %s\n", formatType)
	fmt.Printf("    → 目录: %s\n", outputDir)

	startTime := time.Now()

	r, err := reporter.CreateReporter(formatType, outputDir)
	if err != nil {
		color.New(color.FgRed).Printf("    ✗ 创建报告生成器失败: %v\n", err)
		return
	}

	filepath, err := r.Generate(dbConfig, result)
	if err != nil {
		color.New(color.FgRed).Printf("    ✗ 生成报告失败: %v\n", err)
		return
	}

	elapsed := time.Since(startTime).Seconds()

	var fileSize int64
	if info, err := os.Stat(filepath); err == nil {
		fileSize = info.Size()
	}
	fileSizeStr := fmt.Sprintf("%.1fKB", float64(fileSize)/1024.0)
	if fileSize == 0 {
		fileSizeStr = "未知"
	}

	color.New(color.FgGreen).Printf("    ✓ 报告已生成: %s\n", filepath)
	color.New(color.FgGreen).Printf("    ✓ 文件大小: %s, 耗时: %.1f秒\n", fileSizeStr, elapsed)
}

// PrintSummary 打印巡检摘要表格
func (i *DBInspector) PrintSummary() {
	fmt.Printf("\n%s\n", "============================================================")
	color.New(color.FgGreen, color.Bold).Println("巡检摘要")
	fmt.Printf("%s\n", "============================================================")

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"数据库", "状态", "数据库总数", "疑似备份库", "表总数", "疑似备份表"})
	table.SetBorder(true)
	table.SetRowLine(true)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetHeaderAlignment(tablewriter.ALIGN_CENTER)

	for dbName, result := range i.results {
		status := "✓ 成功"
		dbTotal := "-"
		backupDB := "-"
		tableTotal := "-"
		backupTable := "-"

		if _, hasErr := result["error"]; hasErr {
			status = "✗ 失败"
			table.Append([]string{dbName, status, "-", "-", "-", "-"})
			continue
		}

		if basicInfo, ok := result["basic_info"].(map[string]interface{}); ok {
			if dbs, ok := basicInfo["databases"].(map[string]interface{}); ok {
				if total, ok := dbs["total"]; ok {
					dbTotal = fmt.Sprintf("%v", total)
				}
				if bc, ok := dbs["backup_count"]; ok {
					backupDB = fmt.Sprintf("%v", bc)
				}
			}
			if tables, ok := basicInfo["tables"].(map[string]interface{}); ok {
				if tc, ok := tables["total_count"]; ok {
					tableTotal = fmt.Sprintf("%v", tc)
				}
				if bc, ok := tables["backup_count"]; ok {
					backupTable = fmt.Sprintf("%v", bc)
				}
			}
		}

		table.Append([]string{dbName, status, dbTotal, backupDB, tableTotal, backupTable})
	}

	table.Render()
}

// GetResults 获取巡检结果
func (i *DBInspector) GetResults() map[string]map[string]interface{} {
	return i.results
}

// GetAppConfig 获取应用配置
func (i *DBInspector) GetAppConfig() *models.AppConfig {
	return i.appConfig
}


