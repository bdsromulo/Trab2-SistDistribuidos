package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/security"
)

// RabbitPublisher publica eventos assinados no RabbitMQ.
type RabbitPublisher struct {
	mu       sync.Mutex // um canal AMQP não pode ser usado por duas goroutines ao mesmo tempo
	ch       *amqp.Channel
	produtor string
	signer   security.Signer
}

// NovoPublisher cria um publisher para o produtor informado (events.Principal,
// events.Estoque...), que assina cada evento com o signer.
func NovoPublisher(ch *amqp.Channel, produtor string, signer security.Signer) *RabbitPublisher {
	return &RabbitPublisher{ch: ch, produtor: produtor, signer: signer}
}

// Publish monta o envelope, assina e publica na exchange do evento, usando
// o tipo como routing key. A mensagem é persistente.
func (p *RabbitPublisher) Publish(ctx context.Context, eventType string, data any) error {
	env, err := MontarEnvelope(p.produtor, eventType, data, p.signer)
	if err != nil {
		return err
	}
	corpo, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("serializar envelope: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	err = p.ch.PublishWithContext(ctx, ExchangeDoEvento(eventType), eventType, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    env.EventID,
		Timestamp:    env.OccurredAt,
		Body:         corpo,
	})
	if err != nil {
		return fmt.Errorf("publicar %s: %w", eventType, err)
	}
	return nil
}

// MontarEnvelope cria o envelope de um evento (id novo, horário em UTC) e o
// assina. Fica separado do Publish para poder ser testado sem RabbitMQ.
func MontarEnvelope(produtor, eventType string, data any, signer security.Signer) (events.Envelope, error) {
	dados, err := json.Marshal(data)
	if err != nil {
		return events.Envelope{}, fmt.Errorf("serializar dados de %s: %w", eventType, err)
	}
	env := events.Envelope{
		EventID:    uuid.NewString(),
		EventType:  eventType,
		Producer:   produtor,
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
		Data:       dados,
	}
	if err := signer.Sign(&env); err != nil {
		return events.Envelope{}, fmt.Errorf("assinar %s: %w", eventType, err)
	}
	return env, nil
}

var _ Publisher = (*RabbitPublisher)(nil)
