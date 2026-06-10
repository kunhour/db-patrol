package inspector

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"db-patrol/internal/connection"
	"db-patrol/internal/models"
	"db-patrol/internal/utils"
)

// BasicInfoInspector 基本信息巡检器
type BasicInfoInspector struct {
	conn connection.Connection
	cfg  models.InspectionConfig
}

// NewBasicInfoInspector 创建基本信息巡检器
func NewBasicInfoInspector(conn connection.Connection, cfg models.InspectionConfig) *BasicInfoInspector {
	return &BasicInfoInspector{conn: conn, cfg: cfg}
}

// Name 返回巡检器名称
func (i *BasicInfoInspector) Name() string { return "basic_info" }

// Title 返回巡检器标题
func (i *BasicInfoInspector) Title() string { return "检查基本信息" }

// Inspect 执行巡检
func (i *BasicInfoInspector) Inspect() (map[string]interface{}, error) {
	dbType := i.conn.Config().Type
	if strings.Contains(strings.ToLower(dbType), "pg") || strings.Contains(strings.ToLower(dbType), "postgres") {
		return i.inspectPG()
	}
	return i.inspectMySQL()
}

// ==================== PG ====================

func (i *BasicInfoInspector) inspectPG() (map[string]interface{}, error) {
	// 获取数据库列表
	dbRows, err := i.conn.ExecuteQuery(`
		SELECT d.datname as name,
			pg_database_size(d.datname) as size,
			pg_encoding_to_char(d.encoding) as encoding,
			d.datcollate as collation,
			d.datctype as ctype,
			d.datistemplate as is_template,
			d.datallowconn as allow_conn
		FROM pg_database d
		WHERE d.datistemplate = false
		ORDER BY pg_database_size(d.datname) DESC
	`)
	if err != nil {
		dbRows = []map[string]interface{}{}
	}

	var databases []models.DatabaseInfo
	var dbNames []string
	for _, row := range dbRows {
		dbName := toString(row["name"])
		dbNames = append(dbNames, dbName)
		databases = append(databases, models.DatabaseInfo{
			Name:      dbName,
			Size:      utils.FormatSize(toInt64(row["size"])),
			SizeBytes: toInt64(row["size"]),
			Encoding:  toString(row["encoding"]),
			Collation: toString(row["collation"]),
			Ctype:     toString(row["ctype"]),
		})
	}

	// 并行获取各数据库详细信息
	var allTables []models.TableInfo
	tablesWithoutPK := make(map[string][]models.TableWithoutPK)

	if len(dbNames) > 0 {
		results := i.inspectPGDatabasesParallel(dbNames)
		for idx := range databases {
			dbName := databases[idx].Name
			if r, ok := results[dbName]; ok {
				databases[idx].SchemaCount = r.Stats["schema_count"]
				databases[idx].TableCount = r.Stats["table_count"]
				databases[idx].ViewCount = r.Stats["view_count"]
				databases[idx].TriggerCount = r.Stats["trigger_count"]
				allTables = append(allTables, r.Tables...)
				if len(r.TablesWithoutPK) > 0 {
					tablesWithoutPK[dbName] = r.TablesWithoutPK
				}
			}
		}
	}

	// 备份检测
	databases = i.detectBackupDatabases(databases)
	allTables = i.detectBackupTables(allTables)

	normalDBs, backupDBs := splitDatabases(databases)
	normalTables, backupTables := splitTables(allTables)

	return map[string]interface{}{
		"instance_info":     i.getPGInstanceInfo(),
		"version":           i.getPGVersion(),
		"connection_status": i.checkConnection(),
		"uptime":            i.getPGUptime(),
		"settings":          i.getPGSettings(),
		"databases": map[string]interface{}{
			"total":               len(databases),
			"normal":              normalDBs,
			"backup":              backupDBs,
			"normal_count":        len(normalDBs),
			"backup_count":        len(backupDBs),
			"backup_total_size":   utils.FormatSize(sumDBSize(backupDBs)),
			"backup_total_tables": sumDBTableCount(backupDBs),
			"backup_total_views":  sumDBViewCount(backupDBs),
			"backup_total_triggers": sumDBTriggerCount(backupDBs),
		},
		"tables": map[string]interface{}{
			"all":              allTables,
			"normal":           normalTables,
			"backup":           backupTables,
			"total_count":      len(allTables),
			"normal_count":     len(normalTables),
			"backup_count":     len(backupTables),
			"backup_total_size": utils.FormatSize(sumTableSize(backupTables)),
		},
		"tables_without_pk": tablesWithoutPK,
	}, nil
}

