# Briefing do Trabalho 2

## 1. Visão geral

O trabalho pede o backend distribuído de um e-commerce baseado em microsserviços e em arquitetura orientada a eventos. Os processos devem ser independentes e desacoplados. Eles não podem chamar uns aos outros diretamente: o RabbitMQ será o único meio de comunicação.

A avaliação vale 2,0 pontos e a defesa da aplicação é obrigatória. O desenvolvimento deve ser realizado em dupla.

## 2. Componentes obrigatórios

### Microsserviço Principal

É a interface de terminal do usuário. Deve permitir:

- visualizar produtos;
- realizar pedidos;
- excluir pedidos;
- consultar pedidos e seus status.

Publica:

- `pedido.criado` ao cadastrar um pedido;
- `pedido.excluido` quando o usuário excluir um pedido, quando faltar estoque ou quando o pagamento for recusado.

Consome:

- `pagamento.aprovado`;
- `pagamento.recusado`;
- `pedido.enviado`;
- `pedido.estoque_ok`;
- `estoque.indisponivel`.

Ao consumir esses eventos, atualiza o status local do pedido correspondente.

### Microsserviço Estoque

Mantém os produtos e suas quantidades.

Consome:

- `pedido.criado` para conferir e reservar/baixar os itens;
- `pedido.excluido` para devolver itens previamente reservados.

Publica:

- `pedido.estoque_ok` se todos os itens estiverem disponíveis;
- `estoque.indisponivel` caso algum item não possa ser atendido.

### Microsserviço Pagamento

Consome `pedido.estoque_ok`, simula aleatoriamente o resultado do pagamento e publica:

- `pagamento.aprovado`; ou
- `pagamento.recusado`.

### Microsserviço Entrega

Consome `pagamento.aprovado`, simula emissão da nota e preparação da entrega e publica `pedido.enviado`.

### Microsserviço Promoções

Gera promoções aleatórias e as publica com routing keys hierárquicas, por exemplo:

- `promocao.categoria.A`;
- `promocao.categoria.B`;
- `promocao.categoria.C`.

### Consumidores de promoções

São dois processos separados dos cinco microsserviços:

- C1 recebe somente categorias A e B;
- C2 recebe todas as categorias.

Cada consumidor deve possuir sua própria fila e comunicar-se exclusivamente com o RabbitMQ.

## 3. Topologia do RabbitMQ

### Exchange `eCommerce`

- Tipo: `direct`.
- Uso: fluxo de pedidos, estoque, pagamentos e entregas.
- As filas devem ser vinculadas às routing keys exatas que cada processo consome.

### Exchange `Promocoes`

- Tipo: `topic`.
- Uso: divulgação de promoções por categoria.
- C1 pode usar bindings exatos para A e B.
- C2 pode usar o padrão `promocao.categoria.*` para receber todas as categorias de um nível.

Não deve existir exchange `fanout`. Cada consumidor precisa de sua própria fila; compartilhar uma fila faria os consumidores competirem pelas mensagens em vez de cada um receber os eventos de seu interesse.

## 4. Criptografia exigida

Todo processo que publica um evento deverá:

1. serializar o conteúdo do evento de maneira determinística;
2. gerar o hash do conteúdo;
3. assinar o hash com sua chave privada;
4. incluir a assinatura no campo `Signature` do envelope.

Todo processo consumidor deverá:

1. identificar o produtor do evento;
2. carregar a chave pública desse produtor;
3. verificar a assinatura e a integridade do conteúdo;
4. processar a mensagem somente se a assinatura for válida.

Mensagens com assinatura inválida devem ser descartadas. Cada microsserviço deverá possuir cópias das chaves públicas dos demais produtores de eventos. As chaves privadas nunca devem ser versionadas.

Uma proposta de envelope JSON é:

```json
{
  "event_id": "uuid",
  "event_type": "pedido.criado",
  "producer": "principal",
  "occurred_at": "2026-09-10T12:00:00Z",
  "data": {},
  "Signature": "assinatura-em-base64"
}
```

O formato definitivo deve deixar explícito quais campos entram na assinatura. Em Go, a implementação pode usar os pacotes da biblioteca padrão `crypto/sha256`, `crypto/rsa`, `crypto/rand`, `crypto/x509`, `encoding/pem` e `encoding/base64`.

## 5. Como Go será utilizado

Cada microsserviço e cada consumidor será um processo Go independente. O pacote `github.com/rabbitmq/amqp091-go` fará a conexão AMQP com o RabbitMQ. A biblioteca padrão cuidará de JSON, horários e criptografia; para UUIDs, pode ser utilizado `github.com/google/uuid`.

