package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FileEntry represents a file or directory.
type FileEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

// InputMode represents the current input state of a panel.
type InputMode int

const (
	ModeNormal InputMode = iota
	ModePathEdit         // editing the path bar
	ModeFilter           // filtering file list
)

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

	// Navigation history for back/forward
	history    []string
	historyIdx int

	// Input mode
	Mode      InputMode
	InputText string // text being typed in path edit or filter mode

	// Filtered view
	filtered      []int // indices into Entries that match filter
	filterActive  bool

	// Path bar button positions (for mouse clicks)
	BackBtnX1, BackBtnX2   int
	FwdBtnX1, FwdBtnX2     int
	PathBarX1, PathBarX2   int
	PathBarY               int
}

// NewPanel creates a new file panel.
func NewPanel(title, path string, isRemote bool) Panel {
	return Panel{
		Title:      title,
		Path:       path,
		Selected:   make(map[int]bool),
		IsRemote:   isRemote,
		history:    []string{path},
		historyIdx: 0,
	}
}

// NavigateTo records a new path in history.
func (p *Panel) NavigateTo(newPath string) {
	// Trim forward history
	if p.historyIdx < len(p.history)-1 {
		p.history = p.history[:p.historyIdx+1]
	}
	p.history = append(p.history, newPath)
	p.historyIdx = len(p.history) - 1
	p.Path = newPath
	p.ClearFilter()
}

// GoBack navigates to previous path in history.
func (p *Panel) GoBack() (string, bool) {
	if p.historyIdx <= 0 {
		return "", false
	}
	p.historyIdx--
	p.Path = p.history[p.historyIdx]
	p.ClearFilter()
	return p.Path, true
}

// GoForward navigates to next path in history.
func (p *Panel) GoForward() (string, bool) {
	if p.historyIdx >= len(p.history)-1 {
		return "", false
	}
	p.historyIdx++
	p.Path = p.history[p.historyIdx]
	p.ClearFilter()
	return p.Path, true
}

// CanGoBack returns true if there's history to go back to.
func (p *Panel) CanGoBack() bool {
	return p.historyIdx > 0
}

// CanGoForward returns true if there's forward history.
func (p *Panel) CanGoForward() bool {
	return p.historyIdx < len(p.history)-1
}

// StartPathEdit enters path editing mode.
func (p *Panel) StartPathEdit() {
	p.Mode = ModePathEdit
	p.InputText = p.Path
}

// StartFilter enters filter mode.
func (p *Panel) StartFilter() {
	p.Mode = ModeFilter
	p.InputText = ""
	p.filterActive = false
	p.filtered = nil
}

// CancelInput exits input mode.
func (p *Panel) CancelInput() {
	p.Mode = ModeNormal
	p.InputText = ""
	if p.Mode == ModeFilter {
		p.ClearFilter()
	}
}

// ConfirmPathEdit returns the edited path and exits edit mode.
func (p *Panel) ConfirmPathEdit() string {
	path := strings.TrimSpace(p.InputText)
	p.Mode = ModeNormal
	p.InputText = ""
	return path
}

// UpdateFilter applies the current filter text to entries.
func (p *Panel) UpdateFilter() {
	query := strings.ToLower(p.InputText)
	if query == "" {
		p.ClearFilter()
		return
	}
	p.filterActive = true
	p.filtered = nil
	for i, e := range p.Entries {
		if i == 0 { // always include ".."
			p.filtered = append(p.filtered, i)
			continue
		}
		if strings.Contains(strings.ToLower(e.Name), query) {
			p.filtered = append(p.filtered, i)
		}
	}
	if p.Cursor >= len(p.VisibleEntries()) {
		p.Cursor = 0
	}
}

// ClearFilter removes the active filter.
func (p *Panel) ClearFilter() {
	p.filterActive = false
	p.filtered = nil
	p.InputText = ""
	if p.Mode == ModeFilter {
		p.Mode = ModeNormal
	}
}

// VisibleEntries returns entries visible after filtering.
func (p *Panel) VisibleEntries() []FileEntry {
	if !p.filterActive || p.filtered == nil {
		return p.Entries
	}
	var result []FileEntry
	for _, idx := range p.filtered {
		if idx < len(p.Entries) {
			result = append(result, p.Entries[idx])
		}
	}
	return result
}

// RealIndex maps a visible index to the real entry index.
func (p *Panel) RealIndex(visibleIdx int) int {
	if !p.filterActive || p.filtered == nil {
		return visibleIdx
	}
	if visibleIdx < len(p.filtered) {
		return p.filtered[visibleIdx]
	}
	return visibleIdx
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
			continue
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
	p.ClearFilter()
	return nil
}

// SetRemoteEntries sets entries from a remote listing result.
func (p *Panel) SetRemoteEntries(entries []FileEntry) {
	p.Entries = entries
	p.Cursor = 0
	p.Selected = make(map[int]bool)
	p.Loading = false
	p.Err = nil
	p.ClearFilter()
}

// MoveUp moves cursor up.
func (p *Panel) MoveUp() {
	if p.Cursor > 0 {
		p.Cursor--
	}
}

// MoveDown moves cursor down.
func (p *Panel) MoveDown() {
	max := len(p.VisibleEntries()) - 1
	if p.Cursor < max {
		p.Cursor++
	}
}

// ToggleSelect toggles selection on current item.
func (p *Panel) ToggleSelect() {
	realIdx := p.RealIndex(p.Cursor)
	if realIdx == 0 {
		return // don't select ".."
	}
	if p.Selected[realIdx] {
		delete(p.Selected, realIdx)
	} else {
		p.Selected[realIdx] = true
	}
}

