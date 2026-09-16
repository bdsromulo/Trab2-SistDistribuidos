package security

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// conteudoAssinado é o envelope sem o campo Signature: é exatamente isso que
// a chave privada assina e que a pública confere.
//
// Os nomes e as tags repetem os de events.Envelope de propósito. A ordem dos
// campos de uma struct no json.Marshal do Go é determinística, então produtor
// e consumidor chegam aos mesmos bytes.
type conteudoAssinado struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	Producer   string          `json:"producer"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// canonicalizar devolve os bytes assinados de um envelope, como string porque
// é essa a assinatura de signature.SignPayload.
func canonicalizar(env events.Envelope) (string, error) {
	bytes, err := json.Marshal(conteudoAssinado{
		EventID:    env.EventID,
		EventType:  env.EventType,
		Producer:   env.Producer,
		OccurredAt: env.OccurredAt,
		Data:       env.Data,
	})
	if err != nil {
		return "", fmt.Errorf("serializar conteúdo assinado: %w", err)
	}
	return string(bytes), nil
}

// RSASigner assina envelopes com a chave privada do próprio processo.
type RSASigner struct {
	privada *rsa.PrivateKey
}

// NovoSigner cria o assinante do processo a partir da chave privada dele.
func NovoSigner(privada *rsa.PrivateKey) RSASigner {
	return RSASigner{privada: privada}
}

// Sign canonicaliza o envelope, assina e grava a assinatura em base64.
//
// O recover existe porque signature.SignPayload sinaliza erro com log.Panicf:
// sem ele, um problema na chave derrubaria o processo inteiro.
func (s RSASigner) Sign(env *events.Envelope) (err error) {
	if s.privada == nil {
		return errors.New("signer sem chave privada")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("assinar %s: %v", env.EventType, r)
		}
	}()

	conteudo, err := canonicalizar(*env)
	if err != nil {
		return err
	}
	env.Signature = base64.StdEncoding.EncodeToString(signature.SignPayload(s.privada, conteudo))
	return nil
}

var _ Signer = RSASigner{}

// RSAVerifier confere assinaturas com as chaves públicas dos produtores,
// carregadas uma única vez na partida do processo.
type RSAVerifier struct {
	publicas map[string]*rsa.PublicKey
}

// NovoVerifier cria o verificador a partir do mapa produtor -> chave pública.
func NovoVerifier(publicas map[string]*rsa.PublicKey) RSAVerifier {
	return RSAVerifier{publicas: publicas}
}

// Verify devolve nil somente se a mensagem for autêntica e íntegra.
//
// A ordem das checagens vai do mais barato para o mais caro, e cada recusa
// usa o erro já declarado em security.go para o consumidor poder distinguir
// o motivo no log.
func (v RSAVerifier) Verify(env events.Envelope) error {
	if env.Signature == "" {
		return ErrSemAssinatura
	}

	publica, conhecido := v.publicas[env.Producer]
	if !conhecido {
		return fmt.Errorf("%w: %q", ErrProdutorDesconhecido, env.Producer)
	}

	// Impede um produtor legítimo de assinar um evento que não é dele.
	dono, existe := events.ProdutorDoEvento[env.EventType]
	if !existe || dono != env.Producer {
		return fmt.Errorf("%w: %q não publica %q", ErrEventoNaoPertenceAoProdutor, env.Producer, env.EventType)
	}

	bruta, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return fmt.Errorf("%w: base64 malformado", ErrAssinaturaInvalida)
	}

	conteudo, err := canonicalizar(env)
	if err != nil {
		return err
	}
	if !signature.VerifySignature(publica, conteudo, bruta) {
		return ErrAssinaturaInvalida
	}
	return nil
}

var _ Verifier = RSAVerifier{}
