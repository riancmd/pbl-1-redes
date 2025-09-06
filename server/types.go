package server

import "encoding/json"

// mensagem padrão para conversa cliente-servidor
type Request struct {
	Tag  string          `json:"tag"`
	Data json.RawMessage `json:"data"`
}
