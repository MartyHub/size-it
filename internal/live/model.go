package live

import (
	"fmt"
	"io"

	"github.com/MartyHub/size-it/internal"
)

const (
	SizingTypeStoryPoints = "STORY_POINTS"
	SizingTypeTShirt      = "T_SHIRT"
)

var (
	SizingValueStoryPoints = []string{"1", "2", "3", "5", "8", "13", "20", "40", "﹖"} //nolint:gochecknoglobals
	SizingValueTShirt      = []string{"XS", "S", "M", "L", "XL", "XXL", "﹖"}          //nolint:gochecknoglobals
)

type (
	Event struct {
		Kind string
		Data []byte
	}

	state struct {
		Ticket  *ticket
		History []ticket
		Results []result
		Show    bool
		Team    string
	}

	ticket struct {
		ID          int64
		Summary     string
		URL         string
		SizingType  string
		SizingValue string
	}

	result struct {
		events chan Event
		User   internal.User
		Sizing string
	}
)

func (evt Event) Write(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", evt.Kind); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", evt.Data); err != nil {
		return err
	}

	return nil
}

func (s *state) SizingValue(usr internal.User) string {
	for _, res := range s.Results {
		if res.User == usr {
			return res.Sizing
		}
	}

	return ""
}

func (s *state) reset() {
	s.Ticket.ID = 0
	s.Ticket.Summary = ""
	s.Ticket.URL = ""
	s.Ticket.SizingValue = ""

	s.Show = false

	for i := range s.Results {
		s.Results[i].Sizing = ""
	}
}

func (tck ticket) New() bool {
	return tck.ID == 0
}

func (tck ticket) Valid() bool {
	return tck.Summary != "" && tck.SizingValue != ""
}
