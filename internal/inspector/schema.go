package inspector

import (
	"fmt"
	"strings"
	"unicode"

	"db-patrol/internal/connection"
	"db-patrol/internal/models"
)

// SchemaInspector 设计规范巡检器
type SchemaInspector struct {
	conn connection.Connection
	cfg  models.InspectionConfig
}

// NewSchemaInspector 创建设计规范巡检器
func NewSchemaInspector(conn connection.Connection, cfg models.InspectionConfig) *SchemaInspector {
	return &SchemaInspector{conn: conn, cfg: cfg}
}

func (i *SchemaInspector) Name() string  { return "schema" }
func (i *SchemaInspector) Title() string { return "检查设计规范" }

// Inspect 执行设计规范检查
func (i *SchemaInspector) Inspect() (map[string]interface{}, error) {
	dbType := i.conn.Config().Type
	if strings.Contains(strings.ToLower(dbType), "pg") || strings.Contains(strings.ToLower(dbType), "postgres") {
		return i.inspectPG()
	}
	return i.inspectMySQL()
}

// ==================== PG ====================

func (i *SchemaInspector) inspectPG() (map[string]interface{}, error) {
	return map[string]interface{}{
		"table_naming":   i.checkPGTableNaming(),
		"column_naming":  i.checkPGColumnNaming(),
		"primary_keys":   i.checkPGPrimaryKeys(),
		"indexes":        i.checkPGIndexes(),
		"constraints":    i.checkPGConstraints(),
		"data_types":     i.checkPGDataTypes(),
		"comments":       i.checkPGComments(),
		"large_tables":   i.checkPGLargeTables(),
	}, nil
}

