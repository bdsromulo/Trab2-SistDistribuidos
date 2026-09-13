package messaging

import (
	"context"
	"sync"
)

// Publicado é um evento registrado pelo FakePublisher.
type Publicado struct {
	EventType string
	Data      any
}

// FakePublisher não fala com o RabbitMQ: só guarda o que foi publicado.
// Serve para os testes e para escrever os serviços antes do publisher real.
type FakePublisher struct {
	mu         sync.Mutex
	publicados []Publicado
}

func (f *FakePublisher) Publish(_ context.Context, eventType string, data any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.publicados = append(f.publicados, Publicado{EventType: eventType, Data: data})
	return nil
}

// Publicados devolve uma cópia dos eventos publicados, na ordem.
func (f *FakePublisher) Publicados() []Publicado {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Publicado(nil), f.publicados...)
}

// Tipos devolve só os tipos dos eventos publicados, na ordem.
func (f *FakePublisher) Tipos() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	tipos := make([]string, len(f.publicados))
	for i, p := range f.publicados {
		tipos[i] = p.EventType
	}
	return tipos
}

var _ Publisher = (*FakePublisher)(nil)
