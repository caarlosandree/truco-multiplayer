package engine

// Seat é a posição na mesa (0..3, sentido horário).
type Seat uint8

// Team é a dupla: 0 = assentos 0 e 2; 1 = assentos 1 e 3.
type Team uint8

func TeamOf(s Seat) Team  { return Team(s % 2) }
func Partner(s Seat) Seat { return (s + 2) % 4 }
func Next(s Seat) Seat    { return (s + 1) % 4 }
func Other(t Team) Team   { return 1 - t }

func teamPtr(t Team) *Team { return &t }
func seatPtr(s Seat) *Seat { return &s }

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

type PlayedCard struct {
	Seat    Seat `json:"seat"`
	Card    Card `json:"card"`
	Covered bool `json:"covered"`
}
