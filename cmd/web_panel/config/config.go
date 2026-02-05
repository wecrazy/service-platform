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
	config      TypeConfig
	configMutex sync.RWMutex
	configPath  string
)

// getEnvironment returns the current environment (dev or prod)
// Priority: 1. CONFIG_MODE from conf.yaml, 2. ENV environment variable, 3. GO_ENV, 4. default to "dev"
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

// getConfigSource returns a string indicating where the config mode was determined from
func getConfigSource() string {
	if getConfigModeFromFile() != "" {
		return "conf.yaml"
	}

	if env := os.Getenv("ENV"); env != "" && (env == "dev" || env == "prod") {
		return "ENV variable"
	}

	if env := os.Getenv("GO_ENV"); env != "" {
		return "GO_ENV variable"
	}

	return "default"
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

var yamlFilePaths = []string{
	"/config/conf.%s.yaml",
	"config/conf.%s.yaml",
	"../config/conf.%s.yaml",
	"/../config/conf.%s.yaml",
	"../../config/conf.%s.yaml",
	"/../../config/conf.%s.yaml",
	"D:/Documents/GitHub/service-platform/cmd/web_panel/config/conf.%s.yaml",
	"/home/user/server/service-platform/cmd/web_panel/config/conf.%s.yaml",
	"/home/administrator/server/service-platform/cmd/web_panel/config/conf.%s.yaml",
	"/home/user/Golang/server/service-platform/cmd/web_panel/config/conf.%s.yaml",
	"/home/user/Golang/service-platform/cmd/web_panel/config/conf.%s.yaml",
}

var mainConfigPaths = []string{
	"/config/conf.yaml",
	"config/conf.yaml",
	"../config/conf.yaml",
	"/../config/conf.yaml",
	"../../config/conf.yaml",
	"/../../config/conf.yaml",
	"D:/Documents/GitHub/service-platform/cmd/web_panel/config/conf.yaml",
	"/home/user/server/service-platform/cmd/web_panel/config/conf.yaml",
	"/home/administrator/server/service-platform/cmd/web_panel/config/conf.yaml",
	"/home/user/Golang/server/service-platform/cmd/web_panel/config/conf.yaml",
}

type defValueNewTicket struct {
	Stage        string `yaml:"STAGE"`
	Priority     string `yaml:"PRIORITY"`
	Keterangan   string `yaml:"KETERANGAN"`
	StatusInOdoo string `yaml:"STATUS_IN_ODOO"`
	SLADeadline  string `yaml:"SLA_DEADLINE"`
}

// MainConfig represents the main configuration structure for determining mode
type MainConfig struct {
	ConfigMode string `yaml:"CONFIG_MODE"`
}

type Schedule struct {
	Name    string   `yaml:"NAME"`
	Every   string   `yaml:"EVERY,omitempty"`
	At      []string `yaml:"AT,omitempty"`
	Weekly  string   `yaml:"WEEKLY,omitempty"`
	Monthly string   `yaml:"MONTHLY,omitempty"`
	Yearly  string   `yaml:"YEARLY,omitempty"`
}

type WhatsappErrorMessage struct {
	ID idWaErrMsg `yaml:"ID"`
	EN enWaErrMsg `yaml:"EN"`
}

type idWaErrMsg struct {
	PhoneNumberNotRegistered string `yaml:"PHONE_NUMBER_NOT_REGISTERED"`
	PhoneNumberIsBanned      string `yaml:"PHONE_NUMBER_IS_BANNED"`
	InvalidChat              string `yaml:"INVALID_CHAT"`
	MessageTypeDenied        string `yaml:"MESSAGE_TYPE_DENIED"`
	UnknownPrompt            string `yaml:"UNKNOWN_PROMPT"`
	AccountBannedCozBadWord  string `yaml:"ACCOUNT_BANNED_COZ_BAD_WORD"`
}

type enWaErrMsg struct {
	PhoneNumberNotRegistered string `yaml:"PHONE_NUMBER_NOT_REGISTERED"`
	PhoneNumberIsBanned      string `yaml:"PHONE_NUMBER_IS_BANNED"`
	InvalidChat              string `yaml:"INVALID_CHAT"`
	MessageTypeDenied        string `yaml:"MESSAGE_TYPE_DENIED"`
	UnknownPrompt            string `yaml:"UNKNOWN_PROMPT"`
	AccountBannedCozBadWord  string `yaml:"ACCOUNT_BANNED_COZ_BAD_WORD"`
}

type WhatsmeowDBSQLiteModel struct {
	TBAppStateMutationMacs string `yaml:"TB_APP_STATE_MUTATION_MACS"`
	TBAppStateSyncKeys     string `yaml:"TB_APP_STATE_SYNC_KEYS"`
	TBAppStateVersions     string `yaml:"TB_APP_STATE_VERSIONS"`
	TBChatSettings         string `yaml:"TB_CHAT_SETTINGS"`
	TBContacts             string `yaml:"TB_CONTACTS"`
	TBDevice               string `yaml:"TB_DEVICE"`
	TBEventBuffer          string `yaml:"TB_EVENT_BUFFER"`
	TBIdentityKeys         string `yaml:"TB_IDENTITY_KEYS"`
	TBLIDMap               string `yaml:"TB_LID_MAP"`
	TBMessageSecrets       string `yaml:"TB_MESSAGE_SECRETS"`
	TBPreKeys              string `yaml:"TB_PRE_KEYS"`
	TBPrivacyTokens        string `yaml:"TB_PRIVACY_TOKENS"`
	TBSenderKeys           string `yaml:"TB_SENDER_KEYS"`
	TBSessions             string `yaml:"TB_SESSIONS"`
	TBVersion              string `yaml:"TB_VERSION"`
}

type AIErr struct {
	To                       []string `yaml:"TO"`
	Cc                       []string `yaml:"CC"`
	Bcc                      []string `yaml:"BCC"`
	ActiveDebug              bool     `yaml:"ACTIVE_DEBUG"`
	StartParam               string   `yaml:"START_PARAM"`
	EndParam                 string   `yaml:"END_PARAM"`
	WhatsappSendToIfGotError []string `yaml:"WHATSAPP_SEND_TO_IF_GOT_ERROR"`
}

type Comprd struct {
	ActiveDebug bool   `yaml:"ACTIVE_DEBUG"`
	StartParam  string `yaml:"START_PARAM"`
	EndParam    string `yaml:"END_PARAM"`
}

type TechLoginReport struct {
	To  []string `yaml:"TO"`
	Cc  []string `yaml:"CC"`
	Bcc []string `yaml:"BCC"`
}

type MTIReport struct {
	Penarikan  MTIReportPenarikan  `yaml:"PENARIKAN"`
	Pemasangan MTIReportPemasangan `yaml:"PEMASANGAN"`
	VTR        MTIReportVTR        `yaml:"VTR"`
}

type MTIReportPenarikan struct {
	To  []string `yaml:"TO"`
	Cc  []string `yaml:"CC"`
	Bcc []string `yaml:"BCC"`
}

type MTIReportPemasangan struct {
	To  []string `yaml:"TO"`
	Cc  []string `yaml:"CC"`
	Bcc []string `yaml:"BCC"`
}

type MTIReportVTR struct {
	To  []string `yaml:"TO"`
	Cc  []string `yaml:"CC"`
	Bcc []string `yaml:"BCC"`
}

type ODOOMSMonitoringTicket struct {
	To                       []string `yaml:"TO"`
	Cc                       []string `yaml:"CC"`
	Bcc                      []string `yaml:"BCC"`
	ActiveDebug              bool     `yaml:"ACTIVE_DEBUG"`
	StartParam               string   `yaml:"START_PARAM"`
	EndParam                 string   `yaml:"END_PARAM"`
	ChartWidth               uint     `yaml:"CHART_WIDTH"`
	ChartHeight              uint     `yaml:"CHART_HEIGHT"`
	WhatsappSendToIfGotError []string `yaml:"WHATSAPP_SEND_TO_IF_GOT_ERROR"`
	LogExportChartDebugPath  string   `yaml:"LOG_EXPORT_CHART_DEBUG_PATH"`
}

type ODOOMSMonitoringLoginVisitTechnician struct {
	To                       []string `yaml:"TO"`
	Cc                       []string `yaml:"CC"`
	Bcc                      []string `yaml:"BCC"`
	ActiveDebug              bool     `yaml:"ACTIVE_DEBUG"`
	ChartWidth               uint     `yaml:"CHART_WIDTH"`
	ChartHeight              uint     `yaml:"CHART_HEIGHT"`
	WhatsappSendToIfGotError []string `yaml:"WHATSAPP_SEND_TO_IF_GOT_ERROR"`
	LogExportChartDebugPath  string   `yaml:"LOG_EXPORT_CHART_DEBUG_PATH"`
}

type SLAReport struct {
	To                       []string `yaml:"TO"`
	Cc                       []string `yaml:"CC"`
	Bcc                      []string `yaml:"BCC"`
	GeneratedTypes           []string `yaml:"GENERATED_TYPES"`
	ActiveDebug              bool     `yaml:"ACTIVE_DEBUG"`
	StartParam               string   `yaml:"START_PARAM"`
	EndParam                 string   `yaml:"END_PARAM"`
	WhatsappSendToIfGotError []string `yaml:"WHATSAPP_SEND_TO_IF_GOT_ERROR"`
}

type WhatsappModel struct {
	TBConversation          string `yaml:"TB_WA_CONVERSATION"`
	TBChatMessage           string `yaml:"TB_WA_CHAT_MESSAGE"`
	TBGroupParticipant      string `yaml:"TB_WA_GROUP_PARTICIPANT"`
	TBContactInfo           string `yaml:"TB_WA_CONTACT_INFO"`
	TBMessageDeliveryStatus string `yaml:"TB_WA_MESSAGE_DELIVERY_STATUS"`
	TBMediaFile             string `yaml:"TB_WA_MEDIA_FILE"`
}

type SACODOOMS struct {
	Username    string `yaml:"USERNAME"`
	FullName    string `yaml:"FULLNAME"`
	PhoneNumber string `yaml:"PHONE"`
	Email       string `yaml:"EMAIL"`
	TTDPath     string `yaml:"TTD_PATH"`
	Region      int    `yaml:"REGION"`
}

type DataUserTA struct {
	Name  string `yaml:"NAME"`
	Phone string `yaml:"PHONE"`
}

type HRDPT struct {
	Number      int    `yaml:"NUMBER"`
	Name        string `yaml:"NAME"`
	Email       string `yaml:"EMAIL"`
	PhoneNumber string `yaml:"PHONE_NUMBER"`
	TTDPath     string `yaml:"TTD_PATH"`
}

type PayslipTechnicianDebug struct {
	Active                 bool   `yaml:"ACTIVE"`
	PhoneNumberUsedForTest string `yaml:"PHONE_NUMBER_USED_FOR_TEST"`
	EmailUsedForTest       string `yaml:"EMAIL_USED_FOR_TEST"`
}

type TypeConfig struct {
	App struct {
		Host                 string `yaml:"HOST"`
		GinMode              string `yaml:"GIN_MODE"`
		Name                 string `yaml:"NAME"`
		Logo                 string `yaml:"LOGO"`
		Port                 string `yaml:"PORT"`
		LogLevel             string `yaml:"LOG_LEVEL"`
		LogFormat            string `yaml:"LOG_FORMAT"`
		WebPublicURL         string `yaml:"WEB_PUBLIC_URL"`
		Version              string `yaml:"VERSION"`
		VersionNo            int    `yaml:"VERSION_NO"`
		VersionCode          string `yaml:"VERSION_CODE"`
		VersionName          string `yaml:"VERSION_NAME"`
		StaticDir            string `yaml:"STATIC_DIR"`
		PublishedDir         string `yaml:"PUBLISHED_DIR"`
		LogDir               string `yaml:"LOG_DIR"`
		UploadDir            string `yaml:"UPLOAD_DIR"`
		LoginTimeM           string `yaml:"LOGIN_TIME_M"`
		CookieLoginDomain    string `yaml:"COOKIE_LOGIN_DOMAIN"`
		CookieLoginSecure    bool   `yaml:"COOKIE_LOGIN_SECURE"`
		MaxDisconnectionTime string `yaml:"MAX_DISCONNECTION_TIME"`
		AesKey               string `yaml:"AES_KEY"`
		AesKeyIV             string `yaml:"AES_KEY_IV"`
		MaxRetryLogin        int    `yaml:"MAX_RETRY_LOGIN"`
		LoginLockUntil       int    `yaml:"LOGIN_LOCK_UNTIL"`
		AppLogFilename       string `yaml:"APP_LOG_FILENAME"`
		SystemLogFilename    string `yaml:"SYSTEM_LOG_FILENAME"`
		MemoryProfilePath    string `yaml:"MEMORY_PROFILE_PATH"`
	} `yaml:"APP"`

	Default struct {
		Timezone  string `yaml:"TIMEZONE"`
		Delimiter string `yaml:"DELIMITER"`

		PT        string `yaml:"PT"`
		PTAddress string `yaml:"PT_ADDRESS"`
		PTCity    string `yaml:"PT_CITY"`

		PTHRD []HRDPT `yaml:"PT_HRD"`

		OdooDashboardReportingPHPServer  string `yaml:"ODOO_DASHBOARD_REPORTING_SERVER"`
		OdooDashboardReportingPHPPort    int    `yaml:"ODOO_DASHBOARD_REPORTING_PORT"`
		OdooDashboardReportingGolangPort int    `yaml:"ODOO_DASHBOARD_GOLANG_REPORTING_PORT"`

		CompanyId             int    `yaml:"COMPANY_ID"`
		MinLengthPhoneNumber  int    `yaml:"MIN_LENGTH_PHONE_NUMBER"`
		MaxMessageCharacters  int    `yaml:"MAX_MESSAGE_CHAR"`
		MaxImageSize          int64  `yaml:"MAX_IMAGE_SIZE"`
		XAMPPMySQLPath        string `yaml:"XAMPP_MYSQL_PATH"`
		NssmFullPath          string `yaml:"NSSM_FULLPATH"`
		MagickFullPath        string `yaml:"MAGICK_FULLPATH"`
		ConcurrencyLimit      int    `yaml:"CONCURRENCY_LIMIT"`
		APIKeyApiAnalyticsDev string `yaml:"API_KEY_API_ANALYTICS_DEV"`
		WelcomeID             string `yaml:"WELCOME_ID"`
		WelcomeEN             string `yaml:"WELCOME_EN"`
	} `yaml:"DEFAULT"`

	Email struct {
		Host              string `yaml:"HOST"`
		Port              int    `yaml:"PORT"`
		Username          string `yaml:"USERNAME"`
		Password          string `yaml:"PASSWORD"`
		Sender            string `yaml:"SENDER"`
		MaxRetry          int    `yaml:"MAX_RETRY"`
		RetryDelay        int    `yaml:"RETRY_DELAY"`
		MaxAttachmentSize int64  `yaml:"MAX_ATTACHMENT_SIZE"`
		// Listener
		ListenerHost     string `yaml:"LISTENER_HOST"`
		ListenerPort     int    `yaml:"LISTENER_PORT"`
		ListenerEmail    string `yaml:"LISTENER_EMAIL"`
		ListenerPassword string `yaml:"LISTENER_PASSWORD"`
	} `yaml:"EMAIL"`

	Redis struct {
		Host       string `yaml:"HOST"`
		Port       int    `yaml:"PORT"`
		Password   string `yaml:"PASSWORD"`
		Db         int    `yaml:"DB"`
		MaxRetry   int    `yaml:"MAX_RETRY"`
		RetryDelay int    `yaml:"RETRY_DELAY"`
	} `yaml:"REDIS"`

	Database struct {
		Type                   string `yaml:"TYPE"`
		Host                   string `yaml:"HOST"`
		Port                   string `yaml:"PORT"`
		Username               string `yaml:"USERNAME"`
		Password               string `yaml:"PASSWORD"`
		Name                   string `yaml:"NAME"`
		MaxRetryConnect        int    `yaml:"MAX_RETRY_CONNECT"`
		RetryDelay             int    `yaml:"RETRY_DELAY"`
		MaxIdleConns           int    `yaml:"MAX_IDLE_CONNECTION"`
		MaxOpenConns           int    `yaml:"MAX_OPEN_CONNECTION"`
		MaxLifetimeConns       int    `yaml:"MAX_LIFETIME_CONNECTION"`
		DBConfigPath           string `yaml:"DB_CONFIG_PATH"`
		DBBackupDestinationDir string `yaml:"DB_BACKUP_DESTINATION_DIR"`
		PurgeLogOlderThanDays  int    `yaml:"PURGE_LOG_OLDER_THAN_DAYS"`

		// MAIN TABLE
		TbAdmin             string `yaml:"TB_ADMIN"`
		TbAdminPwdChangelog string `yaml:"TB_ADMIN_PWD_CHANGELOG"`
		TbAdminStatus       string `yaml:"TB_ADMIN_STATUS"`
		TbFeature           string `yaml:"TB_FEATURE"`
		TbLogActivity       string `yaml:"TB_LOG_ACTIVITY"`
		TbRole              string `yaml:"TB_ROLE"`
		TbRolePrivilege     string `yaml:"TB_ROLE_PRIVILEGE"`
		TbLanguage          string `yaml:"TB_LANGUAGE"`
		TbBadWord           string `yaml:"TB_BAD_WORD"`
		TbWebAppConfig      string `yaml:"TB_WEB_APP_CONFIG"`
		TbIndonesiaRegion   string `yaml:"TB_INDONESIA_REGION"`

		TbTicketHommyPayCC string            `yaml:"TB_TICKET_HOMMYPAY_CC"`
		TbTicketType       string            `yaml:"TB_TICKET_TYPE"`
		TbTicketStage      string            `yaml:"TB_TICKET_STAGE"`
		TbMerchant         string            `yaml:"TB_MERCHANT"`
		TbWAMsg            string            `yaml:"TB_WA_MSG"`
		TbWAMsgReply       string            `yaml:"TB_WA_MSG_REPLY"`
		TbWAPhoneUser      string            `yaml:"TB_WA_PHONE_USER"`
		TbMail             string            `yaml:"TB_MAIL"`
		TbTrustedClient    string            `yaml:"TB_TRUSTED_CLIENT"`
		DefaultValues      defValueNewTicket `yaml:"DEFAULT_VALUES"`

		// Fastlink
		HostFastlink       string `yaml:"HOST_FASTLINK"`
		PortFastlink       string `yaml:"PORT_FASTLINK"`
		UsernameFastlink   string `yaml:"USERNAME_FASTLINK"`
		PasswordFastlink   string `yaml:"PASSWORD_FASTLINK"`
		NameFastlink       string `yaml:"NAME_FASTLINK"`
		TbMerchantFastlink string `yaml:"TB_MERCHANT_FASTLINK"`

		// KresekBag
		TbMerchantKresekBag string `yaml:"TB_MERCHANT_KRESEKBAG"`

		// Technical Assistance
		HostTA             string `yaml:"HOST_TA"`
		PortTA             string `yaml:"PORT_TA"`
		UsernameTA         string `yaml:"USERNAME_TA"`
		PasswordTA         string `yaml:"PASSWORD_TA"`
		NameTA             string `yaml:"NAME_TA"`
		HostWebTA          string `yaml:"HOST_WEB_TA"`
		PortWebTA          string `yaml:"PORT_WEB_TA"`
		UsernameWebTA      string `yaml:"USERNAME_WEB_TA"`
		PasswordWebTA      string `yaml:"PASSWORD_WEB_TA"`
		NameWebTA          string `yaml:"NAME_WEB_TA"`
		TbTAError          string `yaml:"TB_TA_ERROR"`
		TbTAPending        string `yaml:"TB_TA_PENDING"`
		TbTALogAct         string `yaml:"TB_TA_LOGACT"`
		TbTATempSubmission string `yaml:"TB_TA_TEMPSUBMISSION"`
		TbTAAdmin          string `yaml:"TB_TA_ADMIN"`
		TbTAHandledData    string `yaml:"TB_TA_HANDLEDDATA"`

		// Report
		TbReportScheduled        string `yaml:"TB_REPORT_SCHEDULED"`
		TbReportEngineersProd    string `yaml:"TB_REPORT_ENGINEERS_PRODUCTIVITY"`
		TbReportCompared         string `yaml:"TB_REPORT_COMPARED"`
		TbReportMonitoringTicket string `yaml:"TB_REPORT_MONITORING_TICKET"`
		TbReportSLA              string `yaml:"TB_REPORT_SLA"`

		// ODOO Manage Service
		TbODOOMSUploadedExcel                   string `yaml:"TB_UPLOADED_EXCEL"`
		TbODOOMSTicketField                     string `yaml:"TB_TICKET_FIELD"`
		TbODOOMSTaskField                       string `yaml:"TB_TASK_FIELD"`
		TbODOOMSJobGroup                        string `yaml:"TB_JOB_GROUP"`
		TbODOOMSDataTech                        string `yaml:"TB_DATA_TECHNICIAN"`
		TbFSParams                              string `yaml:"TB_FS_PARAMS"`
		TbFSParamsPayment                       string `yaml:"TB_FS_PARAMS_PAYMENT"`
		TbProductTemplate                       string `yaml:"TB_PRODUCT_TEMPLATE"`
		TbCompany                               string `yaml:"TB_COMPANY"`
		TbTicketTypeODOOMS                      string `yaml:"TB_TICKET_TYPE_ODOOMS"`
		TbBALost                                string `yaml:"TB_BA_LOST"`
		TbMSTechnicianPayroll                   string `yaml:"TB_MS_TECHNICIAN_PAYROLL"`
		TbMSTechnicianPayrollTicketsRegularEDC  string `yaml:"TB_MS_TECHNICIAN_PAYROLL_TICKETS_REGULAR_EDC"`
		TbMSTechnicianPayrollTicketsBP          string `yaml:"TB_MS_TECHNICIAN_PAYROLL_TICKETS_BP"`
		TbMSTechnicianPayrollTicketsUnworkedEDC string `yaml:"TB_MS_TECHNICIAN_PAYROLL_TICKETS_UNWORKED_EDC"`
		TbMSTechnicianPayrollTicketsRegularATM  string `yaml:"TB_MS_TECHNICIAN_PAYROLL_TICKETS_REGULAR_ATM"`
		TbMSTechnicianPayrollTicketsUnworkedATM string `yaml:"TB_MS_TECHNICIAN_PAYROLL_TICKETS_UNWORKED_ATM"`
		TbMSTechnicianPayrollDedicatedATM       string `yaml:"TB_MS_TECHNICIAN_PAYROLL_DEDICATED_ATM"`

		DumpedIndonesiaRegionSQL string `yaml:"DUMPED_INDONESIA_REGION_SQL"`
	} `yaml:"DATABASE"`

	ApiODOO struct {
		JSONRPC string `yaml:"JSONRPC"`
		// ODOO Manage Service
		Login         string `yaml:"LOGIN"`
		Password      string `yaml:"PASSWORD"`
		Db            string `yaml:"DB"`
		UrlSession    string `yaml:"URL_SESSION"`
		UrlGetData    string `yaml:"URL_GETDATA"`
		UrlUpdateData string `yaml:"URL_UPDATEDATA"`
		UrlCreateData string `yaml:"URL_CREATEDATA"`
		// Hommy Pay
		LoginHommyPay         string `yaml:"LOGIN_HOMMYPAY"`
		PasswordHommyPay      string `yaml:"PASSWORD_HOMMYPAY"`
		DbHommyPay            string `yaml:"DB_HOMMYPAY"`
		UrlSessionHommyPay    string `yaml:"URL_SESSION_HOMMYPAY"`
		UrlGetDataHommyPay    string `yaml:"URL_GETDATA_HOMMYPAY"`
		UrlUpdateDataHommyPay string `yaml:"URL_UPDATEDATA_HOMMYPAY"`
		UrlCreateDataHommyPay string `yaml:"URL_CREATEDATA_HOMMYPAY"`
		// KresekBag
		LoginKresekBag         string `yaml:"LOGIN_KRESEK_BAG"`
		PasswordKresekBag      string `yaml:"PASSWORD_KRESEK_BAG"`
		DbKresekBag            string `yaml:"DB_KRESEK_BAG"`
		UrlSessionKresekBag    string `yaml:"URL_SESSION_KRESEK_BAG"`
		UrlGetDataKresekBag    string `yaml:"URL_GETDATA_KRESEK_BAG"`
		UrlUpdateDataKresekBag string `yaml:"URL_UPDATEDATA_KRESEK_BAG"`
		UrlCreateDataKresekBag string `yaml:"URL_CREATEDATA_KRESEK_BAG"`

		//
		MaxRetry        int    `yaml:"MAX_RETRY"`
		RetryDelay      int    `yaml:"RETRY_DELAY"`
		SessionTimeout  string `yaml:"SESSION_TIMEOUT"`
		GetDataTimeout  string `yaml:"GETDATA_TIMEOUT"`
		CompanyAllowed  []int  `yaml:"COMPANY_ALLOWED"`
		CompanyExcluded []int  `yaml:"COMPANY_EXCLUDED"`
	} `yaml:"ODOO_API"`

	Whatsmeow struct {
		SqlDriver                 string                 `yaml:"SQL_DRIVER"`
		QrCode                    string                 `yaml:"QR_CODE"`
		QrExpired                 int                    `yaml:"QR_EXPIRED"`
		SqlSource                 string                 `yaml:"SQL_SOURCE"`
		WaGroupSource             string                 `yaml:"WA_GROUP_SOURCE"`
		DBSQLiteModel             WhatsmeowDBSQLiteModel `yaml:"DB_SQLITE_MODEL"`
		WaSuperUser               string                 `yaml:"WA_SU"`
		WaSupport                 string                 `yaml:"WA_SUPPORT"`
		WaTechnicalSupport        string                 `yaml:"WA_TECHNICAL_SUPPORT"`
		WaBotUsed                 []string               `yaml:"WA_BOT_USED"`
		WaGroupAllowedToUsePrompt []string               `yaml:"WAG_ALLOWED_PROMPT"`
		WAGTestJID                string                 `yaml:"WAG_TEST_JID"`
		WAGTAJID                  string                 `yaml:"WAG_TA_JID"`
		WAGRegionTechnician       map[int]string         `yaml:"WAG_REGION_TECHNICIAN"`
		InitLanguagePrompt        string                 `yaml:"INITIAL_LANGUAGE_PROMPT"`
		WAReplyPublicURL          string                 `yaml:"WA_REPLY_PUBLIC_URL"`
		OllamaURL                 string                 `yaml:"OLLAMA_URL"`
		OllamaModel               string                 `yaml:"OLLAMA_MODEL"`
		UseAPIRafy                bool                   `yaml:"USE_API_RAFY"`
		KeywordSeparator          string                 `yaml:"KEYWORD_SEPARATOR"`
		MsgReceivedLogFile        string                 `yaml:"MESSAGE_RECEIVED_LOG_FILE"`
		WhatsmeowClientLog        string                 `yaml:"WHATSMEOW_CLIENT_LOG"`
		WhatsmeowClientLogLevel   string                 `yaml:"WHATSMEOW_CLIENT_LOG_LEVEL"`
		WhatsmeowDBLog            string                 `yaml:"WHATSMEOW_DB_LOG"`
		WhatsmeowDBLogLevel       string                 `yaml:"WHATSMEOW_DB_LOG_LEVEL"`
		OpenWeatherMapAPIKey      string                 `yaml:"OPEN_WEATHER_MAP_API"`
		WhatsappMaxDailyQuota     int                    `yaml:"WHATSAPP_MAX_DAILY_QUOTA"`
		WhatsappMaxBadWordStrike  int                    `yaml:"WHATSAPP_MAX_BAD_WORD_STRIKE"`
		RedisExpiry               int                    `yaml:"REDIS_EXPIRY"`
		MaxUploadedDocumentSize   int64                  `yaml:"MAX_UPLOADED_DOCUMENT_SIZE"`
		MaxUploadedImageSize      int64                  `yaml:"MAX_UPLOADED_IMAGE_SIZE"`
		MaxUploadedAudioSize      int64                  `yaml:"MAX_UPLOADED_AUDIO_SIZE"`
		MaxUploadedVideoSize      int64                  `yaml:"MAX_UPLOADED_VIDEO_SIZE"`
		DocumentAllowedExtensions []string               `yaml:"DOCUMENT_ALLOWED_EXTENSIONS"`
		DocumentAllowedMimeTypes  []string               `yaml:"DOCUMENT_ALLOWED_MIME_TYPES"`
		ImageAllowedExtensions    []string               `yaml:"IMAGE_ALLOWED_EXTENSIONS"`
		ImageAllowedMimeTypes     []string               `yaml:"IMAGE_ALLOWED_MIME_TYPES"`
		AudioAllowedExtensions    []string               `yaml:"AUDIO_ALLOWED_EXTENSIONS"`
		AudioAllowedMimeTypes     []string               `yaml:"AUDIO_ALLOWED_MIME_TYPES"`
		VideoAllowedExtensions    []string               `yaml:"VIDEO_ALLOWED_EXTENSIONS"`
		VideoAllowedMimeTypes     []string               `yaml:"VIDEO_ALLOWED_MIME_TYPES"`
		WaErrorMessage            WhatsappErrorMessage   `yaml:"WA_ERROR_MESSAGE"`
		WelcomingUserID           string                 `yaml:"WELCOMING_USER_ID"`
		WelcomingUserEN           string                 `yaml:"WELCOMING_USER_EN"`
		WhatsappModel             WhatsappModel          `yaml:"WHATSAPP_MODEL"`
	} `yaml:"WHATSMEOW"`

	TelegramService struct {
		GRPCHost          string `yaml:"GRPC_HOST"`
		GRPCPort          int    `yaml:"GRPC_PORT"`
		ConnectionTimeout int    `yaml:"CONNECTION_TIMEOUT"` // seconds
		RequestTimeout    int    `yaml:"REQUEST_TIMEOUT"`    // seconds
	} `yaml:"TELEGRAM_SERVICE"`

	HommyPayCCData struct {
		EmailListenerCreateNewTicketInODOO bool   `yaml:"EMAIL_LISTENER_CREATE_NEW_TICKET_IN_ODOO"`
		KetFromOdoo                        string `yaml:"KET_FROM_ODOO"`
		StartTicketId                      int    `yaml:"START_TICKET_ID"`
		WelcomeID                          string `yaml:"WELCOME_ID"`
		WelcomeEN                          string `yaml:"WELCOME_EN"`
	} `yaml:"HOMMYPAY_CC_DATA"`

	FastLinkData struct {
		PublicURLMerchantImage string `yaml:"PUBLIC_URL_MERCHANT_IMAGE"`
	} `yaml:"FASTLINK_DATA"`

	Schedules []Schedule            `yaml:"SCHEDULES"`
	UserTA    map[string]DataUserTA `yaml:"USER_TA"`

	TechnicalAssistanceData struct {
		DashboardTAPublicURL              string   `yaml:"DASHBOARD_TA_PUBLIC_URL"`
		AllowedToAccessReportPhoneNumbers []string `yaml:"ALLOWED_TO_ACCESS_REPORT_PHONE_NUMBERS"`
		ReportTAMentions                  []string `yaml:"REPORT_TA_MENTIONS"`
		PublicURLReportTA                 string   `yaml:"PUBLIC_URL_REPORT_TA"`
		PublicURLReportTechError          string   `yaml:"PUBLIC_URL_REPORT_TECH_ERROR"`
		PublicURLReportCompared           string   `yaml:"PUBLIC_URL_REPORT_COMPARED"`
		LogExportPivotDebugPath           string   `yaml:"LOG_EXPORT_PIVOT_DEBUG_PATH"`
	} `yaml:"TECHNICAL_ASSISTANCE_DATA"`

	Report struct {
		AIError                        AIErr                                `yaml:"AI_ERROR"`
		Compared                       Comprd                               `yaml:"COMPARED"`
		MTI                            MTIReport                            `yaml:"MTI"`
		TechnicianLogin                TechLoginReport                      `yaml:"TECHNICIAN_LOGIN"`
		MonitoringTicketODOOMS         ODOOMSMonitoringTicket               `yaml:"MONITORING_TICKET_ODOOMS"`
		MonitoringLoginVisitTechnician ODOOMSMonitoringLoginVisitTechnician `yaml:"MONITORING_VISIT_LOGIN_TECHNICIAN_ODOOMS"`
		SLA                            SLAReport                            `yaml:"SLA"`
	} `yaml:"REPORT"`

	VMOdooDashboard struct {
		SSHUser string `yaml:"SSH_USER"`
		SSHPwd  string `yaml:"SSH_PWD"`
		SSHAddr string `yaml:"SSH_ADDR"`
	} `yaml:"VM_ODOO_DASHBOARD"`

	ReportMTI struct {
		ReportDir         string `yaml:"REPORT_DIR"`
		DBTablePemasangan string `yaml:"DB_TABLE_PEMASANGAN"`
		DBTablePenarikan  string `yaml:"DB_TABLE_PENARIKAN"`
		DBTableVTR        string `yaml:"DB_TABLE_VTR"`
	} `yaml:"REPORT_MTI"`

	UploadedExcelForODOOMS struct {
		ActiveDebug        bool   `yaml:"ACTIVE_DEBUG"`
		EmailParam         string `yaml:"EMAIL_PARAM"`
		PwdParam           string `yaml:"PWD_PARAM"`
		Timeout            int    `yaml:"TIMEOUT"`       // seconds
		MaxFileSize        int    `yaml:"MAX_FILE_SIZE"` // MB
		ThresholdPurgeFile string `yaml:"THRESHOLD_RANGE_PURGE_FILE"`
	} `yaml:"UPLOADED_EXCEL_FOR_ODOO_MS"`

	SPTechnician struct {
		TBJoPlannedOdooMS          string   `yaml:"TB_JO_PLANNED_ODOO_MS"`
		TBJoPlannedOdooATM         string   `yaml:"TB_JO_PLANNED_ODOO_ATM"`
		TBTechGotSP                string   `yaml:"TB_TECH_GOT_SP"`
		TBSPLGotSP                 string   `yaml:"TB_SPL_GOT_SP"`
		TBSACGotSP                 string   `yaml:"TB_SAC_GOT_SP"`
		TBNomorSuratSP             string   `yaml:"TB_NOMOR_SURAT_SP"`
		TBSPWhatsappMsg            string   `yaml:"TB_SP_WHATSAPP_MESSAGE"`
		LastNomorSuratSP1Generated int      `yaml:"LAST_NOMOR_SURAT_SP1_GENERATED"`
		LastNomorSuratSP2Generated int      `yaml:"LAST_NOMOR_SURAT_SP2_GENERATED"`
		LastNomorSuratSP3Generated int      `yaml:"LAST_NOMOR_SURAT_SP3_GENERATED"`
		ActiveDebug                bool     `yaml:"ACTIVE_DEBUG"`
		PhoneNumberUsedForTest     string   `yaml:"PHONE_NUMBER_USED_FOR_TEST"`
		EmailUsedForTest           string   `yaml:"EMAIL_USED_FOR_TEST"`
		MinResponseSPAtHour        int      `yaml:"MIN_RESPONSE_SP_AT_HOUR"`
		MaxResponseSPAtHour        int      `yaml:"MAX_RESPONSE_SP_AT_HOUR"`
		RunOnWeekends              bool     `yaml:"RUN_ON_WEEKENDS"`
		RunOnHolidays              bool     `yaml:"RUN_ON_HOLIDAYS"`
		ExcludeStages              []string `yaml:"EXCLUDE_STAGES"`
		UncheckLinkPhoto           bool     `yaml:"UNCHECK_LINK_PHOTO"`
		ATMDedicatedTechnician     []string `yaml:"ATM_DEDICATED_TECHNICIAN"`
		MinimumJOVisited           int      `yaml:"MINIMUM_JO_VISITED"`
	} `yaml:"SP_TECHNICIAN"`

	StockOpname struct {
		TbListSPSO                     string   `yaml:"TB_LIST_SP_SO"`
		TbSPSOWhatsappMsg              string   `yaml:"TB_SP_SO_WA_MSG"`
		PhoneNumberUsedForTestSO       string   `yaml:"PHONE_NUMBER_USED_FOR_TEST_SO"`
		SOActiveDebug                  bool     `yaml:"SO_ACTIVE_DEBUG"`
		SORunOnWeekends                bool     `yaml:"SO_RUN_ON_WEEKENDS"`
		SORunOnHolidays                bool     `yaml:"SO_RUN_ON_HOLIDAYS"`
		NumberOfDaysJONotSOYet         int      `yaml:"NUMBER_OF_DAYS_JO_NOT_SO_YET"`
		MaxResponseSPStockOpnameAtHour int      `yaml:"MAX_RESPONSE_SP_SO_AT_HOUR"`
		TbListProductEDC               string   `yaml:"TB_LIST_PRODUCT_EDC"`
		SOReportSendTo                 []string `yaml:"SO_REPORT_SEND_TO"`
		SOReportSendToEmail            []string `yaml:"SO_REPORT_SEND_TO_EMAIL"`
	} `yaml:"STOCK_OPNAME"`

	FolderFileNeeds []string `yaml:"FOLDER_FILE_NEEDS"`

	ODOOMSSAC map[string]SACODOOMS `yaml:"ODOOMS_SAC"`

	ContractTechnicianODOO struct {
		TBContractTechnician               string   `yaml:"TB_CONTRACT_TECHNICIAN"`
		TBNomorSuratContract               string   `yaml:"TB_NOMOR_SURAT_CONTRACT"`
		LastNomorSuratGenerated            int      `yaml:"LAST_NOMOR_SURAT_CONTRACT_GENERATED"`
		ActiveDebug                        bool     `yaml:"ACTIVE_DEBUG"`
		EmailTest                          string   `yaml:"EMAIL_USED_FOR_TEST"`
		CCContractEmail                    []string `yaml:"CC_CONTRACT_EMAIL"`
		PhoneNumberTest                    string   `yaml:"PHONE_NUMBER_USED_FOR_TEST"`
		MustJoinedAfter                    int      `yaml:"MUST_JOINED_AFTER"` // in days
		SkippedTechnician                  []string `yaml:"TECHNICIAN_SKIPPED"`
		ExcludedJSONKeysForTable           []string `yaml:"EXCLUDED_JSON_KEYS_FOR_TABLE"`
		NotifyHRDBeforeContractExpiredDays int      `yaml:"NOTIFY_HRD_BEFORE_CONTRACT_EXPIRED_DAYS"`
	} `yaml:"CONTRACT_TECHNICIAN_ODOO"`

	API struct {
		IndonesianPublicHoliday string `yaml:"INDONESIAN_PUBLIC_HOLIDAY"`
		RafyFAQODOOMS           string `yaml:"RAFY_FAQ_ODOOMS"`
		RafyFAQODOOMSSOP        string `yaml:"RAFY_FAQ_ODOOMS_SOP"`
		RafyFAQNUSACITA         string `yaml:"RAFY_FAQ_NUSACITA"`
		APIRafyTimeout          int    `yaml:"API_RAFY_TIMEOUT"`
		LibreTranslate          string `yaml:"LIBRE_TRANSLATE"`
		KukuhFilestoreURL       string `yaml:"KUKUH_FILESTORE"`
	} `yaml:"API"`

	MTI struct {
		TBDataODOOMS      string `yaml:"TB_DATA_ODOOMS"`
		TBDataPM          string `yaml:"TB_DATA_PM"`
		CompanyIDInODOOMS int    `yaml:"COMPANY_ID_IN_ODOOMS"`
		ActiveDebug       bool   `yaml:"ACTIVE_DEBUG"`
		StartParam        string `yaml:"START_PARAM"`
		EndParam          string `yaml:"END_PARAM"`
	} `yaml:"MTI"`

	DKI struct {
		TBDataTicketODOOMS string `yaml:"TB_DATA_ODOOMS"`
		CompanyIDInODOOMS  int    `yaml:"COMPANY_ID_IN_ODOOMS"`
	} `yaml:"DKI"`

	DSP struct {
		TBDataTicketODOOMS string `yaml:"TB_DATA_ODOOMS"`
		CompanyIDInODOOMS  int    `yaml:"COMPANY_ID_IN_ODOOMS"`
	} `yaml:"DSP"`

	BNI struct {
		TBDataODOOMS      string `yaml:"TB_DATA_ODOOMS"`
		TBDataPM          string `yaml:"TB_DATA_PM"`
		CompanyIDInODOOMS int    `yaml:"COMPANY_ID_IN_ODOOMS"`
		ActiveDebug       bool   `yaml:"ACTIVE_DEBUG"`
		StartParam        string `yaml:"START_PARAM"`
		EndParam          string `yaml:"END_PARAM"`
	} `yaml:"BNI"`

	Indonesia struct {
		CompletedCity map[string]string `yaml:"COMPLETED_CITY"`
	} `yaml:"INDONESIA"`

	ODOOMSParam struct {
		DefaultPrice                   float64                `yaml:"DEFAULT_PRICE"`
		DefaultPenalty                 float64                `yaml:"DEFAULT_PENALTY"`
		DefaultEDCLostFee              float64                `yaml:"DEFAULT_EDC_LOST_FEE"`
		DefaultATMPrice                float64                `yaml:"DEFAULT_ATM_PRICE"`
		PayslipTechnicianSignatureName string                 `yaml:"PAYSLIP_TECHNICIAN_SIGNATURE_NAME"`
		PayslipTechnicianSignatureImg  string                 `yaml:"PAYSLIP_TECHNICIAN_SIGNATURE_IMG"`
		PayslipTechnicianCCEmail       []string               `yaml:"PAYSLIP_TECHNICIAN_CC_EMAIL"`
		PayslipTechnicianDebug         PayslipTechnicianDebug `yaml:"PAYSLIP_TECHNICIAN_DEBUG"`
	} `yaml:"ODOOMS_PARAM"`
}

func YAMLLoad(filePath string) (*TypeConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config TypeConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func InitConfig() (*TypeConfig, error) {
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

	var TypeConfig *TypeConfig

	for _, filePath := range configPaths {
		if _, err := os.Stat(filePath); err == nil {
			TypeConfig, err = YAMLLoad(filePath)
			if err != nil {
				log.Printf("failed to load configuration from '%s': %v", filePath, err)
				continue
			}
			log.Printf("Configuration successfully loaded from '%s' (Environment: %s from %s)", filePath, getEnvironment(), getConfigSource())
			break
		} else if os.IsNotExist(err) {
			log.Printf("configuration file '%s' does not exist. Skipping.", filePath)
		} else {
			log.Printf("error checking file '%s': %v", filePath, err)
		}
	}

	if TypeConfig == nil {
		log.Fatalf("failed to load YAML configuration: no valid configuration file found in paths: %v", configPaths)
	}

	return TypeConfig, nil
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

	var newConfig TypeConfig
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

func GetConfig() TypeConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}
