package protocol

import "github.com/caarlosandree/truco-multiplayer/backend/internal/engine"

type UserInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type HelloPayload struct {
	User            UserInfo `json:"user"`
	ProtocolVersion int      `json:"protocolVersion"`
}

type RoomStatus string

const (
	RoomWaiting RoomStatus = "waiting"
	RoomPlaying RoomStatus = "playing"
)

type SeatSummary struct {
	DisplayName string `json:"displayName"`
}

// RoomSummary é a linha da sala na lista do lobby (só salas públicas aparecem).
type RoomSummary struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Status RoomStatus      `json:"status"`
	Seats  [4]*SeatSummary `json:"seats" tstype:"(SeatSummary | null)[],required"`
	Score  *[2]int         `json:"score,omitempty"`
}

type LobbySnapshotPayload struct {
	Rooms []RoomSummary `json:"rooms"`
}

type RoomUpsertedPayload struct {
	Room RoomSummary `json:"room"`
}

type RoomRemovedPayload struct {
	RoomID string `json:"roomId"`
}

type SeatInfo struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Ready       bool   `json:"ready"`
	Connected   bool   `json:"connected"`
	Absent      bool   `json:"absent"`
}

// RoomStatePayload é a visão da sala para um membro; o código só vai para membros.
type RoomStatePayload struct {
	ID         string       `json:"id"`
	Code       string       `json:"code"`
	Name       string       `json:"name"`
	Visibility Visibility   `json:"visibility"`
	Host       engine.Seat  `json:"host"`
	Status     RoomStatus   `json:"status"`
	Seats      [4]*SeatInfo `json:"seats" tstype:"(SeatInfo | null)[],required"`
	MatchID    string       `json:"matchId,omitempty"`
	YourSeat   engine.Seat  `json:"yourSeat"`
}

type LeftReason string

const (
	LeftLeft      LeftReason = "left"
	LeftKicked    LeftReason = "kicked"
	LeftDestroyed LeftReason = "destroyed"
)

type LeftRoomPayload struct {
	Reason LeftReason `json:"reason"`
}

// StatePayload leva a projeção do assento e os prazos (epoch ms UTC) dos assentos pendentes.
type StatePayload struct {
	View         engine.PlayerView     `json:"view"`
	Deadlines    map[engine.Seat]int64 `json:"deadlines" tstype:"{ [seat: number]: number }"`
	PendingSeats []int                 `json:"pendingSeats"`
}

// ForfeitAvailablePayload: Team é a dupla ausente contra a qual o W.O. pode ser pedido; null retira a oferta.
type ForfeitAvailablePayload struct {
	Team *engine.Team `json:"team" tstype:"engine.Team | null,required"`
}

type EndReason string

const (
	EndGameOver  EndReason = "game_over"
	EndForfeit   EndReason = "forfeit"
	EndAborted   EndReason = "aborted"
	EndAbandoned EndReason = "abandoned"
)

type MatchEndedPayload struct {
	Reason EndReason    `json:"reason"`
	Winner *engine.Team `json:"winner" tstype:"engine.Team | null,required"`
	Score  [2]int       `json:"score"`
}

type SignalBroadcastPayload struct {
	Seat engine.Seat `json:"seat"`
	ID   SignalID    `json:"id"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Seq     int64  `json:"seq,omitempty"`
}
