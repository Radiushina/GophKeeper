package client

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/buildinfo"
	"github.com/Radiushina/GophKeeper/internal/domains/note"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type tuiScreen int

const (
	tuiHome tuiScreen = iota
	tuiLogin
	tuiNoteAdd
	tuiNoteEdit
	tuiNoteDelete
	tuiFileAdd
	tuiFileEdit
	tuiFileDelete
	tuiFileGet
)

const (
	noteTextPlaceholder = "secret (encrypted)"
	noteMetaPlaceholder = "title, tag or site (plaintext)"
	noteMetaHint        = "Meta: title, tag or site (not encrypted)"
	filePathPlaceholder = "path to local file"
	fileDestPlaceholder = "path to save"
	fileMetaHint        = "Meta: title or tag (not encrypted)"
)

type listKind int

const (
	listNotes listKind = iota
	listFiles
)

type tuiNote struct {
	id      uuid.UUID
	version int64
	text    string
	meta    string
}

type tuiFile struct {
	id      uuid.UUID
	version int64
	name    string
	meta    string
	data    []byte
}

type authResultMsg struct {
	login string
	err   error
}

type notesResultMsg struct {
	items  []tuiNote
	status string
	err    error
	stay   bool
	show   bool
}

type filesResultMsg struct {
	items  []tuiFile
	status string
	err    error
	stay   bool
	show   bool
}

type tuiModel struct {
	ctx      context.Context
	app      *App
	screen   tuiScreen
	inputs   []textinput.Model
	table    table.Model
	items    []tuiNote
	files    []tuiFile
	selected tuiNote
	selFile  tuiFile
	focus    int
	busy     bool
	status   string
	err      string
	listOpen bool
	listKind listKind
	width    int
	height   int
}

func RunTUI(ctx context.Context, app *App) error {
	if app != nil {
		prev := app.Log
		app.Log = zap.NewNop()
		defer func() { app.Log = prev }()
	}
	p := tea.NewProgram(newTUIModel(ctx, app), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func newTUIModel(ctx context.Context, app *App) tuiModel {
	login := textinput.New()
	login.Placeholder = "login"
	login.CharLimit = 64
	login.Width = 32

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'
	password.CharLimit = 128
	password.Width = 32

	extra := textinput.New()
	extra.Placeholder = noteMetaPlaceholder
	extra.CharLimit = 256
	extra.Width = 48

	km := table.DefaultKeyMap()
	km.HalfPageDown = key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down"))

	inner := contentWidth(0)
	t := table.New(
		table.WithColumns(noteColumns(inner)),
		table.WithWidth(inner),
		table.WithHeight(8),
		table.WithFocused(true),
		table.WithStyles(table.Styles{
			Header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1),
			Cell:     lipgloss.NewStyle().Padding(0, 1),
			Selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("63")),
		}),
	)
	t.KeyMap = km

	return tuiModel{
		ctx:    ctx,
		app:    app,
		screen: tuiHome,
		inputs: []textinput.Model{login, password, extra},
		table:  t,
	}
}

func contentWidth(termWidth int) int {
	box := max(40, termWidth-4)
	// border (2) + horizontal padding (4)
	return max(28, box-6)
}

func noteColumns(inner int) []table.Column {
	// 4 columns × Padding(0, 1) = 8 extra cells; keep the sum inside the box.
	budget := max(20, inner-8)
	idW, verW := 8, 3
	rest := max(6, budget-idW-verW)
	textW := max(4, rest*2/3)
	metaW := max(3, rest-textW)
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Text", Width: textW},
		{Title: "Meta", Width: metaW},
		{Title: "Version", Width: verW},
	}
}

