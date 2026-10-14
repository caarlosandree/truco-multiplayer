package engine

type ErrorCode string

const (
	ErrInvalidPhase      ErrorCode = "invalid_phase"
	ErrNotYourTurn       ErrorCode = "not_your_turn"
	ErrInvalidSlot       ErrorCode = "invalid_slot"
	ErrInvalidChoice     ErrorCode = "invalid_choice"
	ErrIllegalRaise      ErrorCode = "illegal_raise"
	ErrAlreadyVoted      ErrorCode = "already_voted"
	ErrNotVotingTeam     ErrorCode = "not_voting_team"
	ErrCoveredNotAllowed ErrorCode = "covered_not_allowed"
	ErrBurnNotFour       ErrorCode = "burn_not_four"
	ErrGameOver          ErrorCode = "game_over"
	ErrAlreadyDecided    ErrorCode = "already_decided"
	ErrInvalidSeat       ErrorCode = "invalid_seat"
)

// RuleError indica uma ação ilegal; o estado permanece inalterado.
type RuleError struct {
	Code ErrorCode
	Msg  string
}

func (e *RuleError) Error() string { return string(e.Code) + ": " + e.Msg }

func ruleErr(code ErrorCode, msg string) error { return &RuleError{Code: code, Msg: msg} }
