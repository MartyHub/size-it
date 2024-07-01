package monitoring

import (
	"net/http"

	"github.com/MartyHub/size-it/internal/server"
	"github.com/labstack/echo/v4"
)

func Register(srv *server.Server) {
	hdl := &handler{svc: newService(srv.Clk, srv.Repo)}

	srv.GET("/api/v1/health", hdl.health)
	srv.GET("/api/v1/info", hdl.info)
}

type handler struct {
	svc *service
}

func (hdl *handler) health(c echo.Context) error {
	output := hdl.svc.health(c.Request().Context())

	return c.JSON(http.StatusOK, output)
}

func (hdl *handler) info(c echo.Context) error {
	output, err := hdl.svc.info(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, output)
}
