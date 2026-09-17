package security

import (
	"crypto/rsa"
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

func produtorConhecido(produtor string) bool {
	for _, p := range Produtores() {
		if p == produtor {
			return true
		}
	}
	return false
}

// CaminhoPublica é onde o signature.PersistKeys do produtor deixa a chave
// pública dele, dentro da pasta do processo que consome.
func CaminhoPublica(pasta, produtor string) string {
	return filepath.Join(pasta, produtor+"-pub", "public_key.pem")
}

// lerPublica delega para o leitor do Kauan, contendo o log.Panicf dele: sem o
// recover, um .pem ausente derrubaria o serviço em vez de só recusar a
// mensagem.
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
