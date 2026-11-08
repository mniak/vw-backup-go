package app

import (
	"reflect"
	"testing"
	"time"
)

func TestStrftime(t *testing.T) {
	value := time.Date(2024, time.March, 7, 9, 5, 4, 123456789, time.UTC)

	tests := map[string]string{
		"%Y%m%d":            "20240307",
		"%Y-%m-%d_%H-%M-%S": "2024-03-07_09-05-04",
		"%F %T":             "2024-03-07 09:05:04",
		"%s":                "1709802304",
		"literal":           "literal",
	}

	for format, expected := range tests {
		if result := strftime(format, value); result != expected {
			t.Errorf("strftime(%q) = %q, want %q", format, result, expected)
		}
	}
}

func TestSplitArgs(t *testing.T) {
	input := `-v --config "/path with space/rclone.conf" --s3-no-check-bucket`
	expected := []string{"-v", "--config", "/path with space/rclone.conf", "--s3-no-check-bucket"}

	if result := splitArgs(input); !reflect.DeepEqual(result, expected) {
		t.Errorf("splitArgs(%q) = %#v, want %#v", input, result, expected)
	}
}

func TestSanitizeBackupFileDate(t *testing.T) {
	if result := sanitizeBackupFileDate("%Y%m%d/.x"); result != "%Y%m%dx" {
		t.Errorf("unexpected sanitized date: %q", result)
	}
}
