package catalogo

import (
	"testing"
)

// internal/catalogo fica a dois níveis da raiz, como cmd/<ms>: o
// CaminhoPadrao serve sem ajuste.
func TestCatalogoDoProjeto(t *testing.T) {
	produtos, err := Carregar(CaminhoPadrao)
	if err != nil {
		t.Fatal(err)
	}
	if len(produtos) != 9 {
		t.Fatalf("esperava 9 produtos, vieram %d", len(produtos))
	}

	ids := map[string]bool{}
	porCategoria := map[string]int{}
	for _, p := range produtos {
		if ids[p.ID] {
			t.Errorf("id repetido: %s", p.ID)
		}
		ids[p.ID] = true
		if p.Nome == "" {
			t.Errorf("produto %s sem nome", p.ID)
		}
		if p.Preco <= 0 {
			t.Errorf("produto %s com preço %.2f", p.ID, p.Preco)
		}
		porCategoria[p.Categoria]++
	}
	for _, c := range []string{"A", "B", "C"} {
		if porCategoria[c] != 3 {
			t.Errorf("categoria %s tem %d produtos, esperava 3", c, porCategoria[c])
		}
	}
	if len(porCategoria) != 3 {
		t.Errorf("há categorias fora de A, B e C: %v", porCategoria)
	}
}

func TestCarregarArquivoInexistente(t *testing.T) {
	if _, err := Carregar("nao-existe.json"); err == nil {
		t.Error("esperava erro para arquivo inexistente")
	}
}