type pgDBResult struct {
	Stats         map[string]int
	Tables        []models.TableInfo
	TablesWithoutPK []models.TableWithoutPK
}

func (i *BasicInfoInspector) inspectPGDatabasesParallel(dbNames []string) map[string]pgDBResult {
	results := make(map[string]pgDBResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)

	for _, dbName := range dbNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data := i.inspectPGDatabaseAll(name)
			mu.Lock()
			results[name] = data
			mu.Unlock()
		}(dbName)
	}
	wg.Wait()
	return results
}

func (i *BasicInfoInspector) inspectPGDatabaseAll(dbName string) pgDBResult {
	cfg := i.conn.Config()
	cfg.Database = dbName

	result := pgDBResult{
		Stats:  map[string]int{},
		Tables: []models.TableInfo{},
	}

	conn, err := connection.CreateConnection(cfg)
	if err != nil {
		return result
	}
	defer conn.Close()

	// 统计信息
	statsRows, err := conn.ExecuteQuery(`
		SELECT
			(SELECT COUNT(*) FROM pg_tables
			 WHERE schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')) as table_count,
			(SELECT COUNT(*) FROM pg_views
			 WHERE schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')) as view_count,
			(SELECT COUNT(*) FROM pg_trigger t
			 JOIN pg_class c ON t.tgrelid = c.oid
			 JOIN pg_namespace n ON c.relnamespace = n.oid
			 WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')) as trigger_count,
			(SELECT COUNT(*) FROM information_schema.schemata
			 WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')) as schema_count
	`)
	if err == nil && len(statsRows) > 0 {
		result.Stats["table_count"] = toInt(statsRows[0]["table_count"])
		result.Stats["view_count"] = toInt(statsRows[0]["view_count"])
		result.Stats["trigger_count"] = toInt(statsRows[0]["trigger_count"])
		result.Stats["schema_count"] = toInt(statsRows[0]["schema_count"])
	}

	// 表信息
	tableRows, err := conn.ExecuteQuery(`
		SELECT
			t.schemaname,
			t.tablename as table_name,
			pg_size_pretty(pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename))) as size,
			pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename)) as size_bytes,
			(SELECT COUNT(*) FROM information_schema.columns c
			 WHERE c.table_schema = t.schemaname AND c.table_name = t.tablename) as column_count,
			COALESCE(c.reltuples, 0)::bigint as row_count
		FROM pg_tables t
		JOIN pg_class c ON c.relname = t.tablename
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
		WHERE t.schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY pg_total_relation_size(quote_ident(t.schemaname)||'.'||quote_ident(t.tablename)) DESC
	`)
	if err == nil {
		for _, row := range tableRows {
			result.Tables = append(result.Tables, models.TableInfo{
				Database:    dbName,
				Schema:      toString(row["schemaname"]),
				TableName:   toString(row["table_name"]),
				Size:        toString(row["size"]),
				SizeBytes:   toInt64(row["size_bytes"]),
				ColumnCount: toInt(row["column_count"]),
				RowCount:    toInt64(row["row_count"]),
			})
		}
	}

	// 无主键表
	pkRows, err := conn.ExecuteQuery(`
		SELECT
			t.schemaname,
			t.tablename as table_name,
			COALESCE(c.reltuples, 0)::bigint as row_count,
			EXISTS (
				SELECT 1 FROM information_schema.table_constraints tc
				WHERE tc.table_schema = t.schemaname
				AND tc.table_name = t.tablename
				AND tc.constraint_type = 'PRIMARY KEY'
			) as has_primary_key,
			EXISTS (
				SELECT 1 FROM pg_indexes pi
				WHERE pi.schemaname = t.schemaname
				AND pi.tablename = t.tablename
				AND pi.indexname IN (
					SELECT indexname FROM pg_indexes pi2
					WHERE pi2.schemaname = t.schemaname
					AND pi2.tablename = t.tablename
					AND EXISTS (
						SELECT 1 FROM pg_class pc
						JOIN pg_index pindex ON pc.oid = pindex.indexrelid
						WHERE pc.relname = pi2.indexname
						AND pindex.indisunique = true
					)
				)
			) as has_unique_index
		FROM pg_tables t
		JOIN pg_class c ON c.relname = t.tablename
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.schemaname
		WHERE t.schemaname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY c.reltuples DESC NULLS LAST
	`)
	if err == nil {
		for _, row := range pkRows {
			hasPK := toBool(row["has_primary_key"])
			hasUnique := toBool(row["has_unique_index"])
			if !hasPK && !hasUnique {
				schema := toString(row["schemaname"])
				tableName := toString(row["table_name"])
				// 获取大小
				sizeRows, _ := conn.ExecuteQuery(`
					SELECT
						pg_size_pretty(pg_total_relation_size(quote_ident($1)||'.'||quote_ident($2))) as size,
						pg_total_relation_size(quote_ident($1)||'.'||quote_ident($2)) as size_bytes,
						(SELECT COUNT(*) FROM information_schema.columns c
						 WHERE c.table_schema = $1 AND c.table_name = $2) as column_count
				`, schema, tableName)
				if len(sizeRows) > 0 {
					result.TablesWithoutPK = append(result.TablesWithoutPK, models.TableWithoutPK{
						Schema:      schema,
						TableName:   tableName,
						Size:        toString(sizeRows[0]["size"]),
						SizeBytes:   toInt64(sizeRows[0]["size_bytes"]),
						ColumnCount: toInt(sizeRows[0]["column_count"]),
						RowCount:    toInt64(row["row_count"]),
					})
				} else {
					result.TablesWithoutPK = append(result.TablesWithoutPK, models.TableWithoutPK{
						Schema:    schema,
						TableName: tableName,
						RowCount:  toInt64(row["row_count"]),
					})
				}
			}
		}
	}

	return result
}

