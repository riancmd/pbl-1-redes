package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func NewBattleCoordinator() *BattleCoordinator {
	return &BattleCoordinator{
		waitingQueue:  make([]*ActiveSession, 0),
		activeBattles: make(map[string]*BattleArena),
		nextBattleID:  0,
	}
}

// jogador entra na fila
func (battleCoordinator *BattleCoordinator) JoinQueue(session *ActiveSession) error {
	battleCoordinator.mutex.Lock()
	defer battleCoordinator.mutex.Unlock()

	if session.IsInBattle {
		return errors.New("jogador já está em batalha")
	}

	for _, waiting := range battleCoordinator.waitingQueue {
		if waiting.SID == session.SID {
			return errors.New("jogador já está na fila")
		}
	}

	battleCoordinator.waitingQueue = append(battleCoordinator.waitingQueue, session)
	return nil
}

func (battleCoordinator *BattleCoordinator) StartMatchmaking() {
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			battleCoordinator.processQueue()
		}
	}()
}

func (battleCoordinator *BattleCoordinator) processQueue() {
	battleCoordinator.mutex.Lock()
	defer battleCoordinator.mutex.Unlock()

	if len(battleCoordinator.waitingQueue) >= 2 {
		p1 := battleCoordinator.waitingQueue[0]
		p2 := battleCoordinator.waitingQueue[1]
		battleCoordinator.waitingQueue = battleCoordinator.waitingQueue[2:]

		battleCoordinator.nextBattleID++
		battleID := fmt.Sprintf("battle_%d", battleCoordinator.nextBattleID)

		arena := &BattleArena{
			BID:        battleID,
			Player1:    p1,
			Player2:    p2,
			State:      InProgress,
			Scoreboard: map[string]int{p1.SID: 0, p2.SID: 0},
		}

		p1.IsInBattle = true
		p2.IsInBattle = true

		battleCoordinator.activeBattles[battleID] = arena

		go arena.RunBattle()
	}
}

func (battleCoordinator *BattleCoordinator) selectCards(inventory []*Card, max int) []*Card {
	if len(inventory) == 0 {
		return []*Card{}
	}

	shuffled := make([]*Card, len(inventory))
	copy(shuffled, inventory)

	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	if len(shuffled) > max {
		shuffled = shuffled[:max]
	}
	return shuffled
}

// lógica da batalha com as arenas
func (arena *BattleArena) RunBattle() {
	enc1 := json.NewEncoder(arena.Player1.Connection)
	enc2 := json.NewEncoder(arena.Player2.Connection)

	arena.sendBattleStart(enc1, enc2)
	roundCards := make(map[string]*Card)

	for {
		select {
		case command := <-arena.RoundActions:
			switch command.Command {
			case "play_card":
				var cardRequest struct {
					Card Card `json:"card"`
				}
				if err := json.Unmarshal(command.CardData, &cardRequest); err != nil {
					fmt.Println("Erro ao decodificar carta:", err)
					continue
				}

				if arena.removeCardFromHand(command.SessionID, &cardRequest.Card) {
					roundCards[command.SessionID] = &cardRequest.Card

					playerName := arena.getPlayerName(command.SessionID)
					arena.notifyBothPlayers(enc1, enc2, fmt.Sprintf("%s jogou uma carta.", playerName))

					// Se os dois jogaram, resolve o round
					if len(roundCards) == 2 {
						arena.resolveRound(enc1, enc2, roundCards)
						roundCards = make(map[string]*Card)
					}
				}
			}
		}
	}
}

// notificar início de batalha
func (arena *BattleArena) sendBattleStart(enc1, enc2 *json.Encoder) {
	type battleStart struct {
		OpponentName string  `json:"opponent_name"`
		Turn         string  `json:"turn"`
		Hand         []*Card `json:"hand"`
	}

	start1 := battleStart{
		OpponentName: arena.Player2.Username,
		Turn:         arena.CurrentTurn,
		Hand:         arena.Player1Hand,
	}
	start2 := battleStart{
		OpponentName: arena.Player1.Username,
		Turn:         arena.CurrentTurn,
		Hand:         arena.Player2Hand,
	}

	data1, _ := json.Marshal(start1)
	data2, _ := json.Marshal(start2)

	enc1.Encode(Message{Request: "battle_start", Data: data1})
	enc2.Encode(Message{Request: "battle_start", Data: data2})
}

