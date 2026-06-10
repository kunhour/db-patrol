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

	applyDefaults(&cfg)
	return &cfg, nil
}

// DefaultConfig 返回默认配置（不依赖配置文件）
func DefaultConfig() *models.AppConfig {
	cfg := &models.AppConfig{}
	// 无配置文件时默认启用所有检查项
	cfg.Inspection.Checks.BasicInfo = true
	cfg.Inspection.Checks.Performance = true
	cfg.Inspection.Checks.Schema = true
	applyDefaults(cfg)
	return cfg
}

// applyDefaults 设置默认值
func applyDefaults(cfg *models.AppConfig) {
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
}

// GetDefaultConfigPath 获取默认配置文件路径
func GetDefaultConfigPath() string {
	// 当前目录
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	return "config.yaml"
}
