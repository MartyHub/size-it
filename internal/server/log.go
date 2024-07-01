package server

import (
	"context"
	"log/slog"

	"github.com/MartyHub/size-it/internal"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		HandleError: true,
		LogError:    true,
		LogLatency:  true,
		LogMethod:   true,
		LogStatus:   true,
		LogURI:      true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.Duration(internal.LogKeyLatency, v.Latency),
				slog.String(internal.LogKeyMethod, v.Method),
				slog.String(internal.LogKeyURI, v.URI),
				slog.Int(internal.LogKeyStatus, v.Status),
			}

			if usr, err := internal.GetUser(c.Request().Context()); err == nil {
				attrs = append(attrs, slog.String(internal.LogKeyUser, usr.Name))
			}

			if v.Error != nil {
				attrs = append(attrs, slog.String(internal.LogKeyError, v.Error.Error()))

				slog.LogAttrs(context.Background(), slog.LevelError, "Request", attrs...)
			} else {
				slog.LogAttrs(context.Background(), slog.LevelInfo, "Request", attrs...)
			}

			return nil
		},
	})
}
