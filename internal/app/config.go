package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	EnvFile                 = "/.env"
	BackupDir               = "/bitwarden/backup"
	MailConfigDir           = "/config/mail"
	MailParentMessageIDFile = MailConfigDir + "/parent_message_id"
)

var messageIDPattern = regexp.MustCompile(`^<.*@.+>$`)

// Config holds the environment configuration used by the backup program.
type Config struct {
	DataDir                       string
	DataDB                        string
	DataConfig                    string
	DataRSAKey                    string
	DataRSAKeyDirname             string
	DataRSAKeyBasename            string
	DataAttachments               string
	DataAttachmentsDirname        string
	DataAttachmentsBasename       string
	DataSends                     string
	DataSendsDirname              string
	DataSendsBasename             string
	DBType                        string
	PGHost                        string
	PGPort                        string
	PGDBName                      string
	PGUsername                    string
	PGPassword                    string
	MySQLHost                     string
	MySQLPort                     string
	MySQLDatabase                 string
	MySQLUsername                 string
	MySQLPassword                 string
	MySQLSSL                      string
	MySQLSSLVerifyServerCert      string
	MySQLSSLCA                    string
	MySQLSSLCert                  string
	MySQLSSLKey                   string
	DisplayName                   string
	Cron                          string
	RcloneRemoteName              string
	RcloneRemoteDir               string
	RcloneRemoteList              []string
	RcloneGlobalFlag              string
	ZipEnable                     bool
	ZipPassword                   string
	ZipType                       string
	BackupKeepDays                int
	BackupFileDate                string
	BackupFileDateFormat          string
	Timezone                      string
	PingURL                       string
	PingURLCurlOptions            string
	PingURLWhenStart              string
	PingURLWhenStartCurlOptions   string
	PingURLWhenSuccess            string
	PingURLWhenSuccessCurlOptions string
	PingURLWhenFailure            string
	PingURLWhenFailureCurlOptions string
	MailSMTPEnable                bool
	MailTo                        string
	MailSMTPVariables             string
	MailWhenSuccess               bool
	MailWhenFailure               bool
	MailUseThread                 bool
	MailParentMessageID           string
}

// LoadConfig reads the environment (including the optional /.env file) and
// applies the same defaults and normalization as init_env in includes.sh.
func LoadConfig() (*Config, error) {
	loadDotEnv(EnvFile)

	c := &Config{}
	c.initDir()
	if err := checkDirExist(c.DataDir); err != nil {
		return nil, err
	}
	c.initDB()
	c.initDisplay()
	c.initPing()
	c.initMail()
	c.initRclone()
	c.initBackup()
	c.initTimezone()

	c.printSummary()
	return c, nil
}

func (c *Config) initDir() {
	c.DataDir = defaultString(getEnv("DATA_DIR"), "/bitwarden/data")
	c.DataDB = defaultString(getEnv("DATA_DB"), filepath.Join(c.DataDir, "db.sqlite3"))
	c.DataConfig = filepath.Join(c.DataDir, "config.json")

	c.DataRSAKey = defaultString(getEnv("DATA_RSAKEY"), filepath.Join(c.DataDir, "rsa_key"))
	c.DataRSAKeyDirname = filepath.Dir(c.DataRSAKey)
	c.DataRSAKeyBasename = filepath.Base(c.DataRSAKey)

	attachments := defaultString(getEnv("DATA_ATTACHMENTS"), filepath.Join(c.DataDir, "attachments"))
	c.DataAttachments = filepath.Dir(filepath.Join(attachments, "useless"))
	c.DataAttachmentsDirname = filepath.Dir(c.DataAttachments)
	c.DataAttachmentsBasename = filepath.Base(c.DataAttachments)

	sends := defaultString(getEnv("DATA_SENDS"), filepath.Join(c.DataDir, "sends"))
	c.DataSends = filepath.Dir(filepath.Join(sends, "useless"))
	c.DataSendsDirname = filepath.Dir(c.DataSends)
	c.DataSendsBasename = filepath.Base(c.DataSends)
}

