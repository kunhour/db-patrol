package inspector

import (
	"db-patrol/internal/connection"
	"db-patrol/internal/models"
)

// Inspector 巡检器接口
type Inspector interface {
	Name() string
	Title() string
	Inspect() (map[string]interface{}, error)
}

// Factory 巡检器工厂函数类型
type Factory func(conn connection.Connection, cfg models.InspectionConfig) Inspector

var registry = map[string]Factory{}

// Register 注册巡检器
func Register(name string, factory Factory) {
	registry[name] = factory
}

// GetInspectors 获取所有启用的巡检器
func GetInspectors(conn connection.Connection, cfg models.InspectionConfig) []Inspector {
	checks := cfg.Checks
	var inspectors []Inspector

	if checks.BasicInfo {
		if f, ok := registry["basic_info"]; ok {
			inspectors = append(inspectors, f(conn, cfg))
		}
	}
	if checks.Performance {
		if f, ok := registry["performance"]; ok {
			inspectors = append(inspectors, f(conn, cfg))
		}
	}
	if checks.Schema {
		if f, ok := registry["schema"]; ok {
			inspectors = append(inspectors, f(conn, cfg))
		}
	}

	return inspectors
}

func init() {
	Register("basic_info", func(conn connection.Connection, cfg models.InspectionConfig) Inspector {
		return NewBasicInfoInspector(conn, cfg)
	})
	Register("performance", func(conn connection.Connection, cfg models.InspectionConfig) Inspector {
		return NewPerformanceInspector(conn, cfg)
	})
	Register("schema", func(conn connection.Connection, cfg models.InspectionConfig) Inspector {
		return NewSchemaInspector(conn, cfg)
	})
}
