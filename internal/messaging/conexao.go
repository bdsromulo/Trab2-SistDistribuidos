package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Conectar abre a conexão com o RabbitMQ e um canal sobre ela.
// Quem chama deve fechar os dois ao terminar (defer conn.Close()).
func Conectar(cfg Config) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(cfg.URL())
	if err != nil {
		return nil, nil, fmt.Errorf("conectar ao RabbitMQ em %s:%s: %w", cfg.Host, cfg.Porta, err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("abrir canal: %w", err)
	}
	return conn, ch, nil
}

// DeclararExchanges cria as duas exchanges do trabalho, duráveis.
// Declarar é idempotente: todo processo pode chamar sem problema.
func DeclararExchanges(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeECommerce, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar exchange %s: %w", ExchangeECommerce, err)
	}
	if err := ch.ExchangeDeclare(ExchangePromocoes, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar exchange %s: %w", ExchangePromocoes, err)
	}
	return nil
}

// DeclararFila cria a fila (durável) e liga suas binding keys à exchange,
// conforme BindingsDaFila. Cada consumidor declara a própria fila.
func DeclararFila(ch *amqp.Channel, fila string) error {
	b, ok := BindingsDaFila[fila]
	if !ok {
		return fmt.Errorf("fila desconhecida: %q", fila)
	}
	if _, err := ch.QueueDeclare(fila, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar fila %s: %w", fila, err)
	}
	for _, chave := range b.Chaves {
		if err := ch.QueueBind(fila, chave, b.Exchange, false, nil); err != nil {
			return fmt.Errorf("ligar %s a %s com %q: %w", fila, b.Exchange, chave, err)
		}
	}
	return nil
}
