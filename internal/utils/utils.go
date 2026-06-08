package utils

import (
	"database/sql"
	"fmt"
)

// FormatSize 格式化字节大小
func FormatSize(sizeBytes int64) string {
	if sizeBytes == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unitIndex := 0
	size := float64(sizeBytes)

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", int(size), units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// ToInt 将 interface{} 转换为 int
func ToInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	case string:
		n := 0
		fmt.Sscanf(val, "%d", &n)
		return n
	case []byte:
		n := 0
		fmt.Sscanf(string(val), "%d", &n)
		return n
	case sql.NullInt64:
		if val.Valid {
			return int(val.Int64)
		}
		return 0
	}
	return 0
}

// ToInt64 将 interface{} 转换为 int64
func ToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case float32:
		return int64(val)
	case string:
		n := int64(0)
		fmt.Sscanf(val, "%d", &n)
		return n
	case []byte:
		n := int64(0)
		fmt.Sscanf(string(val), "%d", &n)
		return n
	case sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return 0
	}
	return 0
}

// ToFloat64 将 interface{} 转换为 float64
func ToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f := 0.0
		fmt.Sscanf(val, "%f", &f)
		return f
	case []byte:
		f := 0.0
		fmt.Sscanf(string(val), "%f", &f)
		return f
	}
	return 0
}

// ToString 将 interface{} 转换为 string
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ToBool 将 interface{} 转换为 bool
func ToBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case int64:
		return val != 0
	case string:
		return val == "true" || val == "t" || val == "1"
	case []byte:
		s := string(val)
		return s == "true" || s == "t" || s == "1"
	}
	return false
}

// FormatDuration 格式化持续时间（秒 → 天/小时/分钟）
func FormatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d分%d秒", seconds/60, seconds%60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%d小时%d分", seconds/3600, (seconds%3600)/60)
	}
	return fmt.Sprintf("%d天%d小时%d分", seconds/86400, (seconds%86400)/3600, (seconds%3600)/60)
}
