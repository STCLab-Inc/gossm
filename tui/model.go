package tui

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages
type remoteDirMsg struct {
	entries []FileEntry
	path    string
}
type remoteDirErrMsg struct {
	err  error
	path string
}
type remoteHomeMsg struct {
	home string
}

type transferDoneMsg struct{ msg string }
type transferErrMsg struct{ err error }

// doubleClickMsg is sent after a delay to reset double-click detection
type doubleClickResetMsg struct{}

// Button IDs for the bottom action bar
const (
	btnUpload   = "upload"
	btnDownload = "download"
	btnRefresh  = "refresh"
	btnQuit     = "quit"
)

// button represents a clickable button in the action bar
type button struct {
	id     string
	label  string
	style  lipgloss.Style
	x1, x2 int // screen x positions (calculated during render)
}

// ExplorerModel is the main dual-pane file manager model.
type ExplorerModel struct {
	localPanel  Panel
	remotePanel Panel
	activePanel int // 0=local, 1=remote
	width       int
	height      int
	statusMsg   string
	transfering bool

	// Mouse state
	lastClickTime time.Time
	lastClickY    int
	lastClickPanel int

	// Action buttons
	buttons []button

	// Layout constants
	panelContentY int // Y offset where file entries start in panels
	actionBarY    int // Y position of the action bar

	// AWS/SSM config
	awsConfig  aws.Config
	instanceId string
	s3Bucket   string
	s3Prefix   string
}

// NewExplorerModel creates a new explorer model.
func NewExplorerModel(cfg aws.Config, instanceId, localDir, remoteDir, s3Bucket string) ExplorerModel {
	s3Prefix := fmt.Sprintf("gossm-tmp/%s", instanceId)

	local := NewPanel("Local", localDir, false)
	local.LoadLocal()

	remote := NewPanel("Remote", remoteDir, true)
	remote.Loading = true

	buttons := []button{
		{id: btnUpload, label: " Upload >>> ", style: lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1)},
		{id: btnDownload, label: " <<< Download ", style: lipgloss.NewStyle().Background(lipgloss.Color("33")).Foreground(lipgloss.Color("255")).Bold(true).Padding(0, 1)},
		{id: btnRefresh, label: " Refresh ", style: lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("255")).Padding(0, 1)},
		{id: btnQuit, label: " Quit ", style: lipgloss.NewStyle().Background(lipgloss.Color("124")).Foreground(lipgloss.Color("255")).Padding(0, 1)},
	}

	return ExplorerModel{
		localPanel:  local,
		remotePanel: remote,
		activePanel: 0,
		awsConfig:   cfg,
		instanceId:  instanceId,
		s3Bucket:    s3Bucket,
		s3Prefix:    s3Prefix,
		buttons:     buttons,
		panelContentY: 2, // border(1) + title(1)
	}
}

func (m ExplorerModel) Init() tea.Cmd {
	return m.detectRemoteHome()
}

