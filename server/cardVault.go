package server

import (
	"fmt"
	"math/rand"
	"time"
)

// banco de cartas
func NewCardVault() *CardVault {
	return &CardVault{
		generator: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// carrega cartas genéricas de início
func (cardVault *CardVault) LoadInitialCards() {
	cardTypes := []struct {
		cardType CardType
		name     string
	}{
		{REM, "Dream"},
		{NREM, "Nightmare"},
		{Pill, "Pill"},
	}

	for _, cardType := range cardTypes {
		for i := 1; i <= 10; i++ {
			points := (i % 10) + 1
			cid := fmt.Sprintf("0%d", i-1)
			name := fmt.Sprintf("%s %02d", cardType.name, i)
			cardVault.vault = append(cardVault.vault, Card{
				Name:     name,
				CID:      cid,
				CardType: cardType.cardType,
				Points:   points,
			})
		}
	}
}

// pega cartas aleatórias do banco
func (cardVault *CardVault) DrawRandomCards(count int) []*Card {
	if len(cardVault.vault) == 0 {
		return nil
	}

	drawn := make([]*Card, count)
	for i := 0; i < count; i++ {
		original := cardVault.vault[cardVault.generator.Intn(len(cardVault.vault))]
		cardCopy := original
		drawn[i] = &cardCopy
	}
	return drawn
}
