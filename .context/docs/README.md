# Documentation Index

Bem-vindo à base de conhecimento do projeto `whatsmeow`. Esta documentação foi gerada para fornecer aos desenvolvedores e agentes de IA o contexto necessário para operar no ecossistema da biblioteca.

## Core Guides
- [**Project Overview**](./project-overview.md) — Visão geral da biblioteca, seu propósito e stack tecnológica.
- [**Architecture Notes**](./architecture.md) — Detalhes sobre as camadas de rede, protocolo e design modular.
- [**Security & Compliance**](./security.md) — Fundamental para entender o Signal Protocol e o manuseio de chaves.
- [**Data Flow & Integrations**](./data-flow.md) — Como as mensagens transitam do WebSocket até o disparador de eventos.
- [**Development Workflow**](./development-workflow.md) — Guia de contribuição, branching e padrões de código.
- [**Testing Strategy**](./testing-strategy.md) — Como garantir a estabilidade do protocolo através de testes em Go.
- [**Glossary & Domain Concepts**](./glossary.md) — Definições de termos como JID, MD, PreKey e Noise.
- [**Tooling & Productivity**](./tooling.md) — Configurações de IDE e ferramentas (protoc, go 1.21+).

## Repository Snapshot
O `whatsmeow` é composto pelos seguintes arquivos e diretórios principais:

- `AGENTS.md` - Guia de agentes.
- `appstate/` - Gerenciamento de estado sincronizado.
- `appstate.go` - Implementação de estado.
- `argo/` - Componentes relacionados ao protocolo Argo.
- `armadillomessage.go` - Tratamento de mensagens Armadillo.
- `binary/` - Encoders/Decoders do protocolo binário.
- `broadcast.go` - Lógica de transmissão (broadcast).
- `call.go` - Tratamento de chamadas.
- `client.go` - API principal do cliente.
- `client_test.go` - Testes principais do cliente.
- `connectionevents.go` - Eventos de conexão.
- `download.go` / `download-to-file.go` - Gerenciamento de mídia.
- `errors.go` - Definições de erros.
- `go.mod` / `go.sum` - Dependências Go.
- `group.go` - Gerenciamento de grupos.
- `handshake.go` - Conexão Noise.
- `internals.go` / `internals_generate.go` - Lógica interna.
- `keepalive.go` - Manutenção de socket.
- `mediaconn.go` / `mediaretry.go` - Conexão de mídia.
- `message.go` - Estrutura de mensagens.
- `msgsecret.go` - Segredos de mensagens.
- `newsletter.go` - Suporte a Newsletters/Canais.
- `notification.go` - Tratamento de notificações.
- `pair.go` / `pair-code.go` - Processo de pareamento.
- `prekeys.go` - Gerenciamento de PreKeys Signal.
- `presence.go` - Status de presença (online/typing).
- `privacysettings.go` - Configurações de privacidade.
- `proto/` - Definições de Protocol Buffers.
- `push.go` - Notificações push.
- `qrchan.go` - Canal de QR Code.
- `receipt.go` - Recibos de leitura/entrega.
- `request.go` / `retry.go` - Requisições e novas tentativas.
- `send.go` / `sendfb.go` - Envio de mensagens.
- `socket/` - Abstração de rede.
- `store/` - Persistência de dados (SQLite/Postgres).
- `types/` - Tipos globais.
- `user.go` - Gerenciamento de perfil/usuário.
- `util/` - Utilitários e criptografia.

## Document Map
| Guia | Arquivo | Entradas Principais |
| --- | --- | --- |
| Visão Geral | `project-overview.md` | README, client.go, go.mod |
| Arquitetura | `architecture.md` | handshake.go, binary/, store/ |
| Segurança | `security.md` | msgsecret.go, store/, Signal specs |
| Fluxo de Dados | `data-flow.md` | client.go, binary/, events/ |

> **Análise Avançada**: Para contagens de símbolos e grafos de dependência detalhados, consulte o [`codebase-map.json`](./codebase-map.json).
