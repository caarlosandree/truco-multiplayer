package room

// settle reavalia prazos e timers depois de cada mensagem.
func (r *Room) settle() {}

func (r *Room) stopTimer() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}
