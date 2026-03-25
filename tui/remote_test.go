package tui

import (
	"testing"
)

func TestParseRemoteListing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []FileEntry
	}{
		{
			name:  "basic listing",
			input: "./\n../\napp/\nlogs/\nconfig.yaml\ndata.csv\n",
			expected: []FileEntry{
				{Name: "..", IsDir: true},
				{Name: "app", IsDir: true},
				{Name: "logs", IsDir: true},
				{Name: "config.yaml", IsDir: false},
				{Name: "data.csv", IsDir: false},
			},
		},
		{
			name:  "with executable and symlink indicators",
			input: "./\n../\nbin/\nscript.sh*\nlink@\n",
			expected: []FileEntry{
				{Name: "..", IsDir: true},
				{Name: "bin", IsDir: true},
				{Name: "script.sh", IsDir: false},
				{Name: "link", IsDir: false},
			},
		},
		{
			name:  "empty directory",
			input: "./\n../\n",
			expected: []FileEntry{
				{Name: "..", IsDir: true},
			},
		},
		{
			name:  "trailing whitespace",
			input: "  ./  \n  ../  \n  myfile.txt  \n",
			expected: []FileEntry{
				{Name: "..", IsDir: true},
				{Name: "myfile.txt", IsDir: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRemoteListing(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d entries, got %d", len(tt.expected), len(result))
			}
			for i, e := range tt.expected {
				if result[i].Name != e.Name {
					t.Errorf("[%d] name: expected %q, got %q", i, e.Name, result[i].Name)
				}
				if result[i].IsDir != e.IsDir {
					t.Errorf("[%d] isDir: expected %v, got %v", i, e.IsDir, result[i].IsDir)
				}
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"/home/user/file.txt", "'/home/user/file.txt'"},
		{"it's a test", "'it'\\''s a test'"},
		{"", "''"},
		{"path with spaces", "'path with spaces'"},
	}

	for _, tt := range tests {
		result := shellQuote(tt.input)
		if result != tt.expected {
			t.Errorf("shellQuote(%q): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}
