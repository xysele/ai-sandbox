# AI Sandbox Gateway Skills

This directory contains skills that help Claude Code and other AI assistants work with the AI Sandbox Gateway project.

## Available Skills

### 🧪 test-endpoint.md
Test any endpoint of the AI Sandbox Gateway with proper authentication. Provides examples for all major endpoints and common troubleshooting steps.

**Use when:** You need to verify an endpoint works correctly or debug API responses.

### 🚀 deploy-sandbox.md
Build and deploy the AI Sandbox Gateway to production environments (Docker, ModelSpace, or bare metal). Includes pre-deployment checklists and rollback procedures.

**Use when:** Ready to deploy to production or need to update a running instance.

### ➕ add-endpoint.md
Step-by-step guide to add new HTTP endpoints to the gateway. Covers handler functions, route registration, security patterns, and testing.

**Use when:** Adding new functionality that requires a new API endpoint.

### 🐛 debug-sandbox.md
Diagnose and fix common issues with the gateway. Covers authentication errors, connection problems, build errors, GUI/browser issues, and performance problems.

**Use when:** Something isn't working and you need systematic troubleshooting steps.

### ⚡ benchmark-sandbox.md
Measure performance and identify bottlenecks. Includes latency tests, concurrent load tests, memory profiling, and optimization tips.

**Use when:** Need to verify performance meets requirements or optimize slow endpoints.

### 🤖 integrate-with-ai.md
Integrate the gateway with AI agents and LLM applications. Provides client libraries (Python/JavaScript), LLM tool definitions, and integration patterns.

**Use when:** Building an AI agent that needs to use the sandbox gateway as a tool.

### 🌐 setup-browser.md
Setup and configure browser automation (Playwright) for the sandbox. Covers installation, system dependencies, troubleshooting, and verification.

**Use when:** Need to enable browser automation features or troubleshoot Playwright installation issues.

### 📊 monitor-sandbox.md
Monitor the gateway in production with health checks, alerts, and performance metrics. Includes integration with Kubernetes, Docker, Nagios, and custom monitoring scripts.

**Use when:** Setting up production monitoring, configuring alerts, or troubleshooting monitoring systems.

## How to Use Skills

In Claude Code, you can invoke skills by name when needed. For example:

```
"Test the /batch endpoint to verify it works"
→ Uses test-endpoint.md skill

"Add a new endpoint for database queries"
→ Uses add-endpoint.md skill

"The server returns 401 errors"
→ Uses debug-sandbox.md skill
```

Skills provide reusable workflows and best practices, helping maintain consistency across development tasks.

## Skill Format

Each skill follows this format:

```markdown
---
skill: skill-name
description: Brief description of what the skill does
---

# Skill Title

Detailed instructions, code examples, and best practices...
```

## Adding New Skills

To add a new skill:

1. Create a new `.md` file in this directory
2. Add frontmatter with `skill` and `description` fields
3. Write clear, actionable instructions
4. Include code examples and common pitfalls
5. Update this README with the new skill

Skills should be focused on specific workflows or problem domains, not general documentation.
