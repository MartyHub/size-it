package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MartyHub/size-it/internal"
	"github.com/labstack/echo/v4"
)

func cookieAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if err := setUser(c); err != nil {
				return err
			}

			return next(c)
		}
	}
}

func setUser(c echo.Context) error {
	cookie, err := c.Cookie(internal.CookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil
		}

		return err
	}

	if err = cookie.Valid(); err != nil {
		return err
	}

	data, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return err
	}

	var usr internal.User

	if err = json.Unmarshal(data, &usr); err != nil {
		return err
	}

	ctx := context.WithValue(c.Request().Context(), internal.KeyUser, usr)

	c.SetRequest(c.Request().WithContext(ctx))

	return nil
}
