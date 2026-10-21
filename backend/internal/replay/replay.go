// Package replay reconstrói partidas a partir de seed + log de ações. É puro:
// não faz I/O e depende só do engine.
package replay

import (
	"errors"
	"fmt"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/engine"
)

// Source indica quem originou uma ação gravada.
type Source string

const (
	SourcePlayer  Source = "player"
	SourceTimeout Source = "timeout" // ação padrão resolvida pela sala quando o tempo acaba
	SourceSystem  Source = "system"  // ações de sistema, como sys_start_hand
)

func (s Source) Valid() bool {
	return s == SourcePlayer || s == SourceTimeout || s == SourceSystem
}

// Status é o ciclo de vida de uma partida persistida.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusFinished   Status = "finished"
	StatusAbandoned  Status = "abandoned"
)

// EndReason é o motivo do fim de uma partida encerrada (coluna matches.end_reason).
// Os valores coincidem com protocol.EndReason (há teste que compara). Partidas que
// divergiram do log ("diverged") aparecem como abandoned no histórico.
type EndReason string

const (
	EndGameOver  EndReason = "game_over" // fim normal
	EndForfeit   EndReason = "forfeit"   // W.O.
	EndAborted   EndReason = "aborted"   // abortada por um administrador
	EndAbandoned EndReason = "abandoned" // abandonada (sala vazia, divergência, falha ao iniciar…)
)

type PlayerSeat struct {
	UserID      string
	DisplayName string
}

type LoggedAction struct {
	Ply    int
	Action engine.Action
	Source Source
}

// Record é tudo o que é preciso para reproduzir uma partida.
type Record struct {
	ID            string
	Seed          uint64
	FirstDealer   engine.Seat
	EngineVersion int
	Status        Status
	Players       [4]PlayerSeat
	Actions       []LoggedAction // ordenadas por Ply, a partir de 0
}

// Step é um passo da reconstrução.
type Step struct {
	Logged LoggedAction
	Before engine.GameState
	After  engine.GameState
	Events []engine.Event
}

var ErrIncompatibleVersion = errors.New("replay: versão de regras incompatível")

// CorruptLogError indica um log que não reproduz uma partida válida.
// Ply = -1 quando o problema está no cabeçalho do registro.
type CorruptLogError struct {
	Ply int
	Err error
}

func (e *CorruptLogError) Error() string {
	return fmt.Sprintf("replay: log corrompido no ply %d: %v", e.Ply, e.Err)
}

func (e *CorruptLogError) Unwrap() error { return e.Err }

func (r Record) enginePlayers() [4]engine.Player {
	var players [4]engine.Player
	for i, p := range r.Players {
		players[i] = engine.Player{ID: p.UserID, Name: p.DisplayName}
	}
	return players
}

// Walk reaplica o log no engine chamando fn a cada ação (fn pode ser nil).
// Devolve o estado após a última ação aplicada com sucesso.
func Walk(rec Record, fn func(Step) error) (engine.GameState, error) {
	if rec.EngineVersion != engine.RulesVersion {
		return engine.GameState{}, fmt.Errorf("%w: log v%d, engine v%d", ErrIncompatibleVersion, rec.EngineVersion, engine.RulesVersion)
	}
	if rec.FirstDealer > 3 {
		return engine.GameState{}, &CorruptLogError{Ply: -1, Err: fmt.Errorf("dealer inicial inválido: %d", rec.FirstDealer)}
	}
	g := engine.NewGame(rec.enginePlayers(), rec.Seed, rec.FirstDealer)
	for i, logged := range rec.Actions {
		if logged.Ply != i {
			return g, &CorruptLogError{Ply: i, Err: fmt.Errorf("ply esperado %d, encontrado %d", i, logged.Ply)}
		}
		if logged.Action.Kind == engine.ActSysTimeout {
			return g, &CorruptLogError{Ply: i, Err: errors.New("sys_timeout não deve ser gravado; grave a ação concreta")}
		}
		next, evs, err := engine.Apply(g, logged.Action)
		if err != nil {
			return g, &CorruptLogError{Ply: i, Err: err}
		}
		if fn != nil {
			if err := fn(Step{Logged: logged, Before: g, After: next, Events: evs}); err != nil {
				return next, err
			}
		}
		g = next
	}
	return g, nil
}

// Rebuild reconstrói o estado final do log.
func Rebuild(rec Record) (engine.GameState, error) { return Walk(rec, nil) }
