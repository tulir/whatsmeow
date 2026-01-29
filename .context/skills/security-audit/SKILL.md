---
type: skill
name: Security Audit
description: Checklist de auditoria de segurança para código e infraestrutura do whatsmeow
skillSlug: security-audit
phases: [R, V]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Auditoria de Segurança

## Checklist de Segurança
- [ ] **Armazenamento de Chaves**: `IdentityKey` e `PrivKey` estão armazenados de forma segura? Eles nunca devem ser exportados em JSON puro.
- [ ] **Aleatoriedade**: Todos os nonces/salts criptográficos devem ser gerados usando `crypto/rand`, nunca `math/rand`.
- [ ] **Ataques de Timing**: Use `subtle.ConstantTimeCompare` para verificar MACs e hashes.

## Vulnerabilidades Comuns
- **Logs**: Dê grep por `fmt.Printf`, `log.Println` que possam despejar structs de mensagens brutas.
- **Injeção de SQL**: Garanta que todas as queries em `store/` usem parameter binding (`?` ou `$1`), nunca concatenação de strings.
- **Segurança de Memória**: Verifique o uso do pacote `unsafe`. Ele deve ser evitado a menos que estritamente necessário para performance em `binary/`.

## Autenticação e Autorização
- Valide se `client.go` verifica corretamente a identidade do servidor durante o Noise Handshake.
- Garanta que uploads de `PreKey` sejam assinados corretamente pela `IdentityKey`.

## Validação de Dados
- Todos os inputs via WebSocket (frames binários) devem ser tratados como não confiáveis.
- Valide o tamanho dos buffers em `binary/decoder.go` para prevenir pânico/DoS.
