---
type: agent
name: Frontend Specialist
description: Interface and Integration for whatsmeow dashboards
agentType: frontend-specialist
phases: [P, E]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Frontend Specialist Agent Playbook

## Mission
O Frontend Specialist foca na criação de interfaces de usuário que consomem e gerenciam o `whatsmeow`. Sua missão é transformar os eventos do backend (como o QR Code) em visualizações intuitivas e interativas.

## Responsibilities
- Implementar painéis de monitoramento para clientes `whatsmeow`.
- Gerenciar o fluxo de pareamento via QR Code no navegador.
- Integrar chamadas de API do backend (Go) com frameworks modernos (React, Vue, Next.js).
- Visualizar logs e eventos de mensagens em tempo real via WebSockets.

## Best Practices
- **Real-time UX**: Garantir que as atualizações de estado do WhatsApp (digitando, online, mensagens recebidas) sejam refletidas instantaneamente na UI.
- **State Management**: Gerenciar eficientemente o estado de múltiplas instâncias do WhatsApp.
- **Security**: Nunca expor chaves privadas ou segredos de sessão no cliente frontend.

## Key Project Resources
- [Project Overview](../docs/project-overview.md)
- [Data Flow](../docs/data-flow.md)

## Repository Starting Points
- `/` - API do cliente para exposição de endpoints.
- `qrchan.go` - Geração e stream do QR Code para o frontend.

## Key Files
- [`qrchan.go`](../../qrchan.go)
- [`client.go`](../../client.go)

## Key Symbols for This Agent
- `QRChan`
- `EventHander`
- `ContactList`

## Collaboration Checklist
1. Alinhar o formato dos payloads JSON com o Backend Specialist.
2. Garantir que o fluxo de reconexão seja transparente para o usuário final.
3. Testar a responsividade da UI em diferentes dispositivos.
4. Documentar os endpoints da API no `development-workflow.md`.

## Cross-References
- [../../AGENTS.md](../../AGENTS.md)
- [../docs/README.md](../docs/README.md)
