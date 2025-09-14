package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

func NewPlayerManager() *PlayerManager {
	return &PlayerManager{
		byUID:       make(map[string]*User),
		byUsername:  make(map[string]*User),
		activeByUID: make(map[string]*User),
	}
}

// cria novo usuário
func (pm *PlayerManager) CreatePlayer(username, password string, connection net.Conn) (*User, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.byUsername[username]; exists {
		return nil, errors.New("usuário já existe")
	}
	pm.nextID++
	p := &User{
		UID:        strconv.Itoa(pm.nextID),
		Username:   username,
		Password:   password,
		Deck:       make([]*Card, 0),
		CreatedAt:  time.Now(),
		LastLogin:  time.Now(),
		Connection: connection,
	}
	pm.byUID[p.UID] = p
	pm.byUsername[p.Username] = p
	pm.activeByUID[p.UID] = p
	return p, nil
}

// faz login
func (pm *PlayerManager) Login(username, password string, conn net.Conn) (*User, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byUsername[username]
	if !ok {
		return nil, errors.New("usuário não encontrado")
	}
	if p.Password != password {
		return nil, errors.New("senha inválida")
	}
	p.Connection = conn
	pm.activeByUID[p.UID] = p
	return p, nil
}

// pesquisa por ID
func (pm *PlayerManager) GetByUID(uid string) (*User, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byUID[uid]
	if !ok {
		return nil, errors.New("usuário não encontrado")
	}
	return p, nil
}

// adiciona ao deck
func (pm *PlayerManager) AddToDeck(uid string, cards []*Card) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byUID[uid]
	if !ok {
		return errors.New("usuário não encontrado")
	}
	p.Deck = append(p.Deck, cards...)
	return nil
}

func (pm *PlayerManager) Logout(user *User) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.activeByUID, user.UID)

	fmt.Printf("DEBUG: Tentando deslogar usuário %s (UID: %s)\n", user.Username, user.UID)

	// garante que usuário seja completamente desconectado
	if _, exists := pm.activeByUID[user.UID]; exists {
		delete(pm.activeByUID, user.UID)
		fmt.Printf("DEBUG: Usuário %s removido com sucesso\n", user.Username)
	} else {
		fmt.Printf("DEBUG: Usuário %s não estava na lista de ativos\n", user.Username)
	}

	user.IsInBattle = false
	user.Connection = nil
}
