package config

import (
	"os"

	"db-patrol/internal/models"
	"gopkg.in/yaml.v3"
)

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*models.AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg models.AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	if cfg.Inspection.SlowQueryThreshold == 0 {
		cfg.Inspection.SlowQueryThreshold = 1.0
	}
	if cfg.Inspection.MaxConnectionsThreshold == 0 {
		cfg.Inspection.MaxConnectionsThreshold = 80
	}
	if cfg.Inspection.TableSizeThreshold == 0 {
		cfg.Inspection.TableSizeThreshold = 1024
	}
	if cfg.Inspection.LongTransactionThreshold == 0 {
		cfg.Inspection.LongTransactionThreshold = 300
	}
	if cfg.Report.Format == "" {
		cfg.Report.Format = "html"
	}
	if cfg.Report.OutputDir == "" {
		cfg.Report.OutputDir = "./reports"
	}

	return &cfg, nil
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 当前目录
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return "config.yaml"
}
