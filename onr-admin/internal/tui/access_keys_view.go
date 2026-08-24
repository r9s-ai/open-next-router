package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/r9s-ai/open-next-router/pkg/controlplane"
)

type accessViewState int

const (
	accessListState accessViewState = iota
	accessDetailState
	accessFormState
	accessConfirmState
	accessSecretState
	accessMigrateState
)

type accessKeysModel struct {
	service      *adminService
	state        accessViewState
	table        table.Model
	filter       textinput.Model
	inputs       []textinput.Model
	metadata     textarea.Model
	focus        int
	records      []controlplane.AccessKeyRecord
	selected     *controlplane.AccessKeyRecord
	subjectState controlplane.SubjectState
	action       string
	secret       string
	copied       bool
	migratePath  textinput.Model
	report       migrationReport
	migrateReady bool
	err          error
	notice       string
	width        int
	height       int
}

type accessKeysMsg struct {
	records []controlplane.AccessKeyRecord
	err     error
}
type accessActionMsg struct {
	action string
	secret string
	report migrationReport
	err    error
}

type subjectStateMsg struct {
	state controlplane.SubjectState
	err   error
}

func newAccessKeysModel(service *adminService) accessKeysModel {
	filter := textinput.New()
	filter.Prompt = "filter: "
	filter.Placeholder = "name / subject / status"
	filter.CharLimit = 120
	filter.Blur()
	inputs := make([]textinput.Model, 4)
	labels := []string{"name", "subject type", "subject id", "expires at (RFC3339, optional)"}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Prompt = labels[i] + ": "
		inputs[i].CharLimit = 256
		inputs[i].Blur()
	}
	inputs[1].SetValue("api_key")
	metadata := textarea.New()
	metadata.Prompt = "metadata: "
	metadata.Placeholder = "key=value, one per line"
	metadata.SetHeight(4)
	metadata.Blur()
	path := textinput.New()
	path.Prompt = "keys.yaml: "
	path.SetValue("./keys.yaml")
	path.Blur()
	t := table.New(table.WithColumns([]table.Column{
		{Title: "Name", Width: 18}, {Title: "Status", Width: 10}, {Title: "Subject", Width: 22},
		{Title: "Expires", Width: 20}, {Title: "Version", Width: 8},
	}), table.WithFocused(true), table.WithHeight(10))
	return accessKeysModel{service: service, state: accessListState, table: t, filter: filter, inputs: inputs, metadata: metadata, migratePath: path}
}

func (m accessKeysModel) Init() tea.Cmd { return m.loadCmd() }

func (m accessKeysModel) loadCmd() tea.Cmd {
	service := m.service
	return func() tea.Msg {
		if service == nil {
			return accessKeysMsg{err: fmt.Errorf("admin service is unavailable")}
		}
		records, err := service.ListAccessKeys(context.Background())
		return accessKeysMsg{records: records, err: err}
	}
}

