package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// LoadMailConfig loads only the variables needed by the mail test command.
func LoadMailConfig() *Config {
	loadDotEnv(EnvFile)

	c := &Config{}
	c.initDisplay()
	c.initMail()
	return c
}

// LoadPingConfig loads only the variables needed by the ping test command.
func LoadPingConfig() *Config {
	loadDotEnv(EnvFile)

	c := &Config{}
	c.initDisplay()
	c.initPing()
	return c
}

// LoadRestoreConfig loads the directory and database settings used by restore.
func LoadRestoreConfig() (*Config, error) {
	c := &Config{}
	c.initDir()
	if err := checkDirExist(c.DataDir); err != nil {
		return nil, err
	}
	c.initDB()
	return c, nil
}

// ConfigureTimezone points LOCALTIME_FILE at the configured timezone.
func ConfigureTimezone(c *Config) error {
	path := os.Getenv("LOCALTIME_FILE")
	if path == "" {
		path = "/tmp/localtime"
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(filepath.Join("/usr/share/zoneinfo", c.Timezone), path)
}

// ConfigureCron writes the supercronic crontab, replacing entries from older versions.
func ConfigureCron(c *Config) error {
	path, err := cronConfigFile()
	if err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" ||
				strings.Contains(line, "backup.sh") ||
				strings.Contains(line, "/app/backup") ||
				strings.Contains(line, executable) {
				continue
			}
			lines = append(lines, line)
		}
	}

	lines = append(lines, fmt.Sprintf("%s %s backup --from-cron", c.Cron, executable))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// ExecSupercronic replaces the current process with supercronic.
func ExecSupercronic() error {
	path, err := cronConfigFile()
	if err != nil {
		return err
	}

	executable, err := exec.LookPath("supercronic")
	if err != nil {
		return err
	}

	return syscall.Exec(executable, []string{"supercronic", "-passthrough-logs", "-quiet", path}, os.Environ())
}

func cronConfigFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "crontabs"), nil
}
