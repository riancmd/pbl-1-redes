package server

import (
	"fmt"
	"net"
	"time"
)

// cria um gerenciador de sessões
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*ActiveSession),
		nextSID:  1000,
	}
}

// cria as sessões (usuário conectado)
func (sesReg *SessionRegistry) CreateSession(loginKey, username string, conn net.Conn) *ActiveSession {
	sesReg.mutex.Lock()
	defer sesReg.mutex.Unlock()

	sesReg.nextSID++
	session := &ActiveSession{
		SID:        fmt.Sprintf("sess_%d", sesReg.nextSID),
		Username:   username,
		Connection: conn,
		LastPing:   time.Now(),
		IsInBattle: false,
	}

	sesReg.sessions[loginKey] = session
	return session
}

// funções de gerenciamento de
// buscar sessão
func (sesReg *SessionRegistry) FindSession(loginKey string) *ActiveSession {
	return sesReg.sessions[loginKey]
}

// desconectar usuário
func (sesReg *SessionRegistry) RemoveSession(loginKey string) {
	sesReg.mutex.Lock()
	defer sesReg.mutex.Unlock()
	delete(sesReg.sessions, loginKey)
}

// verificar se sessão está ativa
func (sesReg *SessionRegistry) IsSessionActive(loginKey string) bool {
	_, exists := sesReg.sessions[loginKey]
	return exists
}

// recebe todas as sessões ativas
func (sesReg *SessionRegistry) GetAllActiveSessions() []*ActiveSession {
	var active []*ActiveSession
	for _, session := range sesReg.sessions {
		active = append(active, session)
	}
	return active
}
