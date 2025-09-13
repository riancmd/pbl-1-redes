package server

import (
	"errors"
	"time"
)

func (mm *MatchManager) Enqueue(p *User) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
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

func (mm *MatchManager) dequeue() (*User, error) {
	if len(mm.queue) == 0 {
		return nil, errors.New("fila vazia")
	}
	p := mm.queue[0]
	mm.queue = mm.queue[1:]
	return p, nil
}

func (mm *MatchManager) FindMatchByPlayerUID(uid string) *Match {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	return mm.byPlayer[uid]
}

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

				Hand:   map[string][]*Card{},
				Sanity: map[string]int{p1.UID: 40, p2.UID: 40},
				inbox:  make(chan matchMsg, 16),
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
