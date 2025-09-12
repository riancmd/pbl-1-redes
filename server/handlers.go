package server

import (
	"encoding/json"
	"fmt"
	"net"
)

func connectionHandler(connection net.Conn) {
	defer connection.Close() // vai rodar assim que a função terminar

	// encoders e decoders para as mensagens
	decoder := json.NewDecoder(connection)
	encoder := json.NewEncoder(connection)

	// decodifica msg e roda
	for {
		var request Message // cria a variavel p request

		if error := decoder.Decode(&request); error != nil {
			fmt.Printf("Erro ao decodificar mensagem: %v\n", error)
			return
		}

		switch request.Request {
		case "something":
			return
		}

	}
}
