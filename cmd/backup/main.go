package main

import (
	"errors"
	"os"

	backupapp "github.com/mniak/vw-backup-go/internal/app"
)

func main() {
	application := backupapp.NewApp()

	if err := application.Run(); err != nil {
		var fatal *backupapp.Fatal
		if errors.As(err, &fatal) {
			if fatal.Message != "" {
				backupapp.PrintColor(backupapp.Red, fatal.Message)
			}
			if fatal.Notify != "" && application.Notifier != nil {
				application.Notifier.Send("failure", fatal.Notify)
			}
		} else {
			backupapp.PrintColor(backupapp.Red, err.Error())
		}
		os.Exit(1)
	}
}
