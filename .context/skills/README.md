# Skills

On-demand expertise for AI agents. Skills are task-specific procedures that get activated when relevant.

> Project: whatsmeow

## How Skills Work

1. **Discovery**: AI agents discover available skills
2. **Matching**: When a task matches a skill's description, it's activated
3. **Execution**: The skill's instructions guide the AI's behavior

## Available Skills

### Built-in Skills

| Skill | Description | Phases |
|-------|-------------|--------|
| [Commit Message](./commit-message/SKILL.md) | Generate commit messages following conventional commits with scope detection | E, C |
| [Pr Review](./pr-review/SKILL.md) | Review pull requests against team standards and best practices | R, V |
| [Code Review](./code-review/SKILL.md) | Revisão de qualidade de código e melhores práticas para o whatsmeow | R, V |
| [Test Generation](./test-generation/SKILL.md) | Gerar casos de teste abrangentes para o whatsmeow | E, V |
| [Documentation](./documentation/SKILL.md) | Gerar e atualizar documentação técnica para o whatsmeow | P, C |
| [Refactoring](./refactoring/SKILL.md) | Refatoração segura de código passo-a-passo para o whatsmeow | E |
| [Bug Investigation](./bug-investigation/SKILL.md) | Investigação sistemática de bugs e análise de causa raiz para o whatsmeow | E, V |
| [Feature Breakdown](./feature-breakdown/SKILL.md) | Break down features into implementable tasks | P |
| [Api Design](./api-design/SKILL.md) | Design de APIs da biblioteca Go seguindo melhores práticas para o whatsmeow | P, R |
| [Security Audit](./security-audit/SKILL.md) | Checklist de auditoria de segurança para código e infraestrutura do whatsmeow | R, V |

## Creating Custom Skills

Create a new skill by adding a directory with a `SKILL.md` file:

```
.context/skills/
└── my-skill/
    ├── SKILL.md          # Required: skill definition
    └── templates/        # Optional: helper resources
        └── checklist.md
```

### SKILL.md Format

```yaml
---
name: my-skill
description: When to use this skill
phases: [P, E, V]  # Optional: PREVC phases
mode: false        # Optional: mode command?
---

# My Skill

## When to Use
[Description of when this skill applies]

## Instructions
1. Step one
2. Step two

## Examples
[Usage examples]
```

## PREVC Phase Mapping

| Phase | Name | Skills |
|-------|------|--------|
| P | Planning | feature-breakdown, documentation, api-design |
| R | Review | pr-review, code-review, api-design, security-audit |
| E | Execution | commit-message, test-generation, refactoring, bug-investigation |
| V | Validation | pr-review, code-review, test-generation, security-audit |
| C | Confirmation | commit-message, documentation |
