package session

import (
	"errors"
	"net/http"
	"path"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/live"
	"github.com/MartyHub/size-it/internal/server"
	"github.com/labstack/echo/v4"
)

const sseBufferSize = 8

func Register(srv *server.Server) {
	hdl := &handler{
		path:  srv.Cfg.Path,
		rdr:   srv.Renderer(),
		done:  srv.Done(),
		svc:   newService(srv.Clk, srv.Repo),
		event: srv.Event,
	}

	srv.GET("/", hdl.root)
	srv.POST("/sessions", hdl.createOrJoinSession)
	srv.GET("/sessions/:id", hdl.getSession)
	srv.PATCH("/sessions/:id", hdl.saveTicket)
	srv.POST("/sessions/:id", hdl.addTicketToHistory)
	srv.PUT("/sessions/:id", hdl.resetSession)
	srv.PATCH("/sessions/:id/toggle", hdl.toggleSizings)
	srv.PATCH("/sessions/:id/:sizingType", hdl.switchSizingType)
	srv.PATCH("/sessions/:id/:sizingType/:sizingValue", hdl.setSizingValue)
}

const mimeSSE = "text/event-stream"

type handler struct {
	rdr   echo.Renderer
	done  <-chan struct{}
	path  string
	svc   *service
	event *live.Service
}

func (hdl *handler) root(c echo.Context) error {
	ctx := c.Request().Context()

	usr, err := internal.GetUser(ctx)
	if err != nil && !errors.Is(err, internal.ErrUnauthorized) {
		return err
	}

	teams, err := hdl.svc.teams(ctx)
	if err != nil {
		return err
	}

	return c.Render(http.StatusOK, "newSession.gohtml", map[string]any{
		"path":  hdl.path,
		"teams": teams,
		"user":  usr,
	})
}

func (hdl *handler) createOrJoinSession(c echo.Context) error {
	input, err := internal.Bind[CreateOrJoinSessionInput](c)
	if err != nil {
		return err
	}

	var session Session

	ctx := c.Request().Context()

	if input.ID == "" {
		session, err = hdl.svc.create(ctx, input)
	} else {
		session, err = hdl.svc.get(ctx, input.ID)
	}

	if err != nil {
		return err
	}

	userID, err := internal.UserID()
	if err != nil {
		return err
	}

	usr := internal.User{
		ID:   userID,
		Name: input.Username,
		Team: session.Team,
	}

	if err = internal.SetCookie(c, usr); err != nil {
		return err
	}

	return c.Redirect(http.StatusFound, path.Join(hdl.path, "sessions", session.ID))
}

func (hdl *handler) getSession(c echo.Context) error {
	input, err := internal.Bind[GetSessionInput](c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	session, err := hdl.svc.get(ctx, input.ID)
	if err != nil {
		return err
	}

	isSSE := c.Request().Header.Get(echo.HeaderAccept) == mimeSSE

	usr, err := internal.GetUser(ctx)
	if err != nil {
		if isSSE {
			return err
		}

		if errors.Is(err, internal.ErrUnauthorized) {
			return c.Render(http.StatusOK, "joinSession.gohtml", map[string]any{
				"path":    hdl.path,
				"session": session,
				"user":    usr,
			})
		}

		return err
	}

	if isSSE {
		hdlSSE := &handlerSSE{
			events:  make(chan live.Event, sseBufferSize),
			done:    hdl.done,
			session: session,
			usr:     usr,
			svc:     hdl.event,
		}

		return hdlSSE.handle(c)
	}

	if err = hdl.event.Create(ctx, session.ID, session.Team); err != nil {
		return err
	}

	return c.Render(http.StatusOK, "session.gohtml", map[string]any{
		"path":                   hdl.path,
		"session":                session,
		"sessionID":              session.ID,
		"sizingValueStoryPoints": live.SizingValueStoryPoints,
		"sizingValueTShirt":      live.SizingValueTShirt,
		"user":                   usr,
	})
}

func (hdl *handler) saveTicket(c echo.Context) error {
	input, err := internal.Bind[PatchSessionInput](c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	if _, err = internal.GetUser(ctx); err != nil {
		return err
	}

	if err = hdl.event.SaveTicket(input.SessionID, input.Summary, input.URL); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (hdl *handler) addTicketToHistory(c echo.Context) error {
	input, err := internal.Bind[PatchSessionInput](c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	usr, err := internal.GetUser(ctx)
	if err != nil {
		return err
	}

	if err = hdl.event.AddTicketToHistory(ctx, input.SessionID, usr); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (hdl *handler) toggleSizings(c echo.Context) error {
	input, err := internal.Bind[GetSessionInput](c)
	if err != nil {
		return err
	}

	if _, err = internal.GetUser(c.Request().Context()); err != nil {
		return err
	}

	if err = hdl.event.ToggleSizings(input.ID); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (hdl *handler) switchSizingType(c echo.Context) error {
	input, err := internal.Bind[PatchSizingTypeInput](c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	if _, err = internal.GetUser(ctx); err != nil {
		return err
	}

	if err = hdl.event.SwitchSizingType(ctx, input.SessionID, input.SizingType); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (hdl *handler) setSizingValue(c echo.Context) error {
	input, err := internal.Bind[PatchSizingValueInput](c)
	if err != nil {
		return err
	}

	usr, err := internal.GetUser(c.Request().Context())
	if err != nil {
		return err
	}

	if err = hdl.event.SetSizingValue(input.SessionID, input.SizingValue, usr); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}

func (hdl *handler) resetSession(c echo.Context) error {
	input, err := internal.Bind[GetSessionInput](c)
	if err != nil {
		return err
	}

	if _, err = internal.GetUser(c.Request().Context()); err != nil {
		return err
	}

	if err = hdl.event.ResetSession(input.ID); err != nil {
		return err
	}

	return c.NoContent(http.StatusOK)
}