func fileColumns(inner int) []table.Column {
	budget := max(20, inner-8)
	idW, verW := 8, 3
	rest := max(6, budget-idW-verW)
	nameW := max(4, rest*2/3)
	metaW := max(3, rest-nameW)
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Name", Width: nameW},
		{Title: "Meta", Width: metaW},
		{Title: "Version", Width: verW},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		inner := contentWidth(m.width)
		if m.listKind == listFiles {
			m.table.SetColumns(fileColumns(inner))
		} else {
			m.table.SetColumns(noteColumns(inner))
		}
		m.table.SetWidth(inner)
		m.table.SetHeight(min(12, max(5, m.height-16)))
		return m, nil
	case authResultMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.status = "signed in as " + msg.login
		m.screen = tuiHome
		m.listOpen = false
		m.items = nil
		m.files = nil
		m.table.SetRows(nil)
		return m.blurInputs(), nil
	case notesResultMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m = m.applyNotes(msg.items)
		if msg.show {
			m.listOpen = true
			m.listKind = listNotes
		}
		if msg.status != "" {
			m.status = msg.status
		}
		if msg.stay {
			return m, nil
		}
		m.screen = tuiHome
		return m.blurInputs(), nil
	case filesResultMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m = m.applyFiles(msg.items)
		if msg.show {
			m.listOpen = true
			m.listKind = listFiles
		}
		if msg.status != "" {
			m.status = msg.status
		}
		if msg.stay {
			return m, nil
		}
		m.screen = tuiHome
		return m.blurInputs(), nil
	case tea.InterruptMsg:
		return m, tea.Quit
	case tea.KeyMsg:
		key := shortcutKey(msg)
		if key == "ctrl+c" || (m.screen == tuiHome && key == "q") {
			return m, tea.Quit
		}
		if m.screen == tuiHome {
			return m.updateHome(msg, key)
		}
		return m.updateForm(msg)
	}
	if m.screen != tuiHome {
		return m.updateInputs(msg)
	}
	return m, nil
}

func (m tuiModel) signedIn() bool {
	return m.app != nil && m.app.User() != "" && m.app.Token() != ""
}

func shortcutKey(msg tea.KeyMsg) string {
	key := strings.ToLower(msg.String())
	switch key {
	case "й":
		return "q"
	case "д":
		return "l"
	case "т":
		return "n"
	case "а":
		return "f"
	case "ф":
		return "a"
	case "у":
		return "e"
	case "в":
		return "d"
	case "м":
		return "v"
	case "ы":
		return "s"
	default:
		return key
	}
}

func (m tuiModel) updateHome(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "l":
		if m.signedIn() {
			return m, nil
		}
		return m.openForm(tuiLogin)
	case "esc":
		if m.listOpen {
			m.listOpen = false
			m.err = ""
			m.status = ""
			return m, nil
		}
	case "n":
		if !m.signedIn() {
			return m, nil
		}
		m.busy = true
		m.err = ""
		return m, m.loadNotes("", false, true)
	case "f":
		if !m.signedIn() {
			return m, nil
		}
		m.busy = true
		m.err = ""
		return m, m.loadFiles("", false, true)
	case "a":
		if !m.signedIn() {
			return m, nil
		}
		if m.listOpen && m.listKind == listFiles {
			return m.openForm(tuiFileAdd)
		}
		return m.openForm(tuiNoteAdd)
	case "e", "enter":
		if !m.signedIn() {
			return m, nil
		}
		if m.listOpen && m.listKind == listFiles {
			return m.openSelectedFile(tuiFileEdit)
		}
		return m.openSelected(tuiNoteEdit)
	case "d":
		if !m.signedIn() {
			return m, nil
		}
		if m.listOpen && m.listKind == listFiles {
			return m.openSelectedFile(tuiFileDelete)
		}
		return m.openSelected(tuiNoteDelete)
	case "s":
		if !m.signedIn() || !(m.listOpen && m.listKind == listFiles) {
			return m, nil
		}
		return m.openSelectedFile(tuiFileGet)
	case "v":
		var b strings.Builder
		buildinfo.Fprint(&b)
		m.status = strings.TrimSpace(b.String())
		m.err = ""
		return m, nil
	}
	if m.signedIn() && m.listOpen {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m tuiModel) selectedNote() (tuiNote, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.items) {
		return tuiNote{}, false
	}
	return m.items[i], true
}

func (m tuiModel) openSelected(screen tuiScreen) (tea.Model, tea.Cmd) {
	n, ok := m.selectedNote()
	if !ok {
		m.err = "select a note"
		return m, nil
	}
	next, cmd := m.openForm(screen)
	got := next.(tuiModel)
	got.selected = n
	if screen == tuiNoteEdit {
		got.inputs[0].SetValue(n.text)
		got.inputs[1].SetValue(n.meta)
	}
	return got, cmd
}

func (m tuiModel) selectedFile() (tuiFile, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.files) {
		return tuiFile{}, false
	}
	return m.files[i], true
}

