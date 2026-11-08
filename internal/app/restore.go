package app

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	RestoreDir        = "/bitwarden/restore"
	RestoreExtractDir = "/bitwarden/extract"
)

// RestoreOptions holds the flags accepted by the restore command.
type RestoreOptions struct {
	Password        string
	ZipFile         string
	DBFile          string
	ConfigFile      string
	RSAKeyFile      string
	AttachmentsFile string
	SendsFile       string
	Force           bool
}

// Restore restores a backup from RESTORE_DIR.
func Restore(config *Config, options RestoreOptions) error {
	if err := config.ConfigurePostgresql(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	options.ZipFile = baseNameIfSet(options.ZipFile)
	options.DBFile = baseNameIfSet(options.DBFile)
	options.ConfigFile = baseNameIfSet(options.ConfigFile)
	options.RSAKeyFile = baseNameIfSet(options.RSAKeyFile)
	options.AttachmentsFile = baseNameIfSet(options.AttachmentsFile)
	options.SendsFile = baseNameIfSet(options.SendsFile)

	if options.ZipFile == "" && options.DBFile == "" && options.ConfigFile == "" &&
		options.RSAKeyFile == "" && options.AttachmentsFile == "" && options.SendsFile == "" {
		PrintColor(Yellow, "Empty input")
		PrintColor(None, "")
		PrintColor(None, "Find out more at https://github.com/ttionya/vaultwarden-backup#restore")
		return nil
	}

	if !options.Force {
		PrintColor(Yellow, "Restore will overwrite the existing files, continue? (y/N)")
		fmt.Print("(Default: n): ")

		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.ToUpper(strings.TrimSpace(line)) != "Y" {
			return nil
		}
	}

	return restoreFile(config, options, RestoreDir)
}

func restoreFile(config *Config, options RestoreOptions, dir string) error {
	if options.ZipFile != "" {
		return restoreZip(config, options)
	}

	if options.DBFile != "" {
		if err := checkRestoreFile(dir, options.DBFile, "--db-file"); err != nil {
			return err
		}
		if err := restoreDB(config, filepath.Join(dir, options.DBFile)); err != nil {
			return err
		}
	}

	if options.ConfigFile != "" {
		if err := checkRestoreFile(dir, options.ConfigFile, "--config-file"); err != nil {
			return err
		}
		if err := restoreConfig(config, filepath.Join(dir, options.ConfigFile)); err != nil {
			return err
		}
	}

	if options.RSAKeyFile != "" {
		if err := checkRestoreFile(dir, options.RSAKeyFile, "--rsakey-file"); err != nil {
			return err
		}
		if err := restoreRSAKey(config, filepath.Join(dir, options.RSAKeyFile)); err != nil {
			return err
		}
	}

	if options.AttachmentsFile != "" {
		if err := checkRestoreFile(dir, options.AttachmentsFile, "--attachments-file"); err != nil {
			return err
		}
		if err := restoreDirectory(config.DataAttachments, filepath.Join(dir, options.AttachmentsFile), "attachments"); err != nil {
			return err
		}
	}

	if options.SendsFile != "" {
		if err := checkRestoreFile(dir, options.SendsFile, "--sends-file"); err != nil {
			return err
		}
		if err := restoreDirectory(config.DataSends, filepath.Join(dir, options.SendsFile), "sends"); err != nil {
			return err
		}
	}

	return nil
}

func restoreZip(config *Config, options RestoreOptions) error {
	PrintColor(Blue, "restore vaultwarden backup zip file")

	if err := checkRestoreFile(RestoreDir, options.ZipFile, "--zip-file"); err != nil {
		return err
	}
	zipPath := filepath.Join(RestoreDir, options.ZipFile)

	clearExtractDir()
	defer clearExtractDir()

	args := []string{"e", "-aoa"}
	if options.Password != "" {
		args = append(args, "-p"+options.Password)
	}
	args = append(args, "-o"+RestoreExtractDir, zipPath)

	if err := runCommand("7z", args...); err != nil {
		return &Fatal{Message: "extract vaultwarden backup zip file failed"}
	}
	PrintColor(Green, "extract vaultwarden backup zip file successful")

	options.DBFile = firstGlobMatch(filepath.Join(RestoreExtractDir, "db.*.*"))
	options.ConfigFile = firstGlobMatch(filepath.Join(RestoreExtractDir, "config.*.json"))
	options.RSAKeyFile = firstGlobMatch(filepath.Join(RestoreExtractDir, "rsakey.*.tar"))
	options.AttachmentsFile = firstGlobMatch(filepath.Join(RestoreExtractDir, "attachments.*.tar"))
	options.SendsFile = firstGlobMatch(filepath.Join(RestoreExtractDir, "sends.*.tar"))
	options.ZipFile = ""

	return restoreFile(config, options, RestoreExtractDir)
}

func restoreDB(config *Config, path string) error {
	switch config.DBType {
	case "POSTGRESQL":
		PrintColor(Blue, "restore vaultwarden postgresql database")

		err := runCommand("pg_restore",
			"-h", config.PGHost,
			"-p", config.PGPort,
			"-d", config.PGDBName,
			"-U", config.PGUsername,
			"-c", path,
		)
		if err != nil {
			return &Fatal{Message: "restore vaultwarden postgresql database failed"}
		}
		PrintColor(Green, "restore vaultwarden postgresql database successful")
		return nil
	case "MYSQL":
		PrintColor(Blue, "restore vaultwarden mysql database")

		args := []string{
			"-h", config.MySQLHost,
			"-P", config.MySQLPort,
			"-u", config.MySQLUsername,
			"-p" + config.MySQLPassword,
			"--show-warnings",
		}
		if config.MySQLSSL != "" {
			args = append(args, "--ssl="+config.MySQLSSL)
		}
		if config.MySQLSSLVerifyServerCert != "" {
			args = append(args, "--ssl-verify-server-cert="+config.MySQLSSLVerifyServerCert)
		}
		if config.MySQLSSLCA != "" {
			args = append(args, "--ssl-ca="+config.MySQLSSLCA)
		}
		if config.MySQLSSLCert != "" {
			args = append(args, "--ssl-cert="+config.MySQLSSLCert)
		}
		if config.MySQLSSLKey != "" {
			args = append(args, "--ssl-key="+config.MySQLSSLKey)
		}
		args = append(args, config.MySQLDatabase)

		input, err := os.Open(path)
		if err == nil {
			command := newCommand("mariadb", args...)
			command.Stdin = input
			err = command.Run()
			_ = input.Close()
		}
		if err != nil {
			return &Fatal{Message: "restore vaultwarden mysql database failed"}
		}
		PrintColor(Green, "restore vaultwarden mysql database successful")
		return nil
	default:
		PrintColor(Blue, "restore vaultwarden sqlite database")

		if err := copyFile(path, config.DataDB); err != nil {
			return &Fatal{Message: "restore vaultwarden sqlite database failed"}
		}
		PrintColor(Green, "restore vaultwarden sqlite database successful")
		return nil
	}
}

func restoreConfig(config *Config, path string) error {
	PrintColor(Blue, "restore vaultwarden config")

	if err := copyFile(path, config.DataConfig); err != nil {
		return &Fatal{Message: "restore vaultwarden config failed"}
	}
	PrintColor(Green, "restore vaultwarden config successful")
	return nil
}

func restoreRSAKey(config *Config, path string) error {
	PrintColor(Blue, "restore vaultwarden rsakey")

	if err := os.MkdirAll(config.DataRSAKeyDirname, 0o755); err != nil {
		return &Fatal{Message: "restore vaultwarden rsakey failed"}
	}
	if err := runCommand("tar", "-x", "-C", config.DataRSAKeyDirname, "-f", path); err != nil {
		return &Fatal{Message: "restore vaultwarden rsakey failed"}
	}
	PrintColor(Green, "restore vaultwarden rsakey successful")
	return nil
}

func restoreDirectory(target, archive, name string) error {
	PrintColor(Blue, "restore vaultwarden "+name)

	root, err := tarFirstEntry(archive)
	if err != nil || root == "" {
		return &Fatal{Message: "restore vaultwarden " + name + " failed"}
	}

	extractDir := target + ".extract"
	_ = os.RemoveAll(target)
	_ = os.RemoveAll(extractDir)

	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return &Fatal{Message: "restore vaultwarden " + name + " failed"}
	}
	defer os.RemoveAll(extractDir)

	if err := runCommand("tar", "-x", "-C", extractDir, "-f", archive); err != nil {
		return &Fatal{Message: "restore vaultwarden " + name + " failed"}
	}
	if err := os.Rename(filepath.Join(extractDir, root), target); err != nil {
		return &Fatal{Message: "restore vaultwarden " + name + " failed"}
	}

	PrintColor(Green, "restore vaultwarden "+name+" successful")
	return nil
}

func checkRestoreFile(dir, name, flag string) error {
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return &Fatal{Message: fmt.Sprintf("%s: cannot access %s: No such file", flag, name)}
	}
	return nil
}

func tarFirstEntry(path string) (string, error) {
	output, err := exec.Command("tar", "-tf", path).Output()
	if err != nil {
		return "", err
	}

	line := strings.SplitN(string(output), "\n", 2)[0]
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}
	return filepath.Base(line), nil
}

func clearExtractDir() {
	_ = os.RemoveAll(RestoreExtractDir)
}

func firstGlobMatch(pattern string) string {
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return ""
	}
	return filepath.Base(matches[0])
}

func baseNameIfSet(value string) string {
	if value == "" {
		return ""
	}
	return filepath.Base(value)
}
