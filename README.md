# Trabalho 2 - Sistemas Distribuídos

Backend distribuído de um sistema de e-commerce, desenvolvido em Go, com microsserviços orientados a eventos, RabbitMQ e assinatura digital assimétrica.

> Status: base comum criada (módulo Go, contrato dos eventos e implementações provisórias). Os serviços ainda estão sendo implementados.

## Objetivo

O sistema deverá gerenciar pedidos, estoque, pagamentos, entregas e promoções por meio de processos independentes. Não serão permitidas chamadas diretas entre os processos: toda comunicação deverá ocorrer por eventos publicados e consumidos no RabbitMQ.

O trabalho prevê:

- cinco microsserviços: Principal, Estoque, Pagamento, Entrega e Promoções;
- dois consumidores independentes de promoções (C1 e C2);
- uma exchange `eCommerce`, do tipo `direct`;
- uma exchange `Promocoes`, do tipo `topic`;
- uma fila própria para cada consumidor;
- assinatura digital de todo evento publicado e validação antes do processamento;
- interface de terminal no microsserviço Principal.

O uso de exchange do tipo `fanout` é proibido.

## Material de referência

- [Tutorial oficial do RabbitMQ para Go](https://www.rabbitmq.com/tutorials/tutorial-one-go)

Para este trabalho, devem ser estudados principalmente os tutoriais 1, 2, 3 e 4 na versão para Go. O tutorial 3 ajuda a compreender publish/subscribe e bindings, mas seu exemplo usa `fanout`, que não pode ser utilizada na solução. A implementação deverá usar `direct` para os eventos do e-commerce e `topic` para promoções.

## Execução

### Chaves

Cada processo assina o que publica e confere o que consome. Antes de subir os
serviços, é preciso ter as chaves no lugar:

| Arquivo | Caminho |
|---|---|
| Privada do próprio processo | `cmd/<processo>/key/private_key.pem` |
| Pública de cada produtor | `cmd/<processo>/<produtor>-pub/public_key.pem` |

Os produtores são `principal`, `estoque`, `pagamento`, `entrega` e `promocoes`.
As privadas não são versionadas (`.gitignore`).

Um serviço que não encontrar as chaves **não sobe**, e diz qual arquivo faltou.
É proposital: assinatura desligada em silêncio esconderia justamente o que o
trabalho precisa demonstrar.

> O `cmd/gerar-chaves` ainda não produz esse layout — hoje ele gera um único
> par e distribui a pública sob o nome `gerar-chaves-pub`. Enquanto isso não
> for ajustado, o layout acima é montado à mão para testar.

### Serviços

A definir conforme os processos forem ficando prontos. Um terminal por
processo, a partir da raiz do repositório:

```bash
go run ./cmd/principal
```

## Estrutura do projeto

```text
.
|-- cmd/                    um processo por pasta (go run ./cmd/<nome>)
|   |-- principal/          menu no terminal, pedidos e status
|   |-- estoque/
|   |-- pagamento/
|   |-- entrega/
|   |-- promocoes/
|   |-- c1/                 consumidor de promoções A e B
|   |-- c2/                 consumidor de todas as promoções
|   |-- gerar-chaves/       gera as chaves RSA de cada produtor
|   `-- adulterador/        publica mensagens inválidas (demonstração)
|-- internal/
|   |-- events/             contrato: envelope, tipos de evento e dados
|   |-- security/           assinatura e verificação (Signer, Verifier)
|   |-- messaging/          RabbitMQ: nomes, publicação e consumo (Publisher)
|   `-- catalogo/           leitura do catálogo de produtos
|-- data/catalogo.json      catálogo somente leitura (sem quantidades)
|-- .env.example            modelo do .env com a conexão do RabbitMQ
`-- go.mod
```

Enquanto a assinatura e o publicador reais não existem, os serviços usam as implementações falsas `security.FakeSigner`, `security.FakeVerifier` e `messaging.FakePublisher`.

## Regras combinadas

**Eventos**

- O envelope tem `event_id`, `event_type`, `producer`, `occurred_at` (UTC), `data` e `Signature` (ver `internal/events`).
- O `event_type` é sempre igual à routing key.
- Cada tipo de evento tem um único produtor (tabela `events.ProdutorDoEvento`, igual à Figura 1 do enunciado).
- Motivos de `pedido.excluido`: `usuario`, `falta_estoque` e `pagamento_recusado`. Ele é publicado no máximo uma vez por pedido, e um pedido já enviado não pode ser excluído.

**Assinatura**

- A assinatura cobre todos os campos do envelope, exceto `Signature`: hash SHA-256, assinado com a chave privada do produtor (RSA-2048, PKCS#1 v1.5) e gravado em base64.
- O consumidor escolhe a chave pública pelo campo `producer`. Mensagem sem assinatura, adulterada, de produtor desconhecido ou com evento que não pertence ao produtor é descartada.
- As chaves privadas nunca vão para o Git; cada um gera as suas na própria máquina com `go run ./cmd/gerar-chaves`.

**RabbitMQ**

- Exchanges `eCommerce` (direct) e `Promocoes` (topic, sem acento). Não há exchange fanout.
- Cada consumidor declara a própria fila e os próprios bindings. Filas duráveis e mensagens persistentes.
- Uma mensagem por vez, com ack manual depois de processar. Mensagem inválida ou com erro é descartada sem voltar para a fila.

| Fila | Routing keys |
|---|---|
| `fila.principal` | pedido.estoque_ok, estoque.indisponivel, pagamento.aprovado, pagamento.recusado, pedido.enviado |
| `fila.estoque` | pedido.criado, pedido.excluido |
| `fila.pagamento` | pedido.estoque_ok |
| `fila.entrega` | pagamento.aprovado |
| `fila.C1` | promocao.categoria.A, promocao.categoria.B |
| `fila.C2` | promocao.categoria.* |

**Serviços**

- Pedidos (Principal) e estoque (Estoque) ficam em memória. O catálogo não tem quantidades; o estoque inicial vem de um JSON próprio do Estoque.
- O status de um pedido nunca volta, e eventos de pedidos já excluídos são ignorados.
- Pagamento: 80% de aprovação, com atraso de 1 a 3 s. Entrega: atraso de 1 a 3 s.
- Promoções: a cada 5 a 10 s, um produto do catálogo com 5% a 50% de desconto.

## Repositório

[bdsromulo/Trab2-SistDistribuidos](https://github.com/bdsromulo/Trab2-SistDistribuidos)

