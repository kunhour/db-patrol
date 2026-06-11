package models

import "time"

// DBConfig 数据库连接配置
type DBConfig struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type" yaml:"type"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
	Database string `json:"database" yaml:"database"`
	Schema   string `json:"schema" yaml:"schema"`
}

// InspectionConfig 巡检配置
type InspectionConfig struct {
	SlowQueryThreshold       float64 `yaml:"slow_query_threshold"`
	MaxConnectionsThreshold  int     `yaml:"max_connections_threshold"`
	TableSizeThreshold       int     `yaml:"table_size_threshold"`
	LongTransactionThreshold int     `yaml:"long_transaction_threshold"`
	Checks                   Checks  `yaml:"checks"`
}

// Checks 检查项开关
type Checks struct {
	BasicInfo   bool `yaml:"basic_info"`
	Performance bool `yaml:"performance"`
	Schema      bool `yaml:"schema"`
}

// ReportConfig 报告配置
type ReportConfig struct {
	Format             string `yaml:"format"`
	OutputDir          string `yaml:"output_dir"`
	IncludeSuggestions bool   `yaml:"include_suggestions"`
}

// AppConfig 顶层配置
type AppConfig struct {
	Inspection InspectionConfig `yaml:"inspection"`
	Report     ReportConfig     `yaml:"report"`
	Databases  []DBConfig       `yaml:"databases"`
}

// ConnectionStatus 连接状态
type ConnectionStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Name          string `json:"name"`
	Size          string `json:"size"`
	SizeBytes     int64  `json:"size_bytes"`
	Encoding      string `json:"encoding"`
	Collation     string `json:"collation"`
	Ctype         string `json:"ctype"`
	SchemaCount   int    `json:"schema_count"`
	TableCount    int    `json:"table_count"`
	ViewCount     int    `json:"view_count"`
	TriggerCount  int    `json:"trigger_count"`
	IsBackup      bool   `json:"is_backup"`
}

// TableInfo 表信息
type TableInfo struct {
	Database     string `json:"database"`
	Schema       string `json:"schema"`
	TableName    string `json:"table_name"`
	Size         string `json:"size"`
	SizeBytes    int64  `json:"size_bytes"`
	ColumnCount  int    `json:"column_count"`
	RowCount     int64  `json:"row_count"`
	IsBackup     bool   `json:"is_backup"`
	Engine       string `json:"engine,omitempty"`
}

// TableWithoutPK 无主键表
type TableWithoutPK struct {
	Schema      string `json:"schema"`
	TableName   string `json:"table_name"`
	Size        string `json:"size"`
	SizeBytes   int64  `json:"size_bytes"`
	ColumnCount int    `json:"column_count"`
	RowCount    int64  `json:"row_count"`
}

// InstanceInfo 实例信息
type InstanceInfo struct {
	FullVersion       string `json:"full_version"`
	ProductName       string `json:"product_name"`
	ProductVersion    string `json:"product_version"`
	CurrentDatabase   string `json:"current_database"`
	TotalSize         string `json:"total_size"`
	TotalSizeBytes    int64  `json:"total_size_bytes"`
	DatabaseCount     int    `json:"database_count"`
	MaxConnections    string `json:"max_connections"`
	CurrentConnections int   `json:"current_connections"`
	SharedBuffers     string `json:"shared_buffers"`
	DBTime            string `json:"db_time"`
	Timezone          string `json:"timezone"`
	DataDirectory     string `json:"data_directory"`
	ListenAddresses   string `json:"listen_addresses"`
	Port              string `json:"port"`
	CaseSensitive     string `json:"case_sensitive"`
	Encoding          string `json:"encoding,omitempty"`
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	Current       int     `json:"current"`
	Max           int     `json:"max"`
	Active        int     `json:"active,omitempty"`
	Idle          int     `json:"idle,omitempty"`
	UsagePercent  float64 `json:"usage_percent"`
	Status        string  `json:"status"`
}

// CacheHitRatio 缓存命中率
type CacheHitRatio struct {
	Ratio       float64 `json:"ratio"`
	HeapRead    int64   `json:"heap_read"`
	HeapHit     int64   `json:"heap_hit"`
	Status      string  `json:"status"`
	Suggestion  string  `json:"suggestion"`
}

// IndexHitRatio 索引命中率
type IndexHitRatio struct {
	Ratio       float64 `json:"ratio"`
	IdxScan     int64   `json:"idx_scan"`
	SeqScan     int64   `json:"seq_scan"`
	Status      string  `json:"status"`
	Suggestion  string  `json:"suggestion"`
}

