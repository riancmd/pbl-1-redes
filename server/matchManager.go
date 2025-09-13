package server

import (
	"encoding/json"
	"errors"
	"math/rand"
	"time"
)

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
		if q.ID == p.UID {
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
	enc1 := json.NewEncoder(m.P1.Conn)
	enc2 := json.NewEncoder(m.P2.Conn)

	// são ajustados os estados de sonho de cada jogador
	m.RoundsInState = map[string]int{m.P1.UID: 0, m.P2.UID: 0}
	m.StateLockedUntil = map[string]int{m.P1.UID: 0, m.P2.UID: 0}
	m.currentRound = 1

	// são escolhidas 10 cartas aleatórias do inventário de cada jogador
	m.Hand[m.P1.ID] = drawCards(m.P1.Inventory)
	m.Hand[m.P2.ID] = drawCards(m.P2.Inventory)

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
		Info:        m.P2.UserName,
		Turn:        m.Turn,
		Hand:        m.Hand[m.P1.UID],
		Sanity:      m.Sanity,
		DreamStates: m.DreamStates,
	}

	// Payload para P2
	p2Payload := startPayload{
		Info:        m.P1.UserName,
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
