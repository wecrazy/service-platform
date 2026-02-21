package config

// ServicePlatform holds the configuration for the service platform. It is initialized as a pointer to an instance of TypeServicePlatform, which can be used throughout the application to access service platform-specific configuration settings. The actual fields and methods of TypeServicePlatform can be defined as needed to encapsulate all relevant configuration options for the service platform.
var ServicePlatform = &configs[TypeServicePlatform]{}

// TypeServicePlatform represents the structure of the configuration YAML file
// Fields are organized into sections such as App, Default, Redis, Database, etc.
type TypeServicePlatform struct {
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
		PrometheusPort    int `yaml:"prometheus_port" validate:"required"`
		APIPort           int `yaml:"api_port" validate:"required"`
		GRPCPort          int `yaml:"grpc_port" validate:"required"`
		SchedulerPort     int `yaml:"scheduler_port" validate:"required"`
		WhatsAppPort      int `yaml:"whatsapp_port" validate:"required"`
		TelegramPort      int `yaml:"telegram_port" validate:"required"`
		TwilioPort        int `yaml:"twilio_port" validate:"required"`
		GrafanaPort       int `yaml:"grafana_port" validate:"required"`
		NginxAuthPort     int `yaml:"nginx_auth_port" validate:"required"`
		NginxExporterPort int `yaml:"nginx_exporter_port" validate:"required"`
		GoDocPort         int `yaml:"godoc_port" validate:"required"`
	} `yaml:"metrics" validate:"required"`

	RateLimit       RateLimitConfig `yaml:"rate_limit" validate:"required"`
	TwilioRateLimit RateLimitConfig `yaml:"twilio_rate_limit" validate:"required"`

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
}

// RateLimitConfig represents the configuration for rate limiting, including whether it is enabled, the number of requests allowed, the period for which the limit applies, the burst size, and the cleanup time for expired keys. This struct can be used to configure rate limiting behavior in the application, such as for API endpoints or specific services like Twilio.
// - Enabled: A boolean indicating whether rate limiting is enabled.
// - Requests: The number of requests allowed within the specified period.
// - Period: The time period (in seconds) for which the request limit applies.
// - Burst: The maximum burst size, allowing for short-term spikes in traffic.
// - CleanupTime: The time (in seconds) after which expired keys should be cleaned up from the rate limiter.
type RateLimitConfig struct {
	Enabled     bool `yaml:"enabled"`
	Requests    int  `yaml:"requests" validate:"required"`
	Period      int  `yaml:"period" validate:"required"`
	Burst       int  `yaml:"burst" validate:"required"`
	CleanupTime int  `yaml:"cleanup_time" validate:"required"`
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
