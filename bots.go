package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// ALGUMAS CONSTANTES PRESENTES NO JOGO
// EM NUMBOTS É O NÚMERO DE BOTS A RODAR NO SERVIDOR!!!
const (
	NUMBOTS    int    = 500
	register   string = "register"
	login      string = "login"
	buypack    string = "buyNewPack"
	battle     string = "battle"
	usecard    string = "useCard"
	giveup     string = "giveUp"
	ping       string = "ping"
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

type CardType string
type CardRarity string
type CardEffect string
type DreamState string

type Message struct {
	Request string          `json:"request"`
	UID     string          `json:"uid"`
	Data    json.RawMessage `json:"data"`
}

type PlayerResponse struct {
	UID      string `json:"UID"`
	Username string `json:"username"`
}

type Card struct {
	Name       string     `json:"name"`
	CID        string     `json:"CID"`
	Desc       string     `json:"desc"`
	CardType   CardType   `json:"cardtype"`
	CardRarity CardRarity `json:"cardrarity"`
	CardEffect CardEffect `json:"cardeffect"`
	Points     int        `json:"points"`
}

type MatchInfo struct {
	OpponentUsername string
	Sanity           map[string]int
	DreamStates      map[string]DreamState
	CurrentTurnUID   string
	Round            int
}

// BotClient representa um cliente bot
type BotClient struct {
	id         int
	username   string
	password   string
	uid        string
	conn       net.Conn
	dec        *json.Decoder
	enc        *json.Encoder
	loggedIn   bool
	inBattle   bool
	matchInfo  *MatchInfo
	hand       []*Card
	inventory  []*Card
	logPrefix  string
	turnSignal chan struct{}
}

// NewBotClient cria uma nova instância de bot
func NewBotClient(id int) *BotClient {
	username := fmt.Sprintf("bot_%d", id)
	password := "12345"
	return &BotClient{
		id:         id,
		username:   username,
		password:   password,
		matchInfo:  &MatchInfo{Sanity: make(map[string]int), DreamStates: make(map[string]DreamState)},
		hand:       make([]*Card, 0),
		inventory:  make([]*Card, 0),
		logPrefix:  fmt.Sprintf("[Bot %d - %s]", id, username),
		turnSignal: make(chan struct{}, 1),
	}
}

// logInfo exibe uma mensagem de informação com o prefixo do bot
func (b *BotClient) logInfo(format string, a ...interface{}) {
	fmt.Printf("%s INFO: %s\n", b.logPrefix, fmt.Sprintf(format, a...))
}

// logError exibe uma mensagem de erro com o prefixo do bot
func (b *BotClient) logError(format string, a ...interface{}) {
	fmt.Printf("%s ERRO: %s\n", b.logPrefix, fmt.Sprintf(format, a...))
}

// connect tenta se conectar ao servidor
func (b *BotClient) connect(addr string) error {
	var err error
	b.conn, err = net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	b.dec = json.NewDecoder(b.conn)
	b.enc = json.NewEncoder(b.conn)
	return nil
}

// handleServerMessages escuta e processa as mensagens do servidor
func (b *BotClient) handleServerMessages() {
	for {
		// Define um timeout de leitura para evitar que a goroutine trave indefinidamente
		b.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var msg Message
		if err := b.dec.Decode(&msg); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Se for um timeout, o bot tenta a próxima ação
				b.logError("Timeout ao receber mensagem, a conexão pode estar travada. Tentando prosseguir...")
				return // Retorna para o loop principal do run()
			}
			b.logError("Conexão perdida, encerrando... Erro: %v", err)
			b.conn.Close()
			return
		}
		b.handleResponse(msg)
	}
}

