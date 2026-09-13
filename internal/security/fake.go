package security

import "github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"

// Implementações falsas, usadas nos testes e enquanto a assinatura real não existe.

// AssinaturaFalsa é o valor gravado pelo FakeSigner.
const AssinaturaFalsa = "assinatura-falsa"

// FakeSigner não assina de verdade: só grava AssinaturaFalsa em Signature.
type FakeSigner struct{}

func (FakeSigner) Sign(env *events.Envelope) error {
	env.Signature = AssinaturaFalsa
	return nil
}

// FakeVerifier devolve sempre Err. O valor zero aceita toda mensagem;
// com Err preenchido, simula uma mensagem rejeitada.
type FakeVerifier struct {
	Err error
}

func (f FakeVerifier) Verify(events.Envelope) error {
	return f.Err
}

var (
	_ Signer   = FakeSigner{}
	_ Verifier = FakeVerifier{}
)
