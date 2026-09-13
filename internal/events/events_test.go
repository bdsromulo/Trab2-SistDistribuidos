package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnvelopeIdaEVolta(t *testing.T) {
	dados, err := json.Marshal(PedidoExcluidoDados{PedidoID: "42", Motivo: MotivoUsuario})
	if err != nil {
		t.Fatal(err)
	}
	original := Envelope{
		EventID:    "id-1",
		EventType:  PedidoExcluido,
		Producer:   Principal,
		OccurredAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
		Data:       dados,
		Signature:  "abc",
	}

	bruto, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bruto), `"Signature":"abc"`) {
		t.Errorf("o campo da assinatura deve se chamar Signature: %s", bruto)
	}

	var lido Envelope
	if err := json.Unmarshal(bruto, &lido); err != nil {
		t.Fatal(err)
	}
	if lido.EventID != original.EventID || lido.EventType != original.EventType ||
		lido.Producer != original.Producer || !lido.OccurredAt.Equal(original.OccurredAt) ||
		string(lido.Data) != string(original.Data) || lido.Signature != original.Signature {
		t.Errorf("envelope mudou na ida e volta:\n antes: %+v\ndepois: %+v", original, lido)
	}

	var d PedidoExcluidoDados
	if err := lido.Decodificar(&d); err != nil {
		t.Fatal(err)
	}
	if d != (PedidoExcluidoDados{PedidoID: "42", Motivo: MotivoUsuario}) {
		t.Errorf("dados decodificados errados: %+v", d)
	}
}

func TestProdutorDoEventoCobreTodosOsTipos(t *testing.T) {
	tipos := []string{
		PedidoCriado, PedidoExcluido, PedidoEstoqueOK, EstoqueIndisponivel,
		PagamentoAprovado, PagamentoRecusado, PedidoEnviado,
		TipoPromocao("A"), TipoPromocao("B"), TipoPromocao("C"),
	}
	if len(ProdutorDoEvento) != len(tipos) {
		t.Errorf("esperava %d tipos, a tabela tem %d", len(tipos), len(ProdutorDoEvento))
	}
	for _, tipo := range tipos {
		if _, ok := ProdutorDoEvento[tipo]; !ok {
			t.Errorf("tipo %q sem produtor na tabela", tipo)
		}
	}
}
