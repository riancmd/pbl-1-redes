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
			mm.nextID++
			match := &Match{
				ID:    mm.nextID,
				P1:    p1,
				P2:    p2,
				State: Running,
				Turn:  p1.UID, // p1 começa

				Hand:        map[string][]*Card{},
				Sanity:      map[string]int{p1.UID: 40, p2.UID: 40},
				DreamStates: map[string]DreamState{p1.UID: sleepy, p2.UID: sleepy},
				inbox:       make(chan matchMsg, 16),
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
	// cria codificadores para cada usuário
	enc1 := json.NewEncoder(m.P1.Connection)
	enc2 := json.NewEncoder(m.P2.Connection)

	// são ajustados os estados de sonho de cada jogador
	m.RoundsInState = map[string]int{m.P1.UID: 0, m.P2.UID: 0}
	m.StateLockedUntil = map[string]int{m.P1.UID: 0, m.P2.UID: 0}
	m.currentRound = 1

	// são escolhidas 10 cartas aleatórias do inventário de cada jogador
	m.Hand[m.P1.UID] = drawCards(m.P1.Deck)
	m.Hand[m.P2.UID] = drawCards(m.P2.Deck)

	m.sendGameStart(enc1, enc2)

	// loop do jogo

	for {
		// aqui é verificado se as condições de fim foram atingidas
		if m.checkGameEnd() {
			m.endGame(enc1, enc2)
			return
		}

		// processa-se o turno do jogador atual
		m.processTurn(enc1, enc2)

		// atualiza as infos para cada jogador
		m.updateInfo(enc1, enc2)

		// troca turnos
		m.switchTurn()
		m.currentRound++
	}
}

// pega 10 cartas
func drawCards(deck []*Card) []*Card {
	// embaralha e pega até 10
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
// o dado é enviado sobre a mão e início do jogo pra cada jogador
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

	// uma mensagem pra cada usuário contendo o response e a mão
	msg1 := Message{Request: gamestart}
	msg2 := Message{Request: gamestart}

	data1, _ := json.Marshal(p1Payload)
	data2, _ := json.Marshal(p2Payload)
	msg1.Data = data1
	msg2.Data = data2
	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)
}

func (m *Match) checkGameEnd() bool {
	// verifica se alguém chegou a 0 de sanidade
	for _, sanity := range m.Sanity {
		if sanity <= 0 {
			return true
		}
	}

	// Verificar se acabaram as cartas
	if len(m.Hand[m.P1.UID]) == 0 && len(m.Hand[m.P2.UID]) == 0 {
		return true
	}

	return false
}

func (m *Match) endGame(enc1, enc2 *json.Encoder) {
	// determina qual o vencedor
	var response1 string
	var response2 string
	p1Sanity := m.Sanity[m.P1.UID]
	p2Sanity := m.Sanity[m.P2.UID]

	// quem zerou a sanidade perde
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
		// quem tiver maior sanidade vence, se acabaram as cartas
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

	payload := gameEndPayload{
		Tag: "none",
	}

	// uma mensagem pra cada usuário contendo o resultado
	msg1 := Message{Request: response1}
	msg2 := Message{Request: response2}

	data1, _ := json.Marshal(payload)
	data2, _ := json.Marshal(payload)
	msg1.Data = data1
	msg2.Data = data2
	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)

	// limpando o estado da partida
	m.State = Finished
	m.P1.IsInBattle = false
	m.P2.IsInBattle = false
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

// notifica início do turno pros jogadores
// relevante para enviar a server response de newturn
func (m *Match) notifyTurnStart(enc1, enc2 *json.Encoder, currentPlayerUID string) {
	type turnPayload struct {
		CurrentPlayer string `json:"currentPlayer"`
	}

	payload := turnPayload{CurrentPlayer: currentPlayerUID}
	data, _ := json.Marshal(payload)

	msg := Message{
		Request: newturn,
		Data:    data,
	}

	_ = enc1.Encode(msg)
	_ = enc2.Encode(msg)
}

func (m *Match) processTurn(enc1, enc2 *json.Encoder) {
	currentPlayer := m.getCurrentPlayer()

	// verifica se o jogador está paralisado
	// manda notificação caso esteja
	if m.DreamStates[currentPlayer.UID] == paralyzed {
		m.notifyBoth(enc1, enc2, fmt.Sprintf("%s está paralisado e perde o turno", currentPlayer.Username))
		return
	}

	// notifica que é o turno deste jogador
	m.notifyTurnStart(enc1, enc2, currentPlayer.UID)

	in := <-m.inbox // canal entre goroutines

	// aguarda ação do jogador (usecard ou giveup)
	for {
		// ignora se não é o jogador da vez
		if in.PlayerUID != currentPlayer.UID {
			continue
		}

		switch in.Action {
		case "usecard":
			if m.handleUseCard(enc1, enc2, in) {
				return // acabou turno
			}
		case "giveup":
			m.handleGiveUp(enc1, enc2, in)
			return
		}
		/*case <-time.After(15 * time.Second): // timeout se demorar mais q 15s
		m.handleTurnTimeout(enc1, enc2, currentPlayer.UID)
		return*/
	}
}

