package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
)

func connectionHandler(connection net.Conn) {
	// guarda usuário da conexão
	var currentUser *User

	defer func() {
		if currentUser != nil {
			fmt.Printf("DEBUG: Chamando logout para %s\n", currentUser.Username)
			pm.Logout(currentUser)
		}
		connection.Close()
		fmt.Printf("DEBUG: Conexão fechada\n")
	}()

	// encoders e decoders para as mensagens
	decoder := json.NewDecoder(connection)
	encoder := json.NewEncoder(connection)

	// decodifica msg e roda
	for {
		var request Message // cria a variavel p request

		if error := decoder.Decode(&request); error != nil {
			// cliente desconectou - limpo os dados
			if currentUser != nil {
				pm.Logout(currentUser)
				fmt.Printf("Usuário %s deslogado automaticamente\n", currentUser.Username)
			}
			return
		}

		switch request.Request {
		case register:
			if user := handleRegister(request, encoder); user != nil {
				currentUser = user
			}
		case login:
			// aqui, guardo também o usuário da conexão
			if user := handleLogin(request, encoder, connection); user != nil {
				currentUser = user
			}
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
func handleRegister(request Message, encoder *json.Encoder) *User {
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
		return nil
	}

	// crio o usuário
	player, error := pm.CreatePlayer(temp.Username, temp.Password)

	if error != nil {
		sendError(encoder, error)
		return nil
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

	return player
}

// lida com o login
func handleLogin(request Message, encoder *json.Encoder, connection net.Conn) *User {
	var r struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(request.Data, &r); err != nil {
		sendError(encoder, err)
		return nil
	}

	// trata concorrência
	loginMu.Lock()
	defer loginMu.Unlock()

	// evita múltiplos logins do mesmo usuário
	p, exists := pm.byUsername[r.Username]

	if !exists {
		sendError(encoder, errors.New("usuário não encontrado"))
		return nil
	}

	// DEBUG: Verifique o estado do activeByUID
	fmt.Printf("DEBUG: Verificando login para UID %s\n", p.UID)
	pm.mu.Lock()
	if activeUser, ok := pm.activeByUID[p.UID]; ok {
		pm.mu.Unlock()
		fmt.Printf("DEBUG: Usuário %s (UID: %s) já está ativo\n", activeUser.Username, p.UID)
		sendError(encoder, errors.New("usuário já logado"))
		return nil
	}
	pm.mu.Unlock() // termina debug

	if _, ok := pm.activeByUID[p.UID]; ok {
		sendError(encoder, errors.New("usuário já logado"))
		return nil
	}

	p, err := pm.Login(r.Username, r.Password, connection)
	if err != nil {
		sendError(encoder, err)
		return nil
	}

	resp := PlayerResponse{UID: p.UID, Username: p.Username}
	b, _ := json.Marshal(resp)
	_ = encoder.Encode(Message{Request: loggedin, Data: b})

	return p

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
	_ = encoder.Encode(Message{Request: enqueued, Data: nil})
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