// ClientConnection 客户端连接
type ClientConnection struct {
	ClientIP          string `json:"client_ip"`
	TotalConnections  int    `json:"total_connections"`
	DatabaseCount     int    `json:"database_count"`
	Databases         string `json:"databases"`
	UserCount         int    `json:"user_count"`
	Users             string `json:"users"`
	ApplicationCount  int    `json:"application_count,omitempty"`
	Applications      string `json:"applications,omitempty"`
	Active            int    `json:"active"`
	Idle              int    `json:"idle"`
	IdleInTransaction int    `json:"idle_in_transaction"`
}

// LockInfo 锁信息
type LockInfo struct {
	BlockedPID       int     `json:"blocked_pid,omitempty"`
	BlockedUser      string  `json:"blocked_user,omitempty"`
	BlockingPID      int     `json:"blocking_pid,omitempty"`
	BlockingUser     string  `json:"blocking_user,omitempty"`
	Database         string  `json:"database,omitempty"`
	ApplicationName  string  `json:"application_name,omitempty"`
	ClientAddr       string  `json:"client_addr,omitempty"`
	LockType         string  `json:"locktype,omitempty"`
	BlockedMode      string  `json:"blocked_mode,omitempty"`
	BlockingMode     string  `json:"blocking_mode,omitempty"`
	BlockedState     string  `json:"blocked_state,omitempty"`
	WaitSeconds      float64 `json:"wait_seconds,omitempty"`
	BlockedQuery     string  `json:"blocked_query,omitempty"`
	BlockingQuery    string  `json:"blocking_query,omitempty"`
	Severity         string  `json:"severity,omitempty"`
	SeverityLabel    string  `json:"severity_label,omitempty"`
	WaitDisplay      string  `json:"wait_display,omitempty"`
	// MySQL fields
	WaitingTrxID     string  `json:"waiting_trx_id,omitempty"`
	WaitingThread    int     `json:"waiting_thread,omitempty"`
	BlockingTrxID    string  `json:"blocking_trx_id,omitempty"`
	BlockingThread   int     `json:"blocking_thread,omitempty"`
	WaitingLockMode  string  `json:"waiting_lock_mode,omitempty"`
	WaitingLockType  string  `json:"waiting_lock_type,omitempty"`
}

// LongTransaction 长事务
type LongTransaction struct {
	PID              int       `json:"pid,omitempty"`
	Database         string    `json:"database,omitempty"`
	Username         string    `json:"username,omitempty"`
	ClientAddr       string    `json:"client_addr,omitempty"`
	ApplicationName  string    `json:"application_name,omitempty"`
	State            string    `json:"state,omitempty"`
	QueryStart       time.Time `json:"query_start,omitempty"`
	DurationSeconds  float64   `json:"duration_seconds"`
	Severity         string    `json:"severity"`
	SeverityLabel    string    `json:"severity_label"`
	DurationDisplay  string    `json:"duration_display"`
	Query            string    `json:"query,omitempty"`
	// MySQL fields
	TrxID            string    `json:"trx_id,omitempty"`
	ThreadID         int       `json:"thread_id,omitempty"`
	TablesLocked     int       `json:"tables_locked,omitempty"`
	RowsLocked       int       `json:"rows_locked,omitempty"`
}

// DeadlockInfo 死锁信息
type DeadlockInfo struct {
	// PG fields
	PID              int     `json:"pid,omitempty"`
	Database         string  `json:"database,omitempty"`
	Username         string  `json:"username,omitempty"`
	ApplicationName  string  `json:"application_name,omitempty"`
	ClientAddr       string  `json:"client_addr,omitempty"`
	State            string  `json:"state,omitempty"`
	WaitEvent        string  `json:"wait_event,omitempty"`
	Query            string  `json:"query,omitempty"`
	DurationSeconds  float64 `json:"duration_seconds,omitempty"`
	// MySQL fields
	TrxID            string  `json:"trx_id,omitempty"`
	ThreadID         int     `json:"thread_id,omitempty"`
	State2           string  `json:"state2,omitempty"`
	TablesLocked     int     `json:"tables_locked,omitempty"`
	RowsLocked       int     `json:"rows_locked,omitempty"`
	// Common
	DeadlockCount    int64   `json:"deadlock_count,omitempty"`
	Severity         string  `json:"severity,omitempty"`
	SeverityLabel    string  `json:"severity_label,omitempty"`
	DurationDisplay  string  `json:"duration_display,omitempty"`
	Suggestion       string  `json:"suggestion,omitempty"`
}

// DeadTupleInfo 死元组信息
type DeadTupleInfo struct {
	Schema           string  `json:"schemaname"`
	TableName        string  `json:"table_name"`
	LiveTuples       int64   `json:"live_tuples"`
	DeadTuples       int64   `json:"dead_tuples"`
	DeadTupleRatio   float64 `json:"dead_tuple_ratio"`
	LastVacuum       *time.Time `json:"last_vacuum,omitempty"`
	LastAutovacuum   *time.Time `json:"last_autovacuum,omitempty"`
	LastAnalyze      *time.Time `json:"last_analyze,omitempty"`
	LastAutoanalyze  *time.Time `json:"last_autoanalyze,omitempty"`
	TableSize        string  `json:"table_size"`
	Severity         string  `json:"severity"`
	SeverityLabel    string  `json:"severity_label"`
	Suggestion       string  `json:"suggestion"`
}

