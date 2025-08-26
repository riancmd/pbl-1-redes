package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	// crio o listener
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatal(err)
	}

	defer listener.Close()

	for {
		// cria a conexao (connection)
		connection, err := listener.Accept()

		if err != nil {
			fmt.Println(err)
			return
		}

		// cria as threads (goroutines) p cada solicitação
		go connect(connection)
	}
}

func connect(connection net.Conn) {
	fmt.Printf("Connectin to %s\n", connection.RemoteAddr().String())

	//cria buffer
	buffer := make([]byte, 4096)
	packet := make([]byte, 4096)
	defer connection.Close()

	// vai guardando no buffer o que foi recebido
	for {
		_, err := connection.Read(buffer)

		if err != nil {
			if err != io.EOF {
				fmt.Println("Error while reading: ", err)
			}
			println("EOF")
			break
		}
		packet = append(packet, buffer...) //coloca os dados do buffer
	}
	nBytes, _ := connection.Write(packet)
	fmt.Printf("Wrote %d bytes and echoed '%s'\n", nBytes, string(packet))
}
