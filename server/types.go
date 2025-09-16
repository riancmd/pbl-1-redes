package main

import (
	"encoding/json"
	"math/rand"
	"net"
	"sync"
	"time"
)

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"request"`
	UID     string          `json:"uid"` // user id
	Data    json.RawMessage `json:"data"`
}

// mensagem temporária com dados do usuário pós-registro
type PlayerResponse struct {
	UID      string `json:"UID"`
	Username string `json:"username"`
}

/* REQUESTS POSSÍVEIS
register: registra novo usuário
login: faz login em conta
buyNewPack: compra pacote novo de cartas
battle: coloca usuário na fila
useCard: usa carta
giveUp: desiste da batalha
ping: manda ping
*/

const (
	register string = "register"
	login    string = "login"
	buypack  string = "buyNewPack"
	battle   string = "battle"
	usecard  string = "useCard"
	giveup   string = "giveUp"
	ping     string = "ping"

	registered string = "registered"
	loggedin   string = "loggedIn"
	packbought string = "packBought"
	enqueued   string = "enqueued"
	gamestart  string = "gameStart"
	cardused   string = "cardUsed"
	notify     string = "notify"
	updateinfo string = "updateInfo"
	newturn    string = "newTurn"
	newloss    string = "newLoss"
	newvictory string = "newVictory"
	newtie     string = "newTie"
	pong       string = "pong"
)

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
	IsInBattle  bool
	Connection  net.Conn
}

// AccountStorage gerencia a persistência das contas
// AccountStorage é o gerenciador de persistência de dados
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

// gerenciador de jogadores
type PlayerManager struct {
	mu          sync.Mutex
	nextID      int
	byUID       map[string]*User
	byUsername  map[string]*User
	activeByUID map[string]*User
}

// sobre as cartas
type CardType string

const (
	REM  CardType = "rem"
	NREM CardType = "nrem"
	Pill CardType = "pill"
)

type CardRarity string

const (
	Comum   CardRarity = "comum"
	Incomum CardRarity = "incomum"
	Rara    CardRarity = "rara"
)

type CardEffect string

const (
	AD   CardEffect = "adormecido"
	CONS CardEffect = "consciente"
	PAR  CardEffect = "paralisado"
	AS   CardEffect = "assustado"
	NEN  CardEffect = "nenhum"
)

type DreamState string

const (
	sleepy    DreamState = "adormecido"
	conscious DreamState = "consciente"
	paralyzed DreamState = "paralisado"
	scared    DreamState = "assustado"
)

type Card struct {
	Name       string     `json:"name"`
	CID        string     `json:"CID"`  // card ID
	Desc       string     `json:"desc"` // descrição
	CardType   CardType   `json:"cardtype"`
	CardRarity CardRarity `json:"cardrarity"`
	CardEffect CardEffect `json:"cardeffect"`
	Points     int        `json:"points"`
}

type Booster struct {
	BID     int
	Booster []Card
}

// BANCO DE CARTAS
type CardVault struct {
	CardGlossary map[string]Card
	CardQuantity map[string]int

	Vault           map[int]Booster
	BoosterQuantity int
	Total           int
	Generator       *rand.Rand
}

// struct pra base de dados local das cartas em json porem virtualizada
type CardDatabase struct {
	Cards map[string]Card `json:"cards"`
}

// SISTEMA DE MATCHMAKING
// mensagem interna de jogo para a goroutine do Match
type matchMsg struct {
	PlayerUID string
	Action    string
	Data      json.RawMessage
}

type MatchState int

const (
	Waiting MatchState = iota
	Running
	Finished
)

type Match struct {
	ID     int
	P1, P2 *User
	State  MatchState
	Turn   string // ID do jogador que joga a próxima ação

	Hand             map[string][]*Card // 10 cartas por jogador
	Sanity           map[string]int     // pontos por jogador
	DreamStates      map[string]DreamState
	RoundsInState    map[string]int // para controlar duração dos estados
	StateLockedUntil map[string]int // para controlar quando pode mudar estado
	currentRound     int

	inbox chan matchMsg // canal para trocar msgs entre threads
	mu    sync.Mutex
}

type MatchManager struct {
	mu       sync.Mutex
	queue    []*User
	nextID   int
	matches  map[int]*Match
	byPlayer map[string]*Match
}

// SISTEMA DE BATALHAS
type BattleState int

const (
	WaitingForPlayers BattleState = iota
	InProgress
	Completed
)