Dependências previstas:

```text
github.com/rabbitmq/amqp091-go
github.com/google/uuid
github.com/joho/godotenv
```

`github.com/joho/godotenv` é opcional, mas ajuda a manter endereço, usuário e senha do RabbitMQ fora do código. Os testes podem usar o pacote padrão `testing` e ser executados com `go test ./...`.

Conceitos de Go que serão necessários para a defesa:

- inicializar o módulo com `go mod init` e instalar dependências com `go get`;
- executar processos com `go run ./cmd/nome-do-processo`;
- packages, structs, interfaces, maps e slices;
- leitura e geração de JSON com `encoding/json`;
- tratamento explícito de erros;
- goroutines e channels quando forem necessários para consumo concorrente;
- separação entre código compartilhado e regras de cada processo.

## 6. Esqueleto previsto

> A estrutura adotada está no [README](README.md#estrutura-do-projeto); este esqueleto ficou como registro do planejamento.

```text
.
|-- AGENTS.md
|-- README.md
|-- BRIEFING.md
|-- go.mod
|-- go.sum
|-- .env.example
|-- .gitignore
|-- docker-compose.yml
|-- cmd/
|   |-- principal/main.go
|   |-- estoque/main.go
|   |-- pagamento/main.go
|   |-- entrega/main.go
|   |-- promocoes/main.go
|   |-- promocoes-c1/main.go
|   `-- promocoes-c2/main.go
|-- internal/
|   |-- messaging/
|   |   |-- connection.go
|   |   |-- publisher.go
|   |   `-- consumer.go
|   |-- events/
|   |   `-- envelope.go
|   `-- security/
|       |-- signature.go
|       `-- keys.go
|-- services/
|   |-- principal/
|   |-- estoque/
|   |-- pagamento/
|   |-- entrega/
|   `-- promocoes/
|-- keys/public/
|-- scripts/
|   `-- generate_keys.go
`-- internal/.../*_test.go
```

O esqueleto é uma previsão e poderá ser simplificado durante a implementação. Arquivos de estado local (pedidos e estoque) podem ser mantidos inicialmente em JSON, desde que cada serviço seja dono de seus próprios dados e que isso seja explicado na defesa.

## 7. Decisões que precisam ser tomadas antes da implementação

> Decisões tomadas: ver a seção [Regras combinadas](README.md#regras-combinadas) do README.

- persistência apenas em memória ou em arquivos JSON locais;
- nomes definitivos das filas;
- formato canônico do conteúdo assinado;
- comportamento de acknowledgements em mensagens inválidas ou com erro;
- política de durabilidade das filas e mensagens;
- atraso e frequência da geração de promoções;
- probabilidade usada na simulação de pagamento.

## 8. Próximos passos recomendados

1. Instalar Go, VS Code e Docker Desktop e validar suas versões.
2. Subir RabbitMQ com a interface de gerenciamento e acessar o painel local.
3. Criar `go.mod`, `.gitignore`, `.env.example` e `docker-compose.yml`.
4. Fazer um exercício mínimo do tutorial oficial Go: um produtor e um consumidor.
5. Implementar e testar o envelope assinado isoladamente.
6. Criar a conexão e as declarações de exchanges, filas e bindings.
7. Implementar o fluxo feliz: pedido criado -> estoque OK -> pagamento aprovado -> pedido enviado.
8. Implementar falta de estoque, pagamento recusado, exclusão e devolução de reserva.
9. Implementar Promoções, C1 e C2.
10. Adicionar testes, instruções completas de execução e um roteiro de demonstração/defesa.

## 9. Pontos de atenção para a defesa

- explicar por que não há chamadas diretas entre processos;
- diferenciar exchange, fila, binding key e routing key;
- mostrar a diferença entre `direct`, `topic` e o `fanout` proibido;
- explicar por que cada consumidor possui fila própria;
- percorrer um pedido completo pelas routing keys;
- demonstrar o descarte de uma mensagem adulterada;
- explicar onde ficam as chaves privadas e públicas;
- iniciar os processos separadamente pelo terminal integrado do VS Code.

## 10. Fontes fornecidas

- Enunciado: `Trab1_MOM_ecommerce_2026_2.pdf`.
- Complemento do Classroom: `Especificação Classroom.txt`.
- [Tutorial oficial do RabbitMQ para Go](https://www.rabbitmq.com/tutorials/tutorial-one-go).

