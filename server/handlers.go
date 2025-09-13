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
		case register:
			handleRegister(request, encoder)
		case login:
			handleLogin(request, encoder, connection)
		case buypack:
			handleBuyPack(msg, encoder)
		case battle:
			handleBattle(msg, encoder)
		case usecard:
			handleUseCard(msg, encoder)
		case giveup:
			handleGiveUp(msg, encoder)
		case ping:
			handlePing(encoder)
		default:
			return
		}

	}
}

// lida com o registro
func handleRegister(request Message, encoder *json.Encoder) {
	// crio var temp para guardar as infos
	var temp struct {
		Username,
		Password string
	}

	if err := json.Unmarshal(request.Data, &temp); err != nil {
		sendError(encoder, err)
		return
	}

}

func handleLogin(request Message, encoder *json.Encoder, connection net.Conn) {
	return
}

func handleBuyPack(request Message, encoder *json.Encoder) {
	return
}

func handleBattle(request Message, encoder *json.Encoder) {
	return
}

func handleUseCard(request Message, encoder *json.Encoder) {
	return
}

func handleGiveUp(request Message, encoder *json.Encoder) {
	return
}

func handlePing(encoder *json.Encoder) {
	return
}

func sendError(*json.Encoder, error) {
	return
}
