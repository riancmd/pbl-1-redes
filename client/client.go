package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
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
	sessionID int
	inventory []Card

	hand []Card

	turnMu sync.RWMutex
	turn   int

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
				
				send(register,map[string]string{"username": username, "password": pass})
			}
			prompt(reader, "Você já está conectado...")
			os.sleep(2)
		case "2":
			login := prompt(reader, "Login: ")
			pass := prompt(reader, "Senha: ")
			
			send("login", map[string]string{"username": username, "password": pass})
		case "3":
			if !sessionActive{
				fmt.Println("Precisa estar logado.")
				os.sleep(2)
				continue
			}
			send(buyNewPack, map[string]int{"id": sessionID})
		case "4":
			if !sessionActive{
				fmt.Println("Precisa estar logado.")
				os.sleep(2)
				continue
			}
			printInventory();
		case "5":
			if !sessionActive{
				fmt.Println("Precisa estar logado.")
				os.sleep(2)
				continue
			}
			send("battle", map[string]int{"id": sessionID})
		case "6":
			return
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
		case loggedin:
			var temp struct {
				UID      int
				Username string `json:"username"`
			}
			_ = json.Unmarshal(msg.Data, &temp)
			fmt.Printf("🔓 Login ok! Bem-vindo, %s.\n", temp.UID, temp.Username)
			sessionActive = true
		case packbought:
			var booster []Card
			_ = json.Unmarshal(msg.Data, &booster)
			inventory = append(inventory, cards...)
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
			if turn == sessionID{
				fmt.Printf("⚔️  Pareado com: %s. Você começa.\n", temp.opponent.)
			}
			else{
				fmt.Printf("⚔️  Pareado com: %s. Seu oponente começa.\n", temp.opponent, temp.)
			}
			go gameLoop()
		case cardused:
			var temp struct {
				CID string `json:"CID"`
				YourSanity int `json:"yoursanity"`
				OpponentsSanity int `json:"opponentssanity"`
			}

		case newturn:
			return
		case newloss:
			return
		case newvictory:
			return
		case newtie:
			return
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
func printInventory(){
	return
}