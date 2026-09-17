# Trabalho 2 - Sistemas Distribuídos

Backend distribuído de um sistema de e-commerce, desenvolvido em Go, com microsserviços orientados a eventos, RabbitMQ e assinatura digital assimétrica.

## Objetivo

O sistema gerencia pedidos, estoque, pagamentos, entregas e promoções por meio de processos independentes. Não há chamadas diretas entre os processos: toda comunicação ocorre por eventos publicados e consumidos no RabbitMQ.

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

Para este trabalho, devem ser estudados principalmente os tutoriais 1, 2, 3 e 4 na versão para Go. O tutorial 3 ajuda a compreender publish/subscribe e bindings, mas seu exemplo usa `fanout`, que não pode ser utilizada na solução. A implementação usa `direct` para os eventos do e-commerce e `topic` para promoções.

## Execução

Todos os comandos rodam **a partir da raiz do repositório**: os caminhos das chaves e de `data/` são relativos a ela.

### 1. RabbitMQ

```bash
docker compose up -d
```

Painel em http://localhost:15672 (usuário e senha `ecommerce`).

### 2. Chaves

Uma única vez, antes de subir os serviços:

```bash
go run ./cmd/gerar-chaves
```

Ele gera um par RSA-2048 para cada microsserviço e distribui as públicas:

| Arquivo | Caminho |
|---|---|
| Par do próprio serviço | `cmd/<ms>/keys/private_key.pem` e `cmd/<ms>/keys/public_key.pem` |
| Pública de cada outro serviço | `cmd/<ms>/<produtor>-pub/public_key.pem` |

Nenhum `.pem` é versionado (`.gitignore`). Rodar o `gerar-chaves` de novo troca todos os pares, então os serviços precisam ser reiniciados depois.

### 3. Serviços

Um terminal por processo, nesta ordem:

```bash
go run ./cmd/estoque
go run ./cmd/pagamento
go run ./cmd/entrega
go run ./cmd/c1
go run ./cmd/c2
go run ./cmd/promocoes
go run ./cmd/principal
```

Cada consumidor cria a própria fila quando sobe. Uma mensagem publicada antes de a fila existir não tem para onde ir e se perde: por isso C1 e C2 sobem antes do Promoções, e os consumidores do fluxo de pedidos antes do Principal.

### 4. Demonstração do descarte

Com os serviços no ar:

```bash
go run ./cmd/adulterador
```

Ele publica, para `pedido.criado` e para `pagamento.aprovado`, três mensagens inválidas: sem assinatura, assinada com uma chave desconhecida e assinada pelo produtor verdadeiro com o `data` alterado depois. O Estoque, a Entrega e o Principal mostram "Assinatura inválida ... Evento descartado" para cada uma.

## Estrutura do projeto

```text
.
|-- cmd/                    um processo por pasta (go run ./cmd/<nome>)
|   |-- principal/          menu no terminal, pedidos e status
|   |-- estoque/            reserva e devolução de itens
|   |-- pagamento/          aprova ou recusa pagamentos
|   |-- entrega/            emite a nota fiscal e envia o pedido
|   |-- promocoes/          publica promoções aleatórias
|   |-- c1/                 consumidor de promoções A e B
|   |-- c2/                 consumidor de todas as promoções
|   |-- gerar-chaves/       gera e distribui as chaves RSA dos serviços
|   `-- adulterador/        publica mensagens inválidas (demonstração)
|-- internal/
|   |-- events/             contrato: envelope, tipos de evento e dados
|   |-- signature/          chaves, assinatura e verificação (RSA)
|   |-- utils/              FailOnError
|   `-- catalogo/           leitura do catálogo de produtos
|-- data/catalogo.json      catálogo somente leitura (sem quantidades)
|-- data/estoque.json       estoque inicial do Estoque
`-- go.mod
```

Cada serviço fala com o RabbitMQ diretamente pelo `amqp091-go`: declara a exchange, a própria fila e os bindings, consome e publica.

## Regras combinadas

**Eventos**

- O envelope tem `event_id`, `event_type`, `producer`, `occurred_at` (UTC), `data` e `Signature` (ver `internal/events`).
- O `event_type` é sempre igual à routing key.
- Cada tipo de evento tem um único produtor (tabela `events.ProdutorDoEvento`, igual à Figura 1 do enunciado).
- Motivos de `pedido.excluido`: `usuario`, `falta_estoque` e `pagamento_recusado`. Ele é publicado no máximo uma vez por pedido, e um pedido já enviado não pode ser excluído.

**Assinatura** (`internal/signature`)

- O produtor assina o campo `data` do envelope: hash SHA-256 dos bytes do JSON, assinado com a chave privada dele (RSA-2048, PKCS#1 v1.5) e gravado em base64 em `Signature` (`SignPayload`).
- O consumidor carrega na partida a chave pública de cada produtor de quem recebe eventos e confere com `VerifySignature` antes de processar. Mensagem sem assinatura, com assinatura inválida ou com o `data` alterado é descartada (recebe ack e não é processada).
- C1 e C2 não são microsserviços e não recebem cópia da pública do Promoções: leem `cmd/promocoes/keys/public_key.pem`.

**RabbitMQ**

- Exchanges `eCommerce` (direct) e `Promocoes` (topic, sem acento). Não há exchange fanout.
- Cada consumidor declara a própria fila e os próprios bindings. Filas duráveis e mensagens persistentes, com ack manual.

| Fila | Routing keys |
|---|---|
| `fila.principal` | pedido.estoque_ok, estoque.indisponivel, pagamento.aprovado, pagamento.recusado, pedido.enviado |
| `fila.estoque` | pedido.criado, pedido.excluido |
| `fila.pagamento` | pedido.estoque_ok |
| `fila.entrega` | pagamento.aprovado |
| `fila.C1` | promocao.categoria.A, promocao.categoria.B |
| `fila.C2` | promocao.categoria.* |

**Serviços**

- Pedidos (Principal) e estoque (Estoque) ficam em memória. O catálogo não tem quantidades; o estoque inicial vem de `data/estoque.json`.
- O status de um pedido nunca volta, e eventos de pedidos já excluídos são ignorados.
- Pagamento: aprovação aleatória em cerca de 80% dos casos.
- Entrega: atraso de 1 a 3 s, número de nota fiscal e código de rastreio aleatórios.
- Promoções: a cada 5 a 10 s, um produto do catálogo com 5% a 50% de desconto.

## Repositório

[bdsromulo/Trab2-SistDistribuidos](https://github.com/bdsromulo/Trab2-SistDistribuidos)
