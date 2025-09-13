package server

import (
	"fmt"
	"net"
	"os"
)

func main() {
	address := ":8080"          //porta usada
	envVar := os.Getenv("PORT") // usa env para pode trocar a porta qndo preciso

	if envVar != "" { // coloca porta definida como porta
		address = envVar
	}

	listener, error := net.Listen("tcp", address)

	// verifica erro na conexão
	if error != nil {
		panic(error) // para a execução e sinaliza erro
	}

	fmt.Println("Servidor do Alucinari ouvindo na porta", address)

	// cria loop para as conexões novas
	for {
		connection, error := listener.Accept() // aceita nova conexão

		if error != nil {
			continue
		}

		go connectionHandler(connection)
	}
}

func handlePing() {
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
