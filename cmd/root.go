package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"db-patrol/internal/config"
	"db-patrol/internal/core"
	"db-patrol/internal/models"
	"db-patrol/internal/reporter"
)

var (
	cfgFile    string
	database   string
	format     string
	dbHost     string
	dbPort     int
	dbUser     string
	dbPassword string
	dbName     string
	dbType     string
	dbDatabase string
	dbSchema   string
	dbJSON     string
)

var validDBTypes = []string{"vastbase_pg", "mysql", "postgresql", "vastbase_mysql"}

var rootCmd = &cobra.Command{
	Use:   "db-patrol",
	Short: "数据库巡检工具",
	Long: `数据库巡检工具
支持 Vastbase PG/MySQL 模式、MySQL、PostgreSQL

示例:
  # 使用配置文件（不推荐，数据库配置应通过参数传递）
  db-patrol -c config.yaml

  # 通过参数传递单个数据库配置
  db-patrol --db-host 192.168.1.1 --db-port 5432 --db-user admin --db-password pass \
            --db-name "测试库" --db-type vastbase_pg --db-database segh_yy

  # 通过环境变量传递密码（推荐，避免密码出现在命令行）
  export DB_PASSWORD=your_password
  db-patrol --db-host 192.168.1.1 --db-user admin --db-name "测试库" \
            --db-type vastbase_pg --db-database segh_yy

  # 通过JSON传递多个数据库配置（密码字段支持 $ENV_VAR 引用环境变量）
  db-patrol --db-json '[{"name":"DB1","type":"mysql","host":"192.168.1.1","password":"$DB_PWD",...}]'`,
	RunE: runRoot,
}

func init() {
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", config.GetDefaultConfigPath(), "配置文件路径（用于巡检和报告配置）")
	rootCmd.Flags().StringVarP(&database, "database", "d", "", "指定要巡检的数据库名称")
	rootCmd.Flags().StringVarP(&format, "format", "f", "", "报告输出格式 (html/markdown/json)")
	rootCmd.Flags().StringVar(&dbHost, "db-host", "", "数据库主机地址")
	rootCmd.Flags().IntVar(&dbPort, "db-port", 0, "数据库端口")
	rootCmd.Flags().StringVar(&dbUser, "db-user", "", "数据库用户名")
	rootCmd.Flags().StringVar(&dbPassword, "db-password", os.Getenv("DB_PASSWORD"), "数据库密码（也可通过 DB_PASSWORD 环境变量传递）")
	rootCmd.Flags().StringVar(&dbName, "db-name", "", "数据库标识名称")
	rootCmd.Flags().StringVar(&dbType, "db-type", "", "数据库类型 (vastbase_pg/mysql/postgresql/vastbase_mysql)")
	rootCmd.Flags().StringVar(&dbDatabase, "db-database", "", "要连接的数据库名")
	rootCmd.Flags().StringVar(&dbSchema, "db-schema", "public", "数据库schema")
	rootCmd.Flags().StringVar(&dbJSON, "db-json", "", "数据库配置JSON字符串，支持多个数据库")
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	var databasesConfig []models.DBConfig

	// 检查 --db-json 和单独参数是否冲突
	if dbJSON != "" && (dbHost != "" || dbUser != "" || dbPassword != "" || dbDatabase != "") {
		return fmt.Errorf("不能同时使用 --db-json 参数和单独的数据库连接参数\n请选择要么使用 --db-json 传递配置，要么使用单独的 --db-host 等参数")
	}

	// 解析数据库配置
	if dbJSON != "" {
		var err error
		databasesConfig, err = parseDBJSON(dbJSON)
		if err != nil {
			return fmt.Errorf("数据库JSON格式无效: %w", err)
		}

		// 解析环境变量密码引用
		warnings := resolveEnvPasswords(databasesConfig)
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "警告: %s\n", w)
		}

		// 校验数据库配置
		errors := validateDBConfigs(databasesConfig)
		if len(errors) > 0 {
			for _, e := range errors {
				fmt.Fprintf(os.Stderr, "错误: %s\n", e)
			}
			fmt.Fprintf(os.Stderr, "需要的字段: type, host, user, password, database\n")
			os.Exit(1)
		}
	} else if dbHost != "" || dbUser != "" || dbPassword != "" || dbDatabase != "" {
		// 校验必要参数
		missingParams := []string{}
		if dbHost == "" {
			missingParams = append(missingParams, "db-host")
		}
		if dbUser == "" {
			missingParams = append(missingParams, "db-user")
		}
		if dbPassword == "" {
			missingParams = append(missingParams, "db-password")
		}
		if dbDatabase == "" {
			missingParams = append(missingParams, "db-database")
		}
		if dbType == "" {
			missingParams = append(missingParams, "db-type")
		}

		if len(missingParams) > 0 {
			return fmt.Errorf("缺少必要的数据库参数: %s\n请提供: --db-host, --db-user, --db-password, --db-database, --db-type",
				strings.Join(missingParams, ", "))
		}

		if dbPort != 0 && (dbPort < 1 || dbPort > 65535) {
			return fmt.Errorf("端口 %d 无效，必须是1-65535之间的整数", dbPort)
		}

		dbConfig := buildSingleDBConfig(dbHost, dbUser, dbPassword, dbDatabase, dbType, dbName, dbPort, dbSchema)
		databasesConfig = []models.DBConfig{dbConfig}
	}

	// 加载应用配置
	appConfig, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置文件失败: %w", err)
	}

	// 如果通过命令行指定了数据库配置，覆盖配置文件中的
	if len(databasesConfig) > 0 {
		appConfig.Databases = databasesConfig
	}

	// 校验是否有可巡检的数据库
	if len(appConfig.Databases) == 0 {
		return fmt.Errorf("未找到任何可巡检的数据库配置\n请通过以下方式之一提供数据库配置:\n" +
			"1. 使用 --db-json 参数传递JSON格式的数据库配置\n" +
			"2. 使用 --db-host/--db-user 等单独参数传递数据库配置\n" +
			"3. 在配置文件中配置 databases 字段")
	}

	// 如果指定了报告格式，覆盖配置
	if format != "" {
		appConfig.Report.Format = format
	}

	// 创建巡检器
	inspector := core.NewDBInspector(appConfig)

	// 如果只巡检指定数据库
	if database != "" {
		dbConfig := findDBConfig(appConfig.Databases, database)
		if dbConfig == nil {
			return fmt.Errorf("未找到数据库配置: %s", database)
		}

		result, err := inspector.InspectDatabase(*dbConfig)
		if err != nil {
			return fmt.Errorf("巡检失败: %w", err)
		}

		// 生成报告
		r, err := reporter.CreateReporter(appConfig.Report.Format, appConfig.Report.OutputDir)
		if err != nil {
			return fmt.Errorf("创建报告生成器失败: %w", err)
		}
		filepath, err := r.Generate(*dbConfig, result)
		if err != nil {
			return fmt.Errorf("生成报告失败: %w", err)
		}
		fmt.Printf("报告已生成: %s\n", filepath)
	} else {
		// 巡检所有数据库
		inspector.InspectAll(appConfig.Databases)
		inspector.PrintSummary()
	}

	return nil
}