func (i *BasicInfoInspector) getPGInstanceInfo() models.InstanceInfo {
	info := models.InstanceInfo{}
	// version
	rows, _ := i.conn.ExecuteQuery("SELECT version()")
	if len(rows) > 0 {
		info.FullVersion = toString(rows[0]["version"])
		re := regexp.MustCompile(`^(PostgreSQL|Vastbase|openGauss)\s+([\d.]+)`)
		if m := re.FindStringSubmatch(info.FullVersion); m != nil {
			info.ProductName = m[1]
			info.ProductVersion = m[2]
		}
	}
	// current_database
	rows, _ = i.conn.ExecuteQuery("SELECT current_database()")
	if len(rows) > 0 {
		info.CurrentDatabase = toString(rows[0]["current_database"])
	}
	// total size
	rows, _ = i.conn.ExecuteQuery("SELECT SUM(pg_database_size(datname)) as total_size FROM pg_database WHERE datistemplate = false")
	if len(rows) > 0 {
		info.TotalSizeBytes = toInt64(rows[0]["total_size"])
		info.TotalSize = utils.FormatSize(info.TotalSizeBytes)
	}
	// database count
	rows, _ = i.conn.ExecuteQuery("SELECT COUNT(*) as count FROM pg_database WHERE datistemplate = false")
	if len(rows) > 0 {
		info.DatabaseCount = toInt(rows[0]["count"])
	}
	// max_connections
	rows, _ = i.conn.ExecuteQuery("SHOW max_connections")
	if len(rows) > 0 {
		info.MaxConnections = toString(rows[0]["max_connections"])
	}
	// current connections
	rows, _ = i.conn.ExecuteQuery("SELECT COUNT(*) as count FROM pg_stat_activity")
	if len(rows) > 0 {
		info.CurrentConnections = toInt(rows[0]["count"])
	}
	// shared_buffers
	rows, _ = i.conn.ExecuteQuery("SHOW shared_buffers")
	if len(rows) > 0 {
		info.SharedBuffers = toString(rows[0]["shared_buffers"])
	}
	// db_time
	rows, _ = i.conn.ExecuteQuery("SELECT NOW() as db_time")
	if len(rows) > 0 {
		info.DBTime = fmt.Sprintf("%v", rows[0]["db_time"])
	}
	// timezone
	rows, _ = i.conn.ExecuteQuery("SHOW timezone")
	if len(rows) > 0 {
		info.Timezone = toString(rows[0]["TimeZone"])
	}
	// data_directory
	rows, _ = i.conn.ExecuteQuery("SHOW data_directory")
	if len(rows) > 0 {
		info.DataDirectory = toString(rows[0]["data_directory"])
	}
	// listen_addresses
	rows, _ = i.conn.ExecuteQuery("SHOW listen_addresses")
	if len(rows) > 0 {
		info.ListenAddresses = toString(rows[0]["listen_addresses"])
	}
	// port
	rows, _ = i.conn.ExecuteQuery("SHOW port")
	if len(rows) > 0 {
		info.Port = toString(rows[0]["port"])
	}
	// case_sensitive
	rows, _ = i.conn.ExecuteQuery("SHOW enable_case_sensitive")
	if len(rows) > 0 {
		val := toString(rows[0]["enable_case_sensitive"])
		if val == "on" {
			info.CaseSensitive = "区分大小写"
		} else if val == "off" {
			info.CaseSensitive = "忽略大小写"
		} else {
			info.CaseSensitive = val
		}
	} else {
		info.CaseSensitive = "标准行为(加引号区分)"
	}
	return info
}