func (m tuiModel) openSelectedFile(screen tuiScreen) (tea.Model, tea.Cmd) {
	n, ok := m.selectedFile()
	if !ok {
		m.err = "select a file"
		return m, nil
	}
	next, cmd := m.openForm(screen)
	got := next.(tuiModel)
	got.selFile = n
	if screen == tuiFileEdit {
		got.inputs[1].SetValue(n.meta)
	}
	if screen == tuiFileGet {
		got.inputs[0].SetValue(n.name)
	}
	return got, cmd
}

func (m tuiModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = tuiHome
		m.err = ""
		return m.blurInputs(), nil
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		n := m.fieldCount()
		if n == 0 {
			return m, nil
		}
		if msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftTab {
			m.focus = (m.focus + n - 1) % n
		} else {
			m.focus = (m.focus + 1) % n
		}
		return m.focusInputs()
	case tea.KeyEnter:
		if m.busy {
			return m, nil
		}
		return m.submit()
	}
	return m.updateInputs(msg)
}

func (m tuiModel) fieldCount() int {
	switch m.screen {
	case tuiNoteDelete, tuiFileDelete:
		return 0
	case tuiFileGet:
		return 1
	case tuiNoteEdit, tuiNoteAdd, tuiFileEdit, tuiFileAdd, tuiLogin:
		return 2
	default:
		return 2
	}
}

func (m tuiModel) openForm(screen tuiScreen) (tea.Model, tea.Cmd) {
	m.screen = screen
	m.err = ""
	m.status = ""
	m.focus = 0
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		m.inputs[i].EchoMode = textinput.EchoNormal
	}
	switch screen {
	case tuiNoteAdd, tuiNoteEdit:
		m.inputs[0].Placeholder = noteTextPlaceholder
		m.inputs[1].Placeholder = noteMetaPlaceholder
	case tuiFileAdd, tuiFileEdit:
		m.inputs[0].Placeholder = filePathPlaceholder
		m.inputs[0].CharLimit = 4096
		m.inputs[1].Placeholder = noteMetaPlaceholder
	case tuiFileGet:
		m.inputs[0].Placeholder = fileDestPlaceholder
		m.inputs[0].CharLimit = 4096
	default:
		m.inputs[0].Placeholder = "login"
		m.inputs[0].CharLimit = 64
		m.inputs[1].Placeholder = "password"
		m.inputs[1].EchoMode = textinput.EchoPassword
		m.inputs[1].EchoCharacter = '•'
	}
	return m.focusInputs()
}

func (m tuiModel) submit() (tea.Model, tea.Cmd) {
	switch m.screen {
	case tuiNoteAdd:
		return m.submitNote()
	case tuiNoteEdit:
		return m.submitNoteEdit()
	case tuiNoteDelete:
		return m.submitNoteDelete()
	case tuiFileAdd:
		return m.submitFile()
	case tuiFileEdit:
		return m.submitFileEdit()
	case tuiFileDelete:
		return m.submitFileDelete()
	case tuiFileGet:
		return m.submitFileGet()
	}
	login := strings.TrimSpace(m.inputs[0].Value())
	password := m.inputs[1].Value()
	if login == "" || password == "" {
		m.err = "login and password are required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	return m, func() tea.Msg {
		return authResultMsg{login: login, err: Login(ctx, app, login, password)}
	}
}

