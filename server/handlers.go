package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

func connectionHandler(connection net.Conn) {
	// guarda usuário da conexão
	var currentUser *User

	defer func() {
		if currentUser != nil {
			//fmt.Printf("DEBUG: Chamando logout para %s\n", currentUser.Username)
			pm.Logout(currentUser)
		}
		connection.Close()
		//fmt.Printf("DEBUG: Conexão fechada\n")
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
			if user := handleRegister(request, encoder, connection); user != nil {
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
		case usecard:
			handleUseCardAction(request, encoder)
		case giveup:
			handleGiveUpAction(request, encoder)
		default:
			return
		}

	}
}

// lida com o registro
func handleRegister(request Message, encoder *json.Encoder, connection net.Conn) *User {
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
	player, error := pm.CreatePlayer(temp.Username, temp.Password, connection)

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
	//fmt.Printf("DEBUG: Verificando login para UID %s\n", p.UID)
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
	/*
		if p != nil {
			fmt.Printf("DEBUG: Ao buscar por %s no PlayerManager, ele existe\n", p.Username)
			if p.Connection != nil {
				fmt.Printf("DEBUG: Ao buscar pela conexão de %s no PlayerManager, ela existez\n", p.Username)
			} else {
				fmt.Printf("DEBUG: Ao buscar pela conexão de %s no PlayerManager, ela não existe\n", p.Username)
			}
		}*/ // info dump

	if error := mm.Enqueue(p); error != nil {
		sendError(encoder, error)
		return
	}
	_ = encoder.Encode(Message{Request: enqueued, Data: nil})
}

// função notifica erro
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

// lida com ação de usar carta, enviando pro inbox
func handleUseCardAction(request Message, encoder *json.Encoder) {
	//fmt.Printf("DEBUG: Recebida ação usecard do jogador %s\n", request.UID)

	// verifica se o jogador existe e está ativo
	player, err := pm.GetByUID(request.UID)
	if err != nil {
		//fmt.Printf("DEBUG: Jogador %s não encontrado\n", request.UID)
		sendError(encoder, err)
		return
	}

	if !player.IsInBattle {
		//fmt.Printf("DEBUG: Jogador %s não está em batalha\n", request.UID)
		sendError(encoder, errors.New("jogador não está em partida"))
		return
	}

	// encontra a partida do jogador
	match := mm.FindMatchByPlayerUID(request.UID)
	if match == nil {
		//fmt.Printf("DEBUG: Partida não encontrada para jogador %s\n", request.UID)
		sendError(encoder, errors.New("jogador não está em partida"))
		return
	}

	if match.State != Running {
		//fmt.Printf("DEBUG: Partida não está rodando para jogador %s\n", request.UID)
		sendError(encoder, errors.New("partida não está ativa"))
		return
	}

	// cria mensagem para o canal da partida
	msg := matchMsg{
		PlayerUID: request.UID,
		Action:    "usecard",
		Data:      request.Data,
	}

	//fmt.Printf("DEBUG: Tentando enviar mensagem usecard para inbox da partida %d\n", match.ID)

	// envia para o canal da partida com timeout
	select {
	case match.inbox <- msg:
		//fmt.Printf("DEBUG: Mensagem usecard enviada com sucesso\n")
	case <-time.After(1 * time.Second):
		//fmt.Printf("DEBUG: Timeout ao enviar mensagem para partida\n")
		sendError(encoder, errors.New("timeout ao processar ação"))
	}
}

// lida com ação de desistir, enviando pro inbox
func handleGiveUpAction(request Message, encoder *json.Encoder) {
	//fmt.Printf("DEBUG: Recebida ação giveup do jogador %s\n", request.UID)

	// verifica se o jogador existe e está ativo
	player, err := pm.GetByUID(request.UID)
	if err != nil {
		//fmt.Printf("DEBUG: Jogador %s não encontrado\n", request.UID)
		sendError(encoder, err)
		return
	}

	if !player.IsInBattle {
		//fmt.Printf("DEBUG: Jogador %s não está em batalha\n", request.UID)
		sendError(encoder, errors.New("jogador não está em partida"))
		return
	}

	// encontra a partida do jogador
	match := mm.FindMatchByPlayerUID(request.UID)
	if match == nil {
		//fmt.Printf("DEBUG: Partida não encontrada para jogador %s\n", request.UID)
		sendError(encoder, errors.New("jogador não está em partida"))
		return
	}

	// cria mensagem para o canal da partida
	msg := matchMsg{
		PlayerUID: request.UID,
		Action:    "giveup",
		Data:      request.Data,
	}

	//fmt.Printf("DEBUG: Tentando enviar mensagem giveup para inbox da partida %d\n", match.ID)

	// envia para o canal da partida com timeout
	select {
	case match.inbox <- msg:
		//fmt.Printf("DEBUG: Mensagem giveup enviada com sucesso\n")
	case <-time.After(1 * time.Second):
		//fmt.Printf("DEBUG: Timeout ao enviar mensagem para partida\n")
		sendError(encoder, errors.New("timeout ao processar ação"))
	}
}

func logServerStats() {
	// cria um ticker para logar as estatísticas a cada 10 segundos
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// pega as informações do PlayerManager
		pm.mu.Lock()
		registeredPlayers := len(pm.byUID)
		onlinePlayers := len(pm.activeByUID)
		pm.mu.Unlock()

		// peega as informações do MatchManager
		mm.mu.Lock()
		activeMatches := len(mm.matches)
		mm.mu.Unlock()

		// pega as informações do CardVault
		boosterStock := len(vault.Vault)

		fmt.Println("--- Estatísticas do Servidor ---")
		fmt.Printf("Jogadores inscritos: %d\n", registeredPlayers)
		fmt.Printf("Jogadores online: %d\n", onlinePlayers)
		fmt.Printf("Estoque de boosters: %d\n", boosterStock)
		fmt.Printf("Partidas ativas: %d\n", activeMatches)
		fmt.Println("--------------------------------")
	}
}
