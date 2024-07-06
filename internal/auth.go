package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	CookieMaxAgeInSeconds = 60 * 60 * 24 * 7 * 3 // 3 weeks
	CookieName            = "sizeIt"

	KeyUser contextKey = 1
)

type (
	User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Team string `json:"team"`
	}

	contextKey int
)

func SetCookie(c echo.Context, usr User) error {
	data, err := json.Marshal(usr)
	if err != nil {
		return err
	}

	c.SetCookie(&http.Cookie{
		Name:     CookieName,
		Value:    base64.RawURLEncoding.EncodeToString(data),
		Path:     "/",
		MaxAge:   CookieMaxAgeInSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func GetUser(ctx context.Context) (User, error) {
	usr, ok := ctx.Value(KeyUser).(User)
	if !ok {
		return usr, ErrUnauthorized
	}

	return usr, nil
}
