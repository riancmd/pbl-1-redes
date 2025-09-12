package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
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

	turn sync.RWMutex
	turn int

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

	go readRequests(dec) // thread para ler as msgs do servidor

	reader := bufio.NewReader(os.Stdin) // leitura teclado

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
			if !sessionActive {
				return
			}
			prompt(reader, "Você já está conectado...")
		}
	}

}

func prompt(reader *bufio.Reader, label string) string { // printa e pega input
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// envia as solicitações pro servidor
func send(request string, payload any) {
	data, _ := json.Marshal(payload)
	_ = enc.Encode(Message{Request: request, UID: sessionID, Data: data})
}
