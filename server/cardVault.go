package server

import (
	"encoding/json"
	"fmt"
	"os"
)

// inicializa as cartas do JSON no dicionário usando o cardDatabase
func InitializeCardsFromJSON(filename string) (map[string]Card, error) {
	file, error := os.ReadFile(filename)
	if error != nil {
		return nil, fmt.Errorf("erro ao ler arquivo: %v", error)
	}

	var cardDB CardDatabase
	error = json.Unmarshal(file, &cardDB)
	if error != nil {
		return nil, fmt.Errorf("erro ao deserializar JSON: %v", error)
	}

	return cardDB.Cards, nil
}

//
