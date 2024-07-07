package live

import (
	"fmt"
	"io"
	"sync"
	"time"

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
		mu      sync.Mutex
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
		events          chan Event
		inactive        bool
		maxInactiveTime time.Time
		User            internal.User
		Sizing          string
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
		if res.User.Equals(usr) {
			return res.Sizing
		}
	}

	return ""
}

func (s *state) userJoin(usr internal.User, events chan Event) {
	for i, res := range s.Results {
		if res.User.Equals(usr) {
			close(res.events)

			s.Results[i] = result{
				events: events,
				User:   usr,
				Sizing: res.Sizing,
			}

			return
		}
	}

	s.Results = append(s.Results, result{
		User:   usr,
		events: events,
	})
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

func (s *state) empty() bool {
	for _, res := range s.Results {
		if !res.inactive {
			return false
		}
	}

	return true
}

func (tck ticket) New() bool {
	return tck.ID == 0
}

func (tck ticket) valid() bool {
	return tck.Summary != "" && tck.SizingValue != ""
}

func (res result) Hide() bool {
	return res.inactive
}
