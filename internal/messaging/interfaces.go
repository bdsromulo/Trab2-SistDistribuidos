package messaging

import "context"

// Publisher publica um evento: monta o envelope, assina e envia para a
// exchange certa, usando o tipo do evento como routing key. O produtor e a
// chave privada são definidos quando o publisher é criado.
type Publisher interface {
	Publish(ctx context.Context, eventType string, data any) error
}
