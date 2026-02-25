package config

// ManageService holds the configuration of ODOO MS. It is initialized as a pointer to an empty struct of TypeManageService, which will be populated with the actual configuration values when the configuration files are loaded. This allows other parts of the application to access the ODOO MS configuration through this variable.
var ManageService = &configs[TypeManageService]{}

// TypeManageService holds the configuration structure for the ODOO Manage Service integration.
type TypeManageService struct {
	// ODOOMS holds configuration for ODOO Manage Service integration
	ODOOMS struct {
		JSONRPCVersion  string                  `yaml:"jsonrpc_version" validate:"required"`
		Login           string                  `yaml:"login" validate:"required"`
		Password        string                  `yaml:"password" validate:"required"`
		DB              string                  `yaml:"db" validate:"required"`
		URL             string                  `yaml:"url" validate:"required"`
		PathSession     string                  `yaml:"path_session" validate:"required"`
		PathGetData     string                  `yaml:"path_getdata" validate:"required"`
		PathUpdateData  string                  `yaml:"path_updatedata" validate:"required"`
		PathCreateData  string                  `yaml:"path_createdata" validate:"required"`
		MaxRetry        int                     `yaml:"max_retry" validate:"required"`
		RetryDelay      int                     `yaml:"retry_delay" validate:"required"`
		SessionTimeout  int                     `yaml:"session_timeout" validate:"required"`
		DataTimeout     int                     `yaml:"data_timeout"`
		SkipSSLVerify   bool                    `yaml:"skip_ssl_verify"`
		CompanyIncluded []string                `yaml:"company_included" validate:"required"`
		CompanyExcluded []string                `yaml:"company_excluded" validate:"required"`
		SACData         map[string]ODOOMSACData `yaml:"sac" validate:"required"`
	} `yaml:"odoo_ms" validate:"required"`

	// TechnicalAssistance holds configuration for technical assistance services and configurations
	TechnicalAssistance struct {
		APIHost                string                `yaml:"api_host" validate:"required"`
		APIPort                int                   `yaml:"api_port" validate:"required"`
		MySQLDBHost            string                `yaml:"mysqldb_host" validate:"required"`
		MySQLDBPort            int                   `yaml:"mysqldb_port" validate:"required"`
		MySQLDBUser            string                `yaml:"mysqldb_user" validate:"required"`
		MySQLDBPass            string                `yaml:"mysqldb_password" validate:"required"`
		MySQLDBName            string                `yaml:"mysqldb_dbname" validate:"required"`
		MySQLDBMaxRetryConnect int                   `yaml:"mysqldb_max_retry_connect" validate:"required"`
		MySQLDBRetryDelay      int                   `yaml:"mysqldb_retry_delay" validate:"required"`
		MySQLDBIdleConnection  int                   `yaml:"mysqldb_idle_connection" validate:"required"`
		MySQLDBOpenConnection  int                   `yaml:"mysqldb_open_connection" validate:"required"`
		MySQLDBConnMaxLifetime int                   `yaml:"mysqldb_conn_max_lifetime" validate:"required"`
		MySQLDBConnMaxIdleTime int                   `yaml:"mysqldb_conn_max_idle_time" validate:"required"`
		MySQLDBSSLMode         string                `yaml:"mysqldb_ssl_mode" validate:"required"`
		RedisHost              string                `yaml:"redisdb_host" validate:"required"`
		RedisPort              int                   `yaml:"redisdb_port" validate:"required"`
		RedisDBUsed            int                   `yaml:"redisdb_dbused" validate:"required"`
		UserTA                 map[string]TAUserData `yaml:"user_ta" validate:"required"`
		Tables                 TATables              `yaml:"tables" validate:"required"`
	} `yaml:"technical_assistance" validate:"required"`

	// MSMiddleware holds configuration for the middleware microservice
	MSMiddleware struct {
		MySQLDBHost            string `yaml:"mysqldb_host" validate:"required"`
		MySQLDBPort            int    `yaml:"mysqldb_port" validate:"required"`
		MySQLDBUser            string `yaml:"mysqldb_user" validate:"required"`
		MySQLDBPass            string `yaml:"mysqldb_password" validate:"required"`
		MySQLDBName            string `yaml:"mysqldb_dbname" validate:"required"`
		MySQLDBMaxRetryConnect int    `yaml:"mysqldb_max_retry_connect" validate:"required"`
		MySQLDBRetryDelay      int    `yaml:"mysqldb_retry_delay" validate:"required"`
		MySQLDBIdleConnection  int    `yaml:"mysqldb_idle_connection" validate:"required"`
		MySQLDBOpenConnection  int    `yaml:"mysqldb_open_connection" validate:"required"`
		MySQLDBConnMaxLifetime int    `yaml:"mysqldb_conn_max_lifetime" validate:"required"`
		MySQLDBConnMaxIdleTime int    `yaml:"mysqldb_conn_max_idle_time" validate:"required"`
		MySQLDBSSLMode         string `yaml:"mysqldb_ssl_mode" validate:"required"`
	} `yaml:"ms_middleware" validate:"required"`

	// WebPanelService holds Dashboard & Reporting of ODOO Manage Service
	WebPanelService struct {
		MySQLDBHost            string `yaml:"mysqldb_host" validate:"required"`
		MySQLDBPort            int    `yaml:"mysqldb_port" validate:"required"`
		MySQLDBUser            string `yaml:"mysqldb_user" validate:"required"`
		MySQLDBPass            string `yaml:"mysqldb_password" validate:"required"`
		MySQLDBName            string `yaml:"mysqldb_dbname" validate:"required"`
		MySQLDBMaxRetryConnect int    `yaml:"mysqldb_max_retry_connect" validate:"required"`
		MySQLDBRetryDelay      int    `yaml:"mysqldb_retry_delay" validate:"required"`
		MySQLDBIdleConnection  int    `yaml:"mysqldb_idle_connection" validate:"required"`
		MySQLDBOpenConnection  int    `yaml:"mysqldb_open_connection" validate:"required"`
		MySQLDBConnMaxLifetime int    `yaml:"mysqldb_conn_max_lifetime" validate:"required"`
		MySQLDBConnMaxIdleTime int    `yaml:"mysqldb_conn_max_idle_time" validate:"required"`
		MySQLDBSSLMode         string `yaml:"mysqldb_ssl_mode" validate:"required"`
	} `yaml:"web_panel" validate:"required"`
}

// TAUserData represents a user data structure for Technical Assistance, e.g. email -> name & phone
type TAUserData struct {
	Name  string `yaml:"name" validate:"required"`
	Phone string `yaml:"phone" validate:"required"`
}

// TATables represents the table names used in the Technical Assistance service
type TATables struct {
	TBLogActivity string `yaml:"tb_logact" validate:"required"`
	TBPending     string `yaml:"tb_pending" validate:"required"`
	TBError       string `yaml:"tb_error" validate:"required"`
}

// ODOOMSACData represents SAC (Service Area Coordinator) user data for ODOO Manage Service integration
type ODOOMSACData struct {
	Username string `yaml:"username" validate:"required"`
	Fullname string `yaml:"fullname" validate:"required"`
	Phone    string `yaml:"phone" validate:"required"`
	Email    string `yaml:"email" validate:"required"`
	TTDPath  string `yaml:"ttd_path" validate:"required"`
	Region   int    `yaml:"region" validate:"required"`
}
