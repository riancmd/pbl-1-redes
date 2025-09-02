package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// ============================
// ======= MODELOS BASE =======
// ============================

// Mensagem genérica (protocolo JSON sobre TCP)
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// Mensagem de jogo aninhada (enviada via Action: "Game_Action")
type GameMessage struct {
	PlayerID int             `json:"PlayerID"`
	Action   string          `json:"action"`
	Data     json.RawMessage `json:"data"`
}

// Cores do Alucinari
type Color string

const (
	Red   Color = "red"
	Blue  Color = "blue"
	White Color = "white"
)

// Carta mínima para o Alucinari
// Apenas Nome, Cor e Pontos (0..10)
type Card struct {
	Name   string `json:"name"`
	Color  Color  `json:"color"`
	Points int    `json:"points"`
}

// Jogador em memória (servidor)
type Player struct {
	ID       int
	UserName string
	Login    string
	Password string
	Conn     net.Conn

	Inventory []*Card // cartas do jogador (persistidas em memória)
	InGame    bool
}

// ============================
// ======= GERENCIADORES ======
// ============================

// Gerencia Players em memória
// Responsável por criar, logar e buscar jogadores
// (thread-safe com mutex)
type PlayerManager struct {
	mu      sync.Mutex
	nextID  int
	byID    map[int]*Player
	byLogin map[string]*Player
}

func NewPlayerManager() *PlayerManager {
	return &PlayerManager{
		byID:    make(map[int]*Player),
		byLogin: make(map[string]*Player),
	}
}

func (pm *PlayerManager) CreatePlayer(username, login, password string) (*Player, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.byLogin[login]; exists {
		return nil, errors.New("login já em uso")
	}
	pm.nextID++
	p := &Player{
		ID:        pm.nextID,
		UserName:  username,
		Login:     login,
		Password:  password,
		Inventory: make([]*Card, 0),
	}
	pm.byID[p.ID] = p
	pm.byLogin[p.Login] = p
	return p, nil
}

func (pm *PlayerManager) Login(login, password string, conn net.Conn) (*Player, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byLogin[login]
	if !ok {
		return nil, errors.New("login não encontrado")
	}
	if p.Password != password {
		return nil, errors.New("senha inválida")
	}
	p.Conn = conn
	return p, nil
}

func (pm *PlayerManager) GetByID(id int) (*Player, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byID[id]
	if !ok {
		return nil, errors.New("player não encontrado")
	}
	return p, nil
}

func (pm *PlayerManager) AddToInventory(id int, cards []*Card) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byID[id]
	if !ok {
		return errors.New("player não encontrado")
	}
	p.Inventory = append(p.Inventory, cards...)
	return nil
}

// Estoque de cartas do jogo
// Possui a lista completa de cartas disponíveis

type Stock struct{
	cards []Card
	rng   *rand.Rand
}

