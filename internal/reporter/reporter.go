package reporter

import (
	"fmt"

	"db-patrol/internal/models"
)

// Reporter 报告生成器接口
type Reporter interface {
	Generate(dbConfig models.DBConfig, results map[string]interface{}) (string, error)
}

// CreateReporter 根据格式创建报告生成器
func CreateReporter(format, outputDir string) (Reporter, error) {
	switch format {
	case "html":
		return NewHTMLReporter(outputDir), nil
	case "json":
		return NewJSONReporter(outputDir), nil
	case "markdown":
		return NewMarkdownReporter(outputDir), nil
	default:
		return nil, fmt.Errorf("不支持的报告格式: %s", format)
	}
}