func (m ExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		panelWidth := m.width / 2
		panelHeight := m.height - 5 // panels + status + buttons + help
		m.localPanel.Width = panelWidth
		m.localPanel.Height = panelHeight
		m.remotePanel.Width = m.width - panelWidth
		m.remotePanel.Height = panelHeight
		m.actionBarY = panelHeight + 1
		return m, nil

	case tea.MouseMsg:
		if m.transfering {
			return m, nil
		}
		return m.handleMouse(msg)

	case remoteDirMsg:
		m.remotePanel.Path = msg.path
		m.remotePanel.SetRemoteEntries(msg.entries)
		m.statusMsg = successStyle.Render(fmt.Sprintf("Remote: %s (%d items)", msg.path, len(msg.entries)-1))
		return m, nil

	case remoteHomeMsg:
		m.remotePanel.Path = msg.home
		return m, m.fetchRemoteDir(msg.home)

	case remoteDirErrMsg:
		m.remotePanel.Loading = false
		m.remotePanel.Err = msg.err
		m.statusMsg = errorStyle.Render(fmt.Sprintf("'%s': %s", msg.path, msg.err.Error()))
		if msg.path != "/tmp" && msg.path != "/" {
			m.remotePanel.Loading = true
			m.remotePanel.Err = nil
			m.statusMsg = loadingStyle.Render(fmt.Sprintf("'%s' not found, trying /tmp...", msg.path))
			return m, m.fetchRemoteDir("/tmp")
		}
		return m, nil

	case transferDoneMsg:
		m.transfering = false
		m.statusMsg = successStyle.Render(msg.msg)
		m.localPanel.LoadLocal()
		return m, m.fetchRemoteDir(m.remotePanel.Path)

	case transferErrMsg:
		m.transfering = false
		m.statusMsg = errorStyle.Render("Transfer failed: " + msg.err.Error())
		return m, nil

	case doubleClickResetMsg:
		// no-op, just for timing
		return m, nil

	case tea.KeyMsg:
		if m.transfering {
			return m, nil
		}
		return m.handleKey(msg)
	}

	return m, nil
}

// handleMouse processes mouse events
func (m ExplorerModel) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x, y := msg.X, msg.Y

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.activeP().MoveUp()
		m.activeP().MoveUp()
		m.activeP().MoveUp()
		return m, nil

	case tea.MouseButtonWheelDown:
		m.activeP().MoveDown()
		m.activeP().MoveDown()
		m.activeP().MoveDown()
		return m, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}

		// Check if click is on action buttons (bottom area)
		if y >= m.actionBarY {
			return m.handleButtonClick(x)
		}

		// Determine which panel was clicked
		panelWidth := m.width / 2
		clickedPanel := 0
		if x >= panelWidth {
			clickedPanel = 1
		}

		// Switch active panel
		m.activePanel = clickedPanel
		p := m.activeP()

		// Calculate which entry was clicked
		entryY := y - m.panelContentY
		if entryY < 0 || entryY >= len(p.Entries) {
			return m, nil
		}

		// Adjust for scroll offset
		scrollStart := 0
		contentHeight := p.Height - 4
		if p.Cursor >= contentHeight {
			scrollStart = p.Cursor - contentHeight + 1
		}
		clickedIdx := scrollStart + entryY

		if clickedIdx >= len(p.Entries) {
			return m, nil
		}

		// Double-click detection (within 400ms)
		now := time.Now()
		isDoubleClick := now.Sub(m.lastClickTime) < 400*time.Millisecond &&
			m.lastClickY == y &&
			m.lastClickPanel == clickedPanel

		m.lastClickTime = now
		m.lastClickY = y
		m.lastClickPanel = clickedPanel

		if isDoubleClick {
			// Double click: enter directory or toggle select
			p.Cursor = clickedIdx
			if p.Entries[clickedIdx].IsDir {
				return m.enterDir()
			}
			// Double-click on file: toggle select
			p.ToggleSelect()
			return m, nil
		}

		// Single click: move cursor
		p.Cursor = clickedIdx
		return m, nil

	case tea.MouseButtonRight:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// Right-click: toggle selection
		panelWidth := m.width / 2
		clickedPanel := 0
		if x >= panelWidth {
			clickedPanel = 1
		}
		m.activePanel = clickedPanel
		p := m.activeP()

		entryY := y - m.panelContentY
		scrollStart := 0
		contentHeight := p.Height - 4
		if p.Cursor >= contentHeight {
			scrollStart = p.Cursor - contentHeight + 1
		}
		clickedIdx := scrollStart + entryY
		if clickedIdx >= 0 && clickedIdx < len(p.Entries) {
			p.Cursor = clickedIdx
			p.ToggleSelect()
		}
		return m, nil
	}

	return m, nil
}

