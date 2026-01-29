---
type: doc
name: data-flow
description: How data moves through the system and external integrations
category: data-flow
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Data Flow & Integrations

O fluxo de dados no `whatsmeow` é centrado no processamento reativo de mensagens binárias recebidas via WebSocket e na serialização de comandos de saída.

## Module Dependencies
- **Root (`client.go`)** → Depende de `binary/`, `util/`, `store/` e `proto/`.
- **`binary/`** → Depende de `proto/` para mapear frames binários em tipos estruturados.
- **`store/`** → Independente; provê a interface de dados para o `client`.

## Service Layer
- **`Client`**: Gerencia o ciclo de vida da conexão e o roteamento de mensagens ([`client.go`](../client.go)).
- **`Dispatcher`**: Responsável por emitir eventos para os handlers registrados.
- **`Store`**: Interface para persistência de chaves e dados de contato ([`store/`](../store/)).

## High-level Flow

### Fluxo de Mensagem de Saída (Outgoing)
1. **Chamada da API**: O desenvolvedor chama `client.SendMessage()`.
2. **Marshaling**: O struct Go é convertido em um objeto Protobuf.
3. **Criptografia E2EE**: A mensagem é cifrada através do protocolo Signal usando as chaves de sessão do contato.
4. **Binary Encoding**: O pacote criptografado é envolto em um frame binário (WAProtobuf).
5. **Transporte**: O frame é enviado via WebSocket seguro para o servidor do WhatsApp.

### Fluxo de Mensagem de Entrada (Incoming)
1. **Recebimento**: O socket recebe um frame binário.
2. **Decodificação de Frame**: `binary/` decodifica o frame inicial.
3. **Decriptação Noise**: O túnel de transporte é aberto.
4. **Decriptação E2EE**: Se for uma mensagem cifrada, o `whatsmeow` utiliza as chaves da `Store` para decifrar o conteúdo.
5. **Unmarshaling**: O Protobuf resultante é convertido em structs Go.
6. **Evento**: O `Client` emite um evento (ex: `*events.Message`) para os handlers registrados.

## Internal Movement
A colaboração entre módulos ocorre principalmente através de:
- **Canais (Go Channels)**: Usados para tratar pacotes assíncronos vindos do WebSocket.
- **Event Handlers**: Funções de callback executadas em goroutines separadas para evitar bloqueio do processamento de rede.

## Observability & Failure Modes
- **Logs**: O projeto usa um sistema de logging customizável para monitorar frames binários e erros de decriptação.
- **Reconexão**: Implementa backoff exponencial para tentativas de reconexão em caso de falha no socket.
- **Media Retry**: Fluxo específico para solicitar chaves de média expiradas ao servidor.

## Cross-References
- [Architecture Notes](./architecture.md)
- [Project Overview](./project-overview.md)
