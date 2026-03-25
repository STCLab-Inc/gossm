package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// s3Tracker tracks S3 temporary files for cleanup on exit.
var s3Tracker = &s3CleanupTracker{
	keys: make(map[string]string), // s3Key -> bucket
}

type s3CleanupTracker struct {
	mu   sync.Mutex
	keys map[string]string
}

// Track registers an S3 key for cleanup.
func (t *s3CleanupTracker) Track(bucket, s3Key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.keys[s3Key] = bucket
}

// Untrack removes an S3 key from cleanup tracking.
func (t *s3CleanupTracker) Untrack(s3Key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.keys, s3Key)
}

// CleanupAll deletes all tracked S3 temporary files.
func (t *s3CleanupTracker) CleanupAll() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for s3Key, bucket := range t.keys {
		s3URI := fmt.Sprintf("s3://%s/%s", bucket, s3Key)
		exec.Command("aws", "s3", "rm", s3URI, "--quiet").Run()
	}
	t.keys = make(map[string]string)
}

// InitSignalHandler sets up SIGINT/SIGTERM handler to clean up S3 temp files.
func InitSignalHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		s3Tracker.CleanupAll()
		os.Exit(1)
	}()
}

// CleanupStaleS3Files removes gossm-tmp files older than 24 hours from the bucket.
func CleanupStaleS3Files(bucket, prefix string) {
	cutoff := time.Now().Add(-24 * time.Hour)

	// List objects with the gossm-tmp prefix
	out, err := exec.Command("aws", "s3", "ls", fmt.Sprintf("s3://%s/%s/", bucket, prefix), "--recursive").Output()
	if err != nil {
		return
	}

	for _, line := range splitLines(string(out)) {
		// Format: "2026-03-25 14:30:00  1234 gossm-tmp/i-xxx/1234-file.txt"
		fields := splitFields(line)
		if len(fields) < 4 {
			continue
		}
		dateStr := fields[0] + " " + fields[1]
		t, err := time.Parse("2006-01-02 15:04:05", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			s3Key := fields[3]
			s3URI := fmt.Sprintf("s3://%s/%s", bucket, s3Key)
			exec.Command("aws", "s3", "rm", s3URI, "--quiet").Run()
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range splitByNewline(s) {
		line = trimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func splitFields(s string) []string {
	var fields []string
	inField := false
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			if inField {
				fields = append(fields, s[start:i])
				inField = false
			}
		} else {
			if !inField {
				start = i
				inField = true
			}
		}
	}
	if inField {
		fields = append(fields, s[start:])
	}
	return fields
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\r' || s[j-1] == '\n') {
		j--
	}
	return s[i:j]
}
