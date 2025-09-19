package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// newMatchanager
func NewMatchManager() *MatchManager {
	return &MatchManager{
		mu:       sync.Mutex{},
		queue:    []*User{},
		nextID:   1,
		matches:  make(map[int]*Match),
		byPlayer: make(map[string]*Match),
	}
}

// coloca usuário na fila
func (mm *MatchManager) Enqueue(p *User) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// verifica se já está em jogo
	if p.IsInBattle {
		return errors.New("player já está em jogo")
	}

	// evita-se duplicata na fila
	for _, q := range mm.queue {
		if q.UID == p.UID {
			return errors.New("player já está na fila")
		}
	}

	mm.queue = append(mm.queue, p)
	return nil
}

// tira usuário da fila
func (mm *MatchManager) dequeue() (*User, error) {
	if len(mm.queue) == 0 {
		return nil, errors.New("fila vazia")
	}
	p := mm.queue[0]
	mm.queue = mm.queue[1:]
	return p, nil
}

// busca partida por UID
func (mm *MatchManager) FindMatchByPlayerUID(uid string) *Match {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.byPlayer[uid]
}

// loop de pareamento
func (mm *MatchManager) matchmakingLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		mm.mu.Lock()
		if len(mm.queue) >= 2 {
			p1, _ := mm.dequeue()
			p2, _ := mm.dequeue()

			// valido as conexões
			if p1.Connection == nil || p2.Connection == nil {
				// se a conexão for inválida, coloco de novo na fila
				if p1.Connection != nil {
					mm.queue = append([]*User{p1}, mm.queue...)
				}
				if p2.Connection != nil {
					mm.queue = append([]*User{p2}, mm.queue...)
				}
				mm.mu.Unlock()
				continue
			}

			mm.nextID++
			match := &Match{
				ID:    mm.nextID,
				P1:    p1,
				P2:    p2,
				State: Running,
				Turn:  p1.UID, // p1 começa

				Hand:             map[string][]*Card{},
				Sanity:           map[string]int{p1.UID: 40, p2.UID: 40},
				DreamStates:      map[string]DreamState{p1.UID: sleepy, p2.UID: sleepy},
				RoundsInState:    map[string]int{p1.UID: 0, p2.UID: 0},
				StateLockedUntil: map[string]int{p1.UID: 0, p2.UID: 0},
				currentRound:     1,
				inbox:            make(chan matchMsg, 16),
			}
			p1.IsInBattle, p2.IsInBattle = true, true
			mm.matches[match.ID] = match
			mm.byPlayer[p1.UID] = match
			mm.byPlayer[p2.UID] = match

			go match.run()
		}
		mm.mu.Unlock()
	}
}

// gerencia a batalha
func (m *Match) run() {
	defer func() {
		// cleanup quando a partida termina
		mm.mu.Lock()
		delete(mm.matches, m.ID)
		delete(mm.byPlayer, m.P1.UID)
		delete(mm.byPlayer, m.P2.UID)
		mm.mu.Unlock()

		m.P1.IsInBattle = false
		m.P2.IsInBattle = false
		close(m.inbox)
	}()

	// cria codificadores para cada usuário
	enc1 := json.NewEncoder(m.P1.Connection)
	enc2 := json.NewEncoder(m.P2.Connection)

	// são escolhidas 10 cartas aleatórias do inventário de cada jogador
	m.Hand[m.P1.UID] = drawCards(m.P1.Deck)
	m.Hand[m.P2.UID] = drawCards(m.P2.Deck)

	m.sendGameStart(enc1, enc2)

	// pequena pausa para garantir que os clientes processaram o game start
	time.Sleep(1 * time.Second)

	// loop do jogo
	for m.State == Running {
		fmt.Printf("=== TURNO %d - Jogador %s ===\n", m.currentRound, m.Turn)

		// verifica condições de fim ANTES do turno
		if m.checkGameEnd() {
			//fmt.Printf("DEBUG: Jogo deve terminar - sanidades: P1=%d P2=%d\n", m.Sanity[m.P1.UID], m.Sanity[m.P2.UID])
			break
		}

		// envia informação de turno para AMBOS os jogadores
		m.notifyTurnStart(enc1, enc2, m.Turn)

		// pequena pausa para sincronização
		time.Sleep(500 * time.Millisecond)

		// processa o turno
		actionTaken := m.processTurn(enc1, enc2)

		if !actionTaken {
			fmt.Printf("DEBUG: Nenhuma ação foi tomada no turno\n")
		}

		// atualiza estados e sanidade
		m.updateGameState(enc1, enc2)

		// verifica condições de fim APÓS as atualizações
		if m.checkGameEnd() {
			fmt.Printf("DEBUG: Jogo terminando após atualizações\n")
			break
		}

		// troca de turno
		m.switchTurn()
		m.currentRound++

		// pequena pausa entre turnos
		time.Sleep(1 * time.Second)
	}

	m.endGame(enc1, enc2)
}

