package live

import (
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
	ntf              *notifier
	repo             *db.Repository
	stateBySessionID map[string]*state
}

func NewService(path string, rdr echo.Renderer, repo *db.Repository) *Service {
	return &Service{
		ntf: &notifier{
			path: path,
			rdr:  rdr,
		},
		repo:             repo,
		stateBySessionID: make(map[string]*state),
	}
}

func (svc *Service) Join(ctx context.Context, sessionID string, usr internal.User, events chan Event) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if found {
		s.Results = slices.DeleteFunc(s.Results, func(res result) bool {
			if res.User.Equals(usr) {
				close(res.events)

				return true
			}

			return false
		})
	} else {
		var err error

		s, err = svc.doCreate(ctx, sessionID)
		if err != nil {
			return err
		}
	}

	slog.Info("User joining session",
		slog.String(internal.LogKeyUser, usr.Name),
		slog.String("session", sessionID),
	)

	s.Results = append(s.Results, result{
		User:   usr,
		events: events,
	})

	notifyUser := includeUser(usr)

	if err := svc.ntf.notifyTicket(sessionID, s, notifyUser); err != nil {
		return err
	}

	if err := svc.ntf.notifyTabs(sessionID, s, notifyUser, false); err != nil {
		return err
	}

	if err := svc.ntf.notifyHistory(sessionID, s, notifyUser); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s, allUsers()); err != nil {
		return err
	}

	return nil
}

func (svc *Service) Leave(sessionID string, usr internal.User) {
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

	s.Results = slices.DeleteFunc(s.Results, func(res result) bool {
		return res.User.Equals(usr)
	})

	if len(s.Results) == 0 {
		slog.Info("Closing session", slog.String("session", sessionID))

		delete(svc.stateBySessionID, sessionID)

		return
	}

	_ = svc.ntf.notifyResults(sessionID, s, allUsers())
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

func (svc *Service) SaveTicket(sessionID, summary, url string, usr internal.User) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.Ticket.Summary = summary
	s.Ticket.URL = url

	return svc.ntf.notifyTicket(sessionID, s, excludeUser(usr))
}

func (svc *Service) AddTicketToHistory(ctx context.Context, sessionID string, usr internal.User) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	for _, res := range s.Results {
		if res.User.Equals(usr) {
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

	return svc.ntf.notifyHistory(sessionID, s, allUsers())
}

func (svc *Service) ToggleSizings(sessionID string) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.Show = !s.Show

	return svc.ntf.notifyResults(sessionID, s, allUsers())
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

	if err := svc.ntf.notifyTabs(sessionID, s, allUsers(), false); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s, allUsers()); err != nil {
		return err
	}

	history, err := svc.history(ctx, s.Team, s.Ticket.SizingType)
	if err != nil {
		return err
	}

	s.History = history

	return svc.ntf.notifyHistory(sessionID, s, allUsers())
}

func (svc *Service) SetSizingValue(sessionID, sizingValue string, usr internal.User) error {
	svc.lock.Lock()
	defer svc.lock.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	for i, res := range s.Results {
		if res.User.Equals(usr) {
			s.Results[i].Sizing = sizingValue

			break
		}
	}

	if err := svc.ntf.notifyTabs(sessionID, s, includeUser(usr), true); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s, allUsers()); err != nil {
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

	if err := svc.ntf.notifyTicket(sessionID, s, allUsers()); err != nil {
		return err
	}

	if err := svc.ntf.notifyTabs(sessionID, s, allUsers(), false); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s, allUsers()); err != nil {
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
		History: history,
		Results: make([]result, 0, 1),
		Team:    team,
		Ticket:  &ticket{SizingType: defaultSizingType},
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

		tck, err := svc.repo.CreateTicket(ctx, sqlc.CreateTicketParams{
			Summary:     s.Ticket.Summary,
			Url:         s.Ticket.URL,
			SizingType:  s.Ticket.SizingType,
			SizingValue: s.Ticket.SizingValue,
			SessionID:   sessionID,
		})
		if err != nil {
			return err
		}

		s.Ticket.ID = tck.ID
	}

	return nil
}

func (svc *Service) doCreate(ctx context.Context, sessionID string) (*state, error) {
	session, err := svc.repo.Session(ctx, sessionID)
	if err != nil {
		if db.IsErrNoRows(err) {
			return nil, fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
		}

		return nil, err
	}

	s, err := svc.init(ctx, session.Team)
	if err != nil {
		return nil, err
	}

	svc.stateBySessionID[sessionID] = s

	return s, nil
}

func allUsers() notifyUserFunc {
	return func(_ internal.User) bool {
		return true
	}
}

func includeUser(usr internal.User) notifyUserFunc {
	return func(remoteUser internal.User) bool {
		return remoteUser.Equals(usr)
	}
}

func excludeUser(usr internal.User) notifyUserFunc {
	return func(remoteUser internal.User) bool {
		return !remoteUser.Equals(usr)
	}
}

func sortTickets(ticketsByValue map[string][]ticket, keys []string) []ticket {
	var res []ticket

	for _, key := range keys {
		res = append(res, ticketsByValue[key]...)
	}

	return res
}
