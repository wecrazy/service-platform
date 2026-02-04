package config

// ===== GLOBAL CONFIG INSTANCES =====
// Ready-to-use singleton instances
var TechnicalAssistance = &configs[TypeTechnicalAssistance]{}

// ===== CONFIG TYPE DEFINITIONS =====
// Pure data structures for WebPanel config types
// TypeWebPanel represents web panel configuration
type TypeTechnicalAssistance struct {
	Email struct {
		Host       string `yaml:"HOST"`
		Port       int    `yaml:"PORT"`
		Username   string `yaml:"USERNAME"`
		Password   string `yaml:"PASSWORD"`
		MaxRetry   int    `yaml:"MAX_RETRY"`
		RetryDelay int    `yaml:"RETRY_DELAY"`
	} `yaml:"EMAIL"`

	Default struct {
		TaFeedbackURL string `yaml:"TA_FEEDBACK_URL"`
	} `yaml:"DEFAULT"`

	Odoo struct {
		JSONRPC string `yaml:"JSONRPC"`
		// Manage Service
		Login         string `yaml:"LOGIN"`
		Password      string `yaml:"PASSWORD"`
		Db            string `yaml:"DB"`
		UrlSession    string `yaml:"URL_SESSION"`
		UrlGetData    string `yaml:"URL_GETDATA"`
		LoginATM      string `yaml:"LOGIN_ATM"`
		PasswordATM   string `yaml:"PASSWORD_ATM"`
		DbATM         string `yaml:"DB_ATM"`
		UrlSessionATM string `yaml:"URL_SESSION_ATM"`
		UrlGetDataATM string `yaml:"URL_GETDATA_ATM"`
		// ATM
		MaxRetry       string `yaml:"MAX_RETRY"`
		RetryDelay     string `yaml:"RETRY_DELAY"`
		SessionTimeout string `yaml:"SESSION_TIMEOUT"`
		GetDataTimeout string `yaml:"GETDATA_TIMEOUT"`
		CompanyAllowed []int  `yaml:"COMPANY_ALLOWED"`
	} `yaml:"ODOO"`

	Report struct {
		To  []string `yaml:"TO"`
		Cc  []string `yaml:"CC"`
		Bcc []string `yaml:"BCC"`
	} `yaml:"REPORT"`

	GIN_MODE                            string `yaml:"GIN_MODE"`
	REDIS_HOST                          string `yaml:"REDIS_HOST"`
	REDIS_PORT                          string `yaml:"REDIS_PORT"`
	REDIS_PASSWORD                      string `yaml:"REDIS_PASSWORD"`
	REDIS_DB                            string `yaml:"REDIS_DB"`
	WHATSAPP_REDIS_DB                   string `yaml:"WHATSAPP_REDIS_DB"`
	MYSQL_HOST_DB                       string `yaml:"MYSQL_HOST_DB"`
	MYSQL_PORT_DB                       string `yaml:"MYSQL_PORT_DB"`
	MYSQL_USER_DB                       string `yaml:"MYSQL_USER_DB"`
	MYSQL_PASS_DB                       string `yaml:"MYSQL_PASS_DB"`
	MYSQL_NAME_DB                       string `yaml:"MYSQL_NAME_DB"`
	MYSQL_NAME_CALL_CENTER_DB           string `yaml:"MYSQL_NAME_CALL_CENTER_DB"`
	MYSQL_HOST_DB_KONFIRMASI_PENGERJAAN string `yaml:"MYSQL_HOST_DB_KONFIRMASI_PENGERJAAN"`
	MYSQL_PORT_DB_KONFIRMASI_PENGERJAAN string `yaml:"MYSQL_PORT_DB_KONFIRMASI_PENGERJAAN"`
	MYSQL_USER_DB_KONFIRMASI_PENGERJAAN string `yaml:"MYSQL_USER_DB_KONFIRMASI_PENGERJAAN"`
	MYSQL_PASS_DB_KONFIRMASI_PENGERJAAN string `yaml:"MYSQL_PASS_DB_KONFIRMASI_PENGERJAAN"`
	MYSQL_NAME_DB_KONFIRMASI_PENGERJAAN string `yaml:"MYSQL_NAME_DB_KONFIRMASI_PENGERJAAN"`
	APP_LISTEN                          string `yaml:"APP_LISTEN"`
	WEB_PUBLIC_URL                      string `yaml:"WEB_PUBLIC_URL"`
	FILESTORE_URL                       string `yaml:"FILESTORE_URL"`
	WO_DETAIL_URL                       string `yaml:"WO_DETAIL_URL"`
	WHATSAPP_CIDENG_SERVER              string `yaml:"WHATSAPP_CIDENG_SERVER"`
	ENDPOINT_KUKUH_GET_DATA             string `yaml:"ENDPOINT_KUKUH_GET_DATA"`
	APP_NAME                            string `yaml:"APP_NAME"`
	APP_LOGO                            string `yaml:"APP_LOGO"`
	APP_VERSION_NO                      string `yaml:"APP_VERSION_NO"`
	APP_VERSION_CODE                    string `yaml:"APP_VERSION_CODE"`
	APP_VERSION_NAME                    string `yaml:"APP_VERSION_NAME"`
	APP_STATIC_DIR                      string `yaml:"APP_STATIC_DIR"`
	APP_PUBLISHED_DIR                   string `yaml:"APP_PUBLISHED_DIR"`
	APP_LOG_DIR                         string `yaml:"APP_LOG_DIR"`
	APP_UPLOAD_DIR                      string `yaml:"APP_UPLOAD_DIR"`
	CONFIG_SMTP_HOST                    string `yaml:"CONFIG_SMTP_HOST"`
	CONFIG_SMTP_PORT                    string `yaml:"CONFIG_SMTP_PORT"`
	CONFIG_AUTH_EMAIL                   string `yaml:"CONFIG_AUTH_EMAIL"`
	CONFIG_AUTH_PASSWORD                string `yaml:"CONFIG_AUTH_PASSWORD"`
	CONFIG_SMTP_SENDER                  string `yaml:"CONFIG_SMTP_SENDER"`
	LOGIN_TIME_M                        string `yaml:"LOGIN_TIME_M"`
	COOKIE_LOGIN_DOMAIN                 string `yaml:"COOKIE_LOGIN_DOMAIN"`
	COOKIE_LOGIN_SECURE                 string `yaml:"COOKIE_LOGIN_SECURE"`
	MAX_DISCONECTION_TIME_S             string `yaml:"MAX_DISCONECTION_TIME_S"`
	AES_KEY                             string `yaml:"AES_KEY"`
	AES_KEY_IV                          string `yaml:"AES_KEY_IV"`
}
