# Adaptador de assinatura RSA sobre `internal/signature`

Data: 2026-09-15
Autor: Rômulo
Situação: aprovado, pronto para plano de implementação

## 1. Problema

O Kauan entregou `internal/signature`: um pacote de primitivas RSA soltas
(`GenerateKeys`, `BuildPayloadHash`, `SignPayload`, `VerifySignature`,
`PersistKeys`, `ReadPubKeyFromFile`). O pacote não conhece o envelope do
projeto e não implementa as interfaces `security.Signer` / `security.Verifier`
combinadas no `PLANO_DE_ACAO.md`.

Resultado: nada liga as duas metades. O Principal continua rodando com
`security.FakeSigner{}` e `security.FakeVerifier{}`, ou seja, hoje o trabalho
publica e consome eventos **sem assinatura real**.

Este documento especifica a adaptação do lado do Rômulo: uma camada dentro de
`internal/security` que implementa as interfaces usando as primitivas do Kauan,
sem alterar o código dele.

## 2. Escopo

Entra:

- `internal/security/signature.go` — `RSASigner` e `RSAVerifier`.
- `internal/security/keys.go` — carregamento das chaves do disco.
- `cmd/principal/main.go` — troca dos Fakes pelas implementações reais.
- Testes dos itens acima.

Não entra (é da parte do Kauan, ou de fases posteriores):

- `internal/signature`, `internal/utils`, `cmd/gerar-chaves` — intocados.
- Os serviços ainda em stub (`estoque`, `pagamento`, `entrega`, `promocoes`,
  `c1`, `c2`, `adulterador`).
- Os `FakeSigner` / `FakeVerifier`, que permanecem para uso nos testes.

## 3. Conteúdo assinado

O `SignPayload` do Kauan recebe uma `string`. O contrato do plano é assinar o
envelope inteiro menos o campo `Signature`. O adaptador faz a ponte.

Struct interna, não exportada, com os cinco campos na ordem do `Envelope` e com
as mesmas tags JSON:

```go
type conteudoAssinado struct {
    EventID    string          `json:"event_id"`
    EventType  string          `json:"event_type"`
    Producer   string          `json:"producer"`
    OccurredAt time.Time       `json:"occurred_at"`
    Data       json.RawMessage `json:"data"`
}
```

`json.Marshal` dessa struct produz os bytes assinados. A verificação refaz
exatamente o mesmo marshal a partir do envelope recebido.

O round-trip é estável por três motivos, e cada um precisa continuar valendo:

1. `MontarEnvelope` já grava `OccurredAt` em UTC e truncado em milissegundo,
   sem leitura monotônica. O marshal/unmarshal de `time.Time` devolve o mesmo
   texto RFC 3339.
2. `Data` é `json.RawMessage`. O `json.Marshal` compacta os bytes dos dois
   lados de forma idêntica e não reordena chaves, então o que o produtor
   assinou é byte a byte o que o consumidor confere.
3. A ordem dos campos de uma struct Go no marshal é determinística.

## 4. Assinatura no envelope

`SignPayload` devolve `[]byte`; `Envelope.Signature` é `string`.

- Assinar: `base64.StdEncoding.EncodeToString` do resultado.
- Verificar: decode antes de chamar `VerifySignature`. Falha de decode é
  assinatura malformada e vira `ErrAssinaturaInvalida`.

## 5. Chaves

### Layout

Adotado o layout que o `PersistKeys` do Kauan já produz, lido a partir da raiz
do repositório (os serviços rodam com `go run ./cmd/<processo>`, então o
diretório corrente é a raiz):

| Arquivo | Caminho |
|---|---|
| Privada do próprio processo | `cmd/<processo>/key/private_key.pem` |
| Pública de cada produtor | `cmd/<processo>/<produtor>-pub/public_key.pem` |

Ambas em PEM PKCS#1 (`RSA PRIVATE KEY` / `RSA PUBLIC KEY`), que é o formato
que o `PersistKeys` e o `ReadPubKeyFromFile` já usam.

### Assinatura da função

```go
func CarregarChaves(raiz, processo string) (*rsa.PrivateKey, map[string]*rsa.PublicKey, error)
```

`raiz` é parâmetro em vez de constante para os testes poderem montar o layout
num `t.TempDir()`. Os `main` passam `"."`.

As públicas carregadas são as dos cinco produtores declarados em
`events.tipos` (`principal`, `estoque`, `pagamento`, `entrega`, `promocoes`),
obtidos como o conjunto dos valores distintos de `events.ProdutorDoEvento`.

