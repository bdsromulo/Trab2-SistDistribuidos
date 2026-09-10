# Trabalho 2 - Sistemas Distribuídos

Backend distribuído de um sistema de e-commerce, desenvolvido em Ruby, com microsserviços orientados a eventos, RabbitMQ e assinatura digital assimétrica.

> Status: documentação e planejamento iniciais. A implementação e as instruções definitivas de execução ainda serão adicionadas.

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

- [Tutorial oficial do RabbitMQ para Ruby](https://www.rabbitmq.com/tutorials/tutorial-one-ruby)

Para este trabalho, devem ser estudados principalmente os tutoriais 1, 2, 3 e 4. O tutorial 3 ajuda a compreender publish/subscribe e bindings, mas seu exemplo usa `fanout`, que não pode ser utilizada na solução. A implementação deverá usar `direct` para os eventos do e-commerce e `topic` para promoções.

## Execução

A definir após a criação do código.

## Estrutura do projeto

A definir após a validação e criação do esqueleto inicial. A proposta está descrita em [`BRIEFING.md`](BRIEFING.md).

## Repositório

[bdsromulo/Trab2-SistDistribuidos](https://github.com/bdsromulo/Trab2-SistDistribuidos)

