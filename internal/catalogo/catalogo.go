// Package catalogo lê o catálogo de produtos (data/catalogo.json).
// O catálogo é somente leitura e não tem quantidades: quem controla
// as quantidades é o serviço Estoque.
package catalogo

import (
	"encoding/json"
	"fmt"
	"os"
)

// CaminhoPadrao é o local do catálogo visto de dentro de cmd/<ms>, de onde
// os serviços rodam.
const CaminhoPadrao = "../../data/catalogo.json"

// Produto é um item do catálogo.
type Produto struct {
	ID        string  `json:"id"`
	Nome      string  `json:"nome"`
	Categoria string  `json:"categoria"` // A, B ou C
	Preco     float64 `json:"preco"`
}

// Carregar lê o catálogo do arquivo informado.
func Carregar(caminho string) ([]Produto, error) {
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		return nil, fmt.Errorf("lendo catálogo: %w", err)
	}
	var produtos []Produto
	if err := json.Unmarshal(conteudo, &produtos); err != nil {
		return nil, fmt.Errorf("decodificando catálogo %s: %w", caminho, err)
	}
	return produtos, nil
}