// handleResponse processa as respostas do servidor, tipo, q que o servidor mandou?
func (b *BotClient) handleResponse(msg Message) {
	switch msg.Request {
	case registered:
		var resp PlayerResponse
		json.Unmarshal(msg.Data, &resp)
		b.uid = resp.UID
		b.loggedIn = true
		b.logInfo("Registrado com sucesso! UID: %s", b.uid)
	case loggedin:
		var resp PlayerResponse
		json.Unmarshal(msg.Data, &resp)
		b.uid = resp.UID
		b.loggedIn = true
		b.logInfo("Login bem-sucedido! UID: %s", b.uid)
	case packbought:
		var cards []Card
		json.Unmarshal(msg.Data, &cards)
		for i := range cards {
			c := cards[i]
			b.inventory = append(b.inventory, &c)
		}
		b.logInfo("Booster adquirido! Inventário agora tem %d cartas.", len(b.inventory))
	case enqueued:
		b.logInfo("Entrou na fila de batalha.")
	case gamestart:
		var payload struct {
			Info        string
			Turn        string
			Hand        []Card
			Sanity      map[string]int
			DreamStates map[string]DreamState
		}
		json.Unmarshal(msg.Data, &payload)
		b.inBattle = true
		b.matchInfo.OpponentUsername = payload.Info
		b.matchInfo.Sanity = payload.Sanity
		b.matchInfo.DreamStates = payload.DreamStates
		b.matchInfo.CurrentTurnUID = payload.Turn
		b.hand = make([]*Card, len(payload.Hand))
		for i := range payload.Hand {
			b.hand[i] = &payload.Hand[i]
		}
		b.logInfo("Partida encontrada contra %s! Começando a batalha...", b.matchInfo.OpponentUsername)
		if b.matchInfo.CurrentTurnUID == b.uid {
			b.logInfo("É o nosso turno! Vamo jogar!")
			b.turnSignal <- struct{}{}
		} else {
			b.logInfo("Turno do oponente. Aguardando a vez...")
		}
	case newturn:
		var payload struct {
			Turn string
		}
		json.Unmarshal(msg.Data, &payload)
		b.matchInfo.CurrentTurnUID = payload.Turn
		if b.matchInfo.CurrentTurnUID == b.uid {
			b.logInfo("É o nosso turno! Chegou a hora de jogar uma carta.")
			b.turnSignal <- struct{}{}
		} else {
			b.logInfo("Turno do oponente, aguardando...")
		}
	case notify:
		var payload struct {
			Message string
		}
		json.Unmarshal(msg.Data, &payload)
		// b.logInfo("Notificação do jogo: %s", payload.Message) // tirei por info dump
	case updateinfo:
		var payload struct {
			Turn        string
			Sanity      map[string]int
			DreamStates map[string]DreamState
			Round       int
		}
		json.Unmarshal(msg.Data, &payload)
		b.matchInfo.Sanity = payload.Sanity
		b.matchInfo.DreamStates = payload.DreamStates
		b.matchInfo.Round = payload.Round
		b.logInfo("Estado atualizado. Nossa sanidade: %d, Sanidade do oponente: %d", b.matchInfo.Sanity[b.uid], b.matchInfo.Sanity[b.getOpponentUID()])
	case newvictory:
		b.inBattle = false
		b.logInfo("Vitória! Desconectando")
	case newloss:
		b.inBattle = false
		b.logInfo("Derrota. Desconectando")
	case newtie:
		b.inBattle = false
		b.logInfo("Empate. Desconectando")
	default:
		var errPayload struct {
			Error string `json:"error"`
		}
		json.Unmarshal(msg.Data, &errPayload)
		if errPayload.Error != "" {
			b.logError("Erro do servidor: %s", errPayload.Error)
		} else {
			b.logInfo("Mensagem desconhecida do servidor: %s", msg.Request)
		}
	}
}

// send envia uma mensagem para o servidor
func (b *BotClient) send(requestType string, data interface{}) {
	reqData, _ := json.Marshal(data)
	req := Message{
		Request: requestType,
		UID:     b.uid,
		Data:    reqData,
	}
	if b.enc == nil {
		b.logError("Encoder nulo, não posso enviar mensagem")
		return
	}
	if err := b.enc.Encode(req); err != nil {
		b.logError("Erro ao enviar mensagem: %v", err)
	}
}

// register registra um novo bot
func (b *BotClient) register() {
	b.logInfo("Tentando registrar...")
	b.send(register, map[string]string{"username": b.username, "password": b.password})
}

