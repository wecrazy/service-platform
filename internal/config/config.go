package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

var (
	config      YamlConfig
	configMutex sync.RWMutex
	configPath  string
)

var yamlFilePaths = []string{
	"internal/config/config.%s.yaml",
	"/internal/config/config.%s.yaml",
	"internal/config/config.%s.yaml",
	"../internal/config/config.%s.yaml",
	"/../internal/config/config.%s.yaml",
	"../../internal/config/config.%s.yaml",
	"/../../internal/config/config.%s.yaml",
}

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
	ConfigMode string `yaml:"config_mode"`
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

	configMutex.Lock()
	config = newConfig
	configMutex.Unlock()

	return nil
}

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
				if err := LoadConfig(); err != nil {
					log.Printf("failed to reload config:%v", err)
				} else {
					log.Println("config reloaded successfully.")
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

func GetConfig() YamlConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}

type YamlConfig struct {
	App struct {
		Host                 string `yaml:"host"`
		GinMode              string `yaml:"gin_mode"`
		Name                 string `yaml:"name"`
		Description          string `yaml:"description"`
		Logo                 string `yaml:"logo"`
		LogoJPG              string `yaml:"logo_jpg"`
		Port                 int    `yaml:"port"`
		LogLevel             string `yaml:"log_level"`
		LogFormat            string `yaml:"log_format"`
		WebPublicURL         string `yaml:"web_public_url"`
		Version              string `yaml:"version"`
		VersionNo            int    `yaml:"version_no"`
		VersionCode          string `yaml:"version_code"`
		VersionName          string `yaml:"version_name"`
		StaticDir            string `yaml:"static_dir"`
		PublishedDir         string `yaml:"published_dir"`
		LogDir               string `yaml:"log_dir"`
		UploadDir            string `yaml:"upload_dir"`
		LoginTimeM           int    `yaml:"login_time_m"`
		CookieLoginDomain    string `yaml:"cookie_login_domain"`
		CookieLoginSecure    bool   `yaml:"cookie_login_secure"`
		MaxDisconnectionTime int    `yaml:"max_disconnection_time"`
		AesKey               string `yaml:"aes_key"`
		AesKeyIV             string `yaml:"aes_key_iv"`
		MaxRetryLogin        int    `yaml:"max_retry_login"`
		LoginLockUntil       int    `yaml:"login_lock_until"`
		AppLogFilename       string `yaml:"app_log_filename"`
		AppLogMaxSize        int    `yaml:"app_log_max_size"`
		AppLogMaxAge         int    `yaml:"app_log_max_age"`
		AppLogMaxBackups     int    `yaml:"app_log_max_backups"`
		AppLogCompress       bool   `yaml:"app_log_compress"`
		SystemLogFilename    string `yaml:"system_log_filename"`
		MemoryProfilePath    string `yaml:"memory_profile_path"`
		Debug                bool   `yaml:"debug"`
	} `yaml:"app"`

	Default struct {
		LogMaxSize                      int    `yaml:"log_max_size"`
		LogMaxAge                       int    `yaml:"log_max_age"`
		LogMaxBackups                   int    `yaml:"log_max_backups"`
		LogCompress                     bool   `yaml:"log_compress"`
		CSVTimestampFormat              string `yaml:"csv_timestamp_format"`
		NSSMFullPath                    string `yaml:"nssm_fullpath"`
		SuperUserEmail                  string `yaml:"super_user_email"`
		SuperUserPassword               string `yaml:"super_user_password"`
		SuperUserPhone                  string `yaml:"super_user_phone"`
		MinLengthPhoneNumber            int    `yaml:"min_length_phone_number"`
		DialingCodeDefault              string `yaml:"dialing_code_default"`
		CPUCaptureInterval              int    `yaml:"cpu_capture_interval"`
		MaxBadWordStrikes               int64  `yaml:"max_bad_word_strikes"`
		DataSeparator                   string `yaml:"data_separator"`
		PurgeOldBackupLogFilesOlderThan string `yaml:"purge_old_backup_log_files_older_than"`
		RemoveOldNeedsDirOlderThan      string `yaml:"remove_old_needs_dir_older_than"`
	} `yaml:"default"`

	Redis struct {
		Host       string `yaml:"host"`
		Port       int    `yaml:"port"`
		Password   string `yaml:"password"`
		Db         int    `yaml:"db"`
		MaxRetry   int    `yaml:"max_retry"`
		RetryDelay int    `yaml:"retry_delay"`
		PoolSize   int    `yaml:"pool_size"`
	} `yaml:"redis"`

	Database struct {
		Type                     string `yaml:"type"`
		Host                     string `yaml:"host"`
		Port                     int    `yaml:"port"`
		Username                 string `yaml:"username"`
		Password                 string `yaml:"password"`
		Name                     string `yaml:"name"`
		MaxRetryConnect          int    `yaml:"max_retry_connect"`
		RetryDelay               int    `yaml:"retry_delay"`
		MaxIdleConnection        int    `yaml:"max_idle_connection"`
		MaxOpenConnection        int    `yaml:"max_open_connection"`
		ConnMaxLifeTime          int    `yaml:"conn_max_lifetime"`
		ConnMaxIdleTime          int    `yaml:"conn_max_idle_time"`
		SSLMode                  string `yaml:"ssl_mode"`
		DBConfigPath             string `yaml:"db_config_path"`
		DBBackupDestinationDir   string `yaml:"db_backup_destination_dir"`
		PurgeOlderThan           string `yaml:"purge_older_than"`
		DumpedIndonesiaRegionSQL string `yaml:"dumped_indonesia_region_sql"`

		// Main tables
		TbUser                     string `yaml:"tb_user"`
		TbUserStatus               string `yaml:"tb_user_status"`
		TbUserPasswordChangeLog    string `yaml:"tb_user_password_change_log"`
		TbRole                     string `yaml:"tb_role"`
		TbRolePrivilege            string `yaml:"tb_role_privilege"`
		TbFeature                  string `yaml:"tb_feature"`
		TbLogActivity              string `yaml:"tb_log_activity"`
		TbLanguage                 string `yaml:"tb_language"`
		TbBadWord                  string `yaml:"tb_bad_word"`
		TbWebAppConfig             string `yaml:"tb_web_app_config"`
		TbIndonesiaRegion          string `yaml:"tb_indonesia_region"`
		TbWhatsappUser             string `yaml:"tb_whatsapp_user"`
		TbWhatsappMessage          string `yaml:"tb_whatsapp_message"`
		TbWhatsappMessageAutoReply string `yaml:"tb_whatsapp_message_auto_reply"`
	} `yaml:"database"`

	FolderFileNeeds []string `yaml:"folder_file_needs"`

	Schedules struct {
		Host     string      `yaml:"host"`
		Port     int         `yaml:"port"`
		Timezone string      `yaml:"timezone"`
		List     []Scheduler `yaml:"list"`
	} `yaml:"schedules"`

	API struct {
		AnalyticsDevAPIKey string `yaml:"analytics_dev_api_key"`
	} `yaml:"api"`

	Email struct {
		Host              string `yaml:"host"`
		Port              int    `yaml:"port"`
		Username          string `yaml:"username"`
		Password          string `yaml:"password"`
		Sender            string `yaml:"sender"`
		MaxRetry          int    `yaml:"max_retry"`
		RetryDelay        int    `yaml:"retry_delay"`
		MaxAttachmentSize int64  `yaml:"max_attachment_size"`
	} `yaml:"email"`

	Whatsnyan struct {
		DBLog                          string          `yaml:"db_log"`
		DBLogLevel                     string          `yaml:"db_log_level"`
		ClientLog                      string          `yaml:"client_log"`
		ClientLogLevel                 string          `yaml:"client_log_level"`
		LogMaxSize                     int             `yaml:"log_max_size"`
		LogMaxAge                      int             `yaml:"log_max_age"`
		LogMaxBackups                  int             `yaml:"log_max_backups"`
		LogCompress                    bool            `yaml:"log_compress"`
		GRPCHost                       string          `yaml:"grpc_host"`
		GRPCPort                       int             `yaml:"grpc_port"`
		WATechnicalSupport             string          `yaml:"wa_technical_support"`
		MaxMessageLength               int             `yaml:"max_message_length"`
		Tables                         WhatsnyanTables `yaml:"tables"`
		WAReplyPublicURL               string          `yaml:"wa_reply_public_url"`
		NeedVerifyAccount              bool            `yaml:"need_verify_account"`
		LanguageExpiry                 int             `yaml:"language_expiry"`
		LanguagePromptShownExpiry      int             `yaml:"language_prompt_shown_expiry"`
		NotRegisteredPhoneExpiry       int             `yaml:"not_registered_phone_expiry"`
		QuotaLimitExpiry               int             `yaml:"quota_limit_expiry"`
		LanguagePrompt                 string          `yaml:"language_prompt"`
		WAGAllowedToInteract           []string        `yaml:"wag_allowed_to_interact"`
		Files                          WhatsnyanFiles  `yaml:"files"`
		MessageProcessedToleranceHours int             `yaml:"message_processed_tolerance_hours"`
		PurgeMessageOlderThan          string          `yaml:"purge_message_older_than"`
	} `yaml:"whatsnyan"`

	GRPC struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"grpc"`

	Metrics struct {
		APIPort       int `yaml:"api_port"`
		GRPCPort      int `yaml:"grpc_port"`
		SchedulerPort int `yaml:"scheduler_port"`
		WhatsAppPort  int `yaml:"whatsapp_port"`
	} `yaml:"metrics"`

	RateLimit struct {
		Enabled     bool `yaml:"enabled"`
		Requests    int  `yaml:"requests"`     // requests per period
		Period      int  `yaml:"period"`       // period in seconds
		Burst       int  `yaml:"burst"`        // burst allowance
		CleanupTime int  `yaml:"cleanup_time"` // cleanup time in seconds
	} `yaml:"rate_limit"`
}

type Scheduler struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Every       string   `yaml:"every,omitempty"`
	At          []string `yaml:"at,omitempty"`
	Weekly      string   `yaml:"weekly,omitempty"`
	Monthly     string   `yaml:"monthly,omitempty"`
	Yearly      string   `yaml:"yearly,omitempty"`
}

type WhatsnyanTables struct {
	TBWhatsnyanMessage          string `yaml:"tb_whatsnyan_message"`
	TBWhatsnyanIncomingMessage  string `yaml:"tb_whatsnyan_incoming_message"`
	TBWhatsnyanGroup            string `yaml:"tb_whatsnyan_group"`
	TBWhatsnyanGroupParticipant string `yaml:"tb_whatsnyan_group_participant"`
	// Whatsmeow tables
	TBAppStateMutationMacs string `yaml:"tb_app_state_mutation_macs"`
	TBAppStateSyncKeys     string `yaml:"tb_app_state_sync_keys"`
	TBAppStateVersions     string `yaml:"tb_app_state_versions"`
	TBChatSettings         string `yaml:"tb_chat_settings"`
	TBContacts             string `yaml:"tb_contacts"`
	TBDevice               string `yaml:"tb_device"`
	TBEventBuffer          string `yaml:"tb_event_buffer"`
	TBIdentityKeys         string `yaml:"tb_identity_keys"`
	TBLIDMap               string `yaml:"tb_lid_map"`
	TBMessageSecrets       string `yaml:"tb_message_secrets"`
	TBPreKeys              string `yaml:"tb_pre_keys"`
	TBPrivacyTokens        string `yaml:"tb_privacy_tokens"`
	TBSenderKeys           string `yaml:"tb_sender_keys"`
	TBSessions             string `yaml:"tb_sessions"`
	TBVersion              string `yaml:"tb_version"`
}

type WhatsnyanFiles struct {
	Image struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds"`
		MaxSize           int64    `yaml:"max_size"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types"`
		AllowedExtensions []string `yaml:"allowed_extensions"`
	} `yaml:"image"`
	Video struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds"`
		MaxSize           int64    `yaml:"max_size"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types"`
		AllowedExtensions []string `yaml:"allowed_extensions"`
	} `yaml:"video"`
	Document struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds"`
		MaxSize           int64    `yaml:"max_size"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types"`
		AllowedExtensions []string `yaml:"allowed_extensions"`
	} `yaml:"document"`
	Audio struct {
		MaxDailyQuota     int      `yaml:"max_daily_quota"`
		CoolDownSeconds   int      `yaml:"cooldown_seconds"`
		MaxSize           int64    `yaml:"max_size"`
		AllowedMimeTypes  []string `yaml:"allowed_mime_types"`
		AllowedExtensions []string `yaml:"allowed_extensions"`
	} `yaml:"audio"`
}
