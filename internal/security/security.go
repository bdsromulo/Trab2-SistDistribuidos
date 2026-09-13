// Package security define a assinatura e a verificação dos eventos.
// As implementações reais (RSA) ficam em signature.go e keys.go.
package security

import (
	"errors"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
)

// Signer assina um envelope com a chave privada do produtor,
// preenchendo o campo Signature.
type Signer interface {
	Sign(env *events.Envelope) error
}

// Verifier confere a assinatura de um envelope com a chave pública do
// produtor declarado. Devolve nil somente se a mensagem for válida.
type Verifier interface {
	Verify(env events.Envelope) error
}

// Motivos para rejeitar uma mensagem.
var (
	ErrSemAssinatura               = errors.New("mensagem sem assinatura")
	ErrAssinaturaInvalida          = errors.New("assinatura inválida")
	ErrProdutorDesconhecido        = errors.New("produtor desconhecido")
	ErrEventoNaoPertenceAoProdutor = errors.New("evento não pertence ao produtor declarado")
)
