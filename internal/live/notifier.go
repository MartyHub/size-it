package live

import (
	"bytes"
	"log/slog"

	"github.com/MartyHub/size-it/internal"
	"github.com/labstack/echo/v4"
)

type (
	notifier struct {
		path string
		rdr  echo.Renderer
	}

	notifyUserFunc func(remoteUser internal.User) bool
)

func (ntf *notifier) notifyTicket(sessionID string, s *state, notifyUser notifyUserFunc) error {
	return ntf.notify(sessionID, "ticket", "components/ticket.gohtml", s, notifyUser)
}

func (ntf *notifier) notifyTabs(sessionID string, s *state, notifyUser notifyUserFunc, renderByUser bool) error {
	if renderByUser {
		return ntf.notifyByUser(sessionID, "tabs", "components/tabs.gohtml", s, notifyUser)
	}

	return ntf.notify(sessionID, "tabs", "components/tabs.gohtml", s, notifyUser)
}

func (ntf *notifier) notifyHistory(sessionID string, s *state, notifyUser notifyUserFunc) error {
	return ntf.notify(sessionID, "history", "components/history.gohtml", s, notifyUser)
}

func (ntf *notifier) notifyResults(sessionID string, s *state, notifyUser notifyUserFunc) error {
	return ntf.notify(sessionID, "results", "components/results.gohtml", s, notifyUser)
}

func (ntf *notifier) notify(sessionID, kind, template string, s *state, notifyUser notifyUserFunc) error {
	slog.Info("Broadcasting...", slog.String("event", kind))

	var buf bytes.Buffer

	data := map[string]any{
		"path":                   ntf.path,
		"sessionID":              sessionID,
		"sizingValueStoryPoints": SizingValueStoryPoints,
		"sizingValueTShirt":      SizingValueTShirt,
		"state":                  s,
		"userSizingValue":        "",
	}

	if err := ntf.rdr.Render(&buf, template, data, nil); err != nil {
		return err
	}

	evt := Event{
		Kind: kind,
		Data: bytes.ReplaceAll(buf.Bytes(), []byte{'\n'}, []byte{}),
	}

	for _, res := range s.Results {
		if notifyUser(res.User) {
			res.events <- evt
		}
	}

	return nil
}

func (ntf *notifier) notifyByUser(sessionID, kind, template string, s *state, notifyUser notifyUserFunc) error {
	var buf bytes.Buffer

	slog.Info("Broadcasting by user...", slog.String("event", kind))

	for _, res := range s.Results {
		if !notifyUser(res.User) {
			continue
		}

		data := map[string]any{
			"path":                   ntf.path,
			"sessionID":              sessionID,
			"sizingValueStoryPoints": SizingValueStoryPoints,
			"sizingValueTShirt":      SizingValueTShirt,
			"state":                  s,
			"user":                   res.User,
			"userSizingValue":        res.Sizing,
		}

		buf.Reset()

		if err := ntf.rdr.Render(&buf, template, data, nil); err != nil {
			return err
		}

		res.events <- Event{
			Kind: kind,
			Data: bytes.ReplaceAll(buf.Bytes(), []byte{'\n'}, []byte{}),
		}
	}

	return nil
}
