---
type: doc
name: tooling
description: Scripts, IDE settings, automation, and developer productivity tips
category: tooling
generated: 2026-01-29
status: filled
scaffoldVersion: "2.0.0"
---

# Tooling & Productivity Guide

Este guia detalha as ferramentas e configurações necessárias para desenvolver e manter a biblioteca `whatsmeow` com eficiência.

## Required Tooling
- **Go (Golang)**: Versão 1.21 ou superior é necessária devido ao uso de generics e novas APIs da biblioteca padrão.
- **Protobuf Compiler (`protoc`)**: Necessário para regenerar os arquivos de protocolo se houver mudanças nos arquivos `.proto`.
- **Go Protobuf Plugins**: `protoc-gen-go` para gerar código Go a partir de protobufs.
- **C Compiler (GCC/Clang)**: Caso utilize o driver de banco de dados `github.com/mattn/go-sqlite3` que requer CGO. (O driver `modernc.org/sqlite` é uma alternativa Pure-Go que dispensa CGO).

## Recommended Automation
- **`go mod tidy`**: Mapeia e limpa dependências.
- **`go generate ./...`**: Executa geradores de código, essencial para manter os pacotes `proto/` e `binary/` atualizados.
- **`golangci-lint`**: O linter recomendado para manter a qualidade do código conforme os padrões da comunidade Go.

## IDE / Editor Setup
- **VS Code**:
    - Extensão: **Go (Google)** — Essencial para IntelliSense, testes e linting.
    - Configuração: Habilitar `gofumpt` como formatador para um código mais rigoroso.
- **GoLand**: Suporte nativo completo para o projeto sem configurações adicionais.

## Productivity Tips
- **Pre-commit Hooks**: Recomendamos configurar um hook para rodar `go fmt` e `go mod tidy` antes de cada commit.
- **Terminal Aliases**:
    ```bash
    alias gmt="go mod tidy"
    alias ggen="go generate ./..."
    alias gtests="go test ./... -v"
    ```

## Cross-References
- [Development Workflow](./development-workflow.md)