// EnterDir returns the new path when entering a directory.
func (p *Panel) EnterDir() (string, bool) {
	visible := p.VisibleEntries()
	if p.Cursor >= len(visible) {
		return p.Path, false
	}
	entry := visible[p.Cursor]
	if !entry.IsDir {
		return p.Path, false
	}

	var newPath string
	if entry.Name == ".." {
		if p.IsRemote {
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
	if len(result) == 0 {
		visible := p.VisibleEntries()
		realIdx := p.RealIndex(p.Cursor)
		if realIdx > 0 && realIdx < len(p.Entries) {
			result = append(result, p.Entries[realIdx])
		}
		_ = visible
	}
	return result
}

// View renders the panel.
func (p *Panel) View(active bool) string {
	borderStyle := inactiveBorderStyle
	if active {
		borderStyle = activeBorderStyle
	}

	// Navigation bar: [<] [>] [path                    ]
	navBar := p.renderNavBar(active)

	// Content area
	contentHeight := p.Height - 5 // borders + nav bar + filter/count line
	if contentHeight < 1 {
		contentHeight = 1
	}

	var lines []string

	if p.Loading {
		lines = padLines([]string{"", "  " + loadingStyle.Render("Loading...")}, contentHeight, p.Width-4)
	} else if p.Err != nil {
		lines = padLines([]string{"", "  " + errorStyle.Render(p.Err.Error())}, contentHeight, p.Width-4)
	} else {
		visible := p.VisibleEntries()

		// Scroll window
		start := 0
		if p.Cursor >= contentHeight {
			start = p.Cursor - contentHeight + 1
		}
		end := start + contentHeight
		if end > len(visible) {
			end = len(visible)
		}

		for i := start; i < end; i++ {
			lines = append(lines, p.formatVisibleEntry(i, visible))
		}
		lines = padLines(lines, contentHeight, p.Width-4)
	}

	// Bottom line: filter input or selection count
	bottomLine := p.renderBottomLine()

	content := navBar + "\n" + strings.Join(lines, "\n") + "\n" + bottomLine

	return borderStyle.Width(p.Width - 2).Render(content)
}

// renderNavBar renders the [<] [>] [path] navigation bar.
func (p *Panel) renderNavBar(active bool) string {
	w := p.Width - 4 // inside border padding

	// Back button
	backStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if p.CanGoBack() {
		backStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	}
	backBtn := backStyle.Render(" ◀ ")

	// Forward button
	fwdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if p.CanGoForward() {
		fwdStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	}
	fwdBtn := fwdStyle.Render(" ▶ ")

	// Path bar
	pathBarWidth := w - 8 // minus buttons and spacing
	if pathBarWidth < 10 {
		pathBarWidth = 10
	}

	var pathContent string
	if p.Mode == ModePathEdit {
		// Editable path with cursor
		pathContent = p.InputText + "█"
		if len(pathContent) > pathBarWidth {
			pathContent = pathContent[len(pathContent)-pathBarWidth:]
		}
		pathContent = pathEditStyle.Render(fmt.Sprintf("%-*s", pathBarWidth, pathContent))
	} else {
		// Normal path display
		display := p.Path
		if len(display) > pathBarWidth {
			display = "..." + display[len(display)-pathBarWidth+3:]
		}
		tStyle := titleInactiveStyle
		if active {
			tStyle = titleActiveStyle
		}
		pathContent = tStyle.Render(fmt.Sprintf("%-*s", pathBarWidth, display))
	}

	// Record button positions for mouse clicks
	p.PathBarY = 1 // relative to panel top (after border)
	p.BackBtnX1 = 0
	p.BackBtnX2 = 3
	p.FwdBtnX1 = 4
	p.FwdBtnX2 = 7
	p.PathBarX1 = 8
	p.PathBarX2 = 8 + pathBarWidth

	return backBtn + " " + fwdBtn + " " + pathContent
}

// renderBottomLine renders filter input or selection count.
func (p *Panel) renderBottomLine() string {
	if p.Mode == ModeFilter {
		filterText := p.InputText + "█"
		return filterStyle.Render(fmt.Sprintf(" / %s", filterText))
	}

	selCount := len(p.Selected)
	if selCount > 0 {
		return selectedStyle.Render(fmt.Sprintf(" %d selected", selCount))
	}

	visible := p.VisibleEntries()
	if p.filterActive && len(visible) != len(p.Entries) {
		return statusBarStyle.Render(fmt.Sprintf(" %d/%d items", len(visible), len(p.Entries)))
	}

	return ""
}

func (p *Panel) formatVisibleEntry(visibleIdx int, visible []FileEntry) string {
	if visibleIdx >= len(visible) {
		return ""
	}
	e := visible[visibleIdx]
	isCursor := visibleIdx == p.Cursor
	realIdx := p.RealIndex(visibleIdx)
	isSelected := p.Selected[realIdx]

	prefix := "  "
	if isCursor && isSelected {
		prefix = "◉ "
	} else if isCursor {
		prefix = "▸ "
	} else if isSelected {
		prefix = "● "
	}

	name := e.Name
	if e.IsDir {
		name += "/"
	}

	suffix := ""
	if e.IsDir {
		suffix = "  [DIR]"
	} else if e.Size > 0 {
		suffix = fmt.Sprintf("  %s", formatSize(e.Size))
	}

	maxNameLen := p.Width - 6 - len(suffix)
	if maxNameLen < 5 {
		maxNameLen = 5
	}
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}

	padLen := p.Width - 6 - len(name) - len(suffix)
	if padLen < 0 {
		padLen = 0
	}
	line := prefix + name + strings.Repeat(" ", padLen) + suffix

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