Carrega-se as cinco em todo serviço, e não apenas as que aquele serviço
consome. O Principal, por exemplo, só precisa de `estoque`, `pagamento` e
`entrega`. Carregar todas é deliberado: é o conjunto que o `PersistKeys` já
distribui para cada pasta, mantém um único caminho de código para todos os
serviços e evita que adicionar um binding novo no futuro falhe por chave não
carregada.

Pública ausente é erro na partida, não erro por mensagem: é melhor descobrir
que falta uma chave ao subir o serviço do que descobrir quando o primeiro
evento daquele produtor for descartado.

### Contenção de panic

As funções do Kauan sinalizam erro com `log.Panicf`, e o `ReadPubKeyFromFile`
ainda descarta o erro do `os.ReadFile` — um `.pem` ausente vira panic com
mensagem que não diz qual arquivo faltou.

O carregador envolve cada chamada num helper que faz `recover` e converte o
panic em `error` nomeando o arquivo. Sem isso, chave faltando derruba o
Principal com stack trace em vez de uma mensagem acionável.

## 6. Verificação

`RSAVerifier.Verify` confere, nesta ordem, com o erro já declarado em
`security.go`:

| Ordem | Condição | Erro |
|---|---|---|
| 1 | `Signature` vazio | `ErrSemAssinatura` |
| 2 | `Producer` sem chave pública carregada | `ErrProdutorDesconhecido` |
| 3 | `EventType` não pertence ao `Producer` (via `events.ProdutorDoEvento`) | `ErrEventoNaoPertenceAoProdutor` |
| 4 | `VerifySignature` devolve `false`, ou o base64 não decodifica | `ErrAssinaturaInvalida` |

A checagem 3 é o que impede um produtor legítimo de falsificar evento de
outro: sem ela, o Pagamento poderia publicar um `pedido.enviado` assinado com
a própria chave e o Principal aceitaria.

Nada mais precisa mudar no caminho de consumo: `messaging.Processar` já chama
o `Verifier` antes do handler, e `messaging.Consumir` já loga e dá
`Nack(false, false)` — descarta sem reenfileirar. O `cmd/adulterador` vai
aparecer como `mensagem descartada: ... assinatura inválida`.

## 7. Partida do Principal

`cmd/principal/main.go` carrega as chaves antes de conectar. Se
`CarregarChaves` falhar, `log.Fatal` com a mensagem do erro.

Sem fallback para os Fakes. Um serviço que sobe com assinatura desligada
silenciosamente é pior que um que não sobe — e a defesa exige demonstrar a
diferença entre mensagem válida e adulterada.

## 8. Testes

Escritos antes do código.

`internal/security`:

- assinar e verificar o mesmo envelope: aceita.
- `Data` alterado depois de assinado: `ErrAssinaturaInvalida`.
- envelope assinado com a chave de outro produtor: `ErrAssinaturaInvalida`.
- `Signature` vazio: `ErrSemAssinatura`.
- `Producer` desconhecido: `ErrProdutorDesconhecido`.
- `EventType` que não pertence ao `Producer`: `ErrEventoNaoPertenceAoProdutor`.
- `Signature` com base64 inválido: `ErrAssinaturaInvalida`, sem panic.
- `CarregarChaves` lendo um layout montado em `t.TempDir()`.
- `CarregarChaves` com privada ausente e com pública ausente: erro nomeando o
  arquivo, sem panic.

Os testes geram as chaves em memória com `signature.GenerateKeys()`, sem tocar
o disco, exceto os dois do carregador.

## 9. Pendência para o Kauan

`PersistKeys` grava a privada em `key/private_key.pem` relativo ao diretório
corrente e faz `os.RemoveAll("key")` antes. Gerar os cinco pares em sequência
sobrescreve sempre o mesmo arquivo: sobra só o último, e ele fica em
`cmd/gerar-chaves/`, não no serviço dono.

Correção de uma linha, do lado dele: trocar o `"key"` fixo por
`cmd/<servico>/key`, usando o `servicoNome` que a função já recebe.

Segundo ponto, para quando eu chegar na Fase 3: a lista `microservicos` do
`PersistKeys` tem só os cinco microsserviços. O `c1` e o `c2` não recebem
pasta, e ambos precisam da pública do `promocoes` para verificar as promoções
que consomem. Não bloqueia nada agora — os dois ainda são stubs.

Até isso acontecer, o layout da seção 5 é montado à mão para testar o
Principal. O adaptador não depende dessas correções para funcionar.
