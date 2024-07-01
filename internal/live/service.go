package live

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
	"github.com/MartyHub/size-it/internal/db/sqlc"
	"github.com/labstack/echo/v4"
)

const (
	defaultSizingType = SizingTypeStoryPoints
	maxBucketSize     = 5
)

type Service struct {
	lock             sync.RWMutex
	rdr              echo.Renderer
	repo             *db.Repository
	stateBySessionID map[string]*state
}

func NewService(rdr echo.Renderer, repo *db.Repository) *Service {
	return &Service{
		rdr:              rdr,
		repo:             repo,
		stateBySessionID: make(map[string]*state),
	}
}

func (svc *Service) Join(sessionID string, usr internal.User, client chan Event) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	if slices.ContainsFunc(s.Results, func(res result) bool {
		return usr.Is(res.UserID)
	}) {
		return nil
	}

	slog.Info("User joining session",
		slog.String(internal.LogKeyUser, usr.Name),
		slog.String("session", sessionID),
	)

	s.clients[client] = usr
	s.Results = append(s.Results, result{
		UserID:   usr.ID,
		UserName: usr.Name,
	})

	if err := svc.notifyTicket(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyTabs(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyHistory(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) Leave(sessionID string, usr internal.User, client chan Event) {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return
	}

	slog.Info("User leaving session",
		slog.String(internal.LogKeyUser, usr.Name),
		slog.String("session", sessionID),
	)

	delete(s.clients, client)

	s.Results = slices.DeleteFunc(s.Results, func(res result) bool {
		return usr.Is(res.UserID)
	})

	_ = svc.notifyResults(sessionID, s)
}

func (svc *Service) Create(ctx context.Context, sessionID, team string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	if _, found := svc.stateBySessionID[sessionID]; !found {
		s, err := svc.init(ctx, team)
		if err != nil {
			return err
		}

		svc.stateBySessionID[sessionID] = s
	}

	return nil
}

func (svc *Service) SaveTicket(sessionID, summary, url string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.Ticket.Summary = summary
	s.Ticket.URL = url

	return svc.notifyTicket(sessionID, s)
}

func (svc *Service) AddTicketToHistory(ctx context.Context, sessionID string, usr internal.User) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	for _, res := range s.Results {
		if usr.Is(res.UserID) {
			s.Ticket.SizingValue = res.Sizing

			break
		}
	}

	if !s.Ticket.Valid() {
		return nil
	}

	if err := svc.doSaveTicket(ctx, sessionID, s); err != nil {
		return err
	}

	history, err := svc.history(ctx, s.Team, s.Ticket.SizingType)
	if err != nil {
		return err
	}

	s.History = history

	return svc.notifyHistory(sessionID, s)
}

func (svc *Service) ToggleSizings(sessionID string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.Show = !s.Show

	return svc.notifyResults(sessionID, s)
}

