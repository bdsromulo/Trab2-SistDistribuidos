package security

import (
	"errors"
	"testing"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

func TestFakeSigner(t *testing.T) {
	var env events.Envelope
	if err := (FakeSigner{}).Sign(&env); err != nil {
		t.Fatal(err)
	}
	if env.Signature != AssinaturaFalsa {
		t.Errorf("Signature = %q, esperava %q", env.Signature, AssinaturaFalsa)
	}
}

func TestFakeVerifier(t *testing.T) {
	if err := (FakeVerifier{}).Verify(events.Envelope{}); err != nil {
		t.Errorf("o valor zero deveria aceitar a mensagem, veio %v", err)
	}
	rejeita := FakeVerifier{Err: ErrAssinaturaInvalida}
	if err := rejeita.Verify(events.Envelope{}); !errors.Is(err, ErrAssinaturaInvalida) {
		t.Errorf("esperava ErrAssinaturaInvalida, veio %v", err)
	}
}
