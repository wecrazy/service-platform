package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

var (
	config      YamlConfig
	configMutex sync.RWMutex
	configPath  string
)

// yamlFilePaths defines possible locations for environment-specific config files
// %s will be replaced with "dev" or "prod"
var yamlFilePaths = []string{
	"internal/config/config.%s.yaml",
	"/internal/config/config.%s.yaml",
	"internal/config/config.%s.yaml",
	"../internal/config/config.%s.yaml",
	"/../internal/config/config.%s.yaml",
	"../../internal/config/config.%s.yaml",
	"/../../internal/config/config.%s.yaml",
}

// mainConfigPaths defines possible locations for the main conf.yaml file
var mainConfigPaths = []string{
	"internal/config/conf.yaml",
	"/internal/config/conf.yaml",
	"internal/config/conf.yaml",
	"../internal/config/conf.yaml",
	"/../internal/config/conf.yaml",
	"../../internal/config/conf.yaml",
	"/../../internal/config/conf.yaml",
}

// MainConfig represents the main configuration structure for determining mode
type MainConfig struct {
	ConfigMode string `yaml:"config_mode" validate:"required"`
}

// getEnvironment returns the current environment (dev or prod)
// Priority: 1. config_mode from conf.yaml, 2. ENV environment variable, 3. GO_ENV, 4. default to "dev"
func getEnvironment() string {
	// First try to read from main config file
	if mode := getConfigModeFromFile(); mode != "" {
		return mode
	}

	// Check ENV environment variable
	if env := os.Getenv("ENV"); env != "" {
		if env == "dev" || env == "prod" {
			return env
		}
	}

	// Check GO_ENV environment variable
	if env := os.Getenv("GO_ENV"); env != "" {
		if env == "development" {
			return "dev"
		}
		if env == "production" {
			return "prod"
		}
		if env == "dev" || env == "prod" {
			return env
		}
	}

	// Default to development
	return "dev"
}

// getConfigModeFromFile reads the CONFIG_MODE from the main conf.yaml file
func getConfigModeFromFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	baseDir := cwd

	for _, path := range mainConfigPaths {
		var fullPath string
		if !filepath.IsAbs(path) {
			fullPath = filepath.Join(baseDir, path)
		} else {
			fullPath = path
		}

		if _, err := os.Stat(fullPath); err == nil {
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			var mainConfig MainConfig
			if err := yaml.Unmarshal(data, &mainConfig); err != nil {
				continue
			}

			if mainConfig.ConfigMode == "dev" || mainConfig.ConfigMode == "prod" {
				return mainConfig.ConfigMode
			}
		}
	}

	return ""
}

// getConfigPaths returns the list of config file paths for the current environment
func getConfigPaths() []string {
	env := getEnvironment()
	paths := make([]string, len(yamlFilePaths))

	for i, path := range yamlFilePaths {
		paths[i] = fmt.Sprintf(path, env)
	}

	return paths
}