func NewStock() *Stock {
	return &Stock{rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (s *Stock) Init30GenericCards() {
	// 10 vermelhas, 10 azuis, 10 brancas
	// Nomes genéricos com a cor; pontos variados 1..10
	add := func(color Color, base string) {
		for i := 1; i <= 10; i++ {
			pts := (i%10) + 1 // 1..10
			name := fmt.Sprintf("%s %02d", base, i)
			s.cards = append(s.cards, Card{Name: name, Color: color, Points: pts})
		}
	}
	add(Red, "Red Shade")
	add(Blue, "Blue Shade")
	add(White, "White Shade")
}

// Sorteia um booster de n cartas (com repetição permitida)
func (s *Stock) Booster(n int) []*Card {
	res := make([]*Card, 0, n)
	if len(s.cards) == 0 {
		return res
	}
	for i := 0; i < n; i++ {
		c := s.cards[s.rng.Intn(len(s.cards))]
		// criar cópia heap-alloc para o jogador
		cc := c
		res = append(res, &cc)
	}
	return res
}

// ============================
// ========= MATCHMAKING ======
// ============================

type MatchState int
const (
	Waiting MatchState = iota
	Running
	Finished
)

// Mensagem interna de jogo para a goroutine do Match
type matchMsg struct {
	PlayerID int
	Action   string
	Data     json.RawMessage
}

type Match struct {
	ID      int
	P1, P2  *Player
	State   MatchState
	Turn    int // ID do jogador que joga a próxima ação

	Hand    map[int][]*Card // 7 cartas por jogador
	Score   map[int]int     // pontos por jogador

	inbox   chan matchMsg
	mu      sync.Mutex
	stock   *Stock
}

type MatchManager struct {
	mu       sync.Mutex
	queue    []*Player
	nextID   int
	matches  map[int]*Match
	byPlayer map[int]*Match
}

func NewMatchManager() *MatchManager {
	return &MatchManager{matches: make(map[int]*Match), byPlayer: make(map[int]*Match)}
}

func (mm *MatchManager) Enqueue(p *Player) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if p.InGame {
		return errors.New("player já está em jogo")
	}
	// evitar duplicata na fila
	for _, q := range mm.queue {
		if q.ID == p.ID { return errors.New("player já está na fila") }
	}
	mm.queue = append(mm.queue, p)
	return nil
}

func (mm *MatchManager) dequeue() (*Player, error) {
	if len(mm.queue) == 0 { return nil, errors.New("fila vazia") }
	p := mm.queue[0]
	mm.queue = mm.queue[1:]
	return p, nil
}

func (mm *MatchManager) FindMatchByPlayerID(id int) *Match {
	mm.mu.Lock(); defer mm.mu.Unlock()
	return mm.byPlayer[id]
}

func (mm *MatchManager) matchmakingLoop(stock *Stock) {
	for {
		time.Sleep(50 * time.Millisecond)
		mm.mu.Lock()
		if len(mm.queue) >= 2 {
			p1, _ := mm.dequeue()
			p2, _ := mm.dequeue()
			mm.nextID++
			m := &Match{
				ID:    mm.nextID,
				P1:    p1,
				P2:    p2,
				State: Running,
				Turn:  p1.ID, // p1 começa
				Hand:  map[int][]*Card{},
				Score: map[int]int{p1.ID:0, p2.ID:0},
				inbox: make(chan matchMsg, 16),
				stock: stock,
			}
			p1.InGame, p2.InGame = true, true
			mm.matches[m.ID] = m
			mm.byPlayer[p1.ID] = m
			mm.byPlayer[p2.ID] = m
			go m.run()
		}
		mm.mu.Unlock()
	}
}

// ===== Regras do Alucinari =====
// Vence por cor: red>blue, blue>white, white>red
func beats(a, b Color) bool {
	switch a {
	case Red:
		return b == Blue
	case Blue:
		return b == White
	case White:
		return b == Red
	}
	return false
}

func (m *Match) run() {
	enc1 := json.NewEncoder(m.P1.Conn)
	enc2 := json.NewEncoder(m.P2.Conn)

	// Escolher 7 cartas aleatórias do inventário de cada jogador
	m.Hand[m.P1.ID] = drawUpTo7(m.P1.Inventory)
	m.Hand[m.P2.ID] = drawUpTo7(m.P2.Inventory)

	// Enviar Game_start para cada jogador com SUA própria mão
	type startPayload struct{
		Info string `json:"info"`
		Turn int    `json:"turn"`
		Hand []*Card `json:"hand"`
	}
	msg1 := Message{Action: "Game_start"}
	msg2 := Message{Action: "Game_start"}
	p1 := startPayload{Info: m.P2.UserName, Turn: m.Turn, Hand: m.Hand[m.P1.ID]}
	p2 := startPayload{Info: m.P1.UserName, Turn: m.Turn, Hand: m.Hand[m.P2.ID]}
	b1, _ := json.Marshal(p1); b2, _ := json.Marshal(p2)
	msg1.Data = b1; msg2.Data = b2
	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)

	// Controla rodada atual: quando ambos jogarem, resolve
	var played map[int]*Card = make(map[int]*Card)

	for {
		select {
		case in := <-m.inbox:
			if in.Action == "play_card" {
				// Data: {"card": Card}
				type req struct{ Card Card `json:"card"` }
				var r req
				_ = json.Unmarshal(in.Data, &r)

				// validar se carta está na mão do jogador
				if !m.removeFromHand(in.PlayerID, &r.Card) {
					// ignore inválida
					continue
				}
				played[in.PlayerID] = &r.Card

				// Notificar recebimento e estado do turno
				m.notifyBoth(enc1, enc2, fmt.Sprintf("%s jogou uma carta.", playerName(m, in.PlayerID)))

				// Se ambos já jogaram, resolver rodada
				if len(played) == 2 {
					m.resolveRound(enc1, enc2, played[m.P1.ID], played[m.P2.ID])
					played = make(map[int]*Card)
				}
			}
		}
	}
}

func playerName(m *Match, id int) string {
	if m.P1.ID == id { return m.P1.UserName }
	return m.P2.UserName
}

func (m *Match) notifyBoth(enc1, enc2 *json.Encoder, info string) {
	type payload struct{ Info string `json:"info"`; Turn int `json:"turn"` }
	pl := payload{Info: info, Turn: m.Turn}
	b, _ := json.Marshal(pl)
	_ = enc1.Encode(Message{Action: "game_response", Data: b})
	_ = enc2.Encode(Message{Action: "game_response", Data: b})
}

