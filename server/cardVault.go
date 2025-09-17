package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

const CARDS_PER_BOOSTER int = 5

// inicializa as cartas do JSON no dicionário usando o cardDatabase
// função MTO importante para tirar do arquivo cardsVault.json (base de dados das cartas) e trazer virtualizadas pro jogo
func InitializeCardsFromJSON(filename string) (map[string]Card, error) {
	file, error := os.ReadFile(filename)
	if error != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %v", error)
	}

	// base de dados de cartas, definido no types
	// cardDB contém um map de CID e cartas
	var cardDB CardDatabase
	error = json.Unmarshal(file, &cardDB)
	if error != nil {
		return nil, fmt.Errorf("erro ao deserializar JSON: %v", error)
	}

	return cardDB.Cards, nil
}

// criar cardVault
func NewCardVault() *CardVault {
	return &CardVault{
		CardGlossary:    make(map[string]Card),
		CardQuantity:    make(map[string]int),
		Vault:           make(map[int]Booster),
		BoosterQuantity: 0,
		Total:           0,
		Generator:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// função para verificar se está vazio
func (vault *CardVault) IsEmpty() bool {
	return len(vault.CardGlossary) == 0
}

// inicializa o vault com as cartas da base
func (vault *CardVault) LoadCardsFromFile(filename string) error {
	cards, error := InitializeCardsFromJSON(filename)
	if error != nil {
		return error
	}

	vault.CardGlossary = cards // como já tem o database, cartas vem em dict normalmente ([CID]:card)

	// inicializa quantidades zeradas
	for cid := range cards {
		vault.CardQuantity[cid] = 0
	}

	return nil
}

// calcula quantidade de cópias de cada carta
// coloco a quantidade de boosters que quero
// retorno: map com quantos
func (vault *CardVault) calculateCardCopies(boostersCount int) map[string]int {
	// conta cartas por tipo
	remCards := []string{}
	nremCards := []string{}
	pillCards := []string{}

	// acrescento cada CID nos slices de string contendo os CIDs
	for cid, card := range vault.CardGlossary {
		switch card.CardType {
		case REM:
			remCards = append(remCards, cid)
		case NREM:
			nremCards = append(nremCards, cid)
		case Pill:
			pillCards = append(pillCards, cid)
		}
	}

	totalCardsNeeded := boostersCount * CARDS_PER_BOOSTER

	// faço a distribuição por raridade, considerando
	// 50% das cartas são comuns
	// 40% das cartas são incomuns
	// 10% das cartas são raras
	commonCards := int(float64(totalCardsNeeded) * 0.5)
	uncommonCards := int(float64(totalCardsNeeded) * 0.4)
	rareCards := int(float64(totalCardsNeeded) * 0.1)

	copies := make(map[string]int) // map que contém quantidade de cada carta

	// agora, calculo quantas cópias serão necessárias para cada carta
	for cid, card := range vault.CardGlossary { // passo por cada carta no glossário
		var neededCopies float64

		switch card.CardRarity {
		case Comum:
			// divido as raridades proporcionalmente aos cardType
			commonByType := float64(commonCards) / 3.0 // rem, nrem, pill
			switch card.CardType {
			case REM:
				neededCopies = commonByType / float64(len(remCards))
			case NREM:
				neededCopies = commonByType / float64(len(nremCards))
			case Pill:
				neededCopies = commonByType / float64(len(pillCards))
			}
		case Incomum:
			uncommonByType := float64(uncommonCards) / 3.0
			switch card.CardType {
			case REM:
				neededCopies = uncommonByType / float64(len(remCards))
			case NREM:
				neededCopies = uncommonByType / float64(len(nremCards))
			case Pill:
				neededCopies = uncommonByType / float64(len(pillCards))
			}
		case Rara:
			rareByType := float64(rareCards) / 3.0
			switch card.CardType {
			case REM:
				neededCopies = rareByType / float64(len(remCards))
			case NREM:
				neededCopies = rareByType / float64(len(nremCards))
			case Pill:
				neededCopies = rareByType / float64(len(pillCards))
			}
		}

		finalCopies := int(math.Round(neededCopies))

		// garante pelo menos 1 cópia de cada carta
		if neededCopies < 1 {
			neededCopies = 1
		}

		copies[cid] = finalCopies
	}

	// agora, verifica se o calculado realmente bate com a quantidade
	totalCalculated := 0
	for _, quantity := range copies {
		totalCalculated += quantity
	}

	difference := totalCalculated - totalCardsNeeded

	if difference > 0 {
		// precisa remover cartas, então remove das que têm mais cópias
		for i := 0; i < difference; i++ {
			maxCopies := 1
			maxCardID := ""

			for cardID, quantity := range copies {
				if quantity > maxCopies {
					maxCopies = quantity
					maxCardID = cardID
				}
			}

			if maxCardID != "" {
				copies[maxCardID]--
			}
		}
	} else if difference < 0 {
		// casoo precise adicionar, adiciona nas que têm menos cópias
		for i := 0; i < -difference; i++ {
			minCopies := math.MaxInt32
			minCardID := ""

			for cardID, quantity := range copies {
				if quantity < minCopies {
					minCopies = quantity
					minCardID = cardID
				}
			}

			if minCardID != "" {
				copies[minCardID]++
			}
		}
	}

	return copies
}

// crio um "pool" de cartas baseado nas cópias calculadas
// esse bolo de cartas é utilizado na hora de criar os boosters
func (vault *CardVault) createCardPool(copies map[string]int) []Card {
	var pool []Card

	for cid, quantity := range copies {
		card := vault.CardGlossary[cid]
		for i := 0; i < quantity; i++ {
			pool = append(pool, card)
		}
		// Atualiza estoque
		vault.CardQuantity[cid] += quantity
	}

	return pool
}

// cria os boosters
func (vault *CardVault) createBoosters(boostersCount int) error {
	if vault.IsEmpty() {
		return fmt.Errorf("CardVault não foi inicializado com cartas")
	}

	// calculo quantas cópias de cada carta são necessárias
	copies := vault.calculateCardCopies(boostersCount)

	// crio o pool de cartas
	cardPool := vault.createCardPool(copies)

	// embaralho o pool com o generator
	vault.Generator.Shuffle(len(cardPool), func(i, j int) {
		cardPool[i], cardPool[j] = cardPool[j], cardPool[i]
	})

	// crio os boosters individualmente
	for i := 0; i < boostersCount; i++ {
		booster := Booster{
			BID:     i + 1,
			Booster: make([]Card, 0, CARDS_PER_BOOSTER),
		}

		// pego as próximas n cartas do pool
		startIndex := i * CARDS_PER_BOOSTER
		endIndex := startIndex + CARDS_PER_BOOSTER

		// verifico se ainda tá dentro do tamanho do pool
		if endIndex > len(cardPool) {
			endIndex = len(cardPool)
		}

		// acrescento as cartas
		for j := startIndex; j < endIndex; j++ {
			booster.Booster = append(booster.Booster, cardPool[j])
		}

		vault.Vault[i+1] = booster
	}
	vault.BoosterQuantity = boostersCount
	vault.Total = boostersCount * CARDS_PER_BOOSTER

	return nil
}

// remover um booster do estoque (para dar ao jogador)
func (vault *CardVault) TakeBooster() (Booster, error) {
	if vault.BoosterQuantity == 0 {
		return Booster{}, fmt.Errorf("não há boosters disponíveis")
	}

	// Pega um booster aleatório
	boosterIDs := make([]int, 0, len(vault.Vault))
	for id := range vault.Vault {
		boosterIDs = append(boosterIDs, id)
	}

	randomIndex := vault.Generator.Intn(len(boosterIDs))
	boosterID := boosterIDs[randomIndex]

	booster := vault.Vault[boosterID]
	delete(vault.Vault, boosterID)
	vault.BoosterQuantity--

	return booster, nil
}