func (i *SchemaInspector) checkPGTableNaming() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT schemaname, tablename 
		FROM pg_tables 
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		name := toString(row["tablename"])
		if name != strings.ToLower(name) {
			issues = append(issues, models.SchemaIssue{
				Table: name, Issue: "表名包含大写字母", Suggestion: "建议使用小写字母和下划线",
			})
		}
		if strings.Contains(name, "-") || strings.Contains(name, " ") {
			issues = append(issues, models.SchemaIssue{
				Table: name, Issue: "表名包含非法字符", Suggestion: "建议使用小写字母和下划线",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGColumnNaming() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		name := toString(row["column_name"])
		tableName := toString(row["table_name"])
		if name != strings.ToLower(name) {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: name, Issue: "列名包含大写字母", Suggestion: "建议使用小写字母和下划线",
			})
		}
		if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: name, Issue: "列名以数字开头", Suggestion: "列名应以字母开头",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGPrimaryKeys() []models.SchemaIssue {
	var issues []models.SchemaIssue
	pkRows, err := i.conn.ExecuteQuery(`
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}

	tablesWithPK := make(map[string]bool)
	for _, pk := range pkRows {
		tablesWithPK[toString(pk["table_name"])] = true
	}

	tables, err := i.conn.ExecuteQuery(`
		SELECT tablename FROM pg_tables WHERE schemaname = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, t := range tables {
		name := toString(t["tablename"])
		if !tablesWithPK[name] {
			issues = append(issues, models.SchemaIssue{
				Table: name, Issue: "表缺少主键", Suggestion: "建议为每个表添加主键",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGIndexes() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT schemaname, tablename, indexname
		FROM pg_indexes
		WHERE schemaname = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		name := toString(row["indexname"])
		tableName := toString(row["tablename"])
		if !strings.HasPrefix(name, "idx_") && !strings.HasPrefix(name, "pk_") &&
			!strings.HasPrefix(name, "fk_") && !strings.HasPrefix(name, "uq_") &&
			!strings.Contains(name, "pkey") {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Index: name, Issue: "索引命名不规范", Suggestion: "建议以 idx_, pk_, fk_, uq_ 开头",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGConstraints() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT tc.table_name, tc.constraint_name, kcu.column_name,
		       ccu.table_name AS foreign_table_name,
		       ccu.column_name AS foreign_column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		issues = append(issues, models.SchemaIssue{
			Table:      toString(row["table_name"]),
			Column:     toString(row["column_name"]),
			Constraint: toString(row["constraint_name"]),
			Issue:      "存在外键约束",
			Suggestion: "确保外键列上有索引以提高性能",
		})
	}
	return issues
}

func (i *SchemaInspector) checkPGDataTypes() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, column_name, data_type, character_maximum_length
		FROM information_schema.columns
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		dataType := toString(row["data_type"])
		if dataType == "character varying" && row["character_maximum_length"] == nil {
			issues = append(issues, models.SchemaIssue{
				Table: toString(row["table_name"]), Column: toString(row["column_name"]),
				Issue: "使用 varchar 未指定长度", Suggestion: "建议指定 varchar 长度或使用 text",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGComments() []models.SchemaIssue {
	var issues []models.SchemaIssue
	rows, err := i.conn.ExecuteQuery(`
		SELECT c.relname as table_name, obj_description(c.oid) as comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname = 'public'
	`)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		comment := toString(row["comment"])
		if comment == "" {
			issues = append(issues, models.SchemaIssue{
				Table: toString(row["table_name"]), Issue: "表缺少注释", Suggestion: "建议为表添加注释说明",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkPGLargeTables() []models.SchemaIssue {
	var issues []models.SchemaIssue
	threshold := int64(i.cfg.TableSizeThreshold) * 1024 * 1024
	rows, err := i.conn.ExecuteQuery(fmt.Sprintf(`
		SELECT schemaname, relname as table_name,
		       pg_total_relation_size(relid) as total_size
		FROM pg_stat_user_tables
		WHERE pg_total_relation_size(relid) > %d
		ORDER BY pg_total_relation_size(relid) DESC
	`, threshold))
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		sizeBytes := toInt64(row["total_size"])
		sizeMB := float64(sizeBytes) / (1024 * 1024)
		issues = append(issues, models.SchemaIssue{
			Table:      toString(row["table_name"]),
			Issue:      fmt.Sprintf("表过大 (%.2f MB)", sizeMB),
			Suggestion: "考虑分区或归档历史数据",
		})
	}
	return issues
}

// ==================== MySQL ====================

func (i *SchemaInspector) inspectMySQL() (map[string]interface{}, error) {
	return map[string]interface{}{
		"table_naming":    i.checkMySQLTableNaming(),
		"column_naming":   i.checkMySQLColumnNaming(),
		"primary_keys":    i.checkMySQLPrimaryKeys(),
		"indexes":         i.checkMySQLIndexes(),
		"constraints":     i.checkMySQLConstraints(),
		"data_types":      i.checkMySQLDataTypes(),
		"comments":        i.checkMySQLComments(),
		"large_tables":    i.checkMySQLLargeTables(),
		"engine_charset":  i.checkMySQLEngineCharset(),
	}, nil
}

func (i *SchemaInspector) checkMySQLTableNaming() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		name := toString(row["table_name"])
		if name != strings.ToLower(name) {
			issues = append(issues, models.SchemaIssue{
				Table: name, Issue: "表名包含大写字母", Suggestion: "建议使用小写字母和下划线",
			})
		}
		if strings.HasPrefix(name, "tb_") || strings.HasPrefix(name, "t_") {
			issues = append(issues, models.SchemaIssue{
				Table: name, Issue: "表名使用冗余前缀", Suggestion: "建议直接使用名词，不加 tb_ 或 t_ 前缀",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLColumnNaming() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	reservedWords := map[string]bool{
		"select": true, "insert": true, "update": true, "delete": true, "order": true, "group": true,
	}
	for _, row := range rows {
		name := toString(row["column_name"])
		tableName := toString(row["table_name"])
		if name != strings.ToLower(name) {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: name, Issue: "列名包含大写字母", Suggestion: "建议使用小写字母和下划线",
			})
		}
		if reservedWords[strings.ToLower(name)] {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: name, Issue: "列名使用保留字", Suggestion: fmt.Sprintf("避免使用 %s 作为列名", name),
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLPrimaryKeys() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	tables, err := i.conn.ExecuteQuery(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, t := range tables {
		tableName := toString(t["table_name"])
		pk, err := i.conn.ExecuteQuery(`
			SELECT column_name FROM information_schema.key_column_usage
			WHERE table_schema = ? AND table_name = ?
			AND constraint_name = 'PRIMARY'
		`, dbName, tableName)
		if err != nil {
			continue
		}
		if len(pk) == 0 {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Issue: "表缺少主键", Suggestion: "建议为每个表添加主键",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLIndexes() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, index_name, column_name
		FROM information_schema.statistics
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}

	type indexKey struct{ table, index string }
	indexDict := make(map[indexKey][]string)
	var indexOrder []indexKey
	for _, row := range rows {
		key := indexKey{toString(row["table_name"]), toString(row["index_name"])}
		if _, exists := indexDict[key]; !exists {
			indexOrder = append(indexOrder, key)
		}
		indexDict[key] = append(indexDict[key], toString(row["column_name"]))
	}

	type colsKey struct{ table, cols string }
	seen := make(map[colsKey]string)
	for _, key := range indexOrder {
		cols := strings.Join(indexDict[key], ",")
		ck := colsKey{key.table, cols}
		if prev, exists := seen[ck]; exists {
			issues = append(issues, models.SchemaIssue{
				Table: key.table, Index: key.index,
				Issue:      fmt.Sprintf("可能存在重复索引: %s", prev),
				Suggestion: "检查并删除重复索引",
			})
		} else {
			seen[ck] = key.index
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLConstraints() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, constraint_name
		FROM information_schema.table_constraints
		WHERE table_schema = ? AND constraint_type = 'FOREIGN KEY'
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		issues = append(issues, models.SchemaIssue{
			Table:      toString(row["table_name"]),
			Constraint: toString(row["constraint_name"]),
			Issue:      "存在外键约束",
			Suggestion: "确保外键列上有索引以提高性能",
		})
	}
	return issues
}

func (i *SchemaInspector) checkMySQLDataTypes() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, column_name, data_type, column_type
		FROM information_schema.columns
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		dataType := toString(row["data_type"])
		tableName := toString(row["table_name"])
		colName := toString(row["column_name"])
		if dataType == "float" || dataType == "double" {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: colName,
				Issue:      fmt.Sprintf("使用 %s 存储浮点数", dataType),
				Suggestion: "金额类数据建议使用 DECIMAL",
			})
		}
		if dataType == "timestamp" {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Column: colName,
				Issue: "使用 TIMESTAMP 类型", Suggestion: "注意 TIMESTAMP 的时间范围限制 (1970-2038)",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLComments() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, table_comment
		FROM information_schema.tables
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		comment := toString(row["table_comment"])
		if comment == "" {
			issues = append(issues, models.SchemaIssue{
				Table: toString(row["table_name"]), Issue: "表缺少注释", Suggestion: "建议为表添加注释说明",
			})
		}
	}
	return issues
}

func (i *SchemaInspector) checkMySQLLargeTables() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	threshold := int64(i.cfg.TableSizeThreshold) * 1024 * 1024
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, 
		       data_length + index_length as total_size,
		       table_rows
		FROM information_schema.tables
		WHERE table_schema = ?
		AND data_length + index_length > ?
		ORDER BY data_length + index_length DESC
	`, dbName, threshold)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		sizeBytes := toInt64(row["total_size"])
		sizeMB := float64(sizeBytes) / (1024 * 1024)
		tableRows := toInt64(row["table_rows"])
		issues = append(issues, models.SchemaIssue{
			Table:      toString(row["table_name"]),
			Issue:      fmt.Sprintf("表过大 (%.2f MB, %d 行)", sizeMB, tableRows),
			Suggestion: "考虑分区或归档历史数据",
		})
	}
	return issues
}

func (i *SchemaInspector) checkMySQLEngineCharset() []models.SchemaIssue {
	var issues []models.SchemaIssue
	dbName := i.conn.Config().Database
	rows, err := i.conn.ExecuteQuery(`
		SELECT table_name, engine, table_collation
		FROM information_schema.tables
		WHERE table_schema = ?
	`, dbName)
	if err != nil {
		return append(issues, models.SchemaIssue{Issue: err.Error()})
	}
	for _, row := range rows {
		tableName := toString(row["table_name"])
		engine := toString(row["engine"])
		collation := toString(row["table_collation"])
		if engine != "" && strings.ToLower(engine) != "innodb" {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Issue: fmt.Sprintf("使用 %s 引擎", engine),
				Suggestion: "建议使用 InnoDB 引擎以获得更好的事务支持",
			})
		}
		if collation != "" && !strings.Contains(collation, "utf8mb4") {
			issues = append(issues, models.SchemaIssue{
				Table: tableName, Issue: fmt.Sprintf("字符集为 %s", collation),
				Suggestion: "建议使用 utf8mb4 以支持完整的 Unicode",
			})
		}
	}
	return issues
}
