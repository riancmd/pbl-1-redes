package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Mesmo protocolo de mensagens
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type GameMessage struct {
	PlayerID int             `json:"PlayerID"`
	Action   string          `json:"action"`
	Data     json.RawMessage `json:"data"`
}

// Cartas
// (mantém exatamente os mesmos campos do servidor)
type Card struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	Points int    `json:"points"`
}

// Estado do cliente
var (
	sessionID int
	invMu     sync.RWMutex
	inventory []Card

	handMu sync.RWMutex
	hand   []Card // mão atual de 7 cartas (enviada no Game_start)

	turnMu sync.RWMutex
	turn   int // ID do jogador cujo turno é agora

	enc *json.Encoder
)

func main() {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" { addr = "127.0.0.1:8080" }
	conn, err := net.Dial("tcp", addr)
	if err != nil { panic(err) }
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc = json.NewEncoder(conn)

	go readLoop(dec)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\n==============================")
		fmt.Println(" 🎮 Alucinari - Menu Principal ")
		fmt.Println("==============================")
		fmt.Println("1 - Registrar Jogador")
		fmt.Println("2 - Login")
		fmt.Println("3 - Comprar/Abrir Pacote (5 cartas)")
		fmt.Println("4 - Ver Inventário")
		fmt.Println("5 - Entrar na Fila (Matchmaking)")
		fmt.Println("6 - Ping")
		fmt.Println("0 - Sair")
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		switch line {
		case "1":
			username := prompt(reader, "Nome de usuário: ")
			login := prompt(reader, "Login: ")
			pass := prompt(reader, "Senha: ")
			send("create_player", map[string]string{"username": username, "login": login, "password": pass})
		case "2":
			login := prompt(reader, "Login: ")
			pass := prompt(reader, "Senha: ")
			send("login_player", map[string]string{"login": login, "password": pass})
		case "3":
			if sessionID == 0 { fmt.Println("Precisa estar logado."); continue }
			send("open_pack", map[string]int{"id": sessionID})
		case "4":
			if sessionID == 0 { fmt.Println("Precisa estar logado."); continue }
			send("see_inventory", map[string]int{"id": sessionID})
		case "5":
			if sessionID == 0 { fmt.Println("Precisa estar logado."); continue }
			send("enqueue_player", map[string]int{"id": sessionID})
		case "6":
			testLatency()
		case "0":
			fmt.Println("Tchau!")
			return
		default:
			fmt.Println("Opção inválida")
		}
	}
}

func readLoop(dec *json.Decoder) {
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil { return }
		switch msg.Action {
		case "create_player_response":
			var r struct{ ID int; Username, Login string `json:"username","login"` }
			_ = json.Unmarshal(msg.Data, &r)
			fmt.Printf("✅ Criado jogador #%d (%s)\n", r.ID, r.Username)
		case "login_player_response":
			var r struct{ ID int; Username, Login string `json:"username","login"` }
			_ = json.Unmarshal(msg.Data, &r)
			sessionID = r.ID
			fmt.Printf("🔓 Login ok! Bem-vindo, %s (ID %d). Você ganhou 2 boosters iniciais!\n", r.Username, r.ID)
		case "open_pack_response":
			var cards []Card
			_ = json.Unmarshal(msg.Data, &cards)
			invMu.Lock()
			inventory = append(inventory, cards...)
			invMu.Unlock()
			fmt.Printf("🎁 Booster recebido: %d cartas adicionadas ao inventário. Total agora: %d\n", len(cards), len(inventory))
		case "see_inventory_response":
			var cards []Card
			_ = json.Unmarshal(msg.Data, &cards)
			invMu.Lock(); inventory = cards; invMu.Unlock()
			printInventory()
		case "enqueue_response":
			fmt.Println("⏳ Entrou na fila. Aguardando oponente...")
		case "pong_response":
			fmt.Println("🏓 pong", time.Now().Format(time.RFC822))
		case "Game_start":
			var r struct{ Info string `json:"info"`; Turn int `json:"turn"`; Hand []Card `json:"hand"` }
			_ = json.Unmarshal(msg.Data, &r)
			turnMu.Lock(); turn = r.Turn; turnMu.Unlock()
			handMu.Lock(); hand = r.Hand; handMu.Unlock()
			fmt.Printf("⚔️  Pareado com: %s. Primeiro turno: #%d.\n", r.Info, r.Turn)
			go gameLoop()
		case "game_response":
			var r struct{ Info string `json:"info"`; Turn int `json:"turn"` }
			_ = json.Unmarshal(msg.Data, &r)
			turnMu.Lock(); turn = r.Turn; turnMu.Unlock()
			fmt.Println("ℹ️ ", r.Info)
			if r.Turn == sessionID { fmt.Println("👉 É seu turno.") }
		case "game_finish":
			var msgtxt string
			_ = json.Unmarshal(msg.Data, &msgtxt)
			fmt.Println(msgtxt)
			// limpa mão
			handMu.Lock(); hand = nil; handMu.Unlock()
		}
	}
}

func gameLoop() {
	reader := bufio.NewReader(os.Stdin)
	for {
		handMu.RLock(); localHand := append([]Card(nil), hand...); handMu.RUnlock()
		if len(localHand) == 0 { return } // acabou (ou partida terminou)

		turnMu.RLock(); t := turn; turnMu.RUnlock()
		if t != sessionID {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		fmt.Println("\n🃏 Sua mão:")
		for i, c := range localHand {
			fmt.Printf("%d) %s [%s %d]\n", i+1, c.Name, strings.ToUpper(c.Color), c.Points)
		}
		s := prompt(reader, "Escolha uma carta (número): ")
		idx := atoiSafe(s) - 1
		if idx < 0 || idx >= len(localHand) {
			fmt.Println("Escolha inválida")
			continue
		}
		chosen := localHand[idx]

		// remove localmente da mão para UX
		handMu.Lock()
		if idx >= 0 && idx < len(hand) {
			hand = append(hand[:idx], hand[idx+1:]...)
		}
		handMu.Unlock()

		// Envia ação de jogar carta
		b, _ := json.Marshal(map[string]any{"card": chosen})
		gm := GameMessage{PlayerID: sessionID, Action: "play_card", Data: b}
		bb, _ := json.Marshal(gm)
		_ = enc.Encode(Message{Action: "Game_Action", Data: bb})
	}
}

func send(action string, payload any) {
	b, _ := json.Marshal(payload)
	_ = enc.Encode(Message{Action: action, Data: b})
}

func prompt(r *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}

func atoiSafe(s string) int {
	var x int
	for _, ch := range s {
		if ch < '0' || ch > '9' { return 0 }
		x = x*10 + int(ch-'0')
	}
	return x
}

func printInventory() {
	invMu.RLock(); defer invMu.RUnlock()
	if len(inventory) == 0 { fmt.Println("Inventário vazio."); return }
	fmt.Println("\n📦 Inventário:")
	for i, c := range inventory {
		fmt.Printf("%2d) %-16s [%s %d]\n", i+1, c.Name, strings.ToUpper(c.Color), c.Points)
	}
}

func testLatency() {
    serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
    conn, _ := net.DialUDP("udp", nil, serverAddr)
    defer conn.Close()

    start := time.Now()
    conn.Write([]byte("ping"))

    buf := make([]byte, 1024)
    n, _, _ := conn.ReadFromUDP(buf)
    if string(buf[:n]) == "pong" {
        elapsed := time.Since(start).Milliseconds()
        fmt.Printf("Latência: %d ms\n", elapsed)
    }
}