// VacuumStatus VACUUM状态
type VacuumStatus struct {
	Schema           string     `json:"schemaname"`
	TableName        string     `json:"table_name"`
	LiveTuples       int64      `json:"live_tuples"`
	LastVacuum       *time.Time `json:"last_vacuum,omitempty"`
	LastAutovacuum   *time.Time `json:"last_autovacuum,omitempty"`
	LastAnalyze      *time.Time `json:"last_analyze,omitempty"`
	LastAutoanalyze  *time.Time `json:"last_autoanalyze,omitempty"`
	VacuumStatus     string     `json:"vacuum_status"`
	Severity         string     `json:"severity"`
	Suggestion       string     `json:"suggestion"`
}

// IOStats IO统计
type IOStats struct {
	Schema           string  `json:"schemaname"`
	TableName        string  `json:"table_name"`
	DiskReads        int64   `json:"disk_reads"`
	BufferHits       int64   `json:"buffer_hits"`
	IndexDiskReads   int64   `json:"index_disk_reads"`
	IndexBufferHits  int64   `json:"index_buffer_hits"`
	ToastDiskReads   int64   `json:"toast_disk_reads,omitempty"`
	ToastBufferHits  int64   `json:"toast_buffer_hits,omitempty"`
	CacheHitRatio    float64 `json:"cache_hit_ratio"`
	TableSize        string  `json:"table_size"`
	IOLevel          string  `json:"io_level"`
	IOLabel          string  `json:"io_label"`
	Suggestion       string  `json:"suggestion"`
}

// IndexSizeAnalysis 索引大小分析
type IndexSizeAnalysis struct {
	Schema           string  `json:"schemaname"`
	TableName        string  `json:"table_name"`
	RowCount         int64   `json:"row_count"`
	TableSize        string  `json:"table_size"`
	IndexSize        string  `json:"index_size"`
	TotalSize        string  `json:"total_size"`
	TableSizeBytes   int64   `json:"table_size_bytes"`
	IndexSizeBytes   int64   `json:"index_size_bytes"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	IndexRatio       float64 `json:"index_ratio"`
	IndexCount       int     `json:"index_count"`
	Attention        string  `json:"attention"`
	Suggestion       string  `json:"suggestion"`
}

// InvalidIndex 无效索引
type InvalidIndex struct {
	Schema       string `json:"schemaname"`
	TableName    string `json:"table_name"`
	IndexName    string `json:"index_name"`
	IndexSize    string `json:"index_size"`
	IndexScans   int64  `json:"index_scans"`
	IssueType    string `json:"issue_type"`
	Suggestion   string `json:"suggestion"`
	Database     string `json:"database,omitempty"`
}

// DuplicateIndex 重复索引
type DuplicateIndex struct {
	Schema           string `json:"schemaname"`
	TableName        string `json:"table_name"`
	IndexName        string `json:"index_name"`
	IndexDefinition  string `json:"index_definition"`
	IndexSize        string `json:"index_size"`
	IndexScans       int64  `json:"index_scans"`
	Suggestion       string `json:"suggestion"`
	Database         string `json:"database,omitempty"`
}

// SchemaIssue 设计规范问题
type SchemaIssue struct {
	Table        string `json:"table,omitempty"`
	Column       string `json:"column,omitempty"`
	Index        string `json:"index,omitempty"`
	Constraint   string `json:"constraint,omitempty"`
	Issue        string `json:"issue"`
	Suggestion   string `json:"suggestion"`
}

// HealthScore 健康评分
type HealthScore struct {
	Score          int           `json:"score"`
	Level          string        `json:"level"`
	Label          string        `json:"label"`
	Summary        string        `json:"summary"`
	Issues         []string      `json:"issues"`
	Details        []ScoreDetail `json:"details"`
	ProblemCount   int           `json:"problem_count"`
	CriticalCount  int           `json:"critical_count"`
	WarningCount   int           `json:"warning_count"`
}

// ScoreDetail 评分详情
type ScoreDetail struct {
	Name      string `json:"name"`
	Score     int    `json:"score"`
	MaxScore  int    `json:"max_score"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

// KeyFinding 关键发现
type KeyFinding struct {
	Level       string `json:"level"`
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// InspectionResult 巡检结果（单个数据库）
type InspectionResult map[string]interface{}

// DBInspectionResult 带数据库配置的完整结果
type DBInspectionResult struct {
	DBConfig DBConfig
	Result   InspectionResult
}
