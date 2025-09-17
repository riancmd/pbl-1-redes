package main

import (
	"fmt"
	"net"
	"os"
	"sync"
)

var (
	vault      *CardVault
	pm         *PlayerManager
	mm         *MatchManager
	registerMu sync.RWMutex
	loginMu    sync.RWMutex
)

func main() {
	// cria vault e mm
	vault = NewCardVault()
	mm = NewMatchManager()

	error := vault.LoadCardsFromFile("data/cardVault.json")

	// verifica se realmente criou o estoque
	if error != nil {
		fmt.Println("Erro ao criar estoque") // debug
		panic(error)
	}

	// cria os boosters, adicionando-o
	error = vault.createBoosters(1000)

	// verifica se realmente criou os boosters
	if error != nil {
		fmt.Println("Erro ao criar boosters") // debug
		panic(error)
	}

	// cria o gerenciador de usuários
	pm = NewPlayerManager()

	// começa goroutine para pareamento
	go mm.matchmakingLoop()

	// info logs
	go logServerStats() // printa a cada 2 seg

	address := ":8080"          //porta usada
	envVar := os.Getenv("PORT") // usa env para pode trocar a porta qndo preciso

	if envVar != "" { // coloca porta definida como porta
		address = envVar
	}

	listener, error := net.Listen("tcp", address)

	// verifica erro na conexão
	if error != nil {
		fmt.Println("Erro ao criar listener") // debug
		panic(error)                          // para a execução e sinaliza erro
	}

	fmt.Println("Servidor do Alucinari ouvindo na porta", address)

	// cria listener UDP para pings na porta 8081
	go handlerPing()

	// cria loop para as conexões novas
	for {
		connection, error := listener.Accept() // aceita nova conexão

		if error != nil {
			continue
		}

		go connectionHandler(connection)
	}
}

func handlerPing() {
	address, _ := net.ResolveUDPAddr("udp", ":8081")
	connection, _ := net.ListenUDP("udp", address)
	defer connection.Close()

	buffer := make([]byte, 1024)
	for {
		n, remote, _ := connection.ReadFromUDP(buffer)
		msg := string(buffer[:n])
		if msg == "ping" {
			connection.WriteToUDP([]byte("pong"), remote)
		}
	}
}
