package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CheckRcloneConnection mirrors check_rclone_connection from includes.sh.
func (a *App) CheckRcloneConnection(strategy string) error {
	if err := a.rcloneDiscardingOutput("version"); err != nil {
		PrintColor(Red, "illegal rclone global flags")
		PrintColor(Blue, "Please check https://rclone.org/flags/")
		return &Fatal{}
	}

	output, _ := a.rcloneCombinedOutput("config", "show")
	if !strings.Contains(output, "["+a.Config.RcloneRemoteName+"]") {
		PrintColor(Red, "rclone configuration information not found")
		PrintColor(Blue, "Please configure rclone first, check https://github.com/ttionya/vaultwarden-backup/blob/master/README.md#backup")
		return &Fatal{}
	}

	errorCount := 0
	for _, remote := range a.Config.RcloneRemoteList {
		if err := a.rcloneDiscardingOutput("lsd", remote); err != nil {
			PrintColor(Red, "storage system connection may not be initialized, try initializing "+Colorize(Yellow, "["+remote+"]"))

			if err := a.runRclone("mkdir", remote); err != nil {
				PrintColor(Red, "storage system connection failure "+Colorize(Yellow, "["+remote+"]"))
				errorCount++
			}
		}
	}

	if errorCount == 0 {
		return nil
	}

	switch strategy {
	case "all":
		PrintColor(Red, "storage system connection failure exists")
		return &Fatal{}
	case "any":
		if errorCount == len(a.Config.RcloneRemoteList) {
			PrintColor(Red, "all storage system connections failed")
			return &Fatal{}
		}
		PrintColor(Yellow, "some storage system connections failed, but the backup will continue")
	}

	return nil
}

// Upload copies the backup to every configured rclone remote.
func (a *App) Upload() error {
	if _, err := os.Stat(a.UploadFile); err != nil {
		PrintColor(Red, "upload file not found")
		return &Fatal{
			Notify: fmt.Sprintf("File upload failed at %s. Reason: Upload file not found.", timestamp()),
		}
	}

	hasError := false
	for _, remote := range a.Config.RcloneRemoteList {
		PrintColor(Blue, "upload backup file to storage system "+Colorize(Yellow, "["+remote+"]"))

		if err := a.runRclone("copy", a.UploadFile, remote); err != nil {
			PrintColor(Red, "upload failed")
			hasError = true
		}
	}

	if hasError {
		return &Fatal{
			Notify: fmt.Sprintf("File upload failed at %s.", timestamp()),
		}
	}

	return nil
}

// ClearHistory deletes remote backup files older than BACKUP_KEEP_DAYS.
func (a *App) ClearHistory() {
	if a.Config.BackupKeepDays <= 0 {
		return
	}

	for _, remote := range a.Config.RcloneRemoteList {
		PrintColor(Blue, fmt.Sprintf("delete %d days ago backup files ", a.Config.BackupKeepDays)+Colorize(Yellow, "["+remote+"]"))

		output, err := a.rcloneCaptureOutput("lsf", remote, "--min-age", fmt.Sprintf("%dd", a.Config.BackupKeepDays))
		if err != nil {
			continue
		}

		for _, file := range strings.Split(output, "\n") {
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}

			PrintColor(Yellow, fmt.Sprintf("deleting %q", file))
			if err := a.runRclone("delete", remote+"/"+file); err != nil {
				PrintColor(Red, fmt.Sprintf("delete %q failed", file))
			}
		}
	}
}

func (a *App) rcloneArgs(args ...string) []string {
	result := splitArgs(a.Config.RcloneGlobalFlag)
	return append(result, args...)
}

func (a *App) runRclone(args ...string) error {
	return runCommand("rclone", a.rcloneArgs(args...)...)
}

func (a *App) rcloneDiscardingOutput(args ...string) error {
	command := exec.Command("rclone", a.rcloneArgs(args...)...)
	command.Stdout = io.Discard
	command.Stderr = os.Stderr
	return command.Run()
}

func (a *App) rcloneCombinedOutput(args ...string) (string, error) {
	command := exec.Command("rclone", a.rcloneArgs(args...)...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func (a *App) rcloneCaptureOutput(args ...string) (string, error) {
	command := exec.Command("rclone", a.rcloneArgs(args...)...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = os.Stderr
	err := command.Run()
	return output.String(), err
}
