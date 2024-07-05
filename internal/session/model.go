package session

import (
	"time"

	"github.com/MartyHub/size-it/internal/live"
	"github.com/invopop/validation"
)

type (
	CreateOrJoinSessionInput struct {
		ID       string `form:"id"`
		Team     string `form:"team"`
		Username string `form:"username"`
	}

	GetSessionInput struct {
		ID string `param:"id"`
	}

	PatchSessionInput struct {
		SessionID string `param:"id"`

		Summary string `form:"summary"`
		URL     string `form:"url"`
	}

	PatchSizingTypeInput struct {
		SessionID  string `param:"id"`
		SizingType string `param:"sizingType"`
	}

	PatchSizingValueInput struct {
		SessionID   string `param:"id"`
		SizingType  string `param:"sizingType"`
		SizingValue string `param:"sizingValue"`
	}

	Session struct {
		ID        string    `json:"id"`
		Team      string    `json:"team"`
		CreatedAt time.Time `json:"createdAt"`
	}
)

func (input CreateOrJoinSessionInput) Validate() error {
	return validation.ValidateStruct(&input,
		validation.Field(&input.Username, validation.Required),
		validation.Field(&input.Team, validation.When(input.ID == "", validation.Required)),
	)
}

func (input GetSessionInput) Validate() error {
	return validation.ValidateStruct(&input,
		validation.Field(&input.ID, validation.Required),
	)
}

func (input PatchSessionInput) Validate() error {
	return validation.ValidateStruct(&input,
		validation.Field(&input.SessionID, validation.Required),
	)
}

func (input PatchSizingTypeInput) Validate() error {
	return validation.ValidateStruct(&input,
		validation.Field(&input.SessionID, validation.Required),
		validation.Field(&input.SizingType, validation.In(
			live.SizingTypeStoryPoints,
			live.SizingTypeTShirt,
		)),
	)
}

func (input PatchSizingValueInput) Validate() error {
	return validation.ValidateStruct(&input,
		validation.Field(&input.SessionID, validation.Required),
		validation.Field(&input.SizingType, validation.In(
			live.SizingTypeStoryPoints,
			live.SizingTypeTShirt,
		)),
		validation.Field(&input.SizingValue, validation.Required),
	)
}
