package server

import "encoding/json"

// mensagem padrão para conversa cliente-servidor
type Message struct {
	Request string          `json:"tag"`
	UID     string          `json:"uid"`
	Data    json.RawMessage `json:"data"`
}
