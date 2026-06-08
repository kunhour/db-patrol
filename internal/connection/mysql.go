package connection

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConnection MySQL/Vastbase MySQL 连接
type MySQLConnection struct {
	BaseConnection
}

// Connect 建立连接
func (c *MySQLConnection) Connect() error {
	port := c.Config_.Port
	if port == 0 {
		port = 3306
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
		c.Config_.User, c.Config_.Password, c.Config_.Host, port, c.Config_.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	c.DB_ = db
	return db.Ping()
}

// ExecuteQuery 执行查询
func (c *MySQLConnection) ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
	return ExecuteQuery(c.DB_, query, args...)
}

// Execute 执行SQL
func (c *MySQLConnection) Execute(query string, args ...interface{}) error {
	_, err := c.DB_.Exec(query, args...)
	return err
}