// LoadConfig loads the configuration from the appropriate YAML file based on the environment
func LoadConfig() error {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current working directory: %v", err)
	}

	baseDir := cwd
	configPaths := getConfigPaths()

	for i, path := range configPaths {
		if !filepath.IsAbs(path) {
			configPaths[i] = filepath.Join(baseDir, path)
		}
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			// log.Printf("Config file found: %v (Environment: %s from %s)", path, getEnvironment(), getConfigSource())
			configPath = path
			break
		}
	}
	if configPath == "" {
		return fmt.Errorf("no valid config file found from paths: %v", configPaths)
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var newConfig YamlConfig
	if err := yaml.Unmarshal(file, &newConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(&newConfig); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	configMutex.Lock()
	config = newConfig
	configMutex.Unlock()

	return nil
}

// WatchConfig sets up a file watcher to monitor changes to the config file
func WatchConfig() {
	if configPath == "" {
		log.Println("no valid config file found. Skipping watcher.")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("failed to initialize config watcher:%v", err)
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		log.Printf("failed to watch config file:%v", err)
	}

	log.Println("👀 Watching for yaml config changes:", configPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Write {
				log.Println("config file updated. Reloading...")
				// Small delay to handle editors that save in multiple writes
				time.Sleep(100 * time.Millisecond)
				if err := LoadConfig(); err != nil {
					log.Printf("⚠️  Failed to reload config (keeping previous config): %v", err)
				} else {
					log.Println("✅ Config reloaded successfully.")
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("config watcher error:", err)
		}
	}
}

// GetConfig returns the current configuration
func GetConfig() YamlConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}

// YamlConfig represents the structure of the configuration YAML file
// Fields are organized into sections such as App, Default, Redis, Database, etc.
type YamlConfig struct {
	App struct {
		Host                 string `yaml:"host" validate:"required"`
		GinMode              string `yaml:"gin_mode" validate:"required"`
		Name                 string `yaml:"name" validate:"required"`
		Description          string `yaml:"description" validate:"required"`
		Logo                 string `yaml:"logo" validate:"required"`
		LogoJPG              string `yaml:"logo_jpg" validate:"required"`
		Port                 int    `yaml:"port" validate:"required"`
		LogLevel             string `yaml:"log_level" validate:"required"`
		LogFormat            string `yaml:"log_format" validate:"required"`
		WebPublicURL         string `yaml:"web_public_url" validate:"required"`
		Version              string `yaml:"version" validate:"required"`
		VersionNo            int    `yaml:"version_no" validate:"required"`
		VersionCode          string `yaml:"version_code" validate:"required"`
		VersionName          string `yaml:"version_name" validate:"required"`
		StaticDir            string `yaml:"static_dir" validate:"required"`
		PublishedDir         string `yaml:"published_dir" validate:"required"`
		LogDir               string `yaml:"log_dir" validate:"required"`
		UploadDir            string `yaml:"upload_dir" validate:"required"`
		LoginTimeM           int    `yaml:"login_time_m" validate:"required"`
		CookieLoginDomain    string `yaml:"cookie_login_domain"`
		CookieLoginSecure    bool   `yaml:"cookie_login_secure"`
		MaxDisconnectionTime int    `yaml:"max_disconnection_time" validate:"required"`
		AesKey               string `yaml:"aes_key" validate:"required"`
		AesKeyIV             string `yaml:"aes_key_iv" validate:"required"`
		MaxRetryLogin        int    `yaml:"max_retry_login" validate:"required"`
		LoginLockUntil       int    `yaml:"login_lock_until" validate:"required"`
		AppLogFilename       string `yaml:"app_log_filename" validate:"required"`
		AppLogMaxSize        int    `yaml:"app_log_maxsize" validate:"required"`
		AppLogMaxAge         int    `yaml:"app_log_maxage" validate:"required"`
		AppLogMaxBackups     int    `yaml:"app_log_maxbackups" validate:"required"`
		AppLogCompress       bool   `yaml:"app_log_compress"`
		SystemLogFilename    string `yaml:"system_log_filename" validate:"required"`
		MemoryProfilePath    string `yaml:"memory_profile_path" validate:"required"`
		Debug                bool   `yaml:"debug"`
	} `yaml:"app" validate:"required"`

	Default struct {
		LogMaxSize                      int    `yaml:"log_max_size" validate:"required"`
		LogMaxAge                       int    `yaml:"log_max_age" validate:"required"`
		LogMaxBackups                   int    `yaml:"log_max_backups" validate:"required"`
		LogCompress                     bool   `yaml:"log_compress"`
		CSVTimestampFormat              string `yaml:"csv_timestamp_format" validate:"required"`
		NSSMFullPath                    string `yaml:"nssm_fullpath" validate:"required"`
		SuperUserEmail                  string `yaml:"super_user_email" validate:"required"`
		SuperUserPassword               string `yaml:"super_user_password" validate:"required"`
		SuperUserPhone                  string `yaml:"super_user_phone" validate:"required"`
		MinLengthPhoneNumber            int    `yaml:"min_length_phone_number" validate:"required"`
		DialingCodeDefault              string `yaml:"dialing_code_default" validate:"required"`
		CPUCaptureInterval              int    `yaml:"cpu_capture_interval" validate:"required"`
		MaxBadWordStrikes               int64  `yaml:"max_bad_word_strikes" validate:"required"`
		DataSeparator                   string `yaml:"data_separator" validate:"required"`
		PurgeOldBackupLogFilesOlderThan string `yaml:"purge_old_backup_log_files_older_than" validate:"required"`
		RemoveOldNeedsDirOlderThan      string `yaml:"remove_old_needs_dir_older_than" validate:"required"`
	} `yaml:"default" validate:"required"`

	Redis struct {
		Host       string `yaml:"host" validate:"required"`
		Port       int    `yaml:"port" validate:"required"`
		Password   string `yaml:"password"`
		Db         int    `yaml:"db" validate:"required"`
		MaxRetry   int    `yaml:"max_retry" validate:"required"`
		RetryDelay int    `yaml:"retry_delay" validate:"required"`
		PoolSize   int    `yaml:"pool_size" validate:"required"`
	} `yaml:"redis" validate:"required"`

	Database struct {
		Type                     string `yaml:"type" validate:"required"`
		Host                     string `yaml:"host" validate:"required"`
		Port                     int    `yaml:"port" validate:"required"`
		Username                 string `yaml:"username" validate:"required"`
		Password                 string `yaml:"password"`
		Name                     string `yaml:"name" validate:"required"`
		MaxRetryConnect          int    `yaml:"max_retry_connect" validate:"required"`
		RetryDelay               int    `yaml:"retry_delay" validate:"required"`
		MaxIdleConnection        int    `yaml:"max_idle_connection" validate:"required"`
		MaxOpenConnection        int    `yaml:"max_open_connection" validate:"required"`
		ConnMaxLifeTime          int    `yaml:"conn_max_lifetime" validate:"required"`
		ConnMaxIdleTime          int    `yaml:"conn_max_idle_time" validate:"required"`
		SSLMode                  string `yaml:"ssl_mode" validate:"required"`
		DBConfigPath             string `yaml:"db_config_path" validate:"required"`
		DBBackupDestinationDir   string `yaml:"db_backup_destination_dir" validate:"required"`
		PurgeOlderThan           string `yaml:"purge_older_than" validate:"required"`
		DumpedIndonesiaRegionSQL string `yaml:"dumped_indonesia_region_sql" validate:"required"`

		// Main tables
		TbUser                     string `yaml:"tb_user" validate:"required"`
		TbUserStatus               string `yaml:"tb_user_status" validate:"required"`
		TbUserPasswordChangeLog    string `yaml:"tb_user_password_change_log" validate:"required"`
		TbRole                     string `yaml:"tb_role" validate:"required"`
		TbRolePrivilege            string `yaml:"tb_role_privilege" validate:"required"`
		TbFeature                  string `yaml:"tb_feature" validate:"required"`
		TbLogActivity              string `yaml:"tb_log_activity" validate:"required"`
		TbLanguage                 string `yaml:"tb_language" validate:"required"`
		TbBadWord                  string `yaml:"tb_bad_word" validate:"required"`
		TbWebAppConfig             string `yaml:"tb_web_app_config" validate:"required"`
		TbIndonesiaRegion          string `yaml:"tb_indonesia_region" validate:"required"`
		TbWhatsappUser             string `yaml:"tb_whatsapp_user" validate:"required"`
		TbWhatsappMessage          string `yaml:"tb_whatsapp_message" validate:"required"`
		TbWhatsappMessageAutoReply string `yaml:"tb_whatsapp_message_auto_reply" validate:"required"`
	} `yaml:"database" validate:"required"`

	FolderFileNeeds []string `yaml:"folder_file_needs" validate:"required"`

	Schedules struct {
		Host     string      `yaml:"host" validate:"required"`
		Port     int         `yaml:"port" validate:"required"`
		Timezone string      `yaml:"timezone" validate:"required"`
		List     []Scheduler `yaml:"list" validate:"required"`
	} `yaml:"schedules" validate:"required"`

	API struct {
		AnalyticsDevAPIKey string `yaml:"analytics_dev_api_key" validate:"required"`
	} `yaml:"api" validate:"required"`

	Email struct {
		Host              string `yaml:"host" validate:"required"`
		Port              int    `yaml:"port" validate:"required"`
		Username          string `yaml:"username" validate:"required"`
		Password          string `yaml:"password" validate:"required"`
		Sender            string `yaml:"sender" validate:"required"`
		MaxRetry          int    `yaml:"max_retry" validate:"required"`
		RetryDelay        int    `yaml:"retry_delay" validate:"required"`
		MaxAttachmentSize int64  `yaml:"max_attachment_size" validate:"required"`
	} `yaml:"email" validate:"required"`

	GRPC struct {
		Host string `yaml:"host" validate:"required"`
		Port int    `yaml:"port" validate:"required"`
	} `yaml:"grpc" validate:"required"`

	Whatsnyan struct {
		DBLog                          string          `yaml:"db_log" validate:"required"`
		DBLogLevel                     string          `yaml:"db_log_level" validate:"required"`
		ClientLog                      string          `yaml:"client_log" validate:"required"`
		ClientLogLevel                 string          `yaml:"client_log_level" validate:"required"`
		LogMaxSize                     int             `yaml:"log_max_size" validate:"required"`
		LogMaxAge                      int             `yaml:"log_max_age" validate:"required"`
		LogMaxBackups                  int             `yaml:"log_max_backups" validate:"required"`
		LogCompress                    bool            `yaml:"log_compress"`
		GRPCHost                       string          `yaml:"grpc_host" validate:"required"`
		GRPCPort                       int             `yaml:"grpc_port" validate:"required"`
		WATechnicalSupport             string          `yaml:"wa_technical_support" validate:"required"`
		MaxMessageLength               int             `yaml:"max_message_length" validate:"required"`
		Tables                         WhatsnyanTables `yaml:"tables" validate:"required"`
		WAReplyPublicURL               string          `yaml:"wa_reply_public_url" validate:"required"`
		NeedVerifyAccount              bool            `yaml:"need_verify_account"`
		LanguageExpiry                 int             `yaml:"language_expiry" validate:"required"`
		LanguagePromptShownExpiry      int             `yaml:"language_prompt_shown_expiry" validate:"required"`
		NotRegisteredPhoneExpiry       int             `yaml:"not_registered_phone_expiry" validate:"required"`
		QuotaLimitExpiry               int             `yaml:"quota_limit_expiry" validate:"required"`
		LanguagePrompt                 string          `yaml:"language_prompt" validate:"required"`
		WAGAllowedToInteract           []string        `yaml:"wag_allowed_to_interact" validate:"required"`
		Files                          WhatsnyanFiles  `yaml:"files" validate:"required"`
		MessageProcessedToleranceHours int             `yaml:"message_processed_tolerance_hours" validate:"required"`
		PurgeMessageOlderThan          string          `yaml:"purge_message_older_than" validate:"required"`
		EnablePhonePairing             bool            `yaml:"enable_phone_pairing"`
		PairingPhoneNumber             string          `yaml:"pairing_phone_number"`
	} `yaml:"whatsnyan" validate:"required"`

	Metrics struct {
		APIPort       int `yaml:"api_port" validate:"required"`
		GRPCPort      int `yaml:"grpc_port" validate:"required"`
		SchedulerPort int `yaml:"scheduler_port" validate:"required"`
		WhatsAppPort  int `yaml:"whatsapp_port" validate:"required"`
		TelegramPort  int `yaml:"telegram_port" validate:"required"`
		TwilioPort    int `yaml:"twilio_port" validate:"required"`
		GrafanaPort   int `yaml:"grafana_port" validate:"required"`
	} `yaml:"metrics" validate:"required"`

	RateLimit struct {
		Enabled     bool `yaml:"enabled"`
		Requests    int  `yaml:"requests" validate:"required"`     // requests per period
		Period      int  `yaml:"period" validate:"required"`       // period in seconds
		Burst       int  `yaml:"burst" validate:"required"`        // burst allowance
		CleanupTime int  `yaml:"cleanup_time" validate:"required"` // cleanup time in seconds
	} `yaml:"rate_limit" validate:"required"`

	Monitoring struct {
		ServiceName string `yaml:"service_name" validate:"required"`
		Description string `yaml:"description" validate:"required"`
	} `yaml:"monitoring" validate:"required"`

	N8N struct {
		Host              string `yaml:"host" validate:"required"`
		Port              int    `yaml:"port" validate:"required"`
		BridgeHost        string `yaml:"bridge_host" validate:"required"`
		BridgePort        int    `yaml:"bridge_port" validate:"required"`
		BridgeServiceName string `yaml:"bridge_service_name" validate:"required"`
		ApiKey            string `yaml:"api_key"`
	} `yaml:"n8n" validate:"required"`

	LibreTranslate struct {
		Port int `yaml:"port" validate:"required"`
	} `yaml:"libretranslate" validate:"required"`

	K6 struct {
		Enabled        bool         `yaml:"enabled"`
		Port           int          `yaml:"port" validate:"required"`
		PrometheusPort int          `yaml:"prometheus_port" validate:"required"`
		ScriptsDir     string       `yaml:"scripts_dir" validate:"required"`
		Thresholds     K6Thresholds `yaml:"thresholds" validate:"required"`
		Scenarios      []K6Scenario `yaml:"scenarios" validate:"required"`
	} `yaml:"k6" validate:"required"`

	Observability struct {
		Loki struct {
			Enabled        bool              `yaml:"enabled"`
			URL            string            `yaml:"url" validate:"required"`
			BatchSize      int               `yaml:"batch_size" validate:"required"`
			BatchTimeoutMs int               `yaml:"batch_timeout_ms" validate:"required"`
			Labels         map[string]string `yaml:"labels" validate:"required"`
		} `yaml:"loki" validate:"required"`

		Tempo struct {
			Enabled            bool    `yaml:"enabled"`
			OTLPGRPCEndpoint   string  `yaml:"otlp_grpc_endpoint" validate:"required"`
			OTLPHTTPEndpoint   string  `yaml:"otlp_http_endpoint" validate:"required"`
			SampleRate         float64 `yaml:"sample_rate" validate:"required"`
			MaxExportBatchSize int     `yaml:"max_export_batch_size" validate:"required"`
			ExportTimeoutMs    int     `yaml:"export_timeout_ms" validate:"required"`
		} `yaml:"tempo" validate:"required"`

		Jaeger struct {
			Enabled  bool   `yaml:"enabled"`
			Endpoint string `yaml:"endpoint" validate:"required"`
		} `yaml:"jaeger" validate:"required"`
	} `yaml:"observability" validate:"required"`

	MongoDB struct {
		Host        string `yaml:"host" validate:"required"`
		Port        int    `yaml:"port" validate:"required"`
		Username    string `yaml:"username" validate:"required"`
		Password    string `yaml:"password" validate:"required"`
		Database    string `yaml:"database" validate:"required"`
		AuthSource  string `yaml:"auth_source" validate:"required"`
		MaxPoolSize uint64 `yaml:"max_pool_size" validate:"required"`
		MinPoolSize uint64 `yaml:"min_pool_size" validate:"required"`
		MaxIdleTime int    `yaml:"max_idle_time" validate:"required"`
	} `yaml:"mongodb" validate:"required"`

	MongoExpress struct {
		Host     string `yaml:"host" validate:"required"`
		Port     int    `yaml:"port" validate:"required"`
		Username string `yaml:"username" validate:"required"`
		Password string `yaml:"password" validate:"required"`
	} `yaml:"mongoexpress" validate:"required"`

	Telegram struct {
		Debug                 bool           `yaml:"debug"`
		APIURL                string         `yaml:"api_url" validate:"required"`
		BotToken              string         `yaml:"bot_token" validate:"required"`
		Host                  string         `yaml:"host" validate:"required"`
		GRPCPort              int            `yaml:"grpc_port" validate:"required"`
		Tables                TelegramTables `yaml:"tables" validate:"required"`
		TechnicalSupportPhone string         `yaml:"technical_support_phone" validate:"required"`
	} `yaml:"telegram" validate:"required"`

	Twilio struct {
		AccountSID     string `yaml:"account_sid" validate:"required"`
		AuthToken      string `yaml:"auth_token" validate:"required"`
		WhatsAppNumber string `yaml:"whatsapp_number" validate:"required"`
		Host           string `yaml:"host" validate:"required"`
		GRPCPort       int    `yaml:"grpc_port" validate:"required"`
		IsDev          bool   `yaml:"is_dev"`
	} `yaml:"twilio" validate:"required"`

	// ODOOManageService holds configuration for ODOO Manage Service integration
	ODOOManageService struct {
		JsonRPCVersion string                  `yaml:"jsonrpc_version" validate:"required"`
		Login          string                  `yaml:"login" validate:"required"`
		Password       string                  `yaml:"password" validate:"required"`
		DB             string                  `yaml:"db" validate:"required"`
		URL            string                  `yaml:"url" validate:"required"`
		PathSession    string                  `yaml:"path_session" validate:"required"`
		PathGetData    string                  `yaml:"path_getdata" validate:"required"`
		PathUpdateData string                  `yaml:"path_updatedata" validate:"required"`
		PathCreateData string                  `yaml:"path_createdata" validate:"required"`
		MaxRetry       int                     `yaml:"max_retry" validate:"required"`
		RetryDelay     int                     `yaml:"retry_delay" validate:"required"`
		SessionTimeout int                     `yaml:"session_timeout" validate:"required"`
		DataTimeout    int                     `yaml:"data_timeout"`
		SkipSSLVerify  bool                    `yaml:"skip_ssl_verify"`
		SACData        map[string]ODOOMSACData `yaml:"sac" validate:"required"`
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

// K6Thresholds represents default thresholds for k6 tests
type K6Thresholds struct {
	HTTPReqDuration   string `yaml:"http_req_duration" validate:"required"`    // p(95)<500ms
	HTTPReqFailed     string `yaml:"http_req_failed" validate:"required"`      // rate<0.01
	HTTPReqsPerSecond int    `yaml:"http_reqs_per_second" validate:"required"` // min value
	IterationDuration string `yaml:"iteration_duration" validate:"required"`   // p(95)<2s
	ChecksPassRate    string `yaml:"checks_pass_rate" validate:"required"`     // rate>0.95
}

// K6Scenario represents a load test scenario configuration
type K6Scenario struct {
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description,omitempty" validate:"required"`
	Executor    string `yaml:"executor" validate:"required"`    // constant-vus, ramping-vus, etc.
	VUs         int    `yaml:"vus" validate:"required"`         // number of virtual users
	Duration    string `yaml:"duration" validate:"required"`    // test duration
	ScriptPath  string `yaml:"script_path" validate:"required"` // path to k6 test script
}

// Scheduler represents a scheduled task configuration
// tasks can be defined using various scheduling options
// such as every, at, weekly, monthly, yearly
type Scheduler struct {
	Name        string   `yaml:"name" validate:"required"`
	Description string   `yaml:"description,omitempty" validate:"required"`
	Every       string   `yaml:"every,omitempty"`
	At          []string `yaml:"at,omitempty"`
	Weekly      string   `yaml:"weekly,omitempty"`
	Monthly     string   `yaml:"monthly,omitempty"`
	Yearly      string   `yaml:"yearly,omitempty"`
}

// WhatsnyanTables holds the table names used in the Whatsmeow pkg
type WhatsnyanTables struct {
	TBWhatsnyanMessage          string `yaml:"tb_whatsnyan_message" validate:"required"`
	TBWhatsnyanIncomingMessage  string `yaml:"tb_whatsnyan_incoming_message" validate:"required"`
	TBWhatsnyanGroup            string `yaml:"tb_whatsnyan_group" validate:"required"`
	TBWhatsnyanGroupParticipant string `yaml:"tb_whatsnyan_group_participant" validate:"required"`
	// Whatsmeow tables
	TBAppStateMutationMacs string `yaml:"tb_app_state_mutation_macs" validate:"required"`
	TBAppStateSyncKeys     string `yaml:"tb_app_state_sync_keys" validate:"required"`
	TBAppStateVersions     string `yaml:"tb_app_state_versions" validate:"required"`
	TBChatSettings         string `yaml:"tb_chat_settings" validate:"required"`
	TBContacts             string `yaml:"tb_contacts" validate:"required"`
	TBDevice               string `yaml:"tb_device" validate:"required"`
	TBEventBuffer          string `yaml:"tb_event_buffer" validate:"required"`
	TBIdentityKeys         string `yaml:"tb_identity_keys" validate:"required"`
	TBLIDMap               string `yaml:"tb_lid_map" validate:"required"`
	TBMessageSecrets       string `yaml:"tb_message_secrets" validate:"required"`
	TBPreKeys              string `yaml:"tb_pre_keys" validate:"required"`
	TBPrivacyTokens        string `yaml:"tb_privacy_tokens" validate:"required"`
	TBSenderKeys           string `yaml:"tb_sender_keys" validate:"required"`
	TBSessions             string `yaml:"tb_sessions" validate:"required"`
	TBVersion              string `yaml:"tb_version" validate:"required"`
}

// WhatsnyanFiles holds configuration for file handling in Whatsmeow
type WhatsnyanFiles struct {
	Image struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota" validate:"required"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds" validate:"required"`
		MaxSize           int64    `yaml:"max_size" validate:"required"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types" validate:"required"`
		AllowedExtensions []string `yaml:"allowed_extensions" validate:"required"`
	} `yaml:"image" validate:"required"`
	Video struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota" validate:"required"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds" validate:"required"`
		MaxSize           int64    `yaml:"max_size" validate:"required"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types" validate:"required"`
		AllowedExtensions []string `yaml:"allowed_extensions" validate:"required"`
	} `yaml:"video" validate:"required"`
	Document struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota" validate:"required"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds" validate:"required"`
		MaxSize           int64    `yaml:"max_size" validate:"required"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types" validate:"required"`
		AllowedExtensions []string `yaml:"allowed_extensions" validate:"required"`
	} `yaml:"document" validate:"required"`
	Audio struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota" validate:"required"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds" validate:"required"`
		MaxSize           int64    `yaml:"max_size" validate:"required"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types" validate:"required"`
		AllowedExtensions []string `yaml:"allowed_extensions" validate:"required"`
	} `yaml:"audio" validate:"required"`
}

// TelegramTables holds the table names used in the Telegram service
type TelegramTables struct {
	TBTelegramMessage         string `yaml:"tb_telegram_message" validate:"required"`
	TBTelegramIncomingMessage string `yaml:"tb_telegram_incoming_message" validate:"required"`
	TBTelegramUser            string `yaml:"tb_telegram_user" validate:"required"`
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
