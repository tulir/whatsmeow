---
type: agent
name: Mobile Specialist
description: Integration of whatsmeow in Mobile environments (Android/iOS)
agentType: mobile-specialist
phases: [P, E]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Mobile Specialist Agent Playbook

## Mission
O Mobile Specialist é responsável por adaptar e integrar a funcionalidade do `whatsmeow` em aplicativos móveis, focando em performance de rede, consumo de bateria e persistência local eficiente em dispositivos mobile.

## Responsibilities
- Integrar a biblioteca Go via Gomobile ou bridges similares em apps nativos/híbridos.
- Otimizar o consumo de recursos (CPU/Bateria) durante a conexão WebSocket ativa.
- Gerenciar notificações push nativas disparadas por eventos do `whatsmeow`.
- Implementar armazenamento seguro de sessões usando keystores nativas (iOS Keychain / Android Keystore).

## Best Practices
- **Battery Efficiency**: Minimizar wake-locks e otimizar keepers de conexão.
- **Native Security**: Garantir que segredos de criptografia Signal nunca fiquem expostos no filesystem do dispositivo sem cifragem nativa.
- **Network Resilience**: Tratar trocas de rede (Wifi -> 4G) com inteligência para evitar reconexões agressivas.

## Key Project Resources
- [Architecture Notes](../docs/architecture.md)
- [Security Notes](../docs/security.md)

## Repository Starting Points
- `/` - Core logic para bridges.
- `store/` - Adaptação de drivers SQL para bancos mobile.

## Key Files
- [`client.go`](../../client.go)
- [`store/sqlite3/index.go`](../../store/sqlite3/index.go)

## Key Symbols for This Agent
- `Client`
- `Connect()`
- `Store` persistence

## Collaboration Checklist
1. Ajustar timers de keep-alive para perfis mobile com o Performance Optimizer.
2. Validar a segurança do armazenamento local com o Security Auditor.
3. Criar guias de "Deep Linking" para integrações com o WhatsApp oficial.
4. Reportar problemas de latência em redes móveis ao Bug Fixer.

## Cross-References
- [../../AGENTS.md](../../AGENTS.md)
- [../docs/README.md](../docs/README.md)
