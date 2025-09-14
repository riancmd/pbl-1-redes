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

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"request"`
	UID     string          `json:"uid"` // user id
	Data    json.RawMessage `json:"data"`
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

// Estado do cliente
var (
	sessionID     string
	name          string
	sessionActive bool

	invMu     sync.RWMutex
	inventory []Card

	handMu sync.RWMutex
	hand   []Card

	sanity     int
	dreamst    DreamState
	round      int
	turn       string
	IsInBattle bool
	gameResult string

	enc *json.Encoder
)

func main() {
	sessionActive = false

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	// cria conexão tcp
	connection, err := net.Dial("tcp", addr)

	if err != nil {
		panic(err) // cria mensagem de erro usando panic()
	}

	defer connection.Close() // fecha conexão ao fim do programa

	//cria decoder e encoder
	dec := json.NewDecoder(connection)
	enc = json.NewEncoder(connection)

	go readMsgs(dec) // thread para ler as msgs do servidor

	reader := bufio.NewReader(os.Stdin) // leitura teclado
	clearScreen()
	for {
		fmt.Println("\n==============================")
		fmt.Println(" 🎮 Alucinari - Menu Principal ")
		fmt.Println("==============================")
		fmt.Println("1 - Registrar")
		fmt.Println("2 - Login")
		fmt.Println("3 - Comprar pacotes")
		fmt.Println("4 - Ver Inventário")
		fmt.Println("5 - Batalhar")
		fmt.Println("6 - Verificar ping")
		fmt.Println("0 - Sair")
		fmt.Print("> ")

		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		switch line {
		case "1":
			if sessionActive {
				prompt(reader, "Você já está conectado...")
				time.Sleep(2 * time.Second)
			} else {
				username := prompt(reader, "Nome de usuário: ")
				pass := prompt(reader, "Senha: ")
				send(register, map[string]string{"username": username, "password": pass})
			}
		case "2":
			if sessionActive {
				fmt.Println("Você já está logado!")
				time.Sleep(2 * time.Second)
			} else {
				username := prompt(reader, "Login: ")
				pass := prompt(reader, "Senha: ")
				send(login, map[string]string{"username": username, "password": pass})
				time.Sleep(2 * time.Second)
			}
			clearScreen()

		case "3":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2 * time.Second)
				continue
			}
			send(buypack, map[string]string{"UID": sessionID})
			time.Sleep(1 * time.Second)
		case "4":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2 * time.Second)
				continue
			}
			printInventory()
		case "5":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2 * time.Second)
				continue
			}
			send(battle, map[string]string{"UID": sessionID})
		case "6":
			testLatency()
		case "0":
			fmt.Println("Bons sonhos... 🌙🌃💤")
			return
		default:
			fmt.Println("Opção inválida")
			time.Sleep(2 * time.Second)
		}

	}
}

// printa e pega input
func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// envia as solicitações pro servidor
func send(request string, payload any) {
	data, _ := json.Marshal(payload)
	// usa o Encode para enviar pela conexão TCP
	_ = enc.Encode(Message{Request: request, UID: sessionID, Data: data})
}

// função que verifica em loop as respostas do servidor
func readMsgs(dec *json.Decoder) {
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			return
		}

		// verifica cada caso do Request (resposta do server)
		switch msg.Request {
		case registered:
			var temp struct {
				UID      string `json:"UID"`
				Username string `json:"username"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("✅ Criado jogador #%s (%s)\n", temp.UID, temp.Username)
			fmt.Printf("Você ganhou 4 boosters gratuitos! Eles já estão em seu inventário")
			sessionID = temp.UID
			sessionActive = true
			name = temp.Username
		case loggedin:
			var temp struct {
				UID      string `json:"UID"`
				Username string `json:"username"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("🔓 Login ok! Bem-vindo, %s.\n", temp.Username)
			sessionID = temp.UID
			sessionActive = true
			name = temp.Username
		case packbought:
			var booster []Card
			_ = json.Unmarshal(msg.Data, &booster) // coloca array de cartas em variável no client

			// trata concorreência no inventário
			invMu.Lock()
			inventory = append(inventory, booster...)
			invMu.Unlock()

			fmt.Printf("🎁 Novo booster adquirido! Veja em seu inventário\n")
			time.Sleep(2 * time.Second)
		case enqueued:
			fmt.Printf("⏳ Entrou na fila. Aguardando oponente...")
			time.Sleep(2 * time.Second)
		case gamestart:
			var temp struct {
				Opponent string `json:"opponent"`
				Turn     string `json:"turn"`
				Hand     []Card `json:"hand"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			turn = temp.Turn
			hand = temp.Hand
			if turn == sessionID {
				fmt.Printf("⚔️  Pareado com: %s. Você começa.\n", temp.Opponent)
				time.Sleep(2 * time.Second)
			} else {
				fmt.Printf("⚔️  Pareado com: %s. Seu oponente começa.\n", temp.Opponent)
				time.Sleep(2 * time.Second)
			}
			go battleOn() // roda batalha
		case cardused:
			println("Carta usada")
			/*var temp struct {
				CID             string `json:"CID"`
				YourSanity      int    `json:"yoursanity"`
				OpponentsSanity int    `json:"opponentssanity"`
			}*/
		case notify:
			var temp struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("📢 %s\n", temp.Message)
		case newturn:
			var temp struct {
				Turn string `json:"turn"` // vez de quem (1 sua, 2 oponente)
			}
			_ = json.Unmarshal(msg.Data, &temp)
			turn = temp.Turn
		case updateinfo:
			var temp struct {
				Sanity      map[string]int        `json:"sanity"`
				DreamStates map[string]DreamState `json:"dreamStates"`
				Round       int                   `json:"round"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			sanity = temp.Sanity[sessionID]
			dreamst = temp.DreamStates[sessionID]
			round = temp.Round
		case newloss:
			IsInBattle = false
			gameResult = "loss"
		case newvictory:
			IsInBattle = false
			gameResult = "victory"
		case newtie:
			IsInBattle = false
			gameResult = "tie"
		case "erro":
			var temp struct {
				Error string `json:"error"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("%s", temp.Error)
		}
	}
}

