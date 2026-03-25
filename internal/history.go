package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	maxHistoryEntries = 10
	historyFileName   = "history.json"
)

// HistoryEntry records a server connection.
type HistoryEntry struct {
	InstanceId string    `json:"instance_id"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	LastUsed   time.Time `json:"last_used"`
}

// LoadHistory reads connection history from ~/.gossm/history.json.
func LoadHistory() []HistoryEntry {
	path := historyPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// SaveHistory records a connection in history.
func SaveHistory(instanceId, name, region string) {
	entries := LoadHistory()

	// Remove existing entry for this instance
	var filtered []HistoryEntry
	for _, e := range entries {
		if e.InstanceId != instanceId {
			filtered = append(filtered, e)
		}
	}

	// Prepend new entry
	entry := HistoryEntry{
		InstanceId: instanceId,
		Name:       name,
		Region:     region,
		LastUsed:   time.Now(),
	}
	filtered = append([]HistoryEntry{entry}, filtered...)

	// Limit to max entries
	if len(filtered) > maxHistoryEntries {
		filtered = filtered[:maxHistoryEntries]
	}

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return
	}

	path := historyPath()
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	os.WriteFile(path, data, 0600)
}

// GetRecentInstanceIds returns instance IDs from recent history for a region.
func GetRecentInstanceIds(region string) []string {
	entries := LoadHistory()
	var ids []string
	for _, e := range entries {
		if e.Region == region {
			ids = append(ids, e.InstanceId)
		}
	}
	return ids
}

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return historyFileName
	}
	return filepath.Join(home, ".gossm", historyFileName)
}