// findDBConfig 在数据库列表中查找指定名称的配置
func findDBConfig(databases []models.DBConfig, name string) *models.DBConfig {
	for i := range databases {
		if databases[i].Name == name {
			return &databases[i]
		}
	}
	return nil
}

// parseDBJSON 解析数据库配置JSON字符串
func parseDBJSON(dbJSON string) ([]models.DBConfig, error) {
	var configs []models.DBConfig
	if err := json.Unmarshal([]byte(dbJSON), &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// resolveEnvPasswords 解析密码字段中的环境变量引用（以 $ 开头）
func resolveEnvPasswords(configs []models.DBConfig) []string {
	var warnings []string
	for i := range configs {
		if configs[i].Password != "" && strings.HasPrefix(configs[i].Password, "$") {
			envVarName := configs[i].Password[1:]
			envValue := os.Getenv(envVarName)
			if envValue != "" {
				configs[i].Password = envValue
			} else {
				warnings = append(warnings, fmt.Sprintf("第%d个数据库的密码引用环境变量 $%s 未设置", i+1, envVarName))
			}
		}
	}
	return warnings
}

// validateDBConfigs 校验数据库配置列表
func validateDBConfigs(configs []models.DBConfig) []string {
	var errors []string

	for i, cfg := range configs {
		missing := []string{}
		if cfg.Type == "" {
			missing = append(missing, "type")
		}
		if cfg.Host == "" {
			missing = append(missing, "host")
		}
		if cfg.User == "" {
			missing = append(missing, "user")
		}
		if cfg.Password == "" {
			missing = append(missing, "password")
		}
		if cfg.Database == "" {
			missing = append(missing, "database")
		}
		if len(missing) > 0 {
			errors = append(errors, fmt.Sprintf("第%d个数据库配置缺少必要字段: %s", i+1, strings.Join(missing, ", ")))
		}

		// 校验数据库类型
		valid := false
		for _, vt := range validDBTypes {
			if cfg.Type == vt {
				valid = true
				break
			}
		}
		if cfg.Type != "" && !valid {
			errors = append(errors, fmt.Sprintf("第%d个数据库的类型 %s 不支持，支持: %s", i+1, cfg.Type, strings.Join(validDBTypes, ", ")))
		}

		// 校验端口
		if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
			errors = append(errors, fmt.Sprintf("第%d个数据库的端口 %d 无效，必须是1-65535之间的整数", i+1, cfg.Port))
		}
	}

	return errors
}

// buildSingleDBConfig 从命令行参数构建单个数据库配置
func buildSingleDBConfig(host, user, password, database, dbType, name string, port int, schema string) models.DBConfig {
	if name == "" {
		name = database
	}
	if port == 0 {
		if dbType == "mysql" || dbType == "vastbase_mysql" {
			port = 3306
		} else {
			port = 5432
		}
	}
	return models.DBConfig{
		Name:     name,
		Type:     dbType,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		Schema:   schema,
	}
}