func (i *BasicInfoInspector) getPGVersion() string {
	rows, _ := i.conn.ExecuteQuery("SELECT version()")
	if len(rows) > 0 {
		return toString(rows[0]["version"])
	}
	return "未知"
}

func (i *BasicInfoInspector) getPGUptime() string {
	rows, _ := i.conn.ExecuteQuery("SELECT pg_postmaster_start_time() as start_time")
	if len(rows) > 0 {
		return fmt.Sprintf("%v", rows[0]["start_time"])
	}
	return "未知"
}

func (i *BasicInfoInspector) getPGSettings() map[string]string {
	settings := map[string]string{}
	configQueries := []string{
		"max_connections", "shared_buffers", "work_mem", "maintenance_work_mem", "effective_cache_size",
		"wal_level", "max_wal_size", "min_wal_size", "wal_buffers",
		"checkpoint_completion_target", "checkpoint_timeout",
		"autovacuum", "autovacuum_max_workers", "autovacuum_naptime",
		"random_page_cost", "default_statistics_target", "effective_io_concurrency",
		"max_parallel_workers_per_gather", "max_parallel_workers",
		"logging_collector", "log_statement", "log_min_duration_statement",
		"timezone", "max_locks_per_transaction",
	}
	query := "SELECT name, setting FROM pg_settings WHERE name IN ("
	params := make([]interface{}, len(configQueries))
	for i, name := range configQueries {
		if i > 0 {
			query += ","
		}
		query += fmt.Sprintf("$%d", i+1)
		params[i] = name
	}
	query += ")"
	rows, err := i.conn.ExecuteQuery(query, params...)
	if err == nil {
		for _, row := range rows {
			settings[toString(row["name"])] = toString(row["setting"])
		}
	}
	// fallback
	for _, name := range configQueries {
		if _, ok := settings[name]; !ok {
			rows, _ := i.conn.ExecuteQuery("SHOW " + name)
			if len(rows) > 0 {
				for k, v := range rows[0] {
					if k == name || k == "setting" {
						settings[name] = toString(v)
						break
					}
				}
			}
		}
	}
	return settings
}

func (i *BasicInfoInspector) checkConnection() models.ConnectionStatus {
	_, err := i.conn.ExecuteQuery("SELECT 1")
	if err != nil {
		return models.ConnectionStatus{Status: "异常", Message: err.Error()}
	}
	return models.ConnectionStatus{Status: "正常", Message: "连接成功"}
}

// ==================== MySQL ====================