// pega 10 cartas
func drawCards(deck []*Card) []*Card {
	if len(deck) == 0 {
		return []*Card{}
	}

	hand := make([]*Card, len(deck))
	copy(hand, deck)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	random.Shuffle(len(hand), func(i, j int) { hand[i], hand[j] = hand[j], hand[i] })
	if len(hand) > 10 {
		hand = hand[:10]
	}
	return hand
}

// manda response de início do game
func (m *Match) sendGameStart(enc1, enc2 *json.Encoder) {
	type startPayload struct {
		Info        string                `json:"info"`
		Turn        string                `json:"turn"`
		Hand        []*Card               `json:"hand"`
		Sanity      map[string]int        `json:"sanity"`
		DreamStates map[string]DreamState `json:"dreamStates"`
	}

	// Payload para P1
	p1Payload := startPayload{
		Info:        m.P2.Username,
		Turn:        m.Turn,
		Hand:        m.Hand[m.P1.UID],
		Sanity:      m.Sanity,
		DreamStates: m.DreamStates,
	}

	// Payload para P2
	p2Payload := startPayload{
		Info:        m.P1.Username,
		Turn:        m.Turn,
		Hand:        m.Hand[m.P2.UID],
		Sanity:      m.Sanity,
		DreamStates: m.DreamStates,
	}

	msg1 := Message{Request: gamestart}
	msg2 := Message{Request: gamestart}

	data1, _ := json.Marshal(p1Payload)
	data2, _ := json.Marshal(p2Payload)
	msg1.Data = data1
	msg2.Data = data2

	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)

	//fmt.Printf("DEBUG: Game start enviado para ambos jogadores\n")
}

func (m *Match) checkGameEnd() bool {
	// verifica se alguém chegou a 0 de sanidade
	for _, sanity := range m.Sanity {
		if sanity <= 0 {
			return true
		}
	}

	// Verificar se acabaram as cartas para ambos
	if len(m.Hand[m.P1.UID]) == 0 && len(m.Hand[m.P2.UID]) == 0 {
		return true
	}

	return false
}

func (m *Match) endGame(enc1, enc2 *json.Encoder) {
	//fmt.Printf("DEBUG: Finalizando jogo - P1: %d sanidade, P2: %d sanidade\n",
	//	m.Sanity[m.P1.UID], m.Sanity[m.P2.UID])

	m.State = Finished

	// determina o resultado
	var response1, response2 string
	p1Sanity := m.Sanity[m.P1.UID]
	p2Sanity := m.Sanity[m.P2.UID]

	if p1Sanity <= 0 && p2Sanity <= 0 {
		response1 = newtie
		response2 = newtie
	} else if p1Sanity <= 0 {
		response1 = newloss
		response2 = newvictory
	} else if p2Sanity <= 0 {
		response1 = newvictory
		response2 = newloss
	} else {
		// acabaram as cartas - maior sanidade vence
		if p1Sanity > p2Sanity {
			response1 = newvictory
			response2 = newloss
		} else if p2Sanity > p1Sanity {
			response1 = newloss
			response2 = newvictory
		} else {
			response1 = newtie
			response2 = newtie
		}
	}

	type gameEndPayload struct {
		Tag string `json:"Tag"`
	}

	payload := gameEndPayload{Tag: "none"}
	data, _ := json.Marshal(payload)

	msg1 := Message{Request: response1, Data: data}
	msg2 := Message{Request: response2, Data: data}

	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)

	//fmt.Printf("DEBUG: Mensagens de fim enviadas: %s para P1, %s para P2\n", response1, response2)
}

func (m *Match) getCurrentPlayer() *User {
	if m.Turn == m.P1.UID {
		return m.P1
	}
	return m.P2
}

func (m *Match) switchTurn() {
	if m.Turn == m.P1.UID {
		m.Turn = m.P2.UID
	} else {
		m.Turn = m.P1.UID
	}
	//fmt.Printf("DEBUG: Turno trocado para jogador %s\n", m.Turn)
}