func (c *Config) initDB() {
	switch strings.ToUpper(getEnv("DB_TYPE")) {
	case "POSTGRESQL":
		c.DBType = "POSTGRESQL"
		c.PGHost = getEnv("PG_HOST")
		c.PGPort = defaultString(getEnv("PG_PORT"), "5432")
		c.PGDBName = defaultString(getEnv("PG_DBNAME"), "vaultwarden")
		c.PGUsername = defaultString(getEnv("PG_USERNAME"), "vaultwarden")
		c.PGPassword = getEnv("PG_PASSWORD")
	case "MYSQL":
		c.DBType = "MYSQL"
		c.MySQLHost = getEnv("MYSQL_HOST")
		c.MySQLPort = defaultString(getEnv("MYSQL_PORT"), "3306")
		c.MySQLDatabase = defaultString(getEnv("MYSQL_DATABASE"), "vaultwarden")
		c.MySQLUsername = defaultString(getEnv("MYSQL_USERNAME"), "vaultwarden")
		c.MySQLPassword = getEnv("MYSQL_PASSWORD")
		c.MySQLSSL = getEnv("MYSQL_SSL")
		c.MySQLSSLVerifyServerCert = getEnv("MYSQL_SSL_VERIFY_SERVER_CERT")
		c.MySQLSSLCA = getEnv("MYSQL_SSL_CA")
		c.MySQLSSLCert = getEnv("MYSQL_SSL_CERT")
		c.MySQLSSLKey = getEnv("MYSQL_SSL_KEY")
	default:
		c.DBType = "SQLITE"
	}
}

func (c *Config) initDisplay() {
	c.DisplayName = defaultString(getEnv("DISPLAY_NAME"), "vaultwarden")
}

func (c *Config) initPing() {
	c.PingURL = getEnv("PING_URL")
	c.PingURLCurlOptions = getEnv("PING_URL_CURL_OPTIONS")
	c.PingURLWhenStart = getEnv("PING_URL_WHEN_START")
	c.PingURLWhenStartCurlOptions = getEnv("PING_URL_WHEN_START_CURL_OPTIONS")
	c.PingURLWhenSuccess = getEnv("PING_URL_WHEN_SUCCESS")
	c.PingURLWhenSuccessCurlOptions = getEnv("PING_URL_WHEN_SUCCESS_CURL_OPTIONS")
	c.PingURLWhenFailure = getEnv("PING_URL_WHEN_FAILURE")
	c.PingURLWhenFailureCurlOptions = getEnv("PING_URL_WHEN_FAILURE_CURL_OPTIONS")
}

func (c *Config) initMail() {
	c.MailSMTPEnable = strings.EqualFold(getEnv("MAIL_SMTP_ENABLE"), "TRUE") && getEnv("MAIL_TO") != ""
	c.MailTo = getEnv("MAIL_TO")
	c.MailSMTPVariables = getEnv("MAIL_SMTP_VARIABLES")
	c.MailWhenSuccess = !strings.EqualFold(getEnv("MAIL_WHEN_SUCCESS"), "FALSE")
	c.MailWhenFailure = !strings.EqualFold(getEnv("MAIL_WHEN_FAILURE"), "FALSE")

	forceThread := getEnv("MAIL_FORCE_THREAD")
	if strings.EqualFold(forceThread, "TRUE") {
		c.MailUseThread = true
		if info, err := os.Stat(MailParentMessageIDFile); err == nil && info.Size() > 0 {
			content, _ := os.ReadFile(MailParentMessageIDFile)
			c.MailParentMessageID = strings.TrimRight(string(content), "\r\n")
		}
	} else {
		c.MailParentMessageID = forceThread
		c.MailUseThread = false
		_ = os.RemoveAll(MailConfigDir)
	}

	if messageIDPattern.MatchString(c.MailParentMessageID) {
		c.MailUseThread = true
	} else {
		c.MailParentMessageID = ""
	}
}

func (c *Config) initRclone() {
	c.Cron = defaultString(getEnv("CRON"), "5 * * * *")

	c.RcloneRemoteName = defaultString(getEnv("RCLONE_REMOTE_NAME"), "BitwardenBackup")
	c.RcloneRemoteDir = defaultString(getEnv("RCLONE_REMOTE_DIR"), "/BitwardenBackup/")
	_ = os.Setenv("RCLONE_REMOTE_NAME_0", c.RcloneRemoteName)
	_ = os.Setenv("RCLONE_REMOTE_DIR_0", c.RcloneRemoteDir)

	c.RcloneRemoteList = nil
	for i := 0; ; i++ {
		name := getEnv(fmt.Sprintf("RCLONE_REMOTE_NAME_%d", i))
		dir := getEnv(fmt.Sprintf("RCLONE_REMOTE_DIR_%d", i))
		if name == "" || dir == "" {
			break
		}
		remote := strings.TrimRight(name+":"+dir, "/")
		c.RcloneRemoteList = append(c.RcloneRemoteList, remote)
	}

	c.RcloneGlobalFlag = getEnv("RCLONE_GLOBAL_FLAG")
}

