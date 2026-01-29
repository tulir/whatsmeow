---
type: doc
name: glossary
description: Project terminology, type definitions, domain entities, and business rules
category: glossary
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Glossary & Domain Concepts

Este glossário define os termos técnicos e conceitos de domínio específicos do protocolo WhatsApp e da biblioteca `whatsmeow`.

## Core Terms
- **JID (Jabber ID)**: Identificador único no WhatsApp. Exemplos: `user@s.whatsapp.net` (usuário), `12345@g.us` (grupo), `newsletter-jid@newsletter` (canal).
- **Signal Protocol**: Protocolo de criptografia ponta-a-ponta usado pelo WhatsApp para garantir a privacidade das mensagens.
- **Identity Key**: Chave de longo prazo que identifica o par de chaves pública e privada de um dispositivo.
- **PreKey**: Chaves efêmeras publicadas no servidor do WhatsApp para que outros usuários possam iniciar sessões criptografadas de forma assíncrona.
- **Noise Protocol**: Protocolo usado para estabelecer o túnel de transporte seguro via WebSocket.
- **Multi-Device (MD)**: Arquitetura atual do WhatsApp onde múltiplos dispositivos conectam-se de forma independente sem depender do telefone estar online.

## Type Definitions
- **`types.MessageSource`**: Estrutura que identifica a origem de uma mensagem (remetente, chat, se é de mim mesmo).
- **`events.Message`**: O evento disparado quando uma nova mensagem é recebida e decifrada.

## Acronyms & Abbreviations
- **E2EE**: End-to-End Encryption (Criptografia de Ponta a Ponta).
- **WA**: WhatsApp.
- **Protobuf**: Protocol Buffers (Sistema de serialização de dados do Google).
- **ADV**: Device Advanced Identity (Relacionado aos metadados de identidade MD).

## Domain Rules & Invariants
- **Imutabilidade de JID**: Uma vez que um JID é atribuído a uma entidade, ele não muda.
- **Integridade da Sessão**: Se as chaves de identidade mudarem sem aviso, o Signal Protocol detectará uma falha de segurança ("Seu código de segurança mudou").

## Cross-References
- [Project Overview](./project-overview.md)
