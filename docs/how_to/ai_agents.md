---
description: How to use AI coding agents to interact with Ella Core.
---

# Manage Your Network with AI Agents

Ella Core ships with an [Agent Skill](https://agentskills.io/) that lets AI agents manage your 5G network using natural language. The skill provides the OpenAPI specification so agents can discover and call the REST API on your behalf.

## Prerequisites

Before using the skill, you need:

1. **A running Ella Core instance** with its API accessible (e.g. `http://192.168.1.10:5000`).
2. **A user for your AI agent with an API token** — create a user for your agent in the UI with a role that matches the permissions you want to grant (e.g. "network manager" for full network access, "read only" for monitoring). Then generate an API token for that user and copy it.

## 1. Install the skill

### Claude Code (recommended)

Add the marketplace and install the plugin:

```
/plugin marketplace add ellanetworks/core
/plugin install ella-core@ellanetworks-core
```

Updates land with `/plugin marketplace update`.

### Other AI tools

The skill is a folder containing `SKILL.md` and a `references/` directory at [`skills/ella-core/`](https://github.com/ellanetworks/core/tree/main/skills/ella-core). Copy the whole folder into a skills directory your tool discovers (e.g. `<project>/.agents/skills/ella-core/`):

```bash
git clone --depth 1 --filter=blob:none --sparse https://github.com/ellanetworks/core.git /tmp/ella
git -C /tmp/ella sparse-checkout set skills/ella-core
mkdir -p .agents/skills && cp -r /tmp/ella/skills/ella-core .agents/skills/
```

## 2. Prompt the agent

Once the skill is active, you can ask things like "Which subscribers used the most data over the last 7 days?". The agent will ask you for the Ella Core URL and an API token — use the token you generated earlier.

<figure markdown="span">
  <div data-cast="../../casts/ella-demo.cast"></div>
  <figcaption>Claude Opus responds to "Which subscribers used the most data over the last 7 days?" using its Ella Core skill.</figcaption>
</figure>