func (c *Config) initBackup() {
	c.ZipEnable = !strings.EqualFold(getEnv("ZIP_ENABLE"), "FALSE")
	c.ZipPassword = defaultString(getEnv("ZIP_PASSWORD"), "WHEREISMYPASSWORD?")

	c.ZipType = "zip"
	if strings.EqualFold(getEnv("ZIP_TYPE"), "7z") {
		c.ZipType = "7z"
	}

	c.BackupKeepDays = 0
	if value := getEnv("BACKUP_KEEP_DAYS"); value != "" {
		if days, err := strconv.Atoi(value); err == nil {
			c.BackupKeepDays = days
		}
	}

	fileDate := defaultString(getEnv("BACKUP_FILE_DATE"), "%Y%m%d") + getEnv("BACKUP_FILE_DATE_SUFFIX")
	c.BackupFileDate = sanitizeBackupFileDate(fileDate)
	c.BackupFileDateFormat = strings.ReplaceAll(defaultString(getEnv("BACKUP_FILE_SUFFIX"), c.BackupFileDate), "/", "")
}

func (c *Config) initTimezone() {
	timezone := getEnv("TIMEZONE")
	if !validTimezone(timezone) {
		timezone = "UTC"
	}
	c.Timezone = timezone

	if location, err := time.LoadLocation(timezone); err == nil {
		time.Local = location
		_ = os.Setenv("TZ", timezone)
	}
}

// ConfigurePostgresql writes ~/.pgpass, matching configure_postgresql in includes.sh.
func (c *Config) ConfigurePostgresql() error {
	if c.DBType != "POSTGRESQL" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s:%s:%s:%s:%s\n", c.PGHost, c.PGPort, c.PGDBName, c.PGUsername, c.PGPassword)
	return os.WriteFile(filepath.Join(home, ".pgpass"), []byte(content), 0o600)
}

