package connection

import (
	"database/sql"
	"fmt"

	"db-patrol/internal/models"
)

// Connection 数据库连接接口
type Connection interface {
	Connect() error
	ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error)
	Execute(query string, args ...interface{}) error
	Close() error
	DB() *sql.DB
	Config() models.DBConfig
}

// BaseConnection 连接基类
type BaseConnection struct {
	Config_ models.DBConfig
	DB_     *sql.DB
}

func (c *BaseConnection) Config() models.DBConfig {
	return c.Config_
}

func (c *BaseConnection) DB() *sql.DB {
	return c.DB_
}

func (c *BaseConnection) Close() error {
	if c.DB_ != nil {
		return c.DB_.Close()
	}
	return nil
}

// ExecuteQuery 执行查询并返回结果
func ExecuteQuery(db *sql.DB, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// CreateConnection 根据配置创建数据库连接
func CreateConnection(cfg models.DBConfig) (Connection, error) {
	switch cfg.Type {
	case "vastbase_pg", "postgresql":
		conn := &PostgresConnection{BaseConnection{Config_: cfg}}
		return conn, conn.Connect()
	case "vastbase_mysql", "mysql":
		conn := &MySQLConnection{BaseConnection{Config_: cfg}}
		return conn, conn.Connect()
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
}