func (i *BasicInfoInspector) inspectMySQL() (map[string]interface{}, error) {
	databases := i.getMySQLDatabases()
	var allTables []models.TableInfo
	tablesWithoutPK := make(map[string][]models.TableWithoutPK)

	for _, db := range databases {
		dbTables := i.getMySQLTables(db.Name)
		allTables = append(allTables, dbTables...)
		for idx := range databases {
			if databases[idx].Name == db.Name {
				databases[idx].TableCount = len(dbTables)
			}
		}
		withoutPK := i.getMySQLTablesWithoutPK(db.Name)
		if len(withoutPK) > 0 {
			tablesWithoutPK[db.Name] = withoutPK
		}
	}

	databases = i.detectBackupDatabases(databases)
	allTables = i.detectBackupTables(allTables)

	normalDBs, backupDBs := splitDatabases(databases)
	normalTables, backupTables := splitTables(allTables)

	return map[string]interface{}{
		"instance_info":     i.getMySQLInstanceInfo(),
		"version":           i.getMySQLVersion(),
		"connection_status": i.checkConnection(),
		"uptime":            i.getMySQLUptime(),
		"settings":          i.getMySQLSettings(),
		"databases": map[string]interface{}{
			"total":                len(databases),
			"normal":               normalDBs,
			"backup":               backupDBs,
			"normal_count":         len(normalDBs),
			"backup_count":         len(backupDBs),
			"backup_total_size":    utils.FormatSize(sumDBSize(backupDBs)),
			"backup_total_tables":  sumDBTableCount(backupDBs),
			"backup_total_views":   0,
			"backup_total_triggers": 0,
		},
		"tables": map[string]interface{}{
			"all":               allTables,
			"normal":            normalTables,
			"backup":            backupTables,
			"total_count":       len(allTables),
			"normal_count":      len(normalTables),
			"backup_count":      len(backupTables),
			"backup_total_size": utils.FormatSize(sumTableSize(backupTables)),
		},
		"tables_without_pk": tablesWithoutPK,
	}, nil
}

func (i *BasicInfoInspector) getMySQLDatabases() []models.DatabaseInfo {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT schema_name as name, DEFAULT_CHARACTER_SET_NAME as encoding, DEFAULT_COLLATION_NAME as collation
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
		ORDER BY schema_name
	`)
	var databases []models.DatabaseInfo
	for _, row := range rows {
		dbName := toString(row["name"])
		sizeRows, _ := i.conn.ExecuteQuery(`
			SELECT SUM(data_length + index_length) as size
			FROM information_schema.tables
			WHERE table_schema = ?
		`, dbName)
		sizeBytes := int64(0)
		if len(sizeRows) > 0 {
			sizeBytes = toInt64(sizeRows[0]["size"])
		}
		databases = append(databases, models.DatabaseInfo{
			Name:         dbName,
			Size:         utils.FormatSize(sizeBytes),
			SizeBytes:    sizeBytes,
			Encoding:     toString(row["encoding"]),
			Collation:    toString(row["collation"]),
			SchemaCount:  1,
		})
	}
	return databases
}

func (i *BasicInfoInspector) getMySQLTables(dbName string) []models.TableInfo {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT table_name, engine, table_rows, data_length + index_length as size_bytes
		FROM information_schema.tables
		WHERE table_schema = ?
		ORDER BY data_length + index_length DESC
	`, dbName)
	var tables []models.TableInfo
	for _, row := range rows {
		sizeBytes := toInt64(row["size_bytes"])
		tables = append(tables, models.TableInfo{
			Database:   dbName,
			Schema:     dbName,
			TableName:  toString(row["table_name"]),
			Size:       utils.FormatSize(sizeBytes),
			SizeBytes:  sizeBytes,
			RowCount:   toInt64(row["table_rows"]),
			Engine:     toString(row["engine"]),
		})
	}
	return tables
}

