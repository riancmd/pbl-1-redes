package client

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
	UID     int             `json:"uid"` // user id
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
	newturn    string = "newTurn"
	newloss    string = "newLoss"
	newvictory string = "newVictory"
	newtie     string = "newTie"
	pong       string = "pong"
)

type GameMessage struct {
	PlayerID int             `json:"PlayerID"`
	Action   string          `json:"action"`
	Data     json.RawMessage `json:"data"`
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
	sessionID     int
	name          string
	sessionActive bool
	inventory     []Card

	handMu sync.RWMutex
	hand   []Card

	turn       int
	IsInBattle bool
	gameResult string

	enc *json.Encoder
)

func main() {
	sessionActive := false

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

	for {
		clearScreen()
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
			if !sessionActive {
				username := prompt(reader, "Nome de usuário: ")
				pass := prompt(reader, "Senha: ")

				send(register, map[string]string{"username": username, "password": pass})
			}
			prompt(reader, "Você já está conectado...")
			time.Sleep(2)
		case "2":
			username := prompt(reader, "Login: ")
			pass := prompt(reader, "Senha: ")

			send("login", map[string]string{"username": username, "password": pass})
		case "3":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2)
				continue
			}
			send(buypack, map[string]int{"id": sessionID})
		case "4":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2)
				continue
			}
			printInventory()
		case "5":
			if !sessionActive {
				fmt.Println("Precisa estar logado.")
				time.Sleep(2)
				continue
			}
			send("battle", map[string]int{"id": sessionID})
		case "6":
			testLatency()
		case "0":
			fmt.Println("Bons sonhos... 🌙🌃💤")
			return
		default:
			fmt.Println("Opção inválida")
			time.Sleep(2)
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
				UID      int
				Username string `json:"username"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("✅ Criado jogador #%d (%s)\n", temp.UID, temp.Username)
			fmt.Printf("Você ganhou 4 boosters gratuitos! Eles já estão em seu inventário")
			sessionActive = true
			name = temp.Username
		case loggedin:
			var temp struct {
				UID      int
				Username string `json:"username"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("🔓 Login ok! Bem-vindo, %s.\n", temp.UID, temp.Username)
			sessionActive = true
			name = temp.Username
		case packbought:
			var booster []Card
			_ = json.Unmarshal(msg.Data, &booster)
			inventory = append(inventory, booster...)
			fmt.Printf("🎁 Novo booster adquirido! Veja em seu inventário\n")
		case enqueued:
			fmt.Printf("⏳ Entrou na fila. Aguardando oponente...")
		case gamestart:
			var temp struct {
				Opponent string `json:"opponent"`
				Turn     int    `json:"turn"`
				Hand     []Card `json:"hand"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			turn = temp.Turn
			hand = temp.Hand
			if turn == sessionID {
				fmt.Printf("⚔️  Pareado com: %s. Você começa.\n", temp.Opponent)
			} else {
				fmt.Printf("⚔️  Pareado com: %s. Seu oponente começa.\n", temp.Opponent)
			}
			go battleOn() // roda batalha
		case cardused:
			var temp struct {
				CID             string `json:"CID"`
				YourSanity      int    `json:"yoursanity"`
				OpponentsSanity int    `json:"opponentssanity"`
			}
		case newturn:
			var temp struct {
				Turn int `json:"turn"` // vez de quem (1 sua, 2 oponente)
			}
			_ = json.Unmarshal(msg.Data, &temp)
			turn = temp.Turn
		case newloss:
			IsInBattle = false
			gameResult = "loss"
		case newvictory:
			IsInBattle = false
			gameResult = "victory"
		case newtie:
			IsInBattle = false
			gameResult = "tie"
		case pong:
			return
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
	return
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
		if turn == 1 {
			fmt.Println("Vez de %s", name)
			fmt.Println("\n🃏 Sua mão:")
			for i, card := range hand {
				fmt.Println("%d) %s [%s %d]\n", i+1, card.Name, strings.ToUpper(string(card.CardType)), card.Points)
				fmt.Println("%s", card.Desc)
			}

			s := prompt(reader, "Escolha uma carta (número): ")
			index, err := strconv.Atoi(s)
			index -= 1

			if err != nil {
				fmt.Println("Erro na conversão Ascii to Int")
				time.Sleep(1)
				continue
			}

			if index < 0 || index >= len(hand) {
				fmt.Println("Opção inválida.")
				time.Sleep(1)
				continue
			}

			chosenCard := hand[index]

			// remove a carta da mão
			handMu.Lock() // lida com concorrência
			if index >= 0 && index < lend(hand) {
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
