---
type: skill
name: Feature Breakdown
description: Decompor features em tarefas implementáveis para o whatsmeow
skillSlug: feature-breakdown
phases: [P]
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Habilidade: Decomposição de Features

## Abordagem de Decomposição
1. **Entendimento do Protocolo**: Antes de codar, analise os logs binários ou a especificação do WhatsApp para entender a estrutura dos nós XML/Protobuf.
2. **Definição de Tipos**: Comece definindo os structs em `types/` ou `proto/`.
3. **Persistência**: Defina se novos dados precisam ser salvos na `Store`.
4. **Lógica de Cliente**: Implemente os métodos no `Client` para enviar/receber.
5. **Eventos**: Defina novos eventos para serem emitidos ao usuário.

## Diretrizes de Estimativa
- **Pequena (S)**: Mudança apenas em `client.go`, sem persistência. (1-2 horas)
- **Média (M)**: Envolve novos tipos Protobuf e lógica de parsing. (4-8 horas)
- **Grande (L)**: Envolve persistência, criptografia ou mudanças grandes na arquitetura. (Dias)

## Identificação de Dependências
- Verificar se a feature depende de uma versão específica do nó do servidor WhatsApp.
- Verificar conflitos com outras features em andamento (ex: Newsletter vs Groups).

## Pontos de Integração
- `HandleEvent` no `client.go`.
- `Store` interfaces.
- `binary/encoder.go` e `decoder.go`.
