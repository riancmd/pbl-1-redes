package main

import (
	"errors"
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
func (pm *PlayerManager) CreatePlayer(username, password string) (*User, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.byUsername[username]; exists {
		return nil, errors.New("usuário já existe")
	}
	pm.nextID++
	p := &User{
		UID:       strconv.Itoa(pm.nextID),
		Username:  username,
		Password:  password,
		Deck:      make([]*Card, 0),
		CreatedAt: time.Now(),
		LastLogin: time.Now(),
	}
	pm.byUID[p.UID] = p
	pm.byUsername[p.Username] = p
	pm.activeByUID[p.UID] = p
	return p, nil
}

// faz login
func (pm *PlayerManager) Login(login, password string, conn net.Conn) (*User, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p, ok := pm.byUsername[login]
	if !ok {
		return nil, errors.New("usuário não encontrado")
	}
	if p.Password != password {
		return nil, errors.New("senha inválida")
	}
	p.Connection = conn
	pm.activeByUID[p.Username] = p
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
}