func (i *BasicInfoInspector) getMySQLTablesWithoutPK(dbName string) []models.TableWithoutPK {
	rows, _ := i.conn.ExecuteQuery(`
		SELECT t.table_name, t.table_rows, t.data_length + t.index_length as size_bytes
		FROM information_schema.tables t
		WHERE t.table_schema = ? AND t.table_type = 'BASE TABLE'
		AND NOT EXISTS (
			SELECT 1 FROM information_schema.table_constraints tc
			WHERE tc.table_schema = t.table_schema AND tc.table_name = t.table_name
			AND tc.constraint_type = 'PRIMARY KEY'
		)
		ORDER BY t.data_length + t.index_length DESC
	`, dbName)
	var result []models.TableWithoutPK
	for _, row := range rows {
		sizeBytes := toInt64(row["size_bytes"])
		result = append(result, models.TableWithoutPK{
			Schema:    dbName,
			TableName: toString(row["table_name"]),
			Size:      utils.FormatSize(sizeBytes),
			SizeBytes: sizeBytes,
			RowCount:  toInt64(row["table_rows"]),
		})
	}
	return result
}

func (i *BasicInfoInspector) getMySQLVersion() string {
	rows, _ := i.conn.ExecuteQuery("SELECT version() as version")
	if len(rows) > 0 {
		return toString(rows[0]["version"])
	}
	return "未知"
}

