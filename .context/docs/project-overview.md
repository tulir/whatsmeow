---
type: doc
name: project-overview
description: High-level overview of the project, its purpose, and key components
category: overview
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Project Overview

O `whatsmeow` é uma biblioteca em Go poderosa e extensível para interagir com o protocolo Multi-Device do WhatsApp. Ela permite que desenvolvedores criem clientes, bots e integrações que se comunicam diretamente com os servidores do WhatsApp, oferecendo suporte a criptografia de ponta a ponta, mensagens multimídia, grupos e muito mais.

> **Detailed Analysis**: For complete symbol counts, architecture layers, and dependency graphs, see [`codebase-map.json`](./codebase-map.json).

## Quick Facts
- **Root**: `d:/whatsmeow`
- **Primary Language**: Go (Golang)
- **Key Purpose**: Implementação do protocolo WhatsApp MD.
- **Main Client**: `client.go`
- **Full analysis**: [`codebase-map.json`](./codebase-map.json)

## Entry Points
- [`client.go`](../client.go) — O ponto de entrada principal para criar e gerenciar a conexão com o WhatsApp.
- [`message.go`](../message.go) — Define as estruturas e métodos para manipulação de mensagens.
- [`store/`](../store/) — Ponto de entrada para persistência de dados (sessões, contatos, chaves).

## Key Exports
A biblioteca exporta o tipo `Client` para interagir com o servidor, além de diversos tipos de eventos e mensagens definidos em `types/` e `proto/`. Para a lista completa, consulte o [`codebase-map.json`](./codebase-map.json).

## File Structure & Code Organization
- `proto/` — Arquivos `.proto` e código gerado para comunicação com o WhatsApp.
- `store/` — Gerenciamento de estado e autenticação (SQLITE, Postgres).
- `types/` — Definições de tipos comuns usados no protocolo.
- `binary/` — Codificação e decodificação do protocolo binário do WhatsApp (WAProtobuf).
- `util/` — Funções utilitárias para criptografia e tratamento de buffers.

## Technical Foundation & Runtime
- **Runtime**: Go 1.21+
- **Protocolo**: WebSockets com criptografia Noise Handshake.
- **Serialização**: protobuf (v3).
- **Segurança**: Signal Protocol para criptografia ponta-a-ponta.

## Core Framework Stack
- **Messaging**: WebSockets para comunicação em tempo real.
- **Data**: SQL (via `modernc.org/sqlite` ou `lib/pq`) para armazenamento de chaves de criptografia e sessões MD.

## Development Tools Overview
A biblioteca utiliza o sistema de build padrão do Go (`go build`, `go test`). Depende fortemente de geradores de código para protobuf.

## Getting Started Checklist
1. Instale o Go em sua máquina.
2. Clone o repositório e rode `go mod tidy` para instalar as dependências.
3. Explore os exemplos na pasta `examples/` (se disponível) ou veja o `client_test.go` para entender como conectar.
4. Consulte o [Development Workflow](./development-workflow.md) para diretrizes de contribuição.

## Next Steps
Para entender a fundo como a biblioteca funciona, recomendamos a leitura do documento de [Arquitetura](./architecture.md) e o fluxo de [Segurança](./security.md).