func (svc *Service) SwitchSizingType(ctx context.Context, sessionID, sizingType string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.Ticket.SizingType = sizingType
	s.Ticket.SizingValue = ""

	for i := range s.Results {
		s.Results[i].Sizing = ""
	}

	if err := svc.notifyTabs(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyResults(sessionID, s); err != nil {
		return err
	}

	history, err := svc.history(ctx, s.Team, s.Ticket.SizingType)
	if err != nil {
		return err
	}

	s.History = history

	return svc.notifyHistory(sessionID, s)
}

func (svc *Service) SetSizingValue(sessionID, sizingValue string, usr internal.User) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	for i, res := range s.Results {
		if usr.Is(res.UserID) {
			s.Results[i].Sizing = sizingValue

			break
		}
	}

	if err := svc.notifyTabs(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) ResetSession(sessionID string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.reset()

	if err := svc.notifyTicket(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyTabs(sessionID, s); err != nil {
		return err
	}

	if err := svc.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) init(ctx context.Context, team string) (*state, error) {
	history, err := svc.history(ctx, team, defaultSizingType)
	if err != nil {
		return nil, err
	}

	res := &state{
		Ticket:  &ticket{SizingType: defaultSizingType},
		History: history,
		Team:    team,
		clients: make(map[chan Event]internal.User),
	}

	return res, nil
}

func (svc *Service) history(ctx context.Context, team, sizingType string) ([]ticket, error) {
	tickets, err := svc.repo.History(ctx, sqlc.HistoryParams{
		Team:       team,
		SizingType: sizingType,
	})
	if err != nil {
		return nil, err
	}

	ticketsByValue := make(map[string][]ticket)

	for _, tck := range tickets {
		bucket := ticketsByValue[tck.SizingValue]

		if len(bucket) < maxBucketSize {
			ticketsByValue[tck.SizingValue] = append(ticketsByValue[tck.SizingValue], ticket{
				ID:          tck.ID,
				Summary:     tck.Summary,
				URL:         tck.Url,
				SizingType:  tck.SizingType,
				SizingValue: tck.SizingValue,
			})
		}
	}

	switch sizingType {
	case SizingTypeStoryPoints:
		return sortTickets(ticketsByValue, SizingValueStoryPoints), nil
	case SizingTypeTShirt:
		return sortTickets(ticketsByValue, SizingValueTShirt), nil
	}

	return []ticket{}, nil
}

func (svc *Service) notifyTicket(sessionID string, s *state) error {
	return svc.notify(sessionID, "ticket", "components/ticket.gohtml", s)
}

func (svc *Service) notifyTabs(sessionID string, s *state) error {
	return svc.notifyByUser(sessionID, "tabs", "components/tabs.gohtml", s)
}

func (svc *Service) notifyHistory(sessionID string, s *state) error {
	return svc.notify(sessionID, "history", "components/history.gohtml", s)
}

func (svc *Service) notifyResults(sessionID string, s *state) error {
	return svc.notify(sessionID, "results", "components/results.gohtml", s)
}

func (svc *Service) notify(sessionID, kind, template string, s *state) error {
	slog.Info("Broadcasting...", slog.String("event", kind))

	var buf bytes.Buffer

	data := map[string]any{
		"sessionID":              sessionID,
		"sizingValueStoryPoints": SizingValueStoryPoints,
		"sizingValueTShirt":      SizingValueTShirt,
		"state":                  s,
	}

	if err := svc.rdr.Render(&buf, template, data, nil); err != nil {
		return err
	}

	for clt := range s.clients {
		clt <- Event{
			Kind: kind,
			Data: bytes.ReplaceAll(buf.Bytes(), []byte{'\n'}, []byte{}),
		}
	}

	return nil
}

func (svc *Service) notifyByUser(sessionID, kind, template string, s *state) error {
	var buf bytes.Buffer

	slog.Info("Broadcasting by user...", slog.String("event", kind))

	for clt, usr := range s.clients {
		data := map[string]any{
			"sessionID":              sessionID,
			"sizingValueStoryPoints": SizingValueStoryPoints,
			"sizingValueTShirt":      SizingValueTShirt,
			"state":                  s,
			"user":                   usr,
		}

		buf.Reset()

		if err := svc.rdr.Render(&buf, template, data, nil); err != nil {
			return err
		}

		clt <- Event{
			Kind: kind,
			Data: bytes.ReplaceAll(buf.Bytes(), []byte{'\n'}, []byte{}),
		}
	}

	return nil
}

func (svc *Service) doSaveTicket(ctx context.Context, sessionID string, s *state) error {
	if s.Ticket.ID > 0 {
		slog.Info("Updating ticket...",
			slog.String("session", sessionID),
			slog.Int64("ticketID", s.Ticket.ID),
		)

		if err := svc.repo.UpdateTicket(ctx, sqlc.UpdateTicketParams{
			Summary:     s.Ticket.Summary,
			Url:         s.Ticket.URL,
			SizingType:  s.Ticket.SizingType,
			SizingValue: s.Ticket.SizingValue,
			ID:          s.Ticket.ID,
		}); err != nil {
			return err
		}
	} else {
		slog.Info("Creating ticket...", slog.String("session", sessionID))

		uuid, err := internal.ParseUUID(sessionID)
		if err != nil {
			return err
		}

		tck, err := svc.repo.CreateTicket(ctx, sqlc.CreateTicketParams{
			Summary:     s.Ticket.Summary,
			Url:         s.Ticket.URL,
			SizingType:  s.Ticket.SizingType,
			SizingValue: s.Ticket.SizingValue,
			SessionID:   *uuid,
		})
		if err != nil {
			return err
		}

		s.Ticket.ID = tck.ID
	}

	return nil
}

func sortTickets(ticketsByValue map[string][]ticket, keys []string) []ticket {
	var res []ticket

	for _, key := range keys {
		res = append(res, ticketsByValue[key]...)
	}

	return res
}