func (m accessKeysModel) Update(msg tea.Msg) (accessKeysModel, tea.Cmd) {
	switch msg := msg.(type) {
	case accessKeysMsg:
		m.err = msg.err
		if msg.err == nil {
			m.records = msg.records
			m.setRows()
		}
		return m, nil
	case accessActionMsg:
		m.err = msg.err
		if msg.err != nil {
			return m, nil
		}
		m.notice = msg.action + " completed"
		if msg.secret != "" {
			m.secret, m.copied, m.state = msg.secret, false, accessSecretState
			return m, nil
		}
		if msg.action == "migration" {
			m.report = msg.report
			m.migrateReady = false
			m.state = accessListState
			return m, m.loadCmd()
		}
		if msg.report.Total > 0 || msg.report.Migrated > 0 || len(msg.report.Conflicts) > 0 {
			m.report, m.migrateReady, m.state = msg.report, true, accessMigrateState
			return m, m.loadCmd()
		}
		m.state = accessListState
		return m, m.loadCmd()
	case subjectStateMsg:
		m.subjectState, m.err = msg.state, msg.err
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetWidth(maxInt(20, msg.Width-4))
		m.table.SetHeight(maxInt(5, msg.Height-12))
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	if m.state == accessListState {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m accessKeysModel) updateKey(msg tea.KeyMsg) (accessKeysModel, tea.Cmd) {
	switch m.state {
	case accessListState:
		return m.updateListKey(msg)
	case accessDetailState:
		if msg.String() == "esc" {
			m.state = accessListState
		}
	case accessFormState:
		return m.updateFormKey(msg)
	case accessConfirmState:
		if msg.String() == "esc" || msg.String() == "n" {
			m.state = accessListState
		}
		if msg.String() == "enter" || msg.String() == "y" {
			return m, m.actionCmd()
		}
	case accessSecretState:
		return m.updateSecretKey(msg)
	case accessMigrateState:
		return m.updateMigrateKey(msg)
	}
	return m, nil
}

func (m accessKeysModel) updateListKey(msg tea.KeyMsg) (accessKeysModel, tea.Cmd) {
	if m.filter.Focused() {
		if msg.String() == "esc" || msg.String() == "enter" {
			m.filter.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.setRows()
		return m, cmd
	}
	switch msg.String() {
	case "/":
		m.filter.Focus()
		return m, textinput.Blink
	case "esc":
		m.filter.Reset()
		m.setRows()
	case "R":
		return m, m.loadCmd()
	case "n":
		m.resetForm()
		m.state = accessFormState
		m.inputs[0].Focus()
		return m, textinput.Blink
	case "enter":
		m.selected = m.selectedRecord()
		if m.selected != nil {
			m.state = accessDetailState
			m.subjectState = controlplane.SubjectState{}
			return m, m.subjectStateCmd(*m.selected)
		}
	case "r":
		m.selected = m.selectedRecord()
		if m.selected != nil {
			m.action, m.state = "rotate", accessConfirmState
		}
	case "v":
		m.selected = m.selectedRecord()
		if m.selected != nil {
			m.action, m.state = "revoke", accessConfirmState
		}
	case "m":
		m.state, m.err, m.migrateReady, m.report = accessMigrateState, nil, false, migrationReport{}
		m.migratePath.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m accessKeysModel) updateFormKey(msg tea.KeyMsg) (accessKeysModel, tea.Cmd) {
	if msg.String() == "esc" {
		m.state = accessListState
		return m, nil
	}
	if msg.String() == "tab" || msg.String() == "shift+tab" {
		m.blurFormField()
		if msg.String() == "tab" {
			m.focus = (m.focus + 1) % (len(m.inputs) + 1)
		} else {
			m.focus = (m.focus + len(m.inputs)) % (len(m.inputs) + 1)
		}
		return m, m.focusFormField()
	}
	if (msg.String() == "enter" && m.focus != len(m.inputs)) || msg.String() == "ctrl+enter" {
		return m, m.createCmd()
	}
	var cmd tea.Cmd
	if m.focus == len(m.inputs) {
		m.metadata, cmd = m.metadata.Update(msg)
	} else {
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	}
	return m, cmd
}

func (m accessKeysModel) updateSecretKey(msg tea.KeyMsg) (accessKeysModel, tea.Cmd) {
	if msg.String() == "c" {
		if err := clipboard.WriteAll(m.secret); err != nil {
			m.err = err
		} else {
			m.copied = true
		}
	}
	if msg.String() == "enter" || msg.String() == "esc" {
		m.secret = ""
		m.state = accessListState
		return m, m.loadCmd()
	}
	return m, nil
}

func (m accessKeysModel) updateMigrateKey(msg tea.KeyMsg) (accessKeysModel, tea.Cmd) {
	if msg.String() == "esc" {
		m.migratePath.Blur()
		m.state = accessListState
		return m, nil
	}
	if msg.String() == "enter" {
		if m.migrateReady {
			return m, m.migrateApplyCmd()
		}
		return m, m.migrateCmd()
	}
	var cmd tea.Cmd
	m.migratePath, cmd = m.migratePath.Update(msg)
	return m, cmd
}

func (m accessKeysModel) createCmd() tea.Cmd {
	values := make([]string, len(m.inputs))
	for i := range m.inputs {
		values[i] = strings.TrimSpace(m.inputs[i].Value())
	}
	metadataText := strings.TrimSpace(m.metadata.Value())
	return func() tea.Msg {
		if values[0] == "" || values[1] == "" || values[2] == "" {
			return accessActionMsg{err: fmt.Errorf("name, subject type, and subject ID are required")}
		}
		var expires *time.Time
		if values[3] != "" {
			parsed, err := time.Parse(time.RFC3339, values[3])
			if err != nil || parsed.Before(time.Now().UTC()) {
				return accessActionMsg{err: fmt.Errorf("expires at must be a future RFC3339 timestamp")}
			}
			expires = &parsed
		}
		metadata, err := parseMetadata(metadataText)
		if err != nil {
			return accessActionMsg{err: err}
		}
		secret, err := m.service.CreateAccessKey(context.Background(), values[0], values[1], values[2], expires, metadata)
		return accessActionMsg{action: "create", secret: secret, err: err}
	}
}

func (m accessKeysModel) actionCmd() tea.Cmd {
	name, action, service := m.selected.Name, m.action, m.service
	return func() tea.Msg {
		if action == "revoke" {
			return accessActionMsg{action: action, err: service.RevokeAccessKey(context.Background(), name)}
		}
		secret, err := service.RotateAccessKey(context.Background(), name)
		return accessActionMsg{action: action, secret: secret, err: err}
	}
}

func (m accessKeysModel) subjectStateCmd(record controlplane.AccessKeyRecord) tea.Cmd {
	service := m.service
	return func() tea.Msg {
		state, err := service.SubjectState(context.Background(), record.SubjectType, record.SubjectID)
		return subjectStateMsg{state: state, err: err}
	}
}

func (m accessKeysModel) migrateCmd() tea.Cmd {
	path, service := strings.TrimSpace(m.migratePath.Value()), m.service
	return func() tea.Msg {
		report, err := service.Migrate(context.Background(), path, true)
		return accessActionMsg{action: "migration dry-run", report: report, err: err}
	}
}

func (m *accessKeysModel) resetForm() {
	for i := range m.inputs {
		m.inputs[i].Reset()
		m.inputs[i].Blur()
	}
	m.inputs[1].SetValue("api_key")
	m.metadata.Reset()
	m.focus = 0
}

func (m *accessKeysModel) blurFormField() {
	if m.focus == len(m.inputs) {
		m.metadata.Blur()
		return
	}
	m.inputs[m.focus].Blur()
}

func (m *accessKeysModel) focusFormField() tea.Cmd {
	if m.focus == len(m.inputs) {
		return m.metadata.Focus()
	}
	return m.inputs[m.focus].Focus()
}

func (m accessKeysModel) migrateApplyCmd() tea.Cmd {
	path, service := strings.TrimSpace(m.migratePath.Value()), m.service
	return func() tea.Msg {
		report, err := service.Migrate(context.Background(), path, false)
		return accessActionMsg{action: "migration", report: report, err: err}
	}
}

func (m *accessKeysModel) setRows() {
	filter := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	rows := make([]table.Row, 0, len(m.records))
	for _, record := range m.records {
		status := record.Status
		if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) && status == "active" {
			status = "expired"
		}
		text := strings.ToLower(strings.Join([]string{record.Name, status, record.SubjectType, record.SubjectID}, " "))
		if filter != "" && !strings.Contains(text, filter) {
			continue
		}
		expires := "-"
		if record.ExpiresAt != nil {
			expires = record.ExpiresAt.Format("2006-01-02 15:04")
		}
		rows = append(rows, table.Row{record.Name, status, record.SubjectType + "/" + record.SubjectID, expires, strconv.FormatInt(record.Version, 10)})
	}
	m.table.SetRows(rows)
}

func (m accessKeysModel) selectedRecord() *controlplane.AccessKeyRecord {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}
	for i := range m.records {
		if m.records[i].Name == row[0] {
			record := m.records[i]
			return &record
		}
	}
	return nil
}

func (m accessKeysModel) View() string {
	switch m.state {
	case accessDetailState:
		if m.selected == nil {
			return "No access key selected"
		}
		return renderAccessDetail(*m.selected, m.subjectState)
	case accessFormState:
		var b strings.Builder
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Create access key"))
		b.WriteString("\n\n")
		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			b.WriteString("\n")
		}
		b.WriteString(m.metadata.View())
		b.WriteString("\n")
		b.WriteString("\nEnter create · Tab next field · Esc cancel")
		return b.String()
	case accessConfirmState:
		return fmt.Sprintf("Confirm %s\n\nname: %s\nsubject: %s/%s\n\nEnter/y confirm · n/Esc cancel", m.action, m.selected.Name, m.selected.SubjectType, m.selected.SubjectID)
	case accessSecretState:
		copied := "not copied"
		if m.copied {
			copied = "copied to clipboard"
		}
		return "Access key secret (shown once)\n\n" + m.secret + "\n\n" + copied + "\nPress c to copy, Enter/Esc when saved."
	case accessMigrateState:
		if m.migrateReady {
			return fmt.Sprintf("Migration dry-run report\n\nEnter apply · Esc cancel\n\ntotal=%d would_migrate=%d migrated=%d conflicts=%d skipped=%d", m.report.Total, m.report.WouldMigrate, m.report.Migrated, len(m.report.Conflicts), len(m.report.Skipped))
		}
		return fmt.Sprintf("Migrate access keys\n\n%s\n\nEnter dry-run · Esc cancel", m.migratePath.View())
	default:
		return m.filter.View() + "\n" + m.table.View() + "\n n create · Enter detail · r rotate · v revoke · m migrate · R refresh · / filter"
	}
}

func renderAccessDetail(record controlplane.AccessKeyRecord, state controlplane.SubjectState) string {
	expires := "-"
	if record.ExpiresAt != nil {
		expires = record.ExpiresAt.Format(time.RFC3339)
	}
	metadata := "-"
	if len(record.Metadata) > 0 {
		metadata = fmt.Sprint(record.Metadata)
	}
	blocked := "false"
	if state.Blocked {
		blocked = "true (" + valueOrDash(state.BlockedReason) + ")"
	}
	return fmt.Sprintf("Access key: %s\n\nstatus: %s\nsubject: %s/%s\nblocked: %s\ncreated: %s\nexpires: %s\nversion: %d\nmetadata: %s\n\nEsc back", record.Name, record.Status, record.SubjectType, record.SubjectID, blocked, record.CreatedAt.Format(time.RFC3339), expires, record.Version, metadata)
}

func parseMetadata(value string) (map[string]string, error) {
	result := map[string]string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("metadata must use key=value lines")
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
