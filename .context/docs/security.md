---
type: doc
name: security
description: Security policies, authentication, secrets management, and compliance requirements
category: security
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Security & Compliance Notes

A segurança é o pilar central do `whatsmeow`, uma vez que a biblioteca lida com comunicações privadas e sensíveis. A implementação segue rigorosamente o Protocolo Signal para criptografia de ponta a ponta (E2EE) e utiliza padrões modernos de criptografia de transporte.

## Authentication & Authorization
A autenticação no WhatsApp MD não utiliza tokens tradicionais (como JWT), mas baseia-se em uma "confiança persistente" estabelecida através de:
- **Identity Keys**: Chaves públicas/privadas persistentes que identificam o dispositivo.
- **Noise Handshake**: Um protocolo de negociação de chaves para estabelecer um túnel criptografado seguro com os servidores do WhatsApp.
- **QR Code Pairing**: O processo inicial de pareamento sincroniza chaves de identidade entre o telefone principal e a instância do `whatsmeow`.

## Secrets & Sensitive Data
O gerenciamento de segredos é crítico. O `whatsmeow` armazena os seguintes dados sensíveis na `Store`:
- **Client Identity Key**: Usada para assinar pacotes e autenticar o dispositivo.
- **PreKeys**: Chaves pré-geradas para iniciar sessões criptografadas com novos contatos.
- **Session Keys**: Chaves efêmeras usadas para cifrar mensagens individuais.

**Práticas Recomendadas**:
- **Criptografia na Base**: Recomenda-se que o banco de dados (SQLite/Postgres) seja criptografado em repouso.
- **Acesso Restrito**: Somente o processo da aplicação deve ter acesso ao diretório onde o arquivo de banco de dados da `Store` reside.

## Compliance & Policies
Embora o `whatsmeow` seja uma implementação de terceiros, ele visa manter a conformidade técnica com o comportamento esperado de um cliente WhatsApp oficial para evitar detecção e banimentos:
- **Respeito a Rate Limits**: Implementados internamente na biblioteca para evitar spam.
- **E2EE Integrity**: Todas as mensagens enviadas são obrigatoriamente criptografadas seguindo o padrão Signal.

## Incident Response
Em caso de suspeita de comprometimento:
1. Revogar a sessão do "Linked Device" através do aplicativo oficial do WhatsApp no telefone.
2. Excluir o banco de dados da `Store` local.
3. Gerar novas chaves de identidade através de um novo pareamento.

## Cross-References
- [Architecture Notes](./architecture.md)
- [Data Flow](./data-flow.md)
