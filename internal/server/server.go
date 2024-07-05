package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MartyHub/size-it/internal"
	"github.com/MartyHub/size-it/internal/db"
	"github.com/MartyHub/size-it/internal/live"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const shutdownTimeout = 10 * time.Second

type Server struct {
	Cfg      internal.Config
	Clk      internal.Clock
	Event    *live.Service
	Repo     *db.Repository
	e        *echo.Echo
	shutdown chan struct{}
}

func NewServer(cfg internal.Config, repo *db.Repository) *Server {
	res := &Server{
		Cfg:      cfg,
		Clk:      &internal.UTCClock{},
		Repo:     repo,
		e:        echo.New(),
		shutdown: make(chan struct{}),
	}

	res.configure()

	res.Event = live.NewService(cfg.Path, res.e.Renderer, repo)

	return res
}

func (srv *Server) GET(path string, hdl echo.HandlerFunc) {
	srv.e.GET(path, hdl)
}

func (srv *Server) PATCH(path string, hdl echo.HandlerFunc) {
	srv.e.PATCH(path, hdl)
}

func (srv *Server) POST(path string, hdl echo.HandlerFunc) {
	srv.e.POST(path, hdl)
}

func (srv *Server) PUT(path string, hdl echo.HandlerFunc) {
	srv.e.PUT(path, hdl)
}

func (srv *Server) Renderer() echo.Renderer { //nolint:ireturn
	return srv.e.Renderer
}

func (srv *Server) Run(ctx context.Context) error {
	srv.start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	return srv.stop(ctx)
}

func (srv *Server) Done() <-chan struct{} {
	return srv.shutdown
}

func (srv *Server) configure() {
	slog.Info("Configuring server...")

	srv.e.DisableHTTP2 = true
	srv.e.HideBanner = true
	srv.e.HidePort = true
	srv.e.HTTPErrorHandler = srv.httpErrorHandler()

	if srv.Cfg.Dev {
		srv.e.Renderer = newLiveRenderer()
	} else {
		srv.e.Renderer = newEmbedRenderer()
	}

	srv.e.Use(cookieAuth())
	srv.e.Use(requestLogger())
	srv.e.Use(middleware.Recover())

	srv.e.RouteNotFound("/*", func(c echo.Context) error {
		return c.Render(http.StatusOK, "notFound.gohtml", nil)
	})
}

func (srv *Server) start() {
	slog.Info("Starting server...")

	go func() {
		if err := srv.e.Start(srv.Cfg.Address()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			internal.LogError("Failed to start server", err)

			os.Exit(1)
		}
	}()
}

func (srv *Server) stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	slog.Info("Shutting down server...")

	close(srv.shutdown)

	return srv.e.Shutdown(ctx)
}

func (srv *Server) httpErrorHandler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		if strings.Contains(c.Request().Header.Get(echo.HeaderAccept), echo.MIMETextHTML) {
			_ = c.Render(http.StatusOK, "error.gohtml", map[string]any{
				"error": err.Error(),
				"path":  srv.Cfg.Path,
			})
		} else {
			switch {
			case errors.Is(err, internal.ErrInvalidInput):
				err = toHTTPError(err, http.StatusBadRequest)
			case errors.Is(err, internal.ErrNotFound):
				err = toHTTPError(err, http.StatusNotFound)
			}

			srv.e.DefaultHTTPErrorHandler(err, c)
		}
	}
}

func toHTTPError(err error, status int) *echo.HTTPError {
	return &echo.HTTPError{
		Code:     status,
		Message:  err.Error(),
		Internal: err,
	}
}
