package replay

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

// Frame é um passo do replay entregue ao frontend: a ação, todos os eventos e a
// visão de cada assento depois dela. Só é montado para partidas encerradas.
type Frame struct {
	Ply    int                  `json:"ply"`
	Source Source               `json:"source"`
	Action engine.Action        `json:"action"`
	Events []engine.Event       `json:"events"`
	Views  [4]engine.PlayerView `json:"views"`
}

func BuildFrames(rec Record) ([]Frame, error) {
	frames := make([]Frame, 0, len(rec.Actions))
	_, err := Walk(rec, func(s Step) error {
		frame := Frame{Ply: s.Logged.Ply, Source: s.Logged.Source, Action: s.Logged.Action, Events: s.Events}
		for seat := engine.Seat(0); seat < 4; seat++ {
			frame.Views[seat] = engine.ViewFor(s.After, seat)
		}
		frames = append(frames, frame)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return frames, nil
}
