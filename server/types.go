package server

import (
	"encoding/json"
	"math/rand"
	"net"
	"sync"
	"time"
)

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"tag"`
	UID     string          `json:"uid"` // user id
	Data    json.RawMessage `json:"data"`
}

// registro do usuário (dado persistente)
type User struct {
	UID         string    `json:"uid"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	Deck        []*Card   `json:"cards"`
	CreatedAt   time.Time `json:"created_at"`
	LastLogin   time.Time `json:"last_login"`
	TotalWins   int       `json:"total_wins"`
	TotalLosses int       `json:"total_losses"`
}

// AccountStorage gerencia a persistência das contas
type AccountStorage struct {
	filename string
	mutex    sync.RWMutex
}

// representando os usuários como "sessões", quando estão conectados
type ActiveSession struct {
	SID        string // sessionID
	Username   string
	Connection net.Conn
	Deck       []*Card
	LastPing   time.Time
	IsInBattle bool
}

// gerenciador de sessões ativas
type SessionRegistry struct {
	mutex    sync.RWMutex // mutex
	sessions map[string]*ActiveSession
	nextSID  int
}

// sobre as cartas
type CardType string

const (
	REM  CardType = "rem"
	NREM CardType = "nrem"
	Pill CardType = "pill"
)

type Card struct {
	Name     string   `json:"name"`
	CID      string   `json:"CID"` // card ID
	CardType CardType `json:"cardtype"`
	Points   int      `json:"points"`
}

// banco de cartas
type CardVault struct {
	vault     []Card
	generator *rand.Rand
}

// sistema de batalhas
type BattleState int

const (
	WaitingForPlayers BattleState = iota
	InProgress
	Completed
)

type BattleArena struct {
	BID          string // battle ID
	Player1      *ActiveSession
	Player2      *ActiveSession
	State        BattleState
	CurrentTurn  string
	Player1Hand  []*Card
	Player2Hand  []*Card
	Scoreboard   map[string]int
	RoundActions chan BattleCommand
	battleMutex  sync.Mutex
}

type BattleCommand struct {
	SessionID string
	Command   string
	CardData  json.RawMessage
}

// gerenciador de batalhas
type BattleCoordinator struct {
	mutex         sync.RWMutex
	waitingQueue  []*ActiveSession
	activeBattles map[string]*BattleArena
	playerBattles map[string]*BattleArena
	nextBattleID  int
}
