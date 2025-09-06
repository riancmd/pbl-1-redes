package server

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

func NewBattleCoordinator() *BattleCoordinator {
	return &BattleCoordinator{
		waitingQueue:  make([]*ActiveSession, 0),
		activeBattles: make(map[string]*BattleArena),
		playerBattles: make(map[string]*BattleArena),
		nextBattleID:  1,
	}
}

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

func (battleCoordinator *BattleCoordinator) FindBattleForSession(sessionID string) *BattleArena {
	battleCoordinator.mutex.RLock()
	defer battleCoordinator.mutex.RUnlock()
	return battleCoordinator.playerBattles[sessionID]
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

		// Carregar cartas dos jogadores
		p1Cards := battleCoordinator.loadPlayerCards(p1.LoginKey)
		p2Cards := battleCoordinator.loadPlayerCards(p2.LoginKey)

		arena := &BattleArena{
			BID:          battleID,
			Player1:      p1,
			Player2:      p2,
			State:        InProgress,
			CurrentTurn:  p1.SID,
			Player1Hand:  battleCoordinator.selectCards(p1Cards, 7),
			Player2Hand:  battleCoordinator.selectCards(p2Cards, 7),
			Scoreboard:   map[string]int{p1.SID: 0, p2.ID: 0},
			RoundActions: make(chan BattleCommand, 20),
		}

		p1.IsInBattle = true
		p2.IsInBattle = true

		battleCoordinator.activeBattles[battleID] = arena
		battleCoordinator.playerBattles[p1.SID] = arena
		battleCoordinator.playerBattles[p2.SID] = arena

		go arena.RunBattle()
	}
}

func (battleCoordinator *BattleCoordinator) loadPlayerCards(loginKey string) []*Card {
	account, err := accountStorage.LoadAccount(loginKey)
	if err != nil {
		return []*Card{}
	}
	return account.Cards
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
