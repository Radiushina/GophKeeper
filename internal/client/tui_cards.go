package client

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Radiushina/GophKeeper/gen/oas"
	"github.com/Radiushina/GophKeeper/internal/domains/card"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/google/uuid"
)

const (
	cardNumberPlaceholder = "card number (encrypted)"
	cardHolderPlaceholder = "holder name (encrypted)"
	cardExpiryPlaceholder = "MM/YY (encrypted)"
	cardCVVPlaceholder    = "CVV (encrypted)"
	cardMetaPlaceholder   = "bank / alias (plaintext)"
	cardMetaHint          = "Meta: bank or alias (not encrypted)"
)

type tuiCard struct {
	id      uuid.UUID
	version int64
	number  string
	holder  string
	expiry  string
	cvv     string
	meta    string
}

type cardsResultMsg struct {
	items  []tuiCard
	status string
	err    error
	stay   bool
	show   bool
}

func cardColumns(inner int) []table.Column {
	budget := max(20, inner-8)
	idW, verW := 8, 3
	rest := max(8, budget-idW-verW)
	numW := max(4, rest/3)
	holderW := max(3, rest/3)
	metaW := max(3, rest-numW-holderW)
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Number", Width: numW},
		{Title: "Holder", Width: holderW},
		{Title: "Meta", Width: metaW},
		{Title: "Version", Width: verW},
	}
}

func (m tuiModel) selectedCard() (tuiCard, bool) {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.cards) {
		return tuiCard{}, false
	}
	return m.cards[i], true
}

func (m tuiModel) openSelectedCard(screen tuiScreen) (tea.Model, tea.Cmd) {
	n, ok := m.selectedCard()
	if !ok {
		m.err = "select a card"
		return m, nil
	}
	next, cmd := m.openForm(screen)
	got := next.(tuiModel)
	got.selCard = n
	if screen == tuiCardEdit {
		got.inputs[0].SetValue(n.number)
		got.inputs[1].SetValue(n.holder)
		got.inputs[2].SetValue(n.expiry)
		got.inputs[3].SetValue(n.cvv)
		got.inputs[4].SetValue(n.meta)
	}
	return got, cmd
}

