package protocol

// Códigos de erro de sala/lobby. Erros de regra usam engine.ErrorCode.
const (
	CodeBadRequest         = "bad_request"
	CodeInternal           = "internal"
	CodeAlreadyInRoom      = "already_in_room"
	CodeRoomFull           = "room_full"
	CodeRoomNotFound       = "room_not_found"
	CodeSeatTaken          = "seat_taken"
	CodeNotHost            = "not_host"
	CodeNotInRoom          = "not_in_room"
	CodeMatchInProgress    = "match_in_progress"
	CodeForfeitUnavailable = "forfeit_unavailable"
	CodeTooManyRooms       = "too_many_rooms"
	CodeRateLimited        = "rate_limited"
)

var errorTexts = map[string]string{
	CodeBadRequest:         "mensagem inválida",
	CodeInternal:           "erro interno; tente de novo",
	CodeAlreadyInRoom:      "você já está em uma sala",
	CodeRoomFull:           "a sala está cheia",
	CodeRoomNotFound:       "sala não encontrada",
	CodeSeatTaken:          "assento ocupado",
	CodeNotHost:            "só o anfitrião pode fazer isso",
	CodeNotInRoom:          "você não está em uma sala",
	CodeMatchInProgress:    "a partida está em andamento",
	CodeForfeitUnavailable: "W.O. indisponível",
	CodeTooManyRooms:       "limite de salas atingido; tente mais tarde",
	CodeRateLimited:        "devagar: aguarde um pouco",
}

func errorText(code string) string {
	if text, ok := errorTexts[code]; ok {
		return text
	}
	return code
}
