package main

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
		Username string `json:"username"`
		Password string `json:"password"`
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

	// nova request contendo o novo UID para os boosters
	for i := 0; i < 4; i++ {
		boosterRequest := Message{
			Request: buypack,
			UID:     player.UID,
		}
		boosterData, _ := json.Marshal(map[string]string{"UID": player.UID})
		boosterRequest.Data = boosterData

		handleBuyBooster(boosterRequest, encoder)
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
	p, exists := pm.byUsername[r.Login]

	if !exists {
		sendError(encoder, errors.New("usuário não encontrado"))
		return
	}

	if _, ok := pm.activeByUID[p.UID]; ok {
		sendError(encoder, errors.New("usuário já logado"))
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

	if error != nil {
		sendError(encoder, error)
		return
	}

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

	// envia resposta
	data, _ := json.Marshal(cards)
	_ = encoder.Encode(Message{Request: packbought, Data: data})
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

func sendError(encoder *json.Encoder, erro error) {
	type payload struct {
		Error string `json:"error"`
	}

	pld := payload{
		Error: erro.Error(),
	}

	// uma mensagem contendo erro
	msg := Message{Request: "erro"}

	data, _ := json.Marshal(pld)
	msg.Data = data
	_ = encoder.Encode(msg)
}