func (m tuiModel) submitNote() (tea.Model, tea.Cmd) {
	text := m.inputs[0].Value()
	meta := m.inputs[1].Value()
	if strings.TrimSpace(text) == "" && strings.TrimSpace(meta) == "" {
		m.err = "text or meta is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	show := m.listOpen
	return m, func() tea.Msg {
		if err := writeNoteAdd(ctx, app, text, meta); err != nil {
			return notesResultMsg{err: err}
		}
		items, err := fetchTuiNotes(ctx, app)
		if err != nil {
			return notesResultMsg{err: err}
		}
		return notesResultMsg{items: items, status: "note saved", show: show}
	}
}

func (m tuiModel) submitNoteEdit() (tea.Model, tea.Cmd) {
	if m.selected.id == uuid.Nil {
		m.err = "select a note"
		return m, nil
	}
	text := m.inputs[0].Value()
	meta := m.inputs[1].Value()
	if strings.TrimSpace(text) == "" && strings.TrimSpace(meta) == "" {
		m.err = "text or meta is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selected.id
	return m, func() tea.Msg {
		if err := writeNoteUpdate(ctx, app, id, text, meta); err != nil {
			return notesResultMsg{err: err}
		}
		items, err := fetchTuiNotes(ctx, app)
		if err != nil {
			return notesResultMsg{err: err}
		}
		return notesResultMsg{items: items, status: "note updated", show: true}
	}
}

func (m tuiModel) submitNoteDelete() (tea.Model, tea.Cmd) {
	if m.selected.id == uuid.Nil {
		m.err = "select a note"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selected.id
	return m, func() tea.Msg {
		if err := writeNoteDelete(ctx, app, id); err != nil {
			return notesResultMsg{err: err}
		}
		items, err := fetchTuiNotes(ctx, app)
		if err != nil {
			return notesResultMsg{err: err}
		}
		return notesResultMsg{items: items, status: "note deleted", show: true}
	}
}

func (m tuiModel) submitFile() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.inputs[0].Value())
	meta := m.inputs[1].Value()
	if path == "" {
		m.err = "path is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	show := m.listOpen
	return m, func() tea.Msg {
		if err := writeFileAdd(ctx, app, path, meta); err != nil {
			return filesResultMsg{err: err}
		}
		items, err := fetchTuiFiles(ctx, app)
		if err != nil {
			return filesResultMsg{err: err}
		}
		return filesResultMsg{items: items, status: "file saved", show: show}
	}
}

func (m tuiModel) submitFileEdit() (tea.Model, tea.Cmd) {
	if m.selFile.id == uuid.Nil {
		m.err = "select a file"
		return m, nil
	}
	path := strings.TrimSpace(m.inputs[0].Value())
	meta := m.inputs[1].Value()
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selFile.id
	keep := FilePlain{Name: m.selFile.name, Data: m.selFile.data}
	return m, func() tea.Msg {
		if err := writeFileUpdate(ctx, app, id, path, meta, keep); err != nil {
			return filesResultMsg{err: err}
		}
		items, err := fetchTuiFiles(ctx, app)
		if err != nil {
			return filesResultMsg{err: err}
		}
		return filesResultMsg{items: items, status: "file updated", show: true}
	}
}

func (m tuiModel) submitFileDelete() (tea.Model, tea.Cmd) {
	if m.selFile.id == uuid.Nil {
		m.err = "select a file"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selFile.id
	return m, func() tea.Msg {
		if err := writeFileDelete(ctx, app, id); err != nil {
			return filesResultMsg{err: err}
		}
		items, err := fetchTuiFiles(ctx, app)
		if err != nil {
			return filesResultMsg{err: err}
		}
		return filesResultMsg{items: items, status: "file deleted", show: true}
	}
}

func (m tuiModel) submitFileGet() (tea.Model, tea.Cmd) {
	if m.selFile.id == uuid.Nil {
		m.err = "select a file"
		return m, nil
	}
	dest := strings.TrimSpace(m.inputs[0].Value())
	if dest == "" {
		m.err = "path is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selFile.id
	return m, func() tea.Msg {
		if _, _, _, err := downloadFile(ctx, app, id, dest); err != nil {
			return filesResultMsg{err: err}
		}
		items, err := fetchTuiFiles(ctx, app)
		if err != nil {
			return filesResultMsg{err: err}
		}
		return filesResultMsg{items: items, status: "file downloaded", show: true}
	}
}

func writeNoteAdd(ctx context.Context, app *App, text, meta string) error {
	blob, err := sealNote(app, NotePlain{Text: text})
	if err != nil {
		return err
	}
	res, err := app.Client.NoteCreate(ctx, &oas.CreateNote{
		ID:               uuid.New(),
		Version:          note.CreateVersion,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, io.Discard)
}

func writeNoteUpdate(ctx context.Context, app *App, id uuid.UUID, text, meta string) error {
	current, err := fetchNote(ctx, app, id)
	if err != nil {
		return err
	}
	blob, err := sealNote(app, NotePlain{Text: text})
	if err != nil {
		return err
	}
	res, err := app.Client.NoteUpdate(ctx, &oas.UpdateNote{
		Version:          current.Version,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	}, oas.NoteUpdateParams{ID: id})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, io.Discard)
}

func writeNoteDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.NoteDelete(ctx, oas.NoteDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleNoteRes(app, res, io.Discard)
}

func fetchTuiNotes(ctx context.Context, app *App) ([]tuiNote, error) {
	res, err := app.Client.ListNotes(ctx, oas.ListNotesParams{})
	if err != nil {
		if isOffline(err) {
			return tuiNotesFromCache(app)
		}
		return nil, err
	}
	list, ok := res.(*oas.NoteListRes)
	if !ok {
		return nil, noteMsg(res)
	}
	items := make([]tuiNote, 0, len(list.Items))
	live := make([]cachedNote, 0, len(list.Items))
	for i := range list.Items {
		n := list.Items[i]
		if n.DeletedAt.IsSet() {
			continue
		}
		plain, err := openNote(app, n)
		if err != nil {
			return nil, err
		}
		items = append(items, tuiNote{id: n.ID, version: n.Version, text: plain.Text, meta: plain.Meta})
		live = append(live, cachedNote{ID: n.ID, Version: n.Version, Text: plain.Text, Meta: plain.Meta})
	}
	_ = saveNotesCache(app, live)
	return items, nil
}

func tuiNotesFromCache(app *App) ([]tuiNote, error) {
	notes, err := loadNotesCache(app)
	if err != nil {
		return nil, fmt.Errorf("offline notes: %w", err)
	}
	items := make([]tuiNote, 0, len(notes))
	for _, n := range notes {
		items = append(items, tuiNote{id: n.ID, version: n.Version, text: n.Text, meta: n.Meta})
	}
	return items, nil
}

func (m tuiModel) applyNotes(items []tuiNote) tuiModel {
	m.items = items
	m.listKind = listNotes
	inner := contentWidth(m.width)
	m.table.SetColumns(noteColumns(inner))
	rows := make([]table.Row, 0, len(items))
	for _, n := range items {
		rows = append(rows, table.Row{shortID(n.id), n.text, n.meta, fmt.Sprintf("%d", n.version)})
	}
	m.table.SetRows(rows)
	m.table.Focus()
	return m
}

func (m tuiModel) applyFiles(items []tuiFile) tuiModel {
	m.files = items
	m.listKind = listFiles
	inner := contentWidth(m.width)
	m.table.SetColumns(fileColumns(inner))
	rows := make([]table.Row, 0, len(items))
	for _, n := range items {
		rows = append(rows, table.Row{shortID(n.id), n.name, n.meta, fmt.Sprintf("%d", n.version)})
	}
	m.table.SetRows(rows)
	m.table.Focus()
	return m
}

func fetchTuiFiles(ctx context.Context, app *App) ([]tuiFile, error) {
	res, err := app.Client.ListFiles(ctx, oas.ListFilesParams{})
	if err != nil {
		return nil, err
	}
	list, ok := res.(*oas.FileListRes)
	if !ok {
		return nil, fileMsg(res)
	}
	items := make([]tuiFile, 0, len(list.Items))
	for i := range list.Items {
		n := list.Items[i]
		if n.DeletedAt.IsSet() {
			continue
		}
		items = append(items, tuiFile{id: n.ID, version: n.Version, name: n.Name.Value, meta: n.Meta.Value})
	}
	return items, nil
}

func shortID(id uuid.UUID) string {
	s := id.String()
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func (m tuiModel) loadNotes(status string, stay, show bool) tea.Cmd {
	app := m.app
	ctx := m.ctx
	return func() tea.Msg {
		items, err := fetchTuiNotes(ctx, app)
		if err != nil {
			return notesResultMsg{err: err, stay: stay, show: show}
		}
		return notesResultMsg{items: items, status: status, stay: stay, show: show}
	}
}

func (m tuiModel) loadFiles(status string, stay, show bool) tea.Cmd {
	app := m.app
	ctx := m.ctx
	return func() tea.Msg {
		items, err := fetchTuiFiles(ctx, app)
		if err != nil {
			return filesResultMsg{err: err, stay: stay, show: show}
		}
		return filesResultMsg{items: items, status: status, stay: stay, show: show}
	}
}

func (m tuiModel) blurInputs() tuiModel {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	return m
}

func (m tuiModel) focusInputs() (tea.Model, tea.Cmd) {
	n := m.fieldCount()
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focus && i < n {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("GophKeeper")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("vault for notes and files")

	server := m.serverURL()
	session := "not signed in"
	if user := m.app.User(); user != "" && m.app.Token() != "" {
		session = "signed in as " + user
	}

	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Render(
		fmt.Sprintf("server  %s\nsession %s", server, session),
	)

	body := m.homeBody()
	if m.screen != tuiHome {
		body = m.formBody()
	}

	parts := []string{title, subtitle, "", meta, "", body}
	if m.status != "" {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.status))
	}
	if m.err != "" {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(m.err))
	}
	if m.busy {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("working…"))
	}
	parts = append(parts, "", m.help())

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(max(40, m.width-4))
	return box.Render(content)
}

func (m tuiModel) homeBody() string {
	if !m.signedIn() {
		return "Sign in to open the vault."
	}
	if !m.listOpen {
		return "Choose a command."
	}
	if m.listKind == listFiles {
		if len(m.files) == 0 {
			return "No files yet.\nPress a to add one."
		}
		return m.filesGrid()
	}
	if len(m.items) == 0 {
		return "No notes yet.\nPress a to add one."
	}
	return m.notesGrid()
}

func (m tuiModel) notesGrid() string {
	sel := m.table.Cursor()
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("63"))).
		BorderRow(true).
		BorderColumn(true).
		Headers("ID", "Text", "Meta", "Ver").
		Width(contentWidth(m.width)).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == lgtable.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("212"))
			}
			if row == sel {
				return s.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("63"))
			}
			return s
		})
	for _, n := range m.items {
		t.Row(shortID(n.id), n.text, n.meta, fmt.Sprintf("%d", n.version))
	}
	return t.String()
}

