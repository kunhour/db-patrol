package connection

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresConnection PostgreSQL/Vastbase PG 连接
type PostgresConnection struct {
	BaseConnection
}

// Connect 建立连接
func (c *PostgresConnection) Connect() error {
	port := c.Config_.Port
	if port == 0 {
		port = 5432
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.Config_.Host, port, c.Config_.User, c.Config_.Password, c.Config_.Database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	c.DB_ = db
	return db.Ping()
}

// ExecuteQuery 执行查询
func (c *PostgresConnection) ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
	return ExecuteQuery(c.DB_, query, args...)
}

// Execute 执行SQL
func (c *PostgresConnection) Execute(query string, args ...interface{}) error {
	_, err := c.DB_.Exec(query, args...)
	return err
}