// função de limpar a tela
func clearScreen() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// função que mostra inventário
func printInventory() {
	invMu.RLock()
	defer invMu.RUnlock()

	if len(inventory) == 0 {
		fmt.Println("Inventário vazio.")
		time.Sleep(1 * time.Second)
		return
	}
	fmt.Println("\n📦 Inventário:")
	for _, c := range inventory {
		fmt.Printf("%s) %s\n", c.CID, strings.Title(c.Name))
		fmt.Printf("   Tipo:      %s\n", strings.Title(string(c.CardType)))
		if c.Points == 0 {
			fmt.Printf("   Pontos:    %d\n", c.Points)
		} else {
			if c.CardType == Pill {
				fmt.Printf("   Pontos:    +%d\n", c.Points)
			} else {
				fmt.Printf("   Pontos:    -%d\n", c.Points)
			}
		}
		fmt.Printf("   Raridade:  %s\n", strings.Title(string(c.CardRarity)))
		fmt.Printf("   Efeito:    %s\n", strings.Title(string(c.CardEffect)))
		fmt.Printf("   Descrição: %s\n", strings.Title(c.Desc))
		fmt.Println(strings.Repeat("-", 40))
	}
}

// função para ping
func testLatency() {
	serverAddr, err := net.ResolveUDPAddr("udp", ":8081")
	if err != nil {
		fmt.Printf("❌ Erro ao resolver endereço: %v\n", err)
		return
	}

	connection, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Printf("❌ Erro ao conectar: %v\n", err)
		return
	}
	defer connection.Close()

	// timeout de 999 ms
	connection.SetReadDeadline(time.Now().Add(999 * time.Millisecond))

	start := time.Now()
	_, err = connection.Write([]byte("ping"))
	if err != nil {
		fmt.Printf("❌ Erro ao enviar ping: %v\n", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := connection.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("⏰ Timeout: %v\n", err)
		return
	}

	if string(buffer[:n]) == "pong" {
		elapsed := time.Since(start).Milliseconds()
		fmt.Printf("🏓 Latência: %d ms\n", elapsed)
	} else {
		fmt.Printf("❌ Resposta inválida: %s\n", string(buffer[:n]))
	}
}

// loop principal de batalha
func battleOn() {
	reader := bufio.NewReader(os.Stdin)

	for {
		// verifica se está ou não em batalha ainda
		if !IsInBattle {
			// Jogo acabou, mostra resultado final
			clearScreen()
			switch gameResult {
			case "victory":
				fmt.Println("Seu oponente acorda. Você VENCEU a batalha.")
			case "loss":
				fmt.Println("Você acorda. É uma DERROTA... Ou, talvez, um livramento.")
			case "tie":
				fmt.Println("Ambos continuam a dormir. É um EMPATE.")
			default:
				fmt.Println("🏁 Jogo finalizado!")
			}
			fmt.Println("\nPressione ENTER para voltar ao menu...")
			reader.ReadString('\n')
			return
		}

		// rodada caso esteja em batalha
		clearScreen()
		fmt.Printf("👁‍🗨 Alucinação...\n")
		if turn == sessionID {
			fmt.Printf("Vez de %s\n", name)
			fmt.Printf("\n🃏 Sua mão:")
			for i, card := range hand {
				fmt.Printf("%d) %s [%s %d]\n", i+1, card.Name, strings.ToUpper(string(card.CardType)), card.Points)
				fmt.Printf("%s\n", card.Desc)
			}

			s := prompt(reader, "Escolha uma carta (número): ")
			index, err := strconv.Atoi(s)
			index -= 1

			if err != nil {
				fmt.Println("Erro na conversão Ascii to Int")
				time.Sleep(1 * time.Second)
				continue
			}

			if index < 0 || index >= len(hand) {
				fmt.Println("Opção inválida.")
				time.Sleep(1 * time.Second)
				continue
			}

			chosenCard := hand[index]

			// remove a carta da mão
			handMu.Lock() // lida com concorrência
			if index >= 0 && index < len(hand) {
				hand = append(hand[:index], hand[index+1:]...)
			}

			handMu.Unlock()

			// envia a request para jogar a carta
			data, _ := json.Marshal(map[string]any{"card": chosenCard})
			_ = enc.Encode(Message{UID: sessionID, Request: usecard, Data: data})
		} else {
			fmt.Println("⏳ Aguardando jogada do oponente...")
		}

		time.Sleep(500 * time.Millisecond)
	}

}
