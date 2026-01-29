---
type: skill
name: Bug Investigation
description: Investigação sistemática de bugs e análise de causa raiz para o whatsmeow
skillSlug: bug-investigation
phases: [E, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Investigação de Bugs

## Fluxo de Trabalho de Debug
1. **Análise de Logs**: Comece habilitando logs de debug no `client`. Procure por tags `[WARN]` ou `[ERROR]` relacionadas a `binary.Decoder` ou `noise`.
2. **Reprodução**: Crie um caso de reprodução mínima em `client_test.go`. Use o `QRChan` simulado para emular fluxos de login se necessário.
3. **Dump do Protocolo**: Se for um erro de parsing, solicite o hex dump do nó binário usando `hex.EncodeToString`.

## Padrões Comuns de Bugs
- **Falhas de Decriptação**: Geralmente causadas por incompatibilidade de `IdentityKey`. Verifique se o dispositivo foi desconectado remotamente.
    - *Correção*: Re-pareamento é frequentemente a única solução se as chaves estiverem dessincronizadas.
- **Desconexões WebSocket**: Frequentemente devido a timeouts de ping.
    - *Correção*: Verifique as configurações de `KeepAlive` em `client.go`.
- **Alta Contenção (Race Conditions)**: Deadlocks de Mutex em implementações da `Store`.
    - *Correção*: Use `go test -race` para identificar violações de acesso compartilhado.

## Convenções de Log e Erros
- Use `fmt.Errorf("contexto: %w", err)` para envelopar erros e manter a cadeia de causa.
- Nunca logue o conteúdo de `msgsecret.go` ou valores de `PreKey` (segredos).
- Use a interface interna `waLog.Logger` para manter a consistência da saída.

## Etapas de Verificação de Teste
- Adicione um teste de regressão em `binary/decoder_test.go` para qualquer bug de parsing encontrado.
- Garanta que a correção funcione tanto para `sqlite3` quanto para `postgres`.
