package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a file or directory.
type FileEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

// Panel represents one side of the dual-pane file browser.
type Panel struct {
	Title    string
	Path     string
	Entries  []FileEntry
	Cursor   int
	Selected map[int]bool
	Width    int
	Height   int
	IsRemote bool
	Loading  bool
	Err      error
}

// NewPanel creates a new file panel.
func NewPanel(title, path string, isRemote bool) Panel {
	return Panel{
		Title:    title,
		Path:     path,
		Selected: make(map[int]bool),
		IsRemote: isRemote,
	}
}

// LoadLocal reads the local directory and populates entries.
func (p *Panel) LoadLocal() error {
	dirEntries, err := os.ReadDir(p.Path)
	if err != nil {
		p.Err = err
		return err
	}

	p.Entries = []FileEntry{{Name: "..", IsDir: true}}

	var dirs, files []FileEntry
	for _, e := range dirEntries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden files for cleaner view
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		entry := FileEntry{Name: e.Name(), IsDir: e.IsDir(), Size: size}
		if e.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	p.Entries = append(p.Entries, dirs...)
	p.Entries = append(p.Entries, files...)
	p.Cursor = 0
	p.Selected = make(map[int]bool)
	p.Err = nil
	return nil
}

// SetRemoteEntries sets entries from a remote listing result.
func (p *Panel) SetRemoteEntries(entries []FileEntry) {
	p.Entries = entries
	p.Cursor = 0
	p.Selected = make(map[int]bool)
	p.Loading = false
	p.Err = nil
}

// MoveUp moves cursor up.
func (p *Panel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

// MoveDown moves cursor down.
func (p *Panel) MoveDown() {
	if p.Cursor < len(p.Entries)-1 {
		p.Cursor++
	}
}

// ToggleSelect toggles selection on current item.
func (p *Panel) ToggleSelect() {
	if p.Cursor == 0 {
		return // don't select ".."
	}
	if p.Selected[p.Cursor] {
		delete(p.Selected, p.Cursor)
	} else {
		p.Selected[p.Cursor] = true
	}
}

// EnterDir returns the new path when entering a directory.
func (p *Panel) EnterDir() (string, bool) {
	if p.Cursor >= len(p.Entries) {
		return p.Path, false
	}
	entry := p.Entries[p.Cursor]
	if !entry.IsDir {
		return p.Path, false
	}

	var newPath string
	if entry.Name == ".." {
		if p.IsRemote {
			// Use path-style (POSIX) for remote
			newPath = posixDir(p.Path)
		} else {
			newPath = filepath.Dir(p.Path)
		}
	} else {
		if p.IsRemote {
			newPath = posixJoin(p.Path, entry.Name)
		} else {
			newPath = filepath.Join(p.Path, entry.Name)
		}
	}

	return newPath, true
}

// SelectedEntries returns selected entries, or the cursor entry if nothing selected.
func (p *Panel) SelectedEntries() []FileEntry {
	var result []FileEntry
	for idx := range p.Selected {
		if idx < len(p.Entries) {
			result = append(result, p.Entries[idx])
		}
	}
	if len(result) == 0 && p.Cursor > 0 && p.Cursor < len(p.Entries) {
		result = append(result, p.Entries[p.Cursor])
	}
	return result
}

// View renders the panel.
func (p *Panel) View(active bool) string {
	borderStyle := inactiveBorderStyle
	tStyle := titleInactiveStyle
	if active {
		borderStyle = activeBorderStyle
		tStyle = titleActiveStyle
	}

	// Title
	pathDisplay := p.Path
	maxPathLen := p.Width - len(p.Title) - 8
	if maxPathLen < 10 {
		maxPathLen = 10
	}
	if len(pathDisplay) > maxPathLen {
		pathDisplay = "..." + pathDisplay[len(pathDisplay)-maxPathLen+3:]
	}
	title := tStyle.Render(fmt.Sprintf(" %s: %s ", p.Title, pathDisplay))

	// Content area
	contentHeight := p.Height - 4 // borders + title line + count line
	if contentHeight < 1 {
		contentHeight = 1
	}

	var lines []string

	if p.Loading {
		lines = padLines([]string{"", "  " + loadingStyle.Render("Loading...")}, contentHeight, p.Width-4)
	} else if p.Err != nil {
		lines = padLines([]string{"", "  " + errorStyle.Render(p.Err.Error())}, contentHeight, p.Width-4)
	} else {
		// Scroll window
		start := 0
		if p.Cursor >= contentHeight {
			start = p.Cursor - contentHeight + 1
		}
		end := start + contentHeight
		if end > len(p.Entries) {
			end = len(p.Entries)
		}

		for i := start; i < end; i++ {
			lines = append(lines, p.formatEntry(i))
		}
		lines = padLines(lines, contentHeight, p.Width-4)
	}

	// Selection count
	selCount := len(p.Selected)
	countLine := ""
	if selCount > 0 {
		countLine = selectedStyle.Render(fmt.Sprintf(" %d selected", selCount))
	}

	content := title + "\n" + strings.Join(lines, "\n") + "\n" + countLine

	return borderStyle.Width(p.Width - 2).Render(content)
}

func (p *Panel) formatEntry(idx int) string {
	e := p.Entries[idx]
	isCursor := idx == p.Cursor
	isSelected := p.Selected[idx]

	// Prefix
	prefix := "  "
	if isCursor && isSelected {
		prefix = "◉ "
	} else if isCursor {
		prefix = "▸ "
	} else if isSelected {
		prefix = "● "
	}

	// Name
	name := e.Name
	if e.IsDir {
		name += "/"
	}

	// Size
	suffix := ""
	if e.IsDir {
		suffix = "  [DIR]"
	} else if e.Size > 0 {
		suffix = fmt.Sprintf("  %s", formatSize(e.Size))
	}

	// Truncate name if too long
	maxNameLen := p.Width - 6 - len(suffix)
	if maxNameLen < 5 {
		maxNameLen = 5
	}
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}

	// Pad
	padLen := p.Width - 6 - len(name) - len(suffix)
	if padLen < 0 {
		padLen = 0
	}
	line := prefix + name + strings.Repeat(" ", padLen) + suffix

	// Style
	if isCursor {
		return cursorStyle.Render(line)
	}
	if isSelected {
		return selectedStyle.Render(line)
	}
	if e.IsDir {
		return dirEntryStyle.Render(line)
	}
	return fileEntryStyle.Render(line)
}

func padLines(lines []string, targetLen int, width int) []string {
	for len(lines) < targetLen {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return lines
}

func formatSize(size int64) string {
	switch {
	case size < 1024:
		return fmt.Sprintf("%dB", size)
	case size < 1024*1024:
		return fmt.Sprintf("%.1fK", float64(size)/1024)
	case size < 1024*1024*1024:
		return fmt.Sprintf("%.1fM", float64(size)/1024/1024)
	default:
		return fmt.Sprintf("%.1fG", float64(size)/1024/1024/1024)
	}
}

func posixDir(p string) string {
	if p == "/" {
		return "/"
	}
	p = strings.TrimRight(p, "/")
	idx := strings.LastIndex(p, "/")
	if idx <= 0 {
		return "/"
	}
	return p[:idx]
}

func posixJoin(base, name string) string {
	base = strings.TrimRight(base, "/")
	return base + "/" + name
}
