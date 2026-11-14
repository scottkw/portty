<!-- GSD:project-start source:PROJECT.md -->

## Project

**Portty**

Portty is a macOS terminal emulator with built-in remote access over Tailscale. It is a stripped-down fork of AgentHub: it keeps the persistent daemon-backed terminal sessions, Tailscale integration, web/desktop sharing, chat, themes, and notifications, and removes everything specific to AI agent CLIs. Its first user is the author, who runs a terminal (often with Claude Code inside) on an office MacBook Air and controls it from a personal MacBook Air at home.

**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

### Constraints

- **Platform**: macOS only (Apple Silicon MacBook Airs). Both target machines are Macs.
- **Tech stack**: Keep AgentHub's stack (Go/Wails/React/tsnet). This is a strip-down, not a rewrite.
- **Network**: Remote access only over the user's tailnet (plus opt-in Funnel). No new relay servers or cloud dependencies.
- **Security**: Keep AgentHub's capability-gated share model, CSP, and Funnel consent gates intact through the refactor. Remote write access to a shell is powerful.
- **Deletion discipline**: Chesterton's Fence. Read each piece and know why it exists before removing it, especially in daemon, relay, and capability code shared between kept and removed features.

<!-- GSD:project-end -->

<!-- GSD:stack-start source:STACK.md -->

## Technology Stack

Technology stack not yet documented. Will populate after codebase mapping or first phase.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-fast` for a trivial task inline, with no subagents and no PLAN.md
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