// notifica início do turno pros jogadores
func (m *Match) notifyTurnStart(enc1, enc2 *json.Encoder, currentPlayerUID string) {
	type turnPayload struct {
		Turn string `json:"turn"`
	}

	payload := turnPayload{Turn: currentPlayerUID}
	data, _ := json.Marshal(payload)

	msg := Message{
		Request: newturn,
		Data:    data,
	}

	_ = enc1.Encode(msg)
	_ = enc2.Encode(msg)

	//fmt.Printf("DEBUG: Notificação de turno enviada - turno de: %s\n", currentPlayerUID)
	time.Sleep(100 * time.Millisecond) // espera um pouco para garantir que a mensagem chegou ao cliente
}

// processa o turno e retorna se uma ação foi tomada
func (m *Match) processTurn(enc1, enc2 *json.Encoder) bool {
	currentPlayer := m.getCurrentPlayer()

	// verifica se o jogador está paralisado
	if m.DreamStates[currentPlayer.UID] == paralyzed {
		m.notifyBoth(enc1, enc2, fmt.Sprintf("%s está paralisado e perde o turno", currentPlayer.Username))
		time.Sleep(2 * time.Second)
		return false
	}

	//fmt.Printf("DEBUG: Aguardando ação do jogador %s\n", currentPlayer.UID)

	// timeout de 30 segundos
	timeout := time.After(30 * time.Second)

	for {
		select {
		case msg := <-m.inbox:
			//fmt.Printf("DEBUG: Mensagem recebida no inbox: %s de %s\n", msg.Action, msg.PlayerUID)

			// ignora se não é o jogador da vez
			if msg.PlayerUID != currentPlayer.UID {
				//fmt.Printf("DEBUG: Mensagem ignorada - não é turno de %s (turno atual: %s)\n",
				//	msg.PlayerUID, currentPlayer.UID)
				continue
			}

			switch msg.Action {
			case "usecard":
				//fmt.Printf("DEBUG: Processando usecard\n")
				if m.handleUseCard(enc1, enc2, msg) {
					return true
				}
			case "giveup":
				//fmt.Printf("DEBUG: Processando giveup\n")
				m.handleGiveUp(enc1, enc2, msg)
				return true
			}

		case <-timeout:
			//fmt.Printf("DEBUG: Timeout - jogador %s perdeu o turno\n", currentPlayer.UID)
			m.notifyBoth(enc1, enc2, fmt.Sprintf("%s perdeu o turno por timeout", currentPlayer.Username))
			return false
		}
	}
}

// notifica ambos os jogadores
func (m *Match) notifyBoth(enc1, enc2 *json.Encoder, message string) {
	type notifyPayload struct {
		Message string `json:"message"`
	}

	payload := notifyPayload{Message: message}
	data, _ := json.Marshal(payload)

	msg := Message{
		Request: notify,
		Data:    data,
	}

	_ = enc1.Encode(msg)
	_ = enc2.Encode(msg)
}

// remove carta da mão
func (m *Match) removeFromHand(playerUID string, card *Card) bool {
	hand := m.Hand[playerUID]

	for i, handCard := range hand {
		if handCard.CID == card.CID {
			m.Hand[playerUID] = append(hand[:i], hand[i+1:]...)
			//fmt.Printf("DEBUG: Carta %s removida da mão de %s\n", card.Name, playerUID)
			return true
		}
	}

	//fmt.Printf("DEBUG: Carta %s não encontrada na mão de %s\n", card.Name, playerUID)
	return false
}

// aplica o efeito das cartas
func (m *Match) applyCardEffect(playerUID string, card *Card, opponentUID string) {
	// Determina o UID do alvo da carta
	var targetUID string
	if card.CardType == Pill {
		targetUID = playerUID
	} else {
		targetUID = opponentUID
	}

	switch card.CardType {
	case Pill:
		m.Sanity[targetUID] += card.Points
	case NREM, REM:
		m.Sanity[targetUID] -= card.Points
	}

	// garante que sanidade não fique negativa
	if m.Sanity[targetUID] < 0 {
		m.Sanity[targetUID] = 0
	}

	//fmt.Printf("DEBUG: Efeito da carta aplicado - Jogador %s: %d -> %d\n",
	//	targetUID, oldSanity, m.Sanity[targetUID])
}