func (m tuiModel) filesGrid() string {
	sel := m.table.Cursor()
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("63"))).
		BorderRow(true).
		BorderColumn(true).
		Headers("ID", "Name", "Meta", "Ver").
		Width(contentWidth(m.width)).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == lgtable.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("212"))
			}
			if row == sel {
				return s.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("63"))
			}
			return s
		})
	for _, n := range m.files {
		t.Row(shortID(n.id), n.name, n.meta, fmt.Sprintf("%d", n.version))
	}
	return t.String()
}

func (m tuiModel) formBody() string {
	heading := "Login"
	switch m.screen {
	case tuiNoteAdd:
		heading = "Add note"
	case tuiNoteEdit:
		heading = "Edit note"
	case tuiNoteDelete:
		heading = "Delete note"
	case tuiFileAdd:
		heading = "Add file"
	case tuiFileEdit:
		heading = "Edit file"
	case tuiFileDelete:
		heading = "Delete file"
	case tuiFileGet:
		heading = "Download file"
	}
	var b strings.Builder
	b.WriteString(heading)
	if m.screen == tuiNoteAdd || m.screen == tuiNoteEdit {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(noteMetaHint))
	}
	if m.screen == tuiFileAdd || m.screen == tuiFileEdit {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(fileMetaHint))
	}
	if m.selected.id != uuid.Nil && (m.screen == tuiNoteEdit || m.screen == tuiNoteDelete) {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			fmt.Sprintf("%s  %s", shortID(m.selected.id), m.selected.text),
		))
	}
	if m.selFile.id != uuid.Nil && (m.screen == tuiFileEdit || m.screen == tuiFileDelete || m.screen == tuiFileGet) {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
			fmt.Sprintf("%s  %s", shortID(m.selFile.id), m.selFile.name),
		))
	}
	if (m.screen == tuiNoteDelete && m.selected.id != uuid.Nil) || (m.screen == tuiFileDelete && m.selFile.id != uuid.Nil) {
		b.WriteString("\n\nenter confirm")
		return b.String()
	}
	b.WriteString("\n")
	for i := 0; i < m.fieldCount(); i++ {
		b.WriteString("\n")
		b.WriteString(m.inputs[i].View())
	}
	return b.String()
}

func (m tuiModel) help() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if m.screen == tuiHome {
		if m.signedIn() {
			if m.listOpen {
				refresh := "n refresh"
				if m.listKind == listFiles {
					refresh = "f refresh"
					return style.Render("↑↓ move   enter/e edit   s download   d delete   a add   " + refresh + "   esc back   q quit")
				}
				return style.Render("↑↓ move   enter/e edit   d delete   a add   " + refresh + "   esc back   q quit")
			}
			return style.Render("n notes   f files   a add   v version   q quit")
		}
		return style.Render("l login   v version   q quit")
	}
	return style.Render("enter submit   tab next   esc back")
}

func (m tuiModel) serverURL() string {
	if m.app != nil && m.app.Server != "" {
		return m.app.Server
	}
	return "unknown"
}
