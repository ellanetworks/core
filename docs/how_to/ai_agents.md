---
description: How to use AI coding agents to interact with Ella Core.
---

# Manage Your Network with AI Agents

Ella Core ships with an [Agent Skill](https://agentskills.io/) that teaches AI agents how to interact with Ella Core through its REST API using its OpenAPI specification. This lets you manage your 5G network using natural language — querying subscribers, checking data usage, provisioning SIM cards, and more.

## Prerequisites

Before using the skill, you need:

1. **A running Ella Core instance** with its API accessible (e.g. `http://192.168.1.10:5000`).
2. **An API token** — create one in the Ella Core UI under your user profile, or via the API. Tokens are prefixed with `ellacore_`.

## 1. Install the skill

Download the [`SKILL.md`](https://raw.githubusercontent.com/ellanetworks/core/main/.github/skills/ella-core-api/SKILL.md) file and place it in a skills directory that your AI tool can discover (e.g. `<project>/.agents/skills/ella-core-api/SKILL.md`).

## 2. Prompt the agent

Once the skill is active, you can ask things like:

- "Which subscriber consumed the most data in the last 7 days?"
- "List all subscribers and their associated policies."
- "Create a new QoS policy with 50 Mbps uplink and 100 Mbps downlink on the internet data network."
- "Show me the current radios connected to the network."

<figure markdown="span">
  ![Integrate with AI agents](../images/ai_prompt.png){ width="700" }
  <figcaption>5G Overview</figcaption>
</figure>
