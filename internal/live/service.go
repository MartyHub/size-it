package live

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
	"github.com/MartyHub/size-it/internal/db/sqlc"
	"github.com/labstack/echo/v4"
)

const (
	defaultSizingType = SizingTypeStoryPoints
	maxBucketSize     = 5
)

var allActiveUsers notifyUserFunc = func(res result) bool { //nolint:gochecknoglobals
	return !res.inactive
}

type Service struct {
	done             <-chan struct{}
	maxInactiveTime  time.Duration
	mu               sync.RWMutex
	stateBySessionID map[string]*state

	clk  internal.Clock
	ntf  *notifier
	repo *db.Repository
}

func NewService(
	done <-chan struct{},
	cfg internal.Config,
	clk internal.Clock,
	rdr echo.Renderer,
	repo *db.Repository,
) *Service {
	res := &Service{
		clk:             clk,
		done:            done,
		maxInactiveTime: cfg.MaxInactiveTime,
		ntf: &notifier{
			path: cfg.Path,
			clk:  clk,
			rdr:  rdr,
		},
		repo:             repo,
		stateBySessionID: make(map[string]*state),
	}

	go res.startRemoveEmptySessions(cfg.EmptySessionsTick)

	return res
}

func (svc *Service) Join(ctx context.Context, sessionID string, usr internal.User, events chan Event) error {
	var err error

	svc.mu.Lock()
	defer svc.mu.Unlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		s, err = svc.init(ctx, sessionID)
		if err != nil {
			return err
		}
	}

	slog.Info("User joining session",
		slog.String(internal.LogKeySession, sessionID),
		slog.String(internal.LogKeyUser, usr.Name),
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.userJoin(usr, events)

	notifyUser := includeUser(usr)

	if err = svc.ntf.notifyTicket(sessionID, s, notifyUser); err != nil {
		return err
	}

	if err = svc.ntf.notifyTabs(sessionID, s, notifyUser, false); err != nil {
		return err
	}

	if err = svc.ntf.notifyHistory(sessionID, s, notifyUser); err != nil {
		return err
	}

	if err = svc.ntf.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) Leave(sessionID string, usr internal.User) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return
	}

	slog.Info("User leaving session",
		slog.String(internal.LogKeySession, sessionID),
		slog.String(internal.LogKeyUser, usr.Name),
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, res := range s.Results {
		if res.User.Equals(usr) {
			s.Results[i].maxInactiveTime = svc.clk.Now().Add(svc.maxInactiveTime)

			go svc.startDeactivateUsers(sessionID, s)

			return
		}
	}
}

func (svc *Service) UpdateTicket(sessionID, summary, url string, usr internal.User) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Ticket.Summary = summary
	s.Ticket.URL = url

	return svc.ntf.notifyTicket(sessionID, s, excludeUser(usr))
}

func (svc *Service) AddTicketToHistory(ctx context.Context, sessionID string, usr internal.User) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, res := range s.Results {
		if res.User.Equals(usr) {
			s.Ticket.SizingValue = res.Sizing

			break
		}
	}

	if !s.Ticket.valid() {
		return nil
	}

	if err := svc.saveTicket(ctx, sessionID, s); err != nil {
		return err
	}

	history, err := svc.history(ctx, s.Team, s.Ticket.SizingType)
	if err != nil {
		return err
	}

	s.History = history

	return svc.ntf.notifyHistory(sessionID, s, allActiveUsers)
}

func (svc *Service) ToggleSizings(sessionID string) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Show = !s.Show

	return svc.ntf.notifyResults(sessionID, s)
}

func (svc *Service) SwitchSizingType(ctx context.Context, sessionID, sizingType string) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Ticket.SizingType = sizingType
	s.Ticket.SizingValue = ""

	for i := range s.Results {
		s.Results[i].Sizing = ""
	}

	if err := svc.ntf.notifyTabs(sessionID, s, allActiveUsers, false); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s); err != nil {
		return err
	}

	history, err := svc.history(ctx, s.Team, s.Ticket.SizingType)
	if err != nil {
		return err
	}

	s.History = history

	return svc.ntf.notifyHistory(sessionID, s, allActiveUsers)
}

