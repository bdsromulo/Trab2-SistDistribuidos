// Package events define o contrato entre os serviços: o envelope que viaja
// pelo RabbitMQ, os tipos de evento (routing keys) e os dados de cada evento.
package events

import (
	"encoding/json"
	"time"
)

// Envelope é a mensagem publicada no RabbitMQ.
//
// Signature é a assinatura do campo Data (os bytes do JSON, como publicados),
// feita com signature.SignPayload e gravada em base64. Os outros campos não
// entram na assinatura. OccurredAt deve ser preenchido em UTC.
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