// handleButtonClick handles clicks on the action bar buttons
func (m ExplorerModel) handleButtonClick(x int) (tea.Model, tea.Cmd) {
	for _, btn := range m.buttons {
		if x >= btn.x1 && x < btn.x2 {
			switch btn.id {
			case btnUpload:
				m.activePanel = 0 // select from local
				return m.copyFilesFromPanel(0)
			case btnDownload:
				m.activePanel = 1 // select from remote
				return m.copyFilesFromPanel(1)
			case btnRefresh:
				return m.refresh()
			case btnQuit:
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m ExplorerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.activePanel = 1 - m.activePanel
		return m, nil

	case "up", "k":
		m.activeP().MoveUp()
		return m, nil

	case "down", "j":
		m.activeP().MoveDown()
		return m, nil

	case "pgup":
		for i := 0; i < 10; i++ {
			m.activeP().MoveUp()
		}
		return m, nil

	case "pgdown":
		for i := 0; i < 10; i++ {
			m.activeP().MoveDown()
		}
		return m, nil

	case "home":
		m.activeP().Cursor = 0
		return m, nil

	case "end":
		p := m.activeP()
		p.Cursor = len(p.Entries) - 1
		return m, nil

	case "enter":
		return m.enterDir()

	case " ":
		m.activeP().ToggleSelect()
		m.activeP().MoveDown()
		return m, nil

	case "c":
		return m.copyFilesFromPanel(m.activePanel)

	case "r", "f5":
		return m.refresh()

	case "a":
		p := m.activeP()
		if len(p.Selected) > 0 {
			p.Selected = make(map[int]bool)
		} else {
			for i := 1; i < len(p.Entries); i++ {
				p.Selected[i] = true
			}
		}
		return m, nil

	case "?":
		m.statusMsg = helpBarStyle.Render("Tab:switch | Arrows:move | Enter:open | Space/RightClick:select | c:copy | a:all | r:refresh | q:quit | Mouse:click/scroll/double-click")
		return m, nil
	}

	return m, nil
}

func (m *ExplorerModel) activeP() *Panel {
	if m.activePanel == 0 {
		return &m.localPanel
	}
	return &m.remotePanel
}

func (m ExplorerModel) enterDir() (tea.Model, tea.Cmd) {
	p := m.activeP()
	newPath, changed := p.EnterDir()
	if !changed {
		return m, nil
	}

	if p.IsRemote {
		m.remotePanel.Path = newPath
		m.remotePanel.Loading = true
		m.remotePanel.Entries = nil
		m.statusMsg = loadingStyle.Render(fmt.Sprintf("Loading %s...", newPath))
		return m, m.fetchRemoteDir(newPath)
	}

	m.localPanel.Path = newPath
	m.localPanel.LoadLocal()
	return m, nil
}

func (m ExplorerModel) copyFilesFromPanel(fromPanel int) (tea.Model, tea.Cmd) {
	var srcPanel *Panel
	if fromPanel == 0 {
		srcPanel = &m.localPanel
	} else {
		srcPanel = &m.remotePanel
	}

	entries := srcPanel.SelectedEntries()
	if len(entries) == 0 {
		m.statusMsg = "No files selected. Click or Space to select files first."
		return m, nil
	}

	var files []FileEntry
	for _, e := range entries {
		if !e.IsDir {
			files = append(files, e)
		}
	}
	if len(files) == 0 {
		m.statusMsg = "Directory copy not yet supported. Select files only."
		return m, nil
	}

	m.transfering = true

	if fromPanel == 0 {
		m.statusMsg = loadingStyle.Render(fmt.Sprintf("Uploading %d file(s) via S3...", len(files)))
		return m, m.uploadFiles(files)
	}

	m.statusMsg = loadingStyle.Render(fmt.Sprintf("Downloading %d file(s) via S3...", len(files)))
	return m, m.downloadFiles(files)
}

func (m ExplorerModel) refresh() (tea.Model, tea.Cmd) {
	m.localPanel.LoadLocal()
	m.remotePanel.Loading = true
	m.statusMsg = loadingStyle.Render("Refreshing...")
	return m, m.fetchRemoteDir(m.remotePanel.Path)
}

// Async commands

func (m ExplorerModel) detectRemoteHome() tea.Cmd {
	cfg := m.awsConfig
	instId := m.instanceId
	specifiedPath := m.remotePanel.Path
	return func() tea.Msg {
		if specifiedPath != "/home/ec2-user" {
			return remoteHomeMsg{home: specifiedPath}
		}
		output, err := RunRemoteCommand(cfg, instId, "echo $HOME")
		if err == nil {
			home := strings.TrimSpace(output)
			if home != "" && home != "$HOME" {
				return remoteHomeMsg{home: home}
			}
		}
		return remoteHomeMsg{home: specifiedPath}
	}
}

func (m ExplorerModel) fetchRemoteDir(dirPath string) tea.Cmd {
	cfg := m.awsConfig
	instId := m.instanceId
	return func() tea.Msg {
		entries, err := ListRemoteDir(cfg, instId, dirPath)
		if err != nil {
			return remoteDirErrMsg{err: err, path: dirPath}
		}
		return remoteDirMsg{entries: entries, path: dirPath}
	}
}

func (m ExplorerModel) uploadFiles(files []FileEntry) tea.Cmd {
	cfg := m.awsConfig
	instId := m.instanceId
	localBase := m.localPanel.Path
	remotePath := m.remotePanel.Path
	bucket := m.s3Bucket
	prefix := m.s3Prefix
	return func() tea.Msg {
		var uploaded int
		for _, f := range files {
			localFile := localBase + "/" + f.Name
			remoteFile := remotePath + "/" + f.Name
			if err := TransferUpload(cfg, instId, localFile, remoteFile, bucket, prefix); err != nil {
				return transferErrMsg{fmt.Errorf("%s: %w", f.Name, err)}
			}
			uploaded++
		}
		return transferDoneMsg{fmt.Sprintf("Uploaded %d file(s) to %s", uploaded, remotePath)}
	}
}

func (m ExplorerModel) downloadFiles(files []FileEntry) tea.Cmd {
	cfg := m.awsConfig
	instId := m.instanceId
	remotePath := m.remotePanel.Path
	localBase := m.localPanel.Path
	bucket := m.s3Bucket
	prefix := m.s3Prefix
	return func() tea.Msg {
		var downloaded int
		for _, f := range files {
			remoteFile := path.Join(remotePath, f.Name)
			localFile := localBase + "/" + f.Name
			if err := TransferDownload(cfg, instId, remoteFile, localFile, bucket, prefix); err != nil {
				return transferErrMsg{fmt.Errorf("%s: %w", f.Name, err)}
			}
			downloaded++
		}
		return transferDoneMsg{fmt.Sprintf("Downloaded %d file(s) to %s", downloaded, localBase)}
	}
}

// View renders the full TUI.
func (m ExplorerModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Dual panels side by side
	leftView := m.localPanel.View(m.activePanel == 0)
	rightView := m.remotePanel.View(m.activePanel == 1)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// Status bar
	status := m.statusMsg
	if status == "" {
		status = statusBarStyle.Render(fmt.Sprintf("  Instance: %s | S3: %s", m.instanceId, m.s3Bucket))
	}

	// Action buttons bar
	actionBar := m.renderActionBar()

	// Compose
	var b strings.Builder
	b.WriteString(panels)
	b.WriteString("\n")
	b.WriteString(status)
	b.WriteString("\n")
	b.WriteString(actionBar)

	return b.String()
}

// renderActionBar renders clickable action buttons and updates their positions.
func (m *ExplorerModel) renderActionBar() string {
	var parts []string
	x := 2

	for i := range m.buttons {
		rendered := m.buttons[i].style.Render(m.buttons[i].label)
		m.buttons[i].x1 = x
		m.buttons[i].x2 = x + lipgloss.Width(rendered)
		x = m.buttons[i].x2 + 2 // gap between buttons
		parts = append(parts, rendered)
	}

	return "  " + strings.Join(parts, "  ")
}
