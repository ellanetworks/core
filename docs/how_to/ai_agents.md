---
description: How to use AI coding agents to interact with Ella Core.
---

# Use AI Agents with Ella Core

Ella Core ships with an [Agent Skill](https://agentskills.io/) that teaches AI coding agents how to interact with the Ella Core REST API. This lets you manage your 5G network using natural language — querying subscribers, checking data usage, provisioning SIM cards, and more — directly from your editor or terminal.

## Supported tools

The skill works with any tool that supports the [Agent Skills](https://agentskills.io/) open format, including:

- [GitHub Copilot](https://github.com/features/copilot) (agent mode in VS Code, Copilot coding agent, Copilot CLI)
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)

## Prerequisites

Before using the skill, you need:

1. **A running Ella Core instance** with its API accessible (e.g. `http://192.168.1.10:5000`).
2. **An API token** — create one in the Ella Core UI under your user profile, or via the API. Tokens are prefixed with `ellacore_`.

## 1. Install the skill

Download the [`SKILL.md`](https://raw.githubusercontent.com/ellanetworks/core/main/.github/skills/ella-core-api/SKILL.md) file and place it in a skills directory that your AI tool can discover:

```bash
mkdir -p <project>/.agents/skills/ella-core-api
curl -o <project>/.agents/skills/ella-core-api/SKILL.md \
  https://raw.githubusercontent.com/ellanetworks/core/main/.github/skills/ella-core-api/SKILL.md
```

Replace `<project>` with the path to your project root.

!!! note
    Some tools use different skill directories (e.g. `.github/skills/`, `.claude/skills/`, `~/.copilot/skills/`). Check your tool's documentation for the expected location.

## 2. Prompt the agent

Once the skill is active, you can ask things like:
> "Which subscriber consumed the most data in the last 7 days?"
