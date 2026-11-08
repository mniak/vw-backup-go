package app

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Notifier sends mail and ping notifications for backup events.
type Notifier struct {
	Config *Config
}

func NewNotifier(config *Config) *Notifier {
	return &Notifier{Config: config}
}

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05 MST")
}

func (n *Notifier) Send(status, content string) {
	subjectStart := n.Config.DisplayName + " Backup Start"
	subjectSuccess := n.Config.DisplayName + " Backup Success"
	subjectFailure := n.Config.DisplayName + " Backup Failed"

	switch status {
	case "start":
		n.sendPing("start", subjectStart, content)
	case "success":
		if n.Config.MailSMTPEnable && n.Config.MailWhenSuccess {
			n.sendMail(subjectSuccess, content)
		}
		n.sendPing("success", subjectSuccess, content)
		n.sendPing("completion", subjectSuccess, content)
	case "failure":
		if n.Config.MailSMTPEnable && n.Config.MailWhenFailure {
			n.sendMail(subjectFailure, content)
		}
		n.sendPing("failure", subjectFailure, content)
		n.sendPing("completion", subjectFailure, content)
	}
}

// SendPing sends one health-check ping.
func (n *Notifier) SendPing(status, subject, content string) {
	n.sendPing(status, subject, content)
}

// SendMail sends one notification mail.
func (n *Notifier) SendMail(subject, content string) {
	n.sendMail(subject, content)
}

func (n *Notifier) sendPing(status, subject, content string) {
	var (
		url     string
		options string
	)

	switch status {
	case "completion":
		url, options = n.Config.PingURL, n.Config.PingURLCurlOptions
	case "start":
		url, options = n.Config.PingURLWhenStart, n.Config.PingURLWhenStartCurlOptions
	case "success":
		url, options = n.Config.PingURLWhenSuccess, n.Config.PingURLWhenSuccessCurlOptions
	case "failure":
		url, options = n.Config.PingURLWhenFailure, n.Config.PingURLWhenFailureCurlOptions
	default:
		PrintColor(Red, "illegal identifier, only supports completion, start, success, failure")
		return
	}

	if url == "" {
		return
	}

	url = replacePingPlaceholders(url, strings.ReplaceAll(subject, " ", "+"), strings.ReplaceAll(content, " ", "+"))
	options = replacePingPlaceholders(options, subject, content)

	args := []string{"-m", "15", "--retry", "10", "--retry-delay", "1", "-o", "/dev/null", "-s"}
	args = append(args, splitArgs(options)...)
	args = append(args, url)

	if os.Getenv("PING_DEBUG") == "TRUE" {
		command := "curl -m 15 --retry 10 --retry-delay 1 -o /dev/null -s"
		if options != "" {
			command += " " + options
		}
		command += fmt.Sprintf(" %q", url)
		PrintColor(Yellow, "curl command: "+command)
	}

	if err := runCommand("curl", args...); err != nil {
		PrintColor(Red, status+" ping sending has failed")
	} else {
		PrintColor(Blue, status+" ping has been sent successfully")
	}
}

func (n *Notifier) sendMail(subject, content string) {
	var (
		template      string
		usedMessageID string
	)

	if n.Config.MailUseThread {
		if n.Config.MailParentMessageID != "" {
			usedMessageID = n.Config.MailParentMessageID
			template = fmt.Sprintf("References: %s\nIn-Reply-To: %s\n\n", usedMessageID, usedMessageID)
		} else {
			usedMessageID = generateMessageID()
			template = fmt.Sprintf("Message-ID: %s\n\n", usedMessageID)
		}
	}

	args := make([]string, 0, 8)
	if os.Getenv("MAIL_DEBUG") == "TRUE" {
		args = append(args, "-v")
	}
	if template != "" {
		args = append(args, "-t")
	}
	args = append(args, "-s", subject)
	args = append(args, splitArgs(n.Config.MailSMTPVariables)...)
	args = append(args, n.Config.MailTo)

	command := exec.Command("mail", args...)
	command.Stdin = strings.NewReader(template + content + "\n")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		PrintColor(Red, "mail sending has failed")
		return
	}

	PrintColor(Blue, "mail has been sent successfully")

	if n.Config.MailUseThread && n.Config.MailParentMessageID == "" {
		_ = os.MkdirAll(filepath.Dir(MailParentMessageIDFile), 0o755)
		_ = os.WriteFile(MailParentMessageIDFile, []byte(usedMessageID), 0o644)
	}
}

func replacePingPlaceholders(value, subject, content string) string {
	return strings.NewReplacer("%{subject}", subject, "%{content}", content).Replace(value)
}

func generateMessageID() string {
	random := make([]byte, 16)
	_, _ = rand.Read(random)

	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for i := range random {
		random[i] = alphabet[int(random[i])%len(alphabet)]
	}

	hostname, _ := os.Hostname()
	return fmt.Sprintf("<%d.%s.%s@vaultwarden.backup>", time.Now().UnixNano(), string(random), hostname)
}

func newCommand(name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func runCommand(name string, args ...string) error {
	return newCommand(name, args...).Run()
}
