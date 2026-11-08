package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mniak/vw-backup-go/internal/app"
)

var errSilent = errors.New("silent failure")

// Execute runs the command line interface and returns the process exit code.
func Execute() int {
	root := NewRootCommand()
	if err := root.Execute(); err != nil {
		if errors.Is(err, errSilent) {
			return 1
		}
		app.PrintColor(app.Red, err.Error())
		return 1
	}
	return 0
}

// NewRootCommand builds the vaultwarden-backup command tree.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "vaultwarden-backup",
		Short:         "Backup and restore a vaultwarden instance",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          runDaemon,
	}
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newBackupCommand(),
		newRestoreCommand(),
		newMailCommand(),
		newPingCommand(),
		newRcloneCommand(),
	)

	return root
}

func runDaemon(_ *cobra.Command, _ []string) error {
	config, err := app.LoadConfig()
	if err != nil {
		return err
	}

	application := app.NewApp(config)
	if err := application.CheckRcloneConnection("all"); err != nil {
		return reportFatal(nil, err)
	}

	if err := config.ConfigurePostgresql(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := app.ConfigureTimezone(config); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if err := app.ConfigureCron(config); err != nil {
		return err
	}

	return app.ExecSupercronic()
}

func newBackupCommand() *cobra.Command {
	var fromCron bool

	command := &cobra.Command{
		Use:   "backup",
		Short: "Run one backup",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			config, err := app.LoadConfig()
			if err != nil {
				return err
			}

			application := app.NewApp(config)
			if !fromCron {
				if err := application.CheckRcloneConnection("all"); err != nil {
					return reportFatal(nil, err)
				}
				if err := config.ConfigurePostgresql(); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				if err := app.ConfigureTimezone(config); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				if err := app.ConfigureCron(config); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}

			if err := application.Run(); err != nil {
				return reportFatal(application, err)
			}
			return nil
		},
	}

	command.Flags().BoolVar(&fromCron, "from-cron", false, "run as a scheduled backup")
	_ = command.Flags().MarkHidden("from-cron")

	return command
}

func newRestoreCommand() *cobra.Command {
	options := app.RestoreOptions{}

	command := &cobra.Command{
		Use:   "restore",
		Short: "Restore a backup",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			config, err := app.LoadRestoreConfig()
			if err != nil {
				return err
			}
			if err := app.Restore(config, options); err != nil {
				return reportFatal(nil, err)
			}
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&options.Password, "password", "p", "", "archive password")
	flags.StringVar(&options.ZipFile, "zip-file", "", "archive file to restore")
	flags.StringVar(&options.DBFile, "db-file", "", "database file to restore")
	flags.StringVar(&options.ConfigFile, "config-file", "", "config file to restore")
	flags.StringVar(&options.RSAKeyFile, "rsakey-file", "", "rsakey archive to restore")
	flags.StringVar(&options.AttachmentsFile, "attachments-file", "", "attachments archive to restore")
	flags.StringVar(&options.SendsFile, "sends-file", "", "sends archive to restore")
	flags.BoolVarP(&options.Force, "force-restore", "f", false, "restore without confirmation")

	return command
}

func newMailCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mail [to]",
		Short: "Send a test mail",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			config := app.LoadMailConfig()
			config.MailSMTPEnable = true
			_ = os.Setenv("MAIL_DEBUG", "TRUE")

			if len(args) == 1 && args[0] != "" {
				config.MailTo = args[0]
			}

			app.NewNotifier(config).SendMail(config.DisplayName+" Backup Test", "Your SMTP configuration looks correct.")
			return nil
		},
	}
}

func newPingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ping [completion|start|success|failure]",
		Short: "Send a test ping",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			status := ""
			if len(args) == 1 {
				status = args[0]
			}

			config := app.LoadPingConfig()
			_ = os.Setenv("PING_DEBUG", "TRUE")

			app.NewNotifier(config).SendPing(status, config.DisplayName+" Backup Test", "Your PING configuration looks correct.")
			return nil
		},
	}
}

func newRcloneCommand() *cobra.Command {
	return &cobra.Command{
		Use:                "rclone [args...]",
		Short:              "Run rclone with the supplied arguments",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := exec.LookPath("rclone")
			if err != nil {
				return err
			}
			return syscall.Exec(path, append([]string{"rclone"}, args...), os.Environ())
		},
	}
}

func reportFatal(application *app.App, err error) error {
	var fatal *app.Fatal
	if errors.As(err, &fatal) {
		if fatal.Message != "" {
			app.PrintColor(app.Red, fatal.Message)
		}
		if fatal.Notify != "" && application != nil && application.Notifier != nil {
			application.Notifier.Send("failure", fatal.Notify)
		}
		return errSilent
	}
	return err
}