// gerencia o uso das cartas
func (m *Match) handleUseCard(enc1, enc2 *json.Encoder, in matchMsg) bool {
	type cardReq struct {
		Card Card `json:"card"`
	}
	var req cardReq
	if err := json.Unmarshal(in.Data, &req); err != nil {
		//fmt.Printf("DEBUG: Erro ao deserializar carta: %v\n", err)
		return false
	}

	//fmt.Printf("DEBUG: Processando carta %s do jogador %s\n", req.Card.Name, in.PlayerUID)

	// remove a carta da mão
	if !m.removeFromHand(in.PlayerUID, &req.Card) {
		return false
	}

	// determina o UID do oponente
	var opponentUID string
	if m.P1.UID == in.PlayerUID {
		opponentUID = m.P2.UID
	} else {
		opponentUID = m.P1.UID
	}

	// aplica os efeitos de sanidade da carta
	m.applyCardEffect(in.PlayerUID, &req.Card, opponentUID)

	// aplica os efeitos de estado da carta
	switch req.Card.CardEffect {
	case CONS:
		m.DreamStates[in.PlayerUID] = conscious
		m.RoundsInState[in.PlayerUID] = 0
	case AD:
		m.DreamStates[opponentUID] = sleepy
		m.RoundsInState[opponentUID] = 0
	case PAR:
		m.DreamStates[opponentUID] = paralyzed
		m.RoundsInState[opponentUID] = 0
	case AS:
		m.DreamStates[opponentUID] = scared
		m.RoundsInState[opponentUID] = 0
	}

	// notifica jogada
	player, _ := pm.GetByUID(in.PlayerUID)
	m.notifyBoth(enc1, enc2, fmt.Sprintf("%s jogou %s", player.Username, req.Card.Name))

	return true
}

// gerencia desistência
func (m *Match) handleGiveUp(enc1, enc2 *json.Encoder, in matchMsg) {
	m.State = Finished

	var playerEnc, opponentEnc *json.Encoder
	if in.PlayerUID == m.P1.UID {
		playerEnc = enc1
		opponentEnc = enc2
	} else {
		playerEnc = enc2
		opponentEnc = enc1
	}

	lossMsg := Message{Request: newloss, Data: nil}
	victoryMsg := Message{Request: newvictory, Data: nil}

	_ = playerEnc.Encode(lossMsg)
	_ = opponentEnc.Encode(victoryMsg)
}

// atualiza estado do jogo (sanidade, estados de sonho)
func (m *Match) updateGameState(enc1, enc2 *json.Encoder) {
	//fmt.Printf("DEBUG: Atualizando estado do jogo - Round %d\n", m.currentRound)

	// aplica efeitos dos estados de sonho
	for playerUID, state := range m.DreamStates {
		//oldSanity := m.Sanity[playerUID]

		switch state {
		case sleepy:
			m.Sanity[playerUID] -= 3
		case conscious:
			m.Sanity[playerUID] += 1
			m.RoundsInState[playerUID]++
			if m.RoundsInState[playerUID] >= 2 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}
		case paralyzed:
			m.RoundsInState[playerUID]++
			if m.RoundsInState[playerUID] >= 1 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}
		case scared:
			m.Sanity[playerUID] -= 4
			m.RoundsInState[playerUID]++
			if m.RoundsInState[playerUID] >= 2 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}
		}

		// garante sanidade não-negativa
		if m.Sanity[playerUID] < 0 {
			m.Sanity[playerUID] = 0
		}

		//fmt.Printf("DEBUG: Estado %s aplicado ao jogador %s: %d -> %d\n",
		//	string(state), playerUID, oldSanity, m.Sanity[playerUID])
	}

	// envia informações atualizadas
	m.sendUpdateInfo(enc1, enc2)
}

// envia informações atualizadas para os clientes
func (m *Match) sendUpdateInfo(enc1, enc2 *json.Encoder) {
	type updatePayload struct {
		Turn        string                `json:"turn"`
		Sanity      map[string]int        `json:"sanity"`
		DreamStates map[string]DreamState `json:"dreamStates"`
		Round       int                   `json:"round"`
	}

	payload := updatePayload{
		Turn:        m.Turn,
		Sanity:      m.Sanity,
		DreamStates: m.DreamStates,
		Round:       m.currentRound,
	}

	data, _ := json.Marshal(payload)
	msg := Message{Request: updateinfo, Data: data}

	_ = enc1.Encode(msg)
	_ = enc2.Encode(msg)

	//fmt.Printf("DEBUG: Update enviado - P1 sanidade: %d, P2 sanidade: %d\n",
	//	m.Sanity[m.P1.UID], m.Sanity[m.P2.UID])
}