func (i *BasicInfoInspector) getMySQLInstanceInfo() models.InstanceInfo {
	info := models.InstanceInfo{}
	rows, _ := i.conn.ExecuteQuery("SELECT version() as version")
	if len(rows) > 0 {
		info.FullVersion = toString(rows[0]["version"])
		re := regexp.MustCompile(`^([\w\-]+)\s+([\d.]+)`)
		if m := re.FindStringSubmatch(info.FullVersion); m != nil {
			info.ProductName = m[1]
			info.ProductVersion = m[2]
		}
	}
	rows, _ = i.conn.ExecuteQuery("SELECT DATABASE() as db")
	if len(rows) > 0 {
		info.CurrentDatabase = toString(rows[0]["db"])
	}
	rows, _ = i.conn.ExecuteQuery("SELECT NOW() as db_time")
	if len(rows) > 0 {
		info.DBTime = fmt.Sprintf("%v", rows[0]["db_time"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'port'")
	if len(rows) > 0 {
		info.Port = toString(rows[0]["Value"])
	}
	rows, _ = i.conn.ExecuteQuery(`
		SELECT SUM(data_length + index_length) as total_size
		FROM information_schema.tables
		WHERE table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
	`)
	if len(rows) > 0 {
		info.TotalSizeBytes = toInt64(rows[0]["total_size"])
		info.TotalSize = utils.FormatSize(info.TotalSizeBytes)
	}
	rows, _ = i.conn.ExecuteQuery(`
		SELECT COUNT(*) as count FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')
	`)
	if len(rows) > 0 {
		info.DatabaseCount = toInt(rows[0]["count"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'max_connections'")
	if len(rows) > 0 {
		info.MaxConnections = toString(rows[0]["Value"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW STATUS LIKE 'Threads_connected'")
	if len(rows) > 0 {
		info.CurrentConnections = toInt(rows[0]["Value"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'innodb_buffer_pool_size'")
	if len(rows) > 0 {
		info.SharedBuffers = toString(rows[0]["Value"])
	}
	rows, _ = i.conn.ExecuteQuery("SHOW VARIABLES LIKE 'character_set_server'")
	if len(rows) > 0 {
		info.Encoding = toString(rows[0]["Value"])
	}
	info.CaseSensitive = "由 lower_case_table_names 控制"
	return info
}

func (i *BasicInfoInspector) getMySQLUptime() string {
	rows, _ := i.conn.ExecuteQuery("SHOW STATUS LIKE 'Uptime'")
	if len(rows) > 0 {
		seconds := toInt64(rows[0]["Value"])
		return utils.FormatDuration(seconds)
	}
	return "未知"
}

func (i *BasicInfoInspector) getMySQLSettings() map[string]string {
	settings := map[string]string{}
	varMapping := map[string]string{
		"max_connections": "max_connections",
		"shared_buffers":  "innodb_buffer_pool_size",
		"work_mem":        "sort_buffer_size",
		"wal_level":       "binlog_format",
		"autovacuum":      "event_scheduler",
		"timezone":        "time_zone",
	}
	for pgKey, mysqlVar := range varMapping {
		rows, _ := i.conn.ExecuteQuery("SHOW VARIABLES LIKE ?", mysqlVar)
		if len(rows) > 0 {
			settings[pgKey] = toString(rows[0]["Value"])
		}
	}
	return settings
}

// ==================== 通用备份检测 ====================

func (i *BasicInfoInspector) detectBackupDatabases(databases []models.DatabaseInfo) []models.DatabaseInfo {
	allNames := make([]string, len(databases))
	for idx, db := range databases {
		allNames[idx] = db.Name
	}

	strongPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)_backup`), regexp.MustCompile(`(?i)_bak`), regexp.MustCompile(`(?i)_copy`),
		regexp.MustCompile(`(?i)[_\-]old$`), regexp.MustCompile(`_\d{8}$`), regexp.MustCompile(`_\d{6}$`),
		regexp.MustCompile(`_\d{4}[-_]\d{2}[-_]\d{2}`), regexp.MustCompile(`(?i)[_\-]test\d*$`),
		regexp.MustCompile(`(?i)^test[_\-]`), regexp.MustCompile(`(?i)[_\-]temp$`), regexp.MustCompile(`(?i)[_\-]dev$`),
	}
	weakPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)[_\-]new$`), regexp.MustCompile(`(?i)[_\-]prod$`), regexp.MustCompile(`\d{3,}$`), regexp.MustCompile(`\d{4}$`),
	}
	comboPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(test|dev|temp|new|prod)$`),
	}

	excludeNames := map[string]bool{"emate_dev": true}

	for idx := range databases {
		name := databases[idx].Name
		if excludeNames[name] {
			continue
		}
		isBackup := false
		for _, re := range strongPatterns {
			if re.MatchString(name) {
				isBackup = true
				break
			}
		}
		if !isBackup {
			for _, re := range weakPatterns {
				if re.MatchString(name) {
					base := getDBBaseName(name)
					if base != "" && hasSimilarDB(base, name, allNames) {
						isBackup = true
						break
					}
				}
			}
		}
		if !isBackup {
			for _, re := range comboPatterns {
				if loc := re.FindStringIndex(name); loc != nil {
					base := name[:loc[0]]
					if base != "" && hasSimilarDB(base, name, allNames) {
						isBackup = true
						break
					}
				}
			}
		}
		databases[idx].IsBackup = isBackup
	}
	return databases
}

func (i *BasicInfoInspector) detectBackupTables(tables []models.TableInfo) []models.TableInfo {
	excludePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)_nod_old$`), regexp.MustCompile(`(?i)_lin_old$`), regexp.MustCompile(`(?i)_net_old$`),
	}
	backupPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)_backup`), regexp.MustCompile(`(?i)_bak`), regexp.MustCompile(`(?i)_copy`),
		regexp.MustCompile(`(?i)_old$`), regexp.MustCompile(`(?i)_new$`), regexp.MustCompile(`(?i)_temp$`), regexp.MustCompile(`(?i)_tmp$`),
	}
	datePatterns := []*regexp.Regexp{
		regexp.MustCompile(`_\d{8}$`), regexp.MustCompile(`_\d{6}$`), regexp.MustCompile(`_\d{4}[-_]\d{2}[-_]\d{2}`), regexp.MustCompile(`_\d{4}$`),
	}

	for idx := range tables {
		name := tables[idx].TableName
		isBackup := false
		excluded := false
		for _, re := range excludePatterns {
			if re.MatchString(name) {
				excluded = true
				break
			}
		}
		if !excluded {
			for _, re := range backupPatterns {
				if re.MatchString(name) {
					isBackup = true
					break
				}
			}
			if !isBackup {
				for _, re := range datePatterns {
					if re.MatchString(name) {
						base := getTableBaseName(name)
						if base != "" {
							hasSimilar := false
							for _, t := range tables {
								if t.TableName != name && getTableBaseName(t.TableName) == base {
									hasSimilar = true
									break
								}
							}
							if !hasSimilar {
								isBackup = true
							}
						} else {
							isBackup = true
						}
						break
					}
				}
			}
		}
		tables[idx].IsBackup = isBackup
	}
	return tables
}

// ==================== 辅助函数 ====================

func getDBBaseName(dbName string) string {
	suffixPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)_(test|temp|new|dev|prod)$`), regexp.MustCompile(`_\d{4}$`), regexp.MustCompile(`\d{3,}$`),
	}
	for _, re := range suffixPatterns {
		if loc := re.FindStringIndex(dbName); loc != nil {
			return dbName[:loc[0]]
		}
	}
	return ""
}

func hasSimilarDB(baseName, currentDB string, allDBNames []string) bool {
	baseLower := strings.ToLower(baseName)
	currentLower := strings.ToLower(currentDB)
	for _, dbName := range allDBNames {
		dbLower := strings.ToLower(dbName)
		if dbLower == currentLower {
			continue
		}
		if dbLower == baseLower || strings.HasPrefix(dbLower, baseLower+"_") {
			return true
		}
	}
	return false
}

func getTableBaseName(tableName string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`_\d{8}$`), regexp.MustCompile(`_\d{6}$`), regexp.MustCompile(`_\d{4}[-_]\d{2}[-_]\d{2}$`), regexp.MustCompile(`_\d{4}$`),
	}
	for _, re := range patterns {
		if loc := re.FindStringIndex(tableName); loc != nil {
			return tableName[:loc[0]]
		}
	}
	return ""
}

func splitDatabases(dbs []models.DatabaseInfo) (normal, backup []models.DatabaseInfo) {
	for _, db := range dbs {
		if db.IsBackup {
			backup = append(backup, db)
		} else {
			normal = append(normal, db)
		}
	}
	return
}

func splitTables(tables []models.TableInfo) (normal, backup []models.TableInfo) {
	for _, t := range tables {
		if t.IsBackup {
			backup = append(backup, t)
		} else {
			normal = append(normal, t)
		}
	}
	return
}

func sumDBSize(dbs []models.DatabaseInfo) int64 {
	var sum int64
	for _, db := range dbs {
		sum += db.SizeBytes
	}
	return sum
}

func sumDBTableCount(dbs []models.DatabaseInfo) int {
	var sum int
	for _, db := range dbs {
		sum += db.TableCount
	}
	return sum
}

func sumDBViewCount(dbs []models.DatabaseInfo) int {
	var sum int
	for _, db := range dbs {
		sum += db.ViewCount
	}
	return sum
}

func sumDBTriggerCount(dbs []models.DatabaseInfo) int {
	var sum int
	for _, db := range dbs {
		sum += db.TriggerCount
	}
	return sum
}

func sumTableSize(tables []models.TableInfo) int64 {
	var sum int64
	for _, t := range tables {
		sum += t.SizeBytes
	}
	return sum
}

// ==================== 类型转换辅助函数 ====================

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case sql.NullString:
		if val.Valid {
			return val.String
		}
		return ""
	case sql.NullInt64:
		if val.Valid {
			return fmt.Sprintf("%d", val.Int64)
		}
		return "0"
	case sql.NullFloat64:
		if val.Valid {
			return fmt.Sprintf("%v", val.Float64)
		}
		return "0"
	case sql.NullBool:
		if val.Valid {
			return fmt.Sprintf("%t", val.Bool)
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var i int
		fmt.Sscanf(val, "%d", &i)
		return i
	case []byte:
		var i int
		fmt.Sscanf(string(val), "%d", &i)
		return i
	case sql.NullInt64:
		if val.Valid {
			return int(val.Int64)
		}
		return 0
	case sql.NullFloat64:
		if val.Valid {
			return int(val.Float64)
		}
		return 0
	default:
		return 0
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return i
	case []byte:
		var i int64
		fmt.Sscanf(string(val), "%d", &i)
		return i
	case sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return 0
	case sql.NullFloat64:
		if val.Valid {
			return int64(val.Float64)
		}
		return 0
	default:
		return 0
	}
}

func toBool(v interface{}) bool {
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
		return val == "t" || val == "true" || val == "1" || val == "on" || val == "yes"
	case []byte:
		s := string(val)
		return s == "t" || s == "true" || s == "1" || s == "on" || s == "yes"
	case sql.NullBool:
		return val.Valid && val.Bool
	default:
		return false
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	case []byte:
		var f float64
		fmt.Sscanf(string(val), "%f", &f)
		return f
	case sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return 0
	case sql.NullInt64:
		if val.Valid {
			return float64(val.Int64)
		}
		return 0
	default:
		return 0
	}
}
