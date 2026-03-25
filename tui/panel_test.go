package tui

import "testing"

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1572864, "1.5M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		result := formatSize(tt.input)
		if result != tt.expected {
			t.Errorf("formatSize(%d): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestPosixDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{"/home", "/"},
		{"/home/user", "/home"},
		{"/home/user/", "/home"},
		{"/home/user/dir", "/home/user"},
		{"/a", "/"},
	}

	for _, tt := range tests {
		result := posixDir(tt.input)
		if result != tt.expected {
			t.Errorf("posixDir(%q): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestPosixJoin(t *testing.T) {
	tests := []struct {
		base     string
		name     string
		expected string
	}{
		{"/home", "user", "/home/user"},
		{"/home/", "user", "/home/user"},
		{"/", "tmp", "/tmp"},
		{"/home/user", "file.txt", "/home/user/file.txt"},
	}

	for _, tt := range tests {
		result := posixJoin(tt.base, tt.name)
		if result != tt.expected {
			t.Errorf("posixJoin(%q, %q): expected %q, got %q", tt.base, tt.name, tt.expected, result)
		}
	}
}
