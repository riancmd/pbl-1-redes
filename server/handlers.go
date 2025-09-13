package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
)

func connectionHandler(connection net.Conn) {
	defer connection.Close() // vai rodar assim que a função terminar
	// USAR LOGOUT

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
			handleBuyBooster(request, encoder)
		case battle:
			handleEnqueue(request, encoder)
		case usecard:
			handleUseCard(request, encoder)
		case giveup:
			handleGiveUp(request, encoder)
		case ping:
			handlePing(encoder)
		default:
			return
		}

	}
}

// lida com o registro
func handleRegister(request Message, encoder *json.Encoder) {
	registerMu.Lock()
	defer registerMu.Unlock()
	// crio var temp para guardar as infos
	var temp struct {
		Username,
		Password string
	}

	// desserializo dados em temp
	if err := json.Unmarshal(request.Data, &temp); err != nil {
		sendError(encoder, err)
		return
	}

	// crio o usuário
	player, error := pm.CreatePlayer(temp.Username, temp.Password)

	if error != nil {
		sendError(encoder, error)
		return
	}

	// serializo mensagem e envio pro client
	pr := PlayerResponse{UID: player.UID, Username: player.Username}
	data, _ := json.Marshal(pr)
	_ = encoder.Encode(Message{Request: registered, Data: data})

	// Concede 2 boosters de 5 cartas ao logar
	for i := 0; i < 2; i++ {
		handleBuyBooster(request, encoder)
	}
}

// lida com o login
func handleLogin(request Message, encoder *json.Encoder, connection net.Conn) {
	var r struct{ Login, Password string }
	if err := json.Unmarshal(request.Data, &r); err != nil {
		sendError(encoder, err)
		return
	}

	// evita múltiplos logins do mesmo usuário
	loginMu.Lock()
	if _, ok := pm.activeByUID[r.Login]; ok {
		loginMu.Unlock()
		sendError(encoder, errors.New("user already logged"))
		return
	}
	loginMu.Unlock()

	p, err := pm.Login(r.Login, r.Password, connection)
	if err != nil {
		sendError(encoder, err)
		return
	}

	resp := PlayerResponse{UID: p.UID, Username: p.Username}
	b, _ := json.Marshal(resp)
	_ = encoder.Encode(Message{Request: loggedin, Data: b})

}

// lida com compra de boosters
func handleBuyBooster(request Message, encoder *json.Encoder) {
	var temp struct {
		UID string `json:"UID"`
	}

	if error := json.Unmarshal(request.Data, &temp); error != nil {
		sendError(encoder, error)
		return
	}

	p, error := pm.GetByUID(temp.UID)

	var booster Booster

	booster, error = vault.TakeBooster()

	if error != nil {
		sendError(encoder, error)
		return
	}

	cards := booster.Booster

	// passa a tratar dos ponteiros das cartas
	cardPointers := make([]*Card, len(cards))
	for i := range cards {
		cardPointers[i] = &cards[i]
	}

	pm.AddToDeck(p.UID, cardPointers)
}

// lida com pareamento
func handleEnqueue(request Message, encoder *json.Encoder) {
	var temp struct {
		UID string `json:"UID"`
	}

	if error := json.Unmarshal(request.Data, &temp); error != nil {
		sendError(encoder, error)
		return
	}
	p, error := pm.GetByUID(temp.UID)
	if error != nil {
		sendError(encoder, error)
		return
	}
	if error := mm.Enqueue(p); error != nil {
		sendError(encoder, error)
		return
	}
	b, _ := json.Marshal(map[string]string{"Player enqueued": p.Username})
	_ = encoder.Encode(Message{Request: "enqueue_response", Data: b})
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