// remove carta da mão
func (m *Match) removeFromHand(playerUID string, card *Card) bool {
	hand := m.Hand[playerUID] // mão do player

	for i, handCard := range hand {
		if handCard.CID == card.CID {
			// remove carta da mão
			m.Hand[playerUID] = append(hand[:i], hand[i+1:]...)
			return true
		}
	}

	return false // caso a carta  seja encontrada
}

// aplica o efeito das cartas
func (m *Match) applyCardEffect(playerUID string, card *Card) {
	switch card.CardType {
	case "pill":
		m.Sanity[playerUID] += card.Points

	case "nrem":
		m.Sanity[playerUID] -= card.Points

	case "rem":
		m.Sanity[playerUID] -= card.Points
	}
}

// gerencia o uso das cartas
func (m *Match) handleUseCard(enc1, enc2 *json.Encoder, in matchMsg) bool {
	type cardReq struct {
		Card Card `json:"card"`
	}
	var req cardReq
	if err := json.Unmarshal(in.Data, &req); err != nil {
		return false
	}

	// remove a carta da mão
	if !m.removeFromHand(in.PlayerUID, &req.Card) {
		return false
	}

	// agora, aplica os efeitos da carta
	m.applyCardEffect(in.PlayerUID, &req.Card)

	// notificar que jogador usou carta
	player, _ := pm.GetByUID(in.PlayerUID)
	playerName := player.Username
	m.notifyBoth(enc1, enc2, fmt.Sprintf("%s jogou %s", playerName, req.Card.Name))

	return true // turno finalizado
}

// gerencia desistência
func (m *Match) handleGiveUp(enc1, enc2 *json.Encoder, in matchMsg) {
	// determinar qual encoder corresponde a cada jogador
	var playerEnc, opponentEnc *json.Encoder

	if in.PlayerUID == m.P1.UID {
		playerEnc = enc1
		opponentEnc = enc2
	} else {
		playerEnc = enc2
		opponentEnc = enc1
	}

	// enviar derrota para quem desistiu
	lossMsg := Message{Request: "newLoss", Data: nil}
	_ = playerEnc.Encode(lossMsg)

	// enviar vitória para o oponente
	victoryMsg := Message{Request: "newVictory", Data: nil}
	_ = opponentEnc.Encode(victoryMsg)

	// finalizar partida
	m.State = Finished
	m.P1.IsInBattle = false
	m.P2.IsInBattle = false
}

// fase de atualizar informações pros jogadores
// atualiza os estados, sanidade, etc.
func (m *Match) updateInfo(enc1, enc2 *json.Encoder) {
	// aplica os efeitos dos estados de sonho
	for playerUID, state := range m.DreamStates {
		switch state {
		case sleepy:
			m.Sanity[playerUID] -= 3
			// esse estado pode ser alterado na próxima rodada

		case conscious:
			m.Sanity[playerUID] += 1
			m.RoundsInState[playerUID]++
			// depois de uma rodada, volta para sleepy
			if m.RoundsInState[playerUID] >= 1 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}

		case paralyzed:
			// +0 sanidade, já tratado no processTurn
			m.RoundsInState[playerUID]++
			// depois de uma rodada, volta para sleepy
			if m.RoundsInState[playerUID] >= 1 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}

		case scared:
			m.Sanity[playerUID] -= 4
			m.RoundsInState[playerUID]++
			// dura duas rodadas
			if m.RoundsInState[playerUID] >= 2 {
				m.DreamStates[playerUID] = sleepy
				m.RoundsInState[playerUID] = 0
			}
		}
	}

	// verifica se sanidade ficou negativa
	for playerUID, sanity := range m.Sanity {
		if sanity < 0 {
			m.Sanity[playerUID] = 0
		}
	}

	// enviar update para ambos os jogadores
	m.sendUpdateInfo(enc1, enc2)
}

// atualiza informações da partida para ambos os jogadores
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

	// uma mensagem pra cada usuário contendo o response e a mão
	msg1 := Message{Request: updateinfo}
	msg2 := Message{Request: updateinfo}

	data1, _ := json.Marshal(payload)
	data2, _ := json.Marshal(payload)
	msg1.Data = data1
	msg2.Data = data2
	_ = enc1.Encode(msg1)
	_ = enc2.Encode(msg2)
}
