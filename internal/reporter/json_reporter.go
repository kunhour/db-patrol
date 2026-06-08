package reporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db-patrol/internal/models"
)

// JSONReporter JSON报告生成器
type JSONReporter struct {
	outputDir string
}

// NewJSONReporter 创建JSON报告生成器
func NewJSONReporter(outputDir string) *JSONReporter {
	return &JSONReporter{outputDir: outputDir}
}

// Generate 生成JSON报告
func (r *JSONReporter) Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error) {
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

	report := map[string]interface{}{
		"db_name":      dbConfig.Name,
		"db_type":      dbConfig.Type,
		"db_host":      dbConfig.Host,
		"db_port":      dbConfig.Port,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
		"health_score": healthScore,
		"key_findings": keyFindings,
		"results":      results,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	name := strings.ReplaceAll(dbConfig.Name, " ", "_")
	filename := fmt.Sprintf("db_inspection_%s_%s.json", name, time.Now().Format("20060102_150405"))
	filepath := filepath.Join(r.outputDir, filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return filepath, nil
}
