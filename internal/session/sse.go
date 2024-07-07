package session

import (
	"log/slog"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/live"
	"github.com/labstack/echo/v4"
)

type handlerSSE struct {
	events  chan live.Event
	done    <-chan struct{}
	session Session
	usr     internal.User

	svc *live.Service
}

func (hdl *handlerSSE) handle(c echo.Context) error {
	if err := hdl.initSSE(c); err != nil {
		return err
	}

	for {
		select {
		case <-hdl.done:
			slog.Info("Server is shutting down, stopping SSE...",
				slog.String(internal.LogKeyUser, hdl.usr.Name),
				slog.String("session", hdl.session.ID),
			)

			return nil
		case <-c.Request().Context().Done():
			// client is gone
			hdl.svc.Leave(hdl.session.ID, hdl.usr)

			return nil
		case evt, ok := <-hdl.events:
			if !ok {
				slog.Info("User left session without notice",
					slog.String(internal.LogKeySession, hdl.session.ID),
					slog.String(internal.LogKeyUser, hdl.usr.Name),
				)

				return nil
			}

			if err := hdl.writeEvent(c, evt); err != nil {
				hdl.svc.Leave(hdl.session.ID, hdl.usr)

				return err
			}
		}
	}
}

func (hdl *handlerSSE) initSSE(c echo.Context) error {
	header := c.Response().Header()

	header.Set(echo.HeaderContentType, "text/event-stream")
	header.Set(echo.HeaderCacheControl, "no-cache")
	header.Set(echo.HeaderConnection, "keep-alive")

	if err := hdl.svc.Join(c.Request().Context(), hdl.session.ID, hdl.usr, hdl.events); err != nil {
		return err
	}

	return nil
}

func (hdl *handlerSSE) writeEvent(c echo.Context, evt live.Event) error {
	w := c.Response()

	if err := evt.Write(w); err != nil {
		return err
	}

	w.Flush()

	return nil
}