func (arena *BattleArena) getPlayerName(sessionID string) string {
	if arena.Player1.SID == sessionID {
		return arena.Player1.Username
	}
	return arena.Player2.Username
}

func (arena *BattleArena) removeCardFromHand(sessionID string, card *Card) bool {
	arena.battleMutex.Lock()
	defer arena.battleMutex.Unlock()

	var hand *[]*Card
	if arena.Player1.SID == sessionID {
		hand = &arena.Player1Hand
	} else {
		hand = &arena.Player2Hand
	}

	for i, c := range *hand {
		if c.Name == card.Name && c.CardType == card.CardType && c.Points == card.Points {
			*hand = append((*hand)[:i], (*hand)[i+1:]...)
			return true
		}
	}
	return false
}

func (arena *BattleArena) notifyBothPlayers(enc1, enc2 *json.Encoder, message string) {
	notification := struct {
		Info string `json:"info"`
		Turn string `json:"turn"`
	}{Info: message, Turn: arena.CurrentTurn}

	data, _ := json.Marshal(notification)
	msg := Message{Request: "battle_update", Data: data}

	enc1.Encode(msg)
	enc2.Encode(msg)
}

func (arena *BattleArena) resolveRound(enc1, enc2 *json.Encoder, cards map[string]*Card) {
	p1Card := cards[arena.Player1.SID]
	p2Card := cards[arena.Player2.SID]

	winner := arena.determineWinner(p1Card, p2Card)

	var info string

	switch winner {
	case 1:
		arena.Scoreboard[arena.Player1.SID]++
		info = fmt.Sprintf("%s venceu! (%s %d) vs (%s %d)",
			arena.Player1.Username,
			strings.Title(string(p1Card.CardType)), p1Card.Points,
			strings.Title(string(p2Card.CardType)), p2Card.Points)
		arena.CurrentTurn = arena.Player2.SID
	case 2:
		arena.Scoreboard[arena.Player2.SID]++
		info = fmt.Sprintf("%s venceu! (%s %d) vs (%s %d)",
			arena.Player2.Username,
			strings.Title(string(p2Card.CardType)), p2Card.Points,
			strings.Title(string(p1Card.CardType)), p1Card.Points)
		arena.CurrentTurn = arena.Player1.SID
	default:
		info = fmt.Sprintf("Empate! (%s %d) vs (%s %d)",
			strings.Title(string(p1Card.CardType)), p1Card.Points,
			strings.Title(string(p2Card.CardType)), p2Card.Points)
	}

	scoreInfo := fmt.Sprintf("Placar: %s %d x %d %s",
		arena.Player1.Username, arena.Scoreboard[arena.Player1.SID],
		arena.Scoreboard[arena.Player2.SID], arena.Player2.Username)

	arena.notifyBothPlayers(enc1, enc2, info+" | "+scoreInfo)

	if arena.Scoreboard[arena.Player1.SID] >= 3 || arena.Scoreboard[arena.Player2.SID] >= 3 {
		arena.finalizeBattle(enc1, enc2)
	}
}

func (arena *BattleArena) determineWinner(c1, c2 *Card) int {
	if c1.Points > c2.Points {
		return 1
	} else if c2.Points > c1.Points {
		return 2
	}
	return 0
}

func (arena *BattleArena) finalizeBattle(enc1, enc2 *json.Encoder) {
	arena.State = Completed

	var winner string
	if arena.Scoreboard[arena.Player1.SID] > arena.Scoreboard[arena.Player2.SID] {
		winner = arena.Player1.Username
	} else {
		winner = arena.Player2.Username
	}

	finalMsg := fmt.Sprintf("🏆 %s venceu a batalha!", winner)
	data, _ := json.Marshal(finalMsg)

	enc1.Encode(Message{Request: "battle_finish", Data: data})
	enc2.Encode(Message{Request: "battle_finish", Data: data})

	arena.Player1.IsInBattle = false
	arena.Player2.IsInBattle = false
	close(arena.RoundRequests)
}