func (c *Config) printSummary() {
	PrintColor(Yellow, "========================================")
	PrintColor(Yellow, "DATA_DIR: "+c.DataDir)
	PrintColor(Yellow, "DATA_CONFIG: "+c.DataConfig)
	PrintColor(Yellow, "DATA_RSAKEY: "+c.DataRSAKey)
	PrintColor(Yellow, "DATA_ATTACHMENTS: "+c.DataAttachments)
	PrintColor(Yellow, "DATA_SENDS: "+c.DataSends)
	PrintColor(Yellow, "========================================")
	PrintColor(Yellow, "DB_TYPE: "+c.DBType)

	switch c.DBType {
	case "POSTGRESQL":
		PrintColor(Yellow, fmt.Sprintf("DB_URL: postgresql://%s:***(%d Chars)@%s:%s/%s",
			c.PGUsername, utf8.RuneCountInString(c.PGPassword), c.PGHost, c.PGPort, c.PGDBName))
	case "MYSQL":
		PrintColor(Yellow, fmt.Sprintf("DB_URL: mysql://%s:***(%d Chars)@%s:%s/%s",
			c.MySQLUsername, utf8.RuneCountInString(c.MySQLPassword), c.MySQLHost, c.MySQLPort, c.MySQLDatabase))
	default:
		PrintColor(Yellow, "DATA_DB: "+c.DataDB)
	}

	PrintColor(Yellow, "========================================")
	PrintColor(Yellow, "CRON: "+c.Cron)
	for _, remote := range c.RcloneRemoteList {
		PrintColor(Yellow, "RCLONE_REMOTE: "+remote)
	}
	PrintColor(Yellow, "RCLONE_GLOBAL_FLAG: "+c.RcloneGlobalFlag)
	PrintColor(Yellow, "ZIP_ENABLE: "+boolString(c.ZipEnable))
	PrintColor(Yellow, fmt.Sprintf("ZIP_PASSWORD: %d Chars", utf8.RuneCountInString(c.ZipPassword)))
	PrintColor(Yellow, "ZIP_TYPE: "+c.ZipType)
	PrintColor(Yellow, fmt.Sprintf("BACKUP_FILE_DATE_FORMAT: %s (example \"[filename].%s.[ext]\")",
		c.BackupFileDateFormat, strftime(c.BackupFileDateFormat, time.Now())))
	PrintColor(Yellow, fmt.Sprintf("BACKUP_KEEP_DAYS: %d", c.BackupKeepDays))

	if c.PingURL != "" {
		PrintColor(Yellow, fmt.Sprintf("PING_URL: curl%s \"%s\"", curlOptionsSuffix(c.PingURLCurlOptions), c.PingURL))
	}
	if c.PingURLWhenStart != "" {
		PrintColor(Yellow, fmt.Sprintf("PING_URL_WHEN_START: curl%s \"%s\"", curlOptionsSuffix(c.PingURLWhenStartCurlOptions), c.PingURLWhenStart))
	}
	if c.PingURLWhenSuccess != "" {
		PrintColor(Yellow, fmt.Sprintf("PING_URL_WHEN_SUCCESS: curl%s \"%s\"", curlOptionsSuffix(c.PingURLWhenSuccessCurlOptions), c.PingURLWhenSuccess))
	}
	if c.PingURLWhenFailure != "" {
		PrintColor(Yellow, fmt.Sprintf("PING_URL_WHEN_FAILURE: curl%s \"%s\"", curlOptionsSuffix(c.PingURLWhenFailureCurlOptions), c.PingURLWhenFailure))
	}

	PrintColor(Yellow, "MAIL_SMTP_ENABLE: "+boolString(c.MailSMTPEnable))
	if c.MailSMTPEnable {
		PrintColor(Yellow, "MAIL_TO: "+c.MailTo)
		PrintColor(Yellow, "MAIL_WHEN_SUCCESS: "+boolString(c.MailWhenSuccess))
		PrintColor(Yellow, "MAIL_WHEN_FAILURE: "+boolString(c.MailWhenFailure))
		if c.MailUseThread {
			if c.MailParentMessageID != "" {
				PrintColor(Yellow, "MAIL_PARENT_MESSAGE_ID: "+c.MailParentMessageID)
			} else {
				PrintColor(Yellow, "MAIL_MESSAGE_ID: auto generate")
			}
		}
	}
	PrintColor(Yellow, "TIMEZONE: "+c.Timezone)
	PrintColor(Yellow, "DISPLAY_NAME: "+c.DisplayName)
	PrintColor(Yellow, "========================================")
}

// loadDotEnv imports /.env variables with the DOTENV_ prefix, as export_env_file does.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
			if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
				value = value[1 : end+1]
			}
		} else if end := strings.Index(value, " #"); end >= 0 {
			value = strings.TrimSpace(value[:end])
		}

		_ = os.Setenv("DOTENV_"+key, value)
	}
}

// getEnv implements the same priority as get_env in includes.sh.
func getEnv(name string) string {
	var value string

	switch {
	case os.Getenv(name) != "":
		value = os.Getenv(name)
	case os.Getenv(name+"_FILE") != "":
		value = readSecret(os.Getenv(name + "_FILE"))
	case os.Getenv("DOTENV_"+name+"_FILE") != "":
		value = readSecret(os.Getenv("DOTENV_" + name + "_FILE"))
	case os.Getenv("DOTENV_"+name) != "":
		value = os.Getenv("DOTENV_" + name)
	}

	_ = os.Setenv(name, value)
	return value
}

func readSecret(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cat: %s: %v\n", path, err)
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "TRUE"
	}
	return "FALSE"
}

func checkDirExist(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("cannot access %s: No such directory", path)
	}
	return nil
}

func validTimezone(name string) bool {
	if name == "" || strings.Contains(name, "..") {
		return false
	}
	info, err := os.Stat(filepath.Join("/usr/share/zoneinfo", name))
	return err == nil && !info.IsDir()
}

func sanitizeBackupFileDate(value string) string {
	var result strings.Builder
	for _, char := range value {
		if (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			char == '%' || char == '_' || char == '-' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func curlOptionsSuffix(options string) string {
	if options == "" {
		return ""
	}
	return " " + options
}
