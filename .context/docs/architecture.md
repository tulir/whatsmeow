---
type: doc
name: architecture
description: System architecture, layers, patterns, and design decisions
category: architecture
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Architecture Notes

A arquitetura do `whatsmeow` é projetada para ser modular e eficiente no tratamento da comunicação assíncrona com os servidores do WhatsApp. O sistema opera como uma biblioteca cliente-servidor que lida com a complexidade do protocolo binário e da criptografia Signal de forma transparente para o desenvolvedor final.

## System Architecture Overview
A biblioteca segue um modelo monolítico modular em Go. A comunicação atravessa camadas desde a interface pública (`Client`), passando pelo codec binário (`binary/`), pela camada de criptografia (`Noise/Signal`), até chegar no transporte (`WebSockets`). O gerenciamento de estado é delegado a uma camada de persistência (`Store`).

## Architectural Layers
- **API Layer**: Client centralizado em [`client.go`](../client.go) que expõe métodos como `Connect()`, `Send()` e hooks de eventos.
- **Protocol Layer**: Implementação do Noise Handshake em [`handshake.go`](../handshake.go) e do protocolo binário em [`binary/`](../binary/).
- **Crypto Layer**: Implementação do Signal Protocol em Go para criptografia ponta-a-ponta (E2EE).
- **Data Layer**: Persistência de sessões e chaves em [`store/`](../store/).

> See [`codebase-map.json`](./codebase-map.json) for complete symbol counts and dependency graphs.

## Detected Design Patterns
| Pattern | Confidence | Locations | Description |
|---------|------------|-----------|-------------|
| **Event Emitter** | 95% | `Client.AddEventHandler` | Canaliza eventos do servidor (mensagens, recibos, presenças) para handlers registrados. |
| **Strategy** | 80% | `store/` implementations | Permite trocar o backend de armazenamento (SQLite, Postgres, In-memory). |
| **Codec (Encoder/Decoder)** | 100% | `binary/` | Converte estruturas Go para o formato binário do WhatsApp e vice-versa. |

## Entry Points
- [`client.go`](../client.go) — Principal estrutura e métodos de controle.
- [`handshake.go`](../handshake.go) — Início da conexão e negociação de chaves.
- [`store/sqlite3/`](../store/sqlite3/index.go) — Ponto de entrada para persistência SQLite.

## Public API
| Symbol | Type | Description |
|--------|------|-------------|
| `Client` | struct | O struct principal para interagir com o WhatsApp. |
| `Conn` | struct | Gerencia o socket e o estado da conexão. |
| `NewClient` | function | Construtor para instanciar um novo cliente com uma store. |

## External Service Dependencies
- **WhatsApp Web Servers**: Endpoint via WebSocket (geralmente `wss://web.whatsapp.com/ws/chat`).
- **GOSignal**: Implementações internas ou externas de criptografia de chaves.

## Related Resources
- [Project Overview](./project-overview.md)
- [Data Flow](./data-flow.md)
- [`codebase-map.json`](./codebase-map.json)
