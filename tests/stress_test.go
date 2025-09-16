package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// define o endereço do servidor para o teste
const serverAddress = "server:8080" // 'server' é o nome do serviço no docker-compose
const numClients = 50               // número de clientes a serem simulados
const numPacks = 5                  // número de pacotes que cada cliente vai tentar comprar

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"request"`
	UID     string          `json:"uid"` // user id
	Data    json.RawMessage `json:"data"`
}

// clientSession simula um cliente com sua própria conexão e dados
type clientSession struct {
	conn       net.Conn
	enc        *json.Encoder
	dec        *json.Decoder
	uid        string
	username   string
	resultChan chan string
}

// newClientSession cria e inicializa uma nova sessão de cliente
func newClientSession(resultChan chan string) (*clientSession, error) {
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		return nil, err
	}
	return &clientSession{
		conn:       conn,
		enc:        json.NewEncoder(conn),
		dec:        json.NewDecoder(conn),
		resultChan: resultChan,
	}, nil
}

// close fecha a conexão do cliente
func (s *clientSession) close() {
	s.conn.Close()
}

// sendRequest envia uma requisição para o servidor
func (s *clientSession) sendRequest(requestType string, data interface{}) error {
	msg := Message{
		Request: requestType,
		UID:     s.uid,
	}
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		msg.Data = jsonData
	}
	return s.enc.Encode(msg)
}

// receiveResponse recebe e decodifica a resposta do servidor
func (s *clientSession) receiveResponse() (*Message, error) {
	var msg Message
	err := s.dec.Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// registerAndLogin simula o fluxo de registro e login de um jogador
func (s *clientSession) registerAndLogin() error {
	s.username = fmt.Sprintf("tester_%d", time.Now().UnixNano())
	data := map[string]string{
		"username": s.username,
		"password": "password123",
	}

	// envia a requisição de registro
	if err := s.sendRequest("register", data); err != nil {
		return err
	}

	// aguarda a resposta do registro (e os pacotes gratuitos)
	var resp *Message
	var err error
	for i := 0; i < 5; i++ { // espera a resposta do registro + 4 pacotes
		resp, err = s.receiveResponse()
		if err != nil {
			return err
		}
	}

	var pr Message
	if err := json.Unmarshal(resp.Data, &pr); err != nil {
		return err
	}
	s.uid = pr.UID
	s.resultChan <- fmt.Sprintf("✅ cliente %s (uid: %s) registrado e logado.", s.username, s.uid)
	return nil
}

// buyPacks simula a compra de N pacotes em sequência
func (s *clientSession) buyPacks() {
	data := map[string]string{"UID": s.uid}
	for i := 0; i < numPacks; i++ {
		s.sendRequest("buyNewPack", data)
		s.receiveResponse() // ignora a resposta para manter o fluxo simples
	}
	s.resultChan <- fmt.Sprintf("🎁 cliente %s comprou %d pacotes.", s.username, numPacks)
}

// enterBattle simula a entrada na fila de batalha
func (s *clientSession) enterBattle() {
	s.sendRequest("battle", map[string]string{"UID": s.uid})

	// aguarda a resposta de entrada na fila ou timeout
	select {
	case <-time.After(10 * time.Second): // tempo de espera para entrar na fila
		s.resultChan <- fmt.Sprintf("❌ cliente %s (uid: %s) deu timeout na fila de batalha.", s.username, s.uid)
	case <-s.receiveGameStart():
		s.resultChan <- fmt.Sprintf("⚔️ cliente %s (uid: %s) entrou em uma batalha.", s.username, s.uid)
	}
}

// receiveGameStart aguarda a mensagem de início de jogo
func (s *clientSession) receiveGameStart() chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			msg, err := s.receiveResponse()
			if err != nil {
				return
			}
			if msg.Request == "gameStart" {
				close(done)
				return
			}
		}
	}()
	return done
}

// TestStressScenario é o teste principal que usa a concorrência do go
func TestStressScenario(t *testing.T) {
	// t.Parallel() permite que este teste rode em paralelo
	t.Parallel()

	// rodar com 'go test -race -v -run TestStressScenario' para encontrar race conditions

	var wg sync.WaitGroup
	resultChan := make(chan string, numClients)

	// inicia as goroutines para cada cliente
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// a conexão com o servidor pode falhar se o servidor não estiver pronto
			session, err := newClientSession(resultChan)
			if err != nil {
				t.Errorf("erro ao conectar cliente %d: %v", id, err)
				return
			}
			defer session.close()

			// executa o cenário de teste para este cliente
			if err := session.registerAndLogin(); err != nil {
				t.Errorf("erro no login do cliente %d: %v", id, err)
				return
			}

			session.buyPacks()
			session.enterBattle()

		}(i)
	}

	// aguarda todas as goroutines terminarem
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// imprime os resultados em tempo real
	for result := range resultChan {
		fmt.Println(result)
	}
}