func (m *Match) resolveRound(enc1, enc2 *json.Encoder, c1, c2 *Card) {
	// Determinar vencedor
	winner := 0 // 0=empate, 1=P1, 2=P2
	if beats(c1.Color, c2.Color) {
		winner = 1
	} else if beats(c2.Color, c1.Color) {
		winner = 2
	} else {
		// comparar pontos
		if c1.Points > c2.Points { winner = 1 }
		if c2.Points > c1.Points { winner = 2 }
	}

	scoreMsg := func() string {
		return fmt.Sprintf("Placar — %s: %d | %s: %d",
			m.P1.UserName, m.Score[m.P1.ID], m.P2.UserName, m.Score[m.P2.ID])
	}

	var info string
	switch winner {
	case 1:
		m.Score[m.P1.ID]++
		info = fmt.Sprintf("%s venceu a rodada (%s %d) vs (%s %d). %s",
			m.P1.UserName, strings.Title(string(c1.Color)), c1.Points,
			strings.Title(string(c2.Color)), c2.Points, scoreMsg())
		m.Turn = m.P2.ID // alterna turno
	case 2:
		m.Score[m.P2.ID]++
		info = fmt.Sprintf("%s venceu a rodada (%s %d) vs (%s %d). %s",
			m.P2.UserName, strings.Title(string(c2.Color)), c2.Points,
			strings.Title(string(c1.Color)), c1.Points, scoreMsg())
		m.Turn = m.P1.ID
	default:
		info = fmt.Sprintf("Empate (%s %d) vs (%s %d). %s",
			strings.Title(string(c1.Color)), c1.Points,
			strings.Title(string(c2.Color)), c2.Points, scoreMsg())
		// turno permanece
	}

	m.notifyBoth(enc1, enc2, info)

	// Verificar vitória (primeiro a 3)
	if m.Score[m.P1.ID] >= 3 || m.Score[m.P2.ID] >= 3 {
		m.State = Finished
		winnerName := m.P1.UserName
		if m.Score[m.P2.ID] > m.Score[m.P1.ID] { winnerName = m.P2.UserName }
		b, _ := json.Marshal(fmt.Sprintf("🏆 %s venceu a partida!", winnerName))
		_ = enc1.Encode(Message{Action: "game_finish", Data: b})
		_ = enc2.Encode(Message{Action: "game_finish", Data: b})
		m.P1.InGame, m.P2.InGame = false, false
		close(m.inbox)
		return
	}
}

func (m *Match) removeFromHand(playerID int, card *Card) bool {
	m.mu.Lock(); defer m.mu.Unlock()
	h := m.Hand[playerID]
	for i, c := range h {
		if c.Name == card.Name && c.Color == card.Color && c.Points == card.Points {
			// remove da mão
			m.Hand[playerID] = append(h[:i], h[i+1:]...)
			return true
		}
	}
	return false
}

func drawUpTo7(inv []*Card) []*Card {
	// embaralha e pega até 7
	cp := make([]*Card, len(inv))
	copy(cp, inv)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(cp), func(i, j int){ cp[i], cp[j] = cp[j], cp[i] })
	if len(cp) > 7 { cp = cp[:7] }
	return cp
}

// ============================
// ========== SERVIDOR ========
// ============================

type PlayerResponse struct {
	ID       int    `json:"id"`
	UserName string `json:"username"`
	Login    string `json:"login"`
}

var (
	pm            = NewPlayerManager()
	stock         = NewStock()
	mm            = NewMatchManager()
	loggedMu      sync.Mutex
	loggedByLogin = map[string]*Player{}
)

func main() {
	// Inicializa cartas e matchmaking
	stock.Init30GenericCards()
	go mm.matchmakingLoop(stock)

	addr := ":8080"
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		addr = v
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil { panic(err) }
	fmt.Println("Servidor Alucinari ouvindo em", addr)
	for {
		c, err := ln.Accept()
		if err != nil { continue }
		go handleConn(c)
		// conexão udp  para o ping
		go func() {
			addr, _ := net.ResolveUDPAddr("udp", ":8081")
			conn, _ := net.ListenUDP("udp", addr)
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, remote, _ := conn.ReadFromUDP(buf)
				msg := string(buf[:n])
				if msg == "ping" {
					conn.WriteToUDP([]byte("pong"), remote)
				}
			}
		}()
	}
	

}

func handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			return
		}
		switch msg.Action {
		case "create_player":
			handleCreatePlayer(msg, enc)
		case "login_player":
			handleLoginPlayer(msg, enc, conn)
		case "open_pack": // comprar booster (5 cartas)
			handleOpenPack(msg, enc)
		case "see_inventory":
			handleSeeInventory(msg, enc)
		case "enqueue_player":
			handleEnqueue(msg, enc)
		case "ping":
			handlePing(enc)
		case "Game_Action":
			handleGameAction(msg)
		}
	}
}

// ========== Handlers ==========

func handleCreatePlayer(msg Message, enc *json.Encoder) {
	var r struct{ Username, Login, Password string }
	if err := json.Unmarshal(msg.Data, &r); err != nil {
		sendError(enc, err)
		return
	}
	p, err := pm.CreatePlayer(r.Username, r.Login, r.Password)
	if err != nil { sendError(enc, err); return }
	resp := PlayerResponse{ID: p.ID, UserName: p.UserName, Login: p.Login}
	b, _ := json.Marshal(resp)
	_ = enc.Encode(Message{Action: "create_player_response", Data: b})
}

func handleLoginPlayer(msg Message, enc *json.Encoder, conn net.Conn) {
	var r struct{ Login, Password string }
	if err := json.Unmarshal(msg.Data, &r); err != nil { sendError(enc, err); return }

	// evita múltiplos logins do mesmo usuário
	loggedMu.Lock()
	if _, ok := loggedByLogin[r.Login]; ok {
		loggedMu.Unlock()
		sendError(enc, errors.New("user already logged"))
		return
	}
	loggedMu.Unlock()

	p, err := pm.Login(r.Login, r.Password, conn)
	if err != nil { sendError(enc, err); return }

	loggedMu.Lock(); loggedByLogin[r.Login] = p; loggedMu.Unlock()

	resp := PlayerResponse{ID: p.ID, UserName: p.UserName, Login: p.Login}
	b, _ := json.Marshal(resp)
	_ = enc.Encode(Message{Action: "login_player_response", Data: b})

	// Concede 2 boosters de 5 cartas ao logar
	for i := 0; i < 2; i++ {
		cards := stock.Booster(5)
		_ = pm.AddToInventory(p.ID, cards)
		packB, _ := json.Marshal(cards)
		_ = enc.Encode(Message{Action: "open_pack_response", Data: packB})
	}
}

func handleOpenPack(msg Message, enc *json.Encoder) {
	var r struct{ ID int `json:"id"` }
	if err := json.Unmarshal(msg.Data, &r); err != nil { sendError(enc, err); return }
	p, err := pm.GetByID(r.ID)
	if err != nil { sendError(enc, err); return }
	cards := stock.Booster(5)
	_ = pm.AddToInventory(p.ID, cards)
	b, _ := json.Marshal(cards)
	_ = enc.Encode(Message{Action: "open_pack_response", Data: b})
}

func handleSeeInventory(msg Message, enc *json.Encoder) {
	var r struct{ ID int `json:"id"` }
	if err := json.Unmarshal(msg.Data, &r); err != nil { sendError(enc, err); return }
	p, err := pm.GetByID(r.ID)
	if err != nil { sendError(enc, err); return }
	// enviar cópia simples (valores)
	simple := make([]Card, len(p.Inventory))
	for i := range p.Inventory { simple[i] = *p.Inventory[i] }
	b, _ := json.Marshal(simple)
	_ = enc.Encode(Message{Action: "see_inventory_response", Data: b})
}

func handleEnqueue(msg Message, enc *json.Encoder) {
	var r struct{ ID int `json:"id"` }
	if err := json.Unmarshal(msg.Data, &r); err != nil { sendError(enc, err); return }
	p, err := pm.GetByID(r.ID)
	if err != nil { sendError(enc, err); return }
	if err := mm.Enqueue(p); err != nil { sendError(enc, err); return }
	b, _ := json.Marshal(map[string]string{"Player enqueued": p.UserName})
	_ = enc.Encode(Message{Action: "enqueue_response", Data: b})
}

func handlePing(enc *json.Encoder) {
	b, _ := json.Marshal("pong")
	_ = enc.Encode(Message{Action: "pong_response", Data: b})
}

func handleGameAction(msg Message) {
	var gm GameMessage
	if err := json.Unmarshal(msg.Data, &gm); err != nil { return }
	m := mm.FindMatchByPlayerID(gm.PlayerID)
	if m == nil { return }
	m.inbox <- matchMsg{PlayerID: gm.PlayerID, Action: gm.Action, Data: gm.Data}
}

func sendError(enc *json.Encoder, err error) {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	_ = enc.Encode(Message{Action: "error_response", Data: b})
}