package app

import (
	"fmt"
	"strings"
)

// Color is one of the colors used by the command output.
type Color string

const (
	Red    Color = "red"
	Green  Color = "green"
	Yellow Color = "yellow"
	Blue   Color = "blue"
	None   Color = "none"
)

// Colorize wraps a message in the same ANSI escape codes used by the shell scripts.
func Colorize(color Color, message string) string {
	code := map[Color]string{
		Red:    "31",
		Green:  "32",
		Yellow: "33",
		Blue:   "34",
	}[color]

	if code == "" {
		return message
	}
	return "\033[" + code + "m" + message + "\033[0m"
}

// PrintColor prints a colored message to stdout.
func PrintColor(color Color, message string) {
	fmt.Println(Colorize(color, message))
}

// splitArgs splits a command line fragment using the quoting rules that the
// shell would apply to unquoted ${VAR} expansions.
func splitArgs(input string) []string {
	var (
		args    []string
		current strings.Builder
		quote   rune
		escaped bool
		started bool
	)

	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}

	for _, char := range input {
		if escaped {
			current.WriteRune(char)
			escaped = false
			started = true
			continue
		}

		switch quote {
		case '\'':
			if char == '\'' {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		case '"':
			switch char {
			case '"':
				quote = 0
			case '\\':
				escaped = true
			default:
				current.WriteRune(char)
			}
			continue
		}

		switch char {
		case '\\':
			escaped = true
			started = true
		case '\'', '"':
			quote = char
			started = true
		case ' ', '\t', '\n':
			flush()
		default:
			current.WriteRune(char)
			started = true
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	flush()

	return args
}
