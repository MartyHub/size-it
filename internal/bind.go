package internal

import (
	"fmt"

	"github.com/invopop/validation"
	"github.com/labstack/echo/v4"
)

func Bind[T any](c echo.Context) (T, error) { //nolint:ireturn
	var res T

	if err := c.Bind(&res); err != nil {
		return res, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	if v, ok := any(res).(validation.Validatable); ok {
		if err := v.Validate(); err != nil {
			return res, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
	} else if v, ok := any(res).(validation.ValidatableWithContext); ok {
		if err := v.ValidateWithContext(c.Request().Context()); err != nil {
			return res, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
	}

	return res, nil
}