// login tenta logar um bot
func (b *BotClient) login() {
	b.logInfo("Tentando login...")
	b.send(login, map[string]string{"username": b.username, "password": b.password})
}

// buyPack compra um booster
func (b *BotClient) buyPack() {
	b.logInfo("Comprando booster...")
	b.send(buypack, map[string]string{"UID": b.uid})
}

// enqueue entra na fila de matchmaking
func (b *BotClient) enqueue() {
	b.logInfo("Entrando na fila de batalha...")
	b.send(battle, map[string]string{"UID": b.uid})
}

// playCard joga uma carta da mão
func (b *BotClient) playCard() {
	if len(b.hand) == 0 {
		b.logInfo("Mão vazia, sem cartas para jogar...")
		b.giveUp()
		return
	}
	cardToPlay := b.hand[0]
	// b.logInfo("Jogando a carta %s...", cardToPlay.Name) // tirei por ser info dump
	b.send(usecard, map[string]Card{"card": *cardToPlay})

	// Remove a carta da mão localmente pra não confundir o bot
	b.hand = b.hand[1:]
}

// giveUp desiste da partida
func (b *BotClient) giveUp() {
	b.logInfo("Desistindo da partida...")
	b.send(giveup, nil)
}

// getOpponentUID acha o UID do oponente
func (b *BotClient) getOpponentUID() string {
	for id := range b.matchInfo.Sanity {
		if id != b.uid {
			return id
		}
	}
	return ""
}

// run é a função principal do bot
func (b *BotClient) run() {
	// Sincroniza a espera para que os bots não comecem todos ao mesmo tempo
	time.Sleep(time.Duration(b.id) * 200 * time.Millisecond)

	// Conecta ao servidor
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "localhost:8080"
	}
	if err := b.connect(addr); err != nil {
		b.logError("Falha ao conectar: %v", err)
		return
	}
	defer b.conn.Close()

	// Escuta as mensagens do servidor em uma goroutine
	go b.handleServerMessages()

	// Loop para garantir o registro/login antes de prosseguir
	maxAttempts := 5
	for i := 0; i < maxAttempts && !b.loggedIn; i++ {
		// 1. Tenta registrar
		b.register()
		//time.Sleep(1 * time.Second) // Dá um tempo para o servidor responder

		// 2. Tenta fazer login, caso o registro falhe ou já exista
		if !b.loggedIn {
			b.login()
			//time.Sleep(1 * time.Second) // Dá um tempo para o servidor responder
		}
	}

	if !b.loggedIn {
		b.logError("Falha ao se registrar/logar após %d tentativas. Encerrando bot.", maxAttempts)
		return
	}

	// 3. Compra uns boosters para ter carta no inventário
	for i := 0; i < 2; i++ {
		b.buyPack()
		time.Sleep(500 * time.Millisecond)
	}

	// 4. Entra na fila de batalha, bora ver o que acontece
	b.enqueue()
	time.Sleep(1 * time.Second)

	// Loop principal da batalha
	for {
		if !b.inBattle {
			b.logInfo("Aguardando por uma nova partida ou final da atual...")
			time.Sleep(5 * time.Second)
			if !b.inBattle {
				b.enqueue()
				time.Sleep(1 * time.Second)
			}
		} else {
			b.logInfo("Esperando nosso turno...")
			<-b.turnSignal
			if len(b.hand) > 0 {
				b.playCard()
			} else {
				b.logInfo("Mão vazia, desistindo!")
				b.giveUp()
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func main() {
	// quantos bots rodar
	numBotsStr := os.Getenv("NUM_BOTS")
	numBots, err := strconv.Atoi(numBotsStr)
	if err != nil || numBots <= 0 {
		numBots = NUMBOTS // quantos forem necessários
	}
	fmt.Printf("Iniciando %d bots de teste...\n", numBots)

	var wg sync.WaitGroup
	for i := 1; i <= numBots; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bot := NewBotClient(id)
			bot.run()
		}(i)
	}

	wg.Wait()
	fmt.Println("Todos os bots terminaram.")
}
