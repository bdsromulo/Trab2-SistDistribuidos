// Package events define o contrato entre os serviços: o envelope que viaja
// pelo RabbitMQ, os tipos de evento (routing keys) e os dados de cada evento.
package events

import (
	"encoding/json"
	"time"
)

// Envelope é a mensagem publicada no RabbitMQ.
//
// A assinatura cobre todos os campos, exceto o próprio Signature:
// EventID, EventType, Producer, OccurredAt e Data.
// OccurredAt deve ser preenchido em UTC.
type Envelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Producer   string          `json:"producer"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
	Signature  string          `json:"Signature"`
}

// Decodificar converte o campo Data para a struct de dados do evento.
func (e Envelope) Decodificar(destino any) error {
	return json.Unmarshal(e.Data, destino)
}