func (svc *Service) SetSizingValue(sessionID, sizingValue string, usr internal.User) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, res := range s.Results {
		if res.User.Equals(usr) {
			s.Results[i].Sizing = sizingValue

			break
		}
	}

	if err := svc.ntf.notifyTabs(sessionID, s, includeUser(usr), true); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) ResetSession(sessionID string) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	s, found := svc.stateBySessionID[sessionID]
	if !found {
		return fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.reset()

	if err := svc.ntf.notifyTicket(sessionID, s, allActiveUsers); err != nil {
		return err
	}

	if err := svc.ntf.notifyTabs(sessionID, s, allActiveUsers, false); err != nil {
		return err
	}

	if err := svc.ntf.notifyResults(sessionID, s); err != nil {
		return err
	}

	return nil
}

func (svc *Service) init(ctx context.Context, sessionID string) (*state, error) {
	session, err := svc.repo.Session(ctx, sessionID)
	if err != nil {
		if db.IsErrNoRows(err) {
			return nil, fmt.Errorf("%w: session %s", internal.ErrNotFound, sessionID)
		}

		return nil, err
	}

	history, err := svc.history(ctx, session.Team, defaultSizingType)
	if err != nil {
		return nil, err
	}

	res := &state{
		History: history,
		Results: make([]result, 0, 1),
		Team:    session.Team,
		Ticket:  &ticket{SizingType: defaultSizingType},
	}

	svc.stateBySessionID[sessionID] = res

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

func (svc *Service) saveTicket(ctx context.Context, sessionID string, s *state) error {
	if s.Ticket.ID > 0 {
		slog.Info("Updating ticket...",
			slog.String(internal.LogKeySession, sessionID),
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
		slog.Info("Creating ticket...", slog.String(internal.LogKeySession, sessionID))

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

func (svc *Service) startRemoveEmptySessions(d time.Duration) {
	slog.Info("Starting empty sessions remover", slog.String("tick", d.String()))

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-svc.done:
			slog.Info("Server is shutting down, stopping empty sessions remover...")

			return
		case <-ticker.C:
			svc.removeEmptySessions()
		}
	}
}

func (svc *Service) removeEmptySessions() {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	slog.Info("Removing empty sessions...")

	for sessionID, s := range svc.stateBySessionID {
		if s.empty() {
			slog.Info("Removing session...", slog.String(internal.LogKeySession, sessionID))

			delete(svc.stateBySessionID, sessionID)
		}
	}
}

func (svc *Service) startDeactivateUsers(sessionID string, s *state) {
	slog.Info("Starting users deactivation", slog.String("tick", svc.maxInactiveTime.String()))

	ticker := time.NewTicker(svc.maxInactiveTime)
	defer ticker.Stop()

	for {
		select {
		case <-svc.done:
			slog.Info("Server is shutting down, stopping cleaner...")

			return
		case <-ticker.C:
			svc.deactivateUsers(sessionID, s)

			return
		}
	}
}

func (svc *Service) deactivateUsers(sessionID string, s *state) {
	s.mu.Lock()
	defer s.mu.Unlock()

	notify := false
	now := svc.clk.Now()

	for i, res := range s.Results {
		if !res.maxInactiveTime.IsZero() && res.maxInactiveTime.Before(now) {
			slog.Info("Marking user as inactive",
				slog.String(internal.LogKeySession, sessionID),
				slog.String(internal.LogKeyUser, res.User.Name),
			)

			s.Results[i].inactive = true
			notify = true
		}
	}

	if notify {
		_ = svc.ntf.notifyResults(sessionID, s)
	}
}

func includeUser(usr internal.User) notifyUserFunc {
	return func(res result) bool {
		return !res.inactive && res.User.Equals(usr)
	}
}

func excludeUser(usr internal.User) notifyUserFunc {
	return func(res result) bool {
		return !res.inactive && !res.User.Equals(usr)
	}
}

func sortTickets(ticketsByValue map[string][]ticket, keys []string) []ticket {
	var res []ticket

	for _, key := range keys {
		res = append(res, ticketsByValue[key]...)
	}

	return res
}
