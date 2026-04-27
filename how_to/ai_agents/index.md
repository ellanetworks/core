# Manage Your Network with AI Agents

Ella Core ships with an [Agent Skill](https://agentskills.io/) that lets AI agents manage your 5G network using natural language. The skill provides the OpenAPI specification so agents can discover and call the REST API on your behalf.

## Prerequisites

Before using the skill, you need:

1. **A running Ella Core instance** with its API accessible (e.g. `http://192.168.1.10:5000`).
1. **A user for your AI agent with an API token** — create a user for your agent in the UI with a role that matches the permissions you want to grant (e.g. "network manager" for full network access, "read only" for monitoring). Then generate an API token for that user and copy it.

## 1. Install the skill

Download [`SKILL.md`](https://raw.githubusercontent.com/ellanetworks/core/main/.github/skills/ella-core-api/SKILL.md) and place it in a skills directory that your AI tool can discover (e.g. `<project>/.agents/skills/ella-core-api/SKILL.md`).

## 2. Prompt the agent

Once the skill is active, you can ask things like "Which subscribers used the most data over the last 7 days?". The agent will ask you for the Ella Core URL and an API token — use the token you generated earlier.

Claude Opus responds to "Which subscribers used the most data over the last 7 days?" using its Ella Core skill.
