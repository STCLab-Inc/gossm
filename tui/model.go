package tui

import (
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages
type remoteDirMsg struct {
	entries []FileEntry
	path    string
}
type remoteDirErrMsg struct{ err error }

type transferDoneMsg struct{ msg string }
type transferErrMsg struct{ err error }

// ExplorerModel is the main dual-pane file manager model.
type ExplorerModel struct {
	localPanel  Panel
	remotePanel Panel
	activePanel int // 0=local, 1=remote
	width       int
	height      int
	statusMsg   string
	transfering bool

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
	remote := NewPanel("Remote", remoteDir, true)
	remote.Loading = true

	return ExplorerModel{
		localPanel:  local,
		remotePanel: remote,
		activePanel: 0,
		awsConfig:   cfg,
		instanceId:  instanceId,
		s3Bucket:    s3Bucket,
		s3Prefix:    s3Prefix,
	}
}

func (m ExplorerModel) Init() tea.Cmd {
	// Load local dir and fetch remote dir
	m.localPanel.LoadLocal()
	return m.fetchRemoteDir(m.remotePanel.Path)
}

func (m ExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		panelWidth := m.width / 2
		panelHeight := m.height - 3 // room for help bar
		m.localPanel.Width = panelWidth
		m.localPanel.Height = panelHeight
		m.remotePanel.Width = m.width - panelWidth
		m.remotePanel.Height = panelHeight
		return m, nil

	case remoteDirMsg:
		m.remotePanel.Path = msg.path
		m.remotePanel.SetRemoteEntries(msg.entries)
		m.statusMsg = fmt.Sprintf("Remote: %s (%d items)", msg.path, len(msg.entries)-1)
		return m, nil

	case remoteDirErrMsg:
		m.remotePanel.Loading = false
		m.remotePanel.Err = msg.err
		m.statusMsg = errorStyle.Render("Remote error: " + msg.err.Error())
		return m, nil

	case transferDoneMsg:
		m.transfering = false
		m.statusMsg = successStyle.Render(msg.msg)
		// Refresh both panels
		m.localPanel.LoadLocal()
		return m, m.fetchRemoteDir(m.remotePanel.Path)

	case transferErrMsg:
		m.transfering = false
		m.statusMsg = errorStyle.Render("Transfer failed: " + msg.err.Error())
		return m, nil

	case tea.KeyMsg:
		if m.transfering {
			return m, nil // block input during transfer
		}
		return m.handleKey(msg)
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

	case "enter":
		return m.enterDir()

	case " ":
		m.activeP().ToggleSelect()
		m.activeP().MoveDown()
		return m, nil

	case "c":
		return m.copyFiles()

	case "r", "f5":
		return m.refresh()

	case "a":
		// Toggle select all (except ..)
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
		if m.statusMsg == "" {
			m.statusMsg = "Tab:switch  Enter:open  Space:select  c:copy  a:all  r:refresh  q:quit"
		} else {
			m.statusMsg = ""
		}
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
		return m, m.fetchRemoteDir(newPath)
	}

	m.localPanel.Path = newPath
	m.localPanel.LoadLocal()
	return m, nil
}

func (m ExplorerModel) copyFiles() (tea.Model, tea.Cmd) {
	entries := m.activeP().SelectedEntries()
	if len(entries) == 0 {
		m.statusMsg = "No files selected. Use Space to select or position cursor on a file."
		return m, nil
	}

	// Skip directories for now
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

	if m.activePanel == 0 {
		// Local -> Remote (Upload)
		m.statusMsg = loadingStyle.Render(fmt.Sprintf("Uploading %d file(s)...", len(files)))
		return m, m.uploadFiles(files)
	}

	// Remote -> Local (Download)
	m.statusMsg = loadingStyle.Render(fmt.Sprintf("Downloading %d file(s)...", len(files)))
	return m, m.downloadFiles(files)
}

func (m ExplorerModel) refresh() (tea.Model, tea.Cmd) {
	m.localPanel.LoadLocal()
	m.remotePanel.Loading = true
	m.statusMsg = "Refreshing..."
	return m, m.fetchRemoteDir(m.remotePanel.Path)
}

// Async commands

func (m ExplorerModel) fetchRemoteDir(dirPath string) tea.Cmd {
	cfg := m.awsConfig
	instId := m.instanceId
	return func() tea.Msg {
		entries, err := ListRemoteDir(cfg, instId, dirPath)
		if err != nil {
			return remoteDirErrMsg{err}
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
		status = statusBarStyle.Render(fmt.Sprintf("Instance: %s | S3: %s", m.instanceId, m.s3Bucket))
	}

	// Help bar
	help := helpBarStyle.Render("  Tab:switch | ↑↓:navigate | Enter:open | Space:select | c:copy | a:all | r:refresh | ?:help | q:quit")

	// Compose
	var b strings.Builder
	b.WriteString(panels)
	b.WriteString("\n")
	b.WriteString(status)
	b.WriteString("\n")
	b.WriteString(help)

	return b.String()
}
