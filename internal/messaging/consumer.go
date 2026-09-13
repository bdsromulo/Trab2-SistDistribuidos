package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/security"
)

// Handler processa um evento cuja assinatura já foi conferida.
// Se devolver erro, a mensagem é descartada.
type Handler func(ctx context.Context, env events.Envelope) error

// Consumir lê a fila uma mensagem por vez, com ack manual, até o ctx ser
// cancelado ou o canal fechar. Mensagem válida e processada recebe Ack;
// inválida ou com erro recebe Nack sem voltar para a fila (descartada).
func Consumir(ctx context.Context, ch *amqp.Channel, fila string, verifier security.Verifier, handler Handler) error {
	// Qos(1): o RabbitMQ só entrega a próxima mensagem depois do ack da atual.
	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("configurar Qos: %w", err)
	}
	entregas, err := ch.Consume(fila, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumir %s: %w", fila, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-entregas:
			if !ok {
				return errors.New("canal do RabbitMQ fechado")
			}
			if err := Processar(ctx, msg.Body, verifier, handler); err != nil {
				log.Printf("[%s] mensagem descartada: %v", fila, err)
				if err := msg.Nack(false, false); err != nil {
					return fmt.Errorf("nack: %w", err)
				}
				continue
			}
			if err := msg.Ack(false); err != nil {
				return fmt.Errorf("ack: %w", err)
			}
		}
	}
}

// Processar decodifica o envelope, confere a assinatura e só então chama o
// handler. Fica separado do Consumir para poder ser testado sem RabbitMQ.
func Processar(ctx context.Context, corpo []byte, verifier security.Verifier, handler Handler) error {
	var env events.Envelope
	if err := json.Unmarshal(corpo, &env); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	if err := verifier.Verify(env); err != nil {
		return fmt.Errorf("%s de %q: %w", env.EventType, env.Producer, err)
	}
	if err := handler(ctx, env); err != nil {
		return fmt.Errorf("erro ao processar %s: %w", env.EventType, err)
	}
	return nil
}
