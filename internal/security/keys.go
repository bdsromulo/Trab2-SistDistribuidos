package security

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/events"
	"github.com/bdsromulo/Trab2-SistDistribuidos/internal/signature"
)

// Produtores devolve, em ordem, os produtores distintos que assinam eventos.
func Produtores() []string {
	vistos := map[string]bool{}
	lista := make([]string, 0, 5)
	for _, produtor := range events.ProdutorDoEvento {
		if !vistos[produtor] {
			vistos[produtor] = true
			lista = append(lista, produtor)
		}
	}
	sort.Strings(lista)
	return lista
}

// CaminhoPrivada e CaminhoPublica descrevem o layout que o cmd/gerar-chaves
// produz. Os serviços rodam com `go run ./cmd/<processo>`, então o diretório
// corrente é a raiz do repositório e a raiz passada aqui é ".".
func CaminhoPrivada(raiz, processo string) string {
	return filepath.Join(raiz, "cmd", processo, "key", "private_key.pem")
}

func CaminhoPublica(raiz, processo, produtor string) string {
	return filepath.Join(raiz, "cmd", processo, produtor+"-pub", "public_key.pem")
}

// CarregarChaves lê a chave privada do processo e as públicas de todos os
// produtores. Chave faltando é erro de partida, não erro por mensagem: é
// melhor o serviço não subir do que descartar eventos silenciosamente depois.
func CarregarChaves(raiz, processo string) (*rsa.PrivateKey, map[string]*rsa.PublicKey, error) {
	privada, err := lerPrivada(CaminhoPrivada(raiz, processo))
	if err != nil {
		return nil, nil, err
	}

	publicas := make(map[string]*rsa.PublicKey, 5)
	for _, produtor := range Produtores() {
		publica, err := lerPublica(CaminhoPublica(raiz, processo, produtor))
		if err != nil {
			return nil, nil, fmt.Errorf("chave pública de %q: %w", produtor, err)
		}
		publicas[produtor] = publica
	}
	return privada, publicas, nil
}

// lerPrivada existe porque internal/signature só sabe gravar a chave privada
// (PersistKeys); não há um leitor lá. O formato é o mesmo que ele grava:
// PEM PKCS#1.
func lerPrivada(caminho string) (*rsa.PrivateKey, error) {
	bytes, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", caminho, err)
	}
	bloco, _ := pem.Decode(bytes)
	if bloco == nil || bloco.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("%s não é um PEM 'RSA PRIVATE KEY'", caminho)
	}
	privada, err := x509.ParsePKCS1PrivateKey(bloco.Bytes)
	if err != nil {
		return nil, fmt.Errorf("interpretar %s: %w", caminho, err)
	}
	return privada, nil
}

// lerPublica delega para o leitor do Kauan, contendo o log.Panicf dele: sem o
// recover, um .pem ausente derrubaria o serviço com stack trace em vez de uma
// mensagem que diz qual arquivo faltou.
func lerPublica(caminho string) (publica *rsa.PublicKey, err error) {
	defer func() {
		if r := recover(); r != nil {
			publica, err = nil, fmt.Errorf("ler %s: %v", caminho, r)
		}
	}()
	if _, err := os.Stat(caminho); err != nil {
		return nil, fmt.Errorf("ler %s: %w", caminho, err)
	}
	return signature.ReadPubKeyFromFile(caminho), nil
}
