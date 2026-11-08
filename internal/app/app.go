package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Fatal is an error that terminates the backup with an optional failure notification.
type Fatal struct {
	Message string
	Notify  string
}

func (f *Fatal) Error() string {
	if f.Message != "" {
		return f.Message
	}
	return "fatal error"
}

// BackupFiles holds the names of the files generated during a backup run.
type BackupFiles struct {
	DBSQLite     string
	DBPostgresql string
	DBMySQL      string
	Config       string
	RSAKey       string
	Attachments  string
	Sends        string
	Zip          string
}

// App runs the backup workflow.
type App struct {
	Config     *Config
	Notifier   *Notifier
	Files      BackupFiles
	UploadFile string
}

func NewApp() *App {
	return &App{}
}

// Run executes the same steps as the original backup.sh entry point.
func (a *App) Run() error {
	PrintColor(Blue, "running the backup program at "+timestamp())

	config, err := LoadConfig()
	if err != nil {
		return &Fatal{Message: err.Error()}
	}
	a.Config = config
	a.Notifier = NewNotifier(config)

	if err := config.ConfigurePostgresql(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	a.Notifier.Send("start", "Start backup at "+timestamp())

	if err := a.CheckRcloneConnection("any"); err != nil {
		return err
	}

	clearDir()
	a.initFiles()

	if err := a.Backup(); err != nil {
		return err
	}
	if err := a.Package(); err != nil {
		return err
	}
	if err := a.Upload(); err != nil {
		return err
	}

	clearDir()
	a.ClearHistory()

	a.Notifier.Send("success", "The file was successfully uploaded at "+timestamp()+".")
	PrintColor(None, "")

	return nil
}

func (a *App) initFiles() {
	suffix := strftime(a.Config.BackupFileDateFormat, time.Now())

	a.Files = BackupFiles{
		DBSQLite:     filepath.Join(BackupDir, "db."+suffix+".sqlite3"),
		DBPostgresql: filepath.Join(BackupDir, "db."+suffix+".dump"),
		DBMySQL:      filepath.Join(BackupDir, "db."+suffix+".sql"),
		Config:       filepath.Join(BackupDir, "config."+suffix+".json"),
		RSAKey:       filepath.Join(BackupDir, "rsakey."+suffix+".tar"),
		Attachments:  filepath.Join(BackupDir, "attachments."+suffix+".tar"),
		Sends:        filepath.Join(BackupDir, "sends."+suffix+".tar"),
		Zip:          filepath.Join(BackupDir, "backup."+suffix+"."+a.Config.ZipType),
	}
}

// Backup creates the database dump and copies the remaining vaultwarden data.
func (a *App) Backup() error {
	if err := os.MkdirAll(BackupDir, 0o755); err != nil {
		return &Fatal{Message: err.Error()}
	}

	switch a.Config.DBType {
	case "POSTGRESQL":
		if err := a.backupPostgresql(); err != nil {
			return err
		}
	case "MYSQL":
		if err := a.backupMySQL(); err != nil {
			return err
		}
	default:
		if err := a.backupSQLite(); err != nil {
			return err
		}
	}

	a.backupConfig()
	a.backupRSAKey()
	a.backupAttachments()
	a.backupSends()

	_ = runCommand("ls", "-lah", BackupDir)
	return nil
}

func (a *App) backupSQLite() error {
	PrintColor(Blue, "backup vaultwarden sqlite database")

	if info, err := os.Stat(a.Config.DataDB); err != nil || info.IsDir() {
		return &Fatal{Message: fmt.Sprintf("cannot access %s: No such file", a.Config.DataDB)}
	}

	backupCommand := fmt.Sprintf(".backup '%s'", strings.ReplaceAll(a.Files.DBSQLite, "'", "''"))
	if err := runCommand("sqlite3", a.Config.DataDB, backupCommand); err != nil {
		return &Fatal{
			Message: "backup vaultwarden sqlite database failed",
			Notify:  fmt.Sprintf("Backup failed at %s. Reason: Backup sqlite database failed.", timestamp()),
		}
	}

	return nil
}

func (a *App) backupPostgresql() error {
	PrintColor(Blue, "backup vaultwarden postgresql database")

	err := runCommand("pg_dump",
		"-Fc",
		"-h", a.Config.PGHost,
		"-p", a.Config.PGPort,
		"-d", a.Config.PGDBName,
		"-U", a.Config.PGUsername,
		"-f", a.Files.DBPostgresql,
	)
	if err != nil {
		return &Fatal{
			Message: "backup vaultwarden postgresql database failed",
			Notify:  fmt.Sprintf("Backup failed at %s. Reason: Backup postgresql database failed.", timestamp()),
		}
	}

	return nil
}

func (a *App) backupMySQL() error {
	PrintColor(Blue, "backup vaultwarden mysql database")

	args := []string{
		"-h", a.Config.MySQLHost,
		"-P", a.Config.MySQLPort,
		"-u", a.Config.MySQLUsername,
		"-p" + a.Config.MySQLPassword,
	}
	if a.Config.MySQLSSL != "" {
		args = append(args, "--ssl="+a.Config.MySQLSSL)
	}
	if a.Config.MySQLSSLVerifyServerCert != "" {
		args = append(args, "--ssl-verify-server-cert="+a.Config.MySQLSSLVerifyServerCert)
	}
	if a.Config.MySQLSSLCA != "" {
		args = append(args, "--ssl-ca="+a.Config.MySQLSSLCA)
	}
	if a.Config.MySQLSSLCert != "" {
		args = append(args, "--ssl-cert="+a.Config.MySQLSSLCert)
	}
	if a.Config.MySQLSSLKey != "" {
		args = append(args, "--ssl-key="+a.Config.MySQLSSLKey)
	}
	args = append(args, a.Config.MySQLDatabase)

	output, err := os.Create(a.Files.DBMySQL)
	if err == nil {
		command := newCommand("mariadb-dump", args...)
		command.Stdout = output
		command.Stderr = os.Stderr
		err = command.Run()
		_ = output.Close()
	}

	if err != nil {
		return &Fatal{
			Message: "backup vaultwarden mysql database failed",
			Notify:  fmt.Sprintf("Backup failed at %s. Reason: Backup mysql database failed.", timestamp()),
		}
	}

	return nil
}

func (a *App) backupConfig() {
	PrintColor(Blue, "backup vaultwarden config")

	info, err := os.Stat(a.Config.DataConfig)
	if err != nil || !info.Mode().IsRegular() {
		PrintColor(Yellow, "not found vaultwarden config, skipping")
		return
	}

	if err := copyFile(a.Config.DataConfig, a.Files.Config); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (a *App) backupRSAKey() {
	PrintColor(Blue, "backup vaultwarden rsakey")

	names := findMatchingFiles(a.Config.DataRSAKeyDirname, a.Config.DataRSAKeyBasename)
	if len(names) == 0 {
		PrintColor(Yellow, "not found vaultwarden rsakey, skipping")
		return
	}

	if err := tarCreate(a.Files.RSAKey, a.Config.DataRSAKeyDirname, names); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	PrintColor(Blue, "display rsakey tar file list")
	_ = runCommand("tar", "-tf", a.Files.RSAKey)
}

func (a *App) backupAttachments() {
	PrintColor(Blue, "backup vaultwarden attachments")

	info, err := os.Stat(a.Config.DataAttachments)
	if err != nil || !info.IsDir() {
		PrintColor(Yellow, "not found vaultwarden attachments directory, skipping")
		return
	}

	if err := tarCreate(a.Files.Attachments, a.Config.DataAttachmentsDirname, []string{a.Config.DataAttachmentsBasename}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	PrintColor(Blue, "display attachments tar file list")
	_ = runCommand("tar", "-tf", a.Files.Attachments)
}

func (a *App) backupSends() {
	PrintColor(Blue, "backup vaultwarden sends")

	info, err := os.Stat(a.Config.DataSends)
	if err != nil || !info.IsDir() {
		PrintColor(Yellow, "not found vaultwarden sends directory, skipping")
		return
	}

	if err := tarCreate(a.Files.Sends, a.Config.DataSendsDirname, []string{a.Config.DataSendsBasename}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	PrintColor(Blue, "display sends tar file list")
	_ = runCommand("tar", "-tf", a.Files.Sends)
}

// Package creates the encrypted archive, or selects the backup directory for upload.
func (a *App) Package() error {
	if !a.Config.ZipEnable {
		PrintColor(Yellow, "skip package backup files")
		a.UploadFile = BackupDir
		return nil
	}

	PrintColor(Blue, "package backup file")
	a.UploadFile = a.Files.Zip

	files := make([]string, 0, 8)
	entries, _ := os.ReadDir(BackupDir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		files = append(files, filepath.Join(BackupDir, entry.Name()))
	}

	var args []string
	if a.Config.ZipType == "zip" {
		args = []string{"a", "-tzip", "-mx=9", "-p" + a.Config.ZipPassword, a.Files.Zip}
	} else {
		args = []string{"a", "-t7z", "-m0=lzma2", "-mx=9", "-mfb=64", "-md=32m", "-ms=on", "-mhe=on", "-p" + a.Config.ZipPassword, a.Files.Zip}
	}
	args = append(args, files...)

	if err := runCommand("7z", args...); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	_ = runCommand("ls", "-lah", BackupDir)

	PrintColor(Blue, "display backup "+a.Config.ZipType+" file list")
	_ = runCommand("7z", "l", "-p"+a.Config.ZipPassword, a.Files.Zip)

	return nil
}

func clearDir() {
	_ = os.RemoveAll(BackupDir)
}

func findMatchingFiles(dir, basename string) []string {
	pattern := basename + "*"
	seen := make(map[string]bool)
	var names []string

	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		matched, matchErr := filepath.Match(pattern, entry.Name())
		if matchErr != nil {
			matched = strings.HasPrefix(entry.Name(), basename)
		}
		if !matched {
			return nil
		}

		name := filepath.Base(path)
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
		return nil
	})

	return names
}

func tarCreate(archive, dir string, names []string) error {
	command := newCommand("tar", "-c", "-C", dir, "-f", archive, "-T", "-")
	command.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	return command.Run()
}

func copyFile(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err = input.WriteTo(output); err != nil {
		_ = output.Close()
		return err
	}

	return output.Close()
}