func (m tuiModel) submitCard() (tea.Model, tea.Cmd) {
	plain := CardPlain{
		Number: m.inputs[0].Value(),
		Holder: m.inputs[1].Value(),
		Expiry: m.inputs[2].Value(),
		CVV:    m.inputs[3].Value(),
	}
	meta := m.inputs[4].Value()
	if strings.TrimSpace(plain.Number) == "" {
		m.err = "number is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	show := m.listOpen
	return m, func() tea.Msg {
		if err := writeCardAdd(ctx, app, plain, meta); err != nil {
			return cardsResultMsg{err: err}
		}
		items, err := fetchTuiCards(ctx, app)
		if err != nil {
			return cardsResultMsg{err: err}
		}
		return cardsResultMsg{items: items, status: "card saved", show: show}
	}
}

func (m tuiModel) submitCardEdit() (tea.Model, tea.Cmd) {
	if m.selCard.id == uuid.Nil {
		m.err = "select a card"
		return m, nil
	}
	plain := CardPlain{
		Number: m.inputs[0].Value(),
		Holder: m.inputs[1].Value(),
		Expiry: m.inputs[2].Value(),
		CVV:    m.inputs[3].Value(),
	}
	meta := m.inputs[4].Value()
	if strings.TrimSpace(plain.Number) == "" {
		m.err = "number is required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selCard.id
	return m, func() tea.Msg {
		if err := writeCardUpdate(ctx, app, id, plain, meta); err != nil {
			return cardsResultMsg{err: err}
		}
		items, err := fetchTuiCards(ctx, app)
		if err != nil {
			return cardsResultMsg{err: err}
		}
		return cardsResultMsg{items: items, status: "card updated", show: true}
	}
}

func (m tuiModel) submitCardDelete() (tea.Model, tea.Cmd) {
	if m.selCard.id == uuid.Nil {
		m.err = "select a card"
		return m, nil
	}
	m.busy = true
	m.err = ""
	app := m.app
	ctx := m.ctx
	id := m.selCard.id
	return m, func() tea.Msg {
		if err := writeCardDelete(ctx, app, id); err != nil {
			return cardsResultMsg{err: err}
		}
		items, err := fetchTuiCards(ctx, app)
		if err != nil {
			return cardsResultMsg{err: err}
		}
		return cardsResultMsg{items: items, status: "card deleted", show: true}
	}
}

func writeCardAdd(ctx context.Context, app *App, plain CardPlain, meta string) error {
	blob, err := sealCard(app, plain)
	if err != nil {
		return err
	}
	res, err := app.Client.CardCreate(ctx, &oas.CreateCard{
		ID:               uuid.New(),
		Version:          card.CreateVersion,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, io.Discard)
}

func writeCardUpdate(ctx context.Context, app *App, id uuid.UUID, plain CardPlain, meta string) error {
	current, err := fetchCard(ctx, app, id)
	if err != nil {
		return err
	}
	blob, err := sealCard(app, plain)
	if err != nil {
		return err
	}
	res, err := app.Client.CardUpdate(ctx, &oas.UpdateCard{
		Version:          current.Version,
		Nonce:            blob.nonce,
		Meta:             oas.NewOptString(meta),
		Ciphertext:       blob.ciphertext,
		CiphertextSHA256: blob.sum[:],
	}, oas.CardUpdateParams{ID: id})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, io.Discard)
}

func writeCardDelete(ctx context.Context, app *App, id uuid.UUID) error {
	res, err := app.Client.CardDelete(ctx, oas.CardDeleteParams{ID: id})
	if err != nil {
		return err
	}
	return handleCardRes(app, res, io.Discard)
}

func fetchTuiCards(ctx context.Context, app *App) ([]tuiCard, error) {
	res, err := app.Client.ListCards(ctx, oas.ListCardsParams{})
	if err != nil {
		if isOffline(err) {
			return tuiCardsFromCache(app)
		}
		return nil, err
	}
	list, ok := res.(*oas.CardListRes)
	if !ok {
		return nil, cardMsg(res)
	}
	items := make([]tuiCard, 0, len(list.Items))
	live := make([]cachedCard, 0, len(list.Items))
	for i := range list.Items {
		n := list.Items[i]
		if n.DeletedAt.IsSet() {
			continue
		}
		plain, meta, err := openCard(app, n)
		if err != nil {
			return nil, err
		}
		items = append(items, tuiCard{
			id: n.ID, version: n.Version,
			number: plain.Number, holder: plain.Holder, expiry: plain.Expiry, cvv: plain.CVV,
			meta: meta,
		})
		live = append(live, cachedCard{
			ID: n.ID, Version: n.Version,
			Number: plain.Number, Holder: plain.Holder, Expiry: plain.Expiry, CVV: plain.CVV,
			Meta: meta,
		})
	}
	_ = saveCardsCache(app, live)
	return items, nil
}

func tuiCardsFromCache(app *App) ([]tuiCard, error) {
	cards, err := loadCardsCache(app)
	if err != nil {
		return nil, fmt.Errorf("offline cards: %w", err)
	}
	items := make([]tuiCard, 0, len(cards))
	for _, n := range cards {
		items = append(items, tuiCard{
			id: n.ID, version: n.Version,
			number: n.Number, holder: n.Holder, expiry: n.Expiry, cvv: n.CVV, meta: n.Meta,
		})
	}
	return items, nil
}

func (m tuiModel) applyCards(items []tuiCard) tuiModel {
	m.cards = items
	m.listKind = listCards
	inner := contentWidth(m.width)
	m.table.SetColumns(cardColumns(inner))
	rows := make([]table.Row, 0, len(items))
	for _, n := range items {
		rows = append(rows, table.Row{shortID(n.id), maskCardNumber(n.number), n.holder, n.meta, fmt.Sprintf("%d", n.version)})
	}
	m.table.SetRows(rows)
	m.table.SetCursor(0)
	return m
}

func (m tuiModel) loadCards(status string, stay, show bool) tea.Cmd {
	app := m.app
	ctx := m.ctx
	return func() tea.Msg {
		items, err := fetchTuiCards(ctx, app)
		if err != nil {
			return cardsResultMsg{err: err, stay: stay, show: show}
		}
		return cardsResultMsg{items: items, status: status, stay: stay, show: show}
	}
}

func (m tuiModel) cardsGrid() string {
	sel := m.table.Cursor()
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("63"))).
		BorderRow(true).
		BorderColumn(true).
		Headers("ID", "Number", "Holder", "Meta", "Ver").
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
	for _, n := range m.cards {
		t.Row(shortID(n.id), maskCardNumber(n.number), n.holder, n.meta, fmt.Sprintf("%d", n.version))
	}
	return t.String()
}
