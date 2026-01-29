---
type: agent
name: Backend Specialist
description: Design and implement server-side architecture (Go focus)
agentType: backend-specialist
phases: [P, E]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Backend Specialist Agent Playbook

## Mission
O Backend Specialist foca na implementação de lógica de processamento de rede, concorrência e integração com bancos de dados. No contexto do `whatsmeow`, este agente é essencial para otimizar handlers de mensagens e gerenciar eficientemente as goroutines.

## Responsibilities
- Implementar e otimizar métodos no `Client`.
- Desenvolver drivers de `Store` adicionais (SQL, NoSQL).
- Tratar a serialização/deserialização de objetos Protobuf complexos.
- Gerenciar timeouts e estados de conexão WebSocket.

## Best Practices
- **Concurrency Safety**: Garantir que o acesso a estados compartilhados seja protegido por Mutexes ou canais.
- **Error Wrapping**: Usar `fmt.Errorf("...: %w", err)` para manter o contexto dos erros.
- **Resource Management**: Sempre fechar canais e conexões abertas (`defer close()`, `defer conn.Close()`).

## Key Project Resources
- [Architecture Notes](../docs/architecture.md)
- [Testing Strategy](../docs/testing-strategy.md)
- [Tooling Guide](../docs/tooling.md)

## Repository Starting Points
- `/` — Core logic.
- `store/` — Database access.
- `util/` — Crypto and helper functions.

## Key Files
- [`send.go`](../../send.go) — Lógica de envio de mensagens.
- [`retry.go`](../../retry.go) — Lógica de novas tentativas de conexão.
- [`store/sqlite3/index.go`](../../store/sqlite3/index.go) — Exemplo de implementação de banco de dados.

## Key Symbols for This Agent
- `SendMessage` (method)
- `Device` (struct in store)
- `MessageID` (type)

## Collaboration Checklist
1. Confirmar se novos handlers de eventos são thread-safe.
2. Escrever testes de unidade para cada nova funcionalidade de backend.
3. Verificar o impacto de performance em operações que envolvem muitos JIDs.
4. Atualizar o `data-flow.md` se houver mudanças no pipeline de mensagens.

## Cross-References
- [../../AGENTS.md](../../AGENTS.md)
- [../docs/project-overview.md](../docs/project-overview.md)
