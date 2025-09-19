// Arquivo: client.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	connection net.Conn
	enc        *json.Encoder
	dec        *json.Decoder

	// dados do jogador
	uid      string
	username string
	loggedIn bool

	// dados do jogo
	inventory  []*Card
	invMu      sync.RWMutex
	hand       []*Card
	matchInfo  *MatchInfo
	inBattle   bool
	turnSignal chan struct{}

	// Novo mutex para dados da partida
	matchMu sync.RWMutex
)

const (
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

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"request"`
	UID     string          `json:"uid"` // user id
	Data    json.RawMessage `json:"data"`
}

type PlayerResponse struct {
	UID      string `json:"UID"`
	Username string `json:"username"`
}

type Card struct {
	Name       string     `json:"name"`
	CID        string     `json:"CID"`  // card ID
	Desc       string     `json:"desc"` // descrição
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

func main() {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var err error
	connection, err = net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer connection.Close()

	dec = json.NewDecoder(connection)
	enc = json.NewEncoder(connection)

	// Canal com buffer para evitar deadlock
	turnSignal = make(chan struct{}, 1)
	matchInfo = &MatchInfo{
		Sanity:      make(map[string]int),
		DreamStates: make(map[string]DreamState),
	}

	go handleServerMessages()
	showMenu()
}

func handleServerMessages() {
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			if inBattle {
				fmt.Println("❌ Conexão com o servidor perdida. Encerrando o jogo...")
				inBattle = false
			} else {
				fmt.Println("❌ Conexão com o servidor perdida.")
			}
			return
		}

		handleResponse(msg)
	}
}

func showMenu() {
	reader := bufio.NewReader(os.Stdin)
	for {
		if inBattle {
			<-turnSignal
			handleBattleTurn()
			continue
		}

		clearScreen()
		fmt.Println("--- Menu ---")
		if !loggedIn {
			fmt.Println("1. Registrar")
			fmt.Println("2. Login")
		} else {
			fmt.Println("3. Comprar booster")
			fmt.Println("4. Ver inventário")
			fmt.Println("5. Batalhar")
			fmt.Println("6. Ping")
		}
		fmt.Println("7. Sair")
		fmt.Print("Escolha uma opção: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			if !loggedIn {
				handleRegister(reader)
			}
		case "2":
			if !loggedIn {
				handleLogin(reader)
			}
		case "3":
			if loggedIn {
				handleBuyPack()
			}
		case "4":
			if loggedIn {
				printInventory()
			}
		case "5":
			if loggedIn {
				handleEnqueue()
			}
		case "6":
			testLatency()
		case "7":
			fmt.Println("💤 Bons sonhos...")
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}

func handleResponse(msg Message) {
	clearScreen()
	switch msg.Request {
	case registered:
		var resp PlayerResponse
		json.Unmarshal(msg.Data, &resp)
		uid = resp.UID
		username = resp.Username
		loggedIn = true
		fmt.Printf("✅ Criado jogador #%s (%s)\n", uid, username)
		fmt.Printf("Você ganhou 4 boosters gratuitos! Eles já estão em seu inventário\n")
	case loggedin:
		var resp PlayerResponse
		json.Unmarshal(msg.Data, &resp)
		uid = resp.UID
		username = resp.Username
		loggedIn = true
		fmt.Printf("✅ Login bem-sucedido! Bem-vindo, %s!\n", username)
	case packbought:
		var cards []Card
		json.Unmarshal(msg.Data, &cards)
		invMu.Lock()
		for i := range cards {
			c := cards[i]
			inventory = append(inventory, &c)
		}
		invMu.Unlock()
		fmt.Println("🎁 Novo booster adquirido! Veja em seu inventário")
	case enqueued:
		fmt.Println("⏳ Entrou na fila. Aguardando oponente...")
	case gamestart:
		var payload struct {
			Info        string
			Turn        string
			Hand        []Card
			Sanity      map[string]int
			DreamStates map[string]DreamState
		}
		json.Unmarshal(msg.Data, &payload)
		inBattle = true
		matchMu.Lock()
		hand = make([]*Card, len(payload.Hand))
		for i := range payload.Hand {
			hand[i] = &payload.Hand[i]
		}
		matchInfo.OpponentUsername = payload.Info
		matchInfo.Sanity = payload.Sanity
		matchInfo.DreamStates = payload.DreamStates
		matchInfo.CurrentTurnUID = payload.Turn
		matchMu.Unlock()

		fmt.Printf("⚔️ Partida encontrada! Você está batalhando contra %s.\n", matchInfo.OpponentUsername)
		fmt.Println("Sanidade inicial:")
		fmt.Printf("Você: %d\n", matchInfo.Sanity[uid])
		fmt.Printf("Seu oponente: %d\n", matchInfo.Sanity[getOpponentUID()])
		if matchInfo.CurrentTurnUID == uid {
			turnSignal <- struct{}{}
		} else {
			fmt.Printf("⏳ Turno do seu oponente. Aguarde...\n")
		}
	case newturn:
		var payload struct {
			Turn string
		}
		json.Unmarshal(msg.Data, &payload)
		matchMu.Lock()
		matchInfo.CurrentTurnUID = payload.Turn
		matchMu.Unlock()

		if matchInfo.CurrentTurnUID == uid {
			fmt.Printf("\n--- Status do Jogo ---\n")
			fmt.Printf("Rodada: %d\n", matchInfo.Round)
			fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
			opponentUID := getOpponentUID()
			fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))
			fmt.Println("\n➡️ É o seu turno! Escolha uma carta para jogar (pelo número) ou digite `gv` para desistir.")
			// Limpa o canal antes de enviar um novo sinal
			select {
			case <-turnSignal:
			default:
			}
			turnSignal <- struct{}{}
		} else {
			fmt.Printf("\n--- Status do Jogo ---\n")
			fmt.Printf("Rodada: %d\n", matchInfo.Round)
			fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
			opponentUID := getOpponentUID()
			fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))
			fmt.Printf("\n⏳ Turno do seu oponente. Aguarde...\n")
		}
	case notify:
		var payload struct {
			Message string
		}
		json.Unmarshal(msg.Data, &payload)
		fmt.Printf("📣 %s\n", payload.Message)
	case updateinfo:
		var payload struct {
			Turn        string
			Sanity      map[string]int
			DreamStates map[string]DreamState
			Round       int
		}
		json.Unmarshal(msg.Data, &payload)
		matchMu.Lock()
		matchInfo.Sanity = payload.Sanity
		matchInfo.DreamStates = payload.DreamStates
		matchInfo.Round = payload.Round
		matchMu.Unlock()

		fmt.Printf("\n--- Status do Jogo ---\n")
		fmt.Printf("Rodada: %d\n", matchInfo.Round)
		fmt.Printf("Sua Sanidade: %d (%s)\n", matchInfo.Sanity[uid], strings.Title(string(matchInfo.DreamStates[uid])))
		opponentUID := getOpponentUID()
		fmt.Printf("Sanidade do Oponente: %d (%s)\n", matchInfo.Sanity[opponentUID], strings.Title(string(matchInfo.DreamStates[opponentUID])))
	case newvictory:
		inBattle = false
		fmt.Println("\n🎉 Vitória! Você venceu a partida!")
	case newloss:
		inBattle = false
		fmt.Println("\n💔 Derrota. Você perdeu a partida.")
	case newtie:
		inBattle = false
		fmt.Println("\n🤝 Empate! A partida terminou em um empate.")
	default:
		// Se for um erro do servidor, exibe a mensagem de erro
		var errPayload struct {
			Error string `json:"error"`
		}
		json.Unmarshal(msg.Data, &errPayload)
		if errPayload.Error != "" {
			fmt.Printf("❌ Erro do servidor: %s\n", errPayload.Error)
		} else {
			fmt.Printf("Recebida mensagem desconhecida do servidor: %s\n", msg.Request)
		}
	}
}

func handleRegister(reader *bufio.Reader) {
	fmt.Print("Digite seu nome de usuário: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Digite sua senha: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	req := Message{
		Request: register,
		Data:    data,
	}
	enc.Encode(req)
}

func handleLogin(reader *bufio.Reader) {
	fmt.Print("Digite seu nome de usuário: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Digite sua senha: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	data, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	req := Message{
		Request: login,
		Data:    data,
	}
	enc.Encode(req)
}

func handleBuyPack() {
	data, _ := json.Marshal(map[string]string{
		"UID": uid,
	})
	req := Message{
		Request: buypack,
		UID:     uid,
		Data:    data,
	}
	enc.Encode(req)
}

func handleEnqueue() {
	data, _ := json.Marshal(map[string]string{
		"UID": uid,
	})
	req := Message{
		Request: battle,
		UID:     uid,
		Data:    data,
	}
	enc.Encode(req)
}

func handleBattleTurn() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nSua mão atual:\n")
	printHand()
	fmt.Print("Sua jogada: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "gv" {
		giveUp()
		return
	}

	matchMu.RLock()
	index, err := strconv.Atoi(input)
	if err != nil || index < 1 || index > len(hand) {
		matchMu.RUnlock()
		fmt.Println("❌ Entrada inválida. Por favor, jogue uma carta pelo seu número (ex: 1) ou digite `gv` para desistir.")
		// Envia um novo sinal para o canal para que o menu de batalha se repita
		select {
		case <-turnSignal:
		default:
		}
		turnSignal <- struct{}{}
		return
	}
	cardToPlay := hand[index-1]
	matchMu.RUnlock()

	useCard(cardToPlay)
}

func useCard(card *Card) {
	data, _ := json.Marshal(map[string]Card{
		"card": *card,
	})
	req := Message{
		Request: usecard,
		UID:     uid,
		Data:    data,
	}
	enc.Encode(req)

	matchMu.Lock()
	defer matchMu.Unlock()
	// remove a carta da mão localmente
	for i, c := range hand {
		if c.CID == card.CID {
			hand = append(hand[:i], hand[i+1:]...)
			break
		}
	}
}

func giveUp() {
	req := Message{
		Request: giveup,
		UID:     uid,
	}
	enc.Encode(req)
}

func getOpponentUID() string {
	matchMu.RLock()
	defer matchMu.RUnlock()
	for id := range matchInfo.Sanity {
		if id != uid {
			return id
		}
	}
	return ""
}

// função que mostra inventário
func printInventory() {
	invMu.RLock()
	defer invMu.RUnlock()

	if len(inventory) == 0 {
		fmt.Println("inventário vazio.")
		time.Sleep(1 * time.Second)
		return
	}
	fmt.Println("\n📦 inventário:")
	for _, c := range inventory {
		fmt.Printf("%s) %s\n", c.CID, strings.Title(c.Name))
		fmt.Printf(" Tipo: %s\n", strings.Title(string(c.CardType)))
		if c.Points == 0 {
			fmt.Printf(" Pontos: %d\n", c.Points)
		} else {
			if c.CardType == Pill {
				fmt.Printf(" Pontos: +%d\n", c.Points)
			} else {
				fmt.Printf(" Pontos: -%d\n", c.Points)
			}
		}
		fmt.Printf(" Raridade: %s\n", strings.Title(string(c.CardRarity)))
		fmt.Printf(" Efeito: %s\n", strings.Title(string(c.CardEffect)))
		fmt.Printf(" Descrição: %s\n", strings.Title(c.Desc))
		fmt.Println(strings.Repeat("-", 40))
	}

	time.Sleep(2 * time.Second)

}

func printHand() {
	matchMu.RLock()
	defer matchMu.RUnlock()

	if len(hand) == 0 {
		fmt.Println("Sua mão está vazia!")
		return
	}
	fmt.Println(strings.Repeat("=", 40))
	for i, c := range hand {
		fmt.Printf("%d) %s (Tipo: %s, Pontos: %d, Efeito: %s)\n", i+1, c.Name, c.CardType, c.Points, c.CardEffect)
	}
	fmt.Println(strings.Repeat("=", 40))
}

// função para ping
func testLatency() {
	serverAddr, err := net.ResolveUDPAddr("udp", ":8081")
	if err != nil {
		fmt.Printf("❌ erro ao resolver endereço: %v\n", err)
		return
	}

	connection, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Printf("❌ erro ao conectar: %v\n", err)
		return
	}
	defer connection.Close()

	// timeout de 999 ms
	connection.SetReadDeadline(time.Now().Add(999 * time.Millisecond))

	start := time.Now()
	_, err = connection.Write([]byte("ping"))
	if err != nil {
		fmt.Printf("❌ erro ao enviar ping: %v\n", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := connection.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("⏰ timeout: %v\n", err)
		return
	}

	if string(buffer[:n]) == "pong" {
		elapsed := time.Since(start).Milliseconds()
		fmt.Printf("🏓 latência: %d ms\n", elapsed)
		time.Sleep(2 * time.Second)
	} else {
		fmt.Printf("❌ resposta inválida: %s\n", string(buffer[:n]))
	}
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin": // Unix-like systems
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		fmt.Println("\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n") // fallback
	}
}
