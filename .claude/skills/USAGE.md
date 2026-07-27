# How to Use These Skills

This document explains how AI assistants like Claude Code can effectively use the skills in this directory.

## For AI Assistants

When working with the AI Sandbox Gateway codebase, you can automatically invoke these skills when relevant tasks are requested:

### Automatic Skill Selection

**Testing & Verification**
- User asks to test an endpoint → Use `/test-endpoint`
- User reports a bug or issue → Use `/debug-sandbox`

**Development**
- User wants to add a new feature/endpoint → Use `/add-endpoint`
- User asks about code patterns → Reference `/add-endpoint` for guidance

**Performance**
- User asks about performance → Use `/benchmark-sandbox`
- User needs load testing → Use `/benchmark-sandbox`

**Deployment**
- User wants to deploy → Use `/deploy-sandbox`
- User asks about production setup → Use `/deploy-sandbox`

**Integration**
- User wants to integrate with LLM/AI → Use `/integrate-with-ai`
- User needs client library code → Use `/integrate-with-ai`

**Browser Automation**
- User needs to setup Playwright → Use `/setup-browser`
- User has browser automation issues → Use `/setup-browser`

**Monitoring**
- User asks about health checks → Use `/monitor-sandbox`
- User needs production monitoring → Use `/monitor-sandbox`

## Skill Invocation

Skills can be invoked using the `Skill` tool:

```javascript
// Example: Invoke test-endpoint skill
await Skill({
  skill: "test-endpoint",
  args: "/exec endpoint with authentication"
})
```

## Skill Format

Each skill follows this structure:

```markdown
---
skill: skill-name
description: Brief description of what this skill does
---

# Skill Title

Use this skill when [specific situation].

## Quick Start
[Immediate action items]

## Common Patterns
[Reusable code and commands]

## Troubleshooting
[Solutions to common issues]
```

## Creating New Skills

When creating new skills for this project:

1. **Identify a common workflow** - Look for tasks that require multiple steps or domain knowledge

2. **Create the markdown file** in `.claude/skills/` with frontmatter:
   ```markdown
   ---
   skill: your-skill-name
   description: One-line description
   ---
   ```

3. **Include practical examples** - Real commands that work, not pseudocode

4. **Add troubleshooting** - Common issues and solutions

5. **Update README.md** - Add your skill to the list with usage guidance

6. **Test it** - Verify all commands and code examples actually work

## Skill Best Practices

### For Skill Authors

- **Be specific** - Include actual commands, not general instructions
- **Be complete** - Cover the full workflow from start to finish
- **Be practical** - Focus on what users actually need to do
- **Include error cases** - Show how to handle failures
- **Keep it focused** - One skill = one workflow
- **Use code blocks** - Make examples easy to copy/paste
- **Show expected output** - Help users verify success

### For AI Assistants Using Skills

- **Read the full skill** - Don't skip sections
- **Follow the workflow** - Execute steps in order
- **Adapt to context** - Modify examples for user's specific case
- **Check prerequisites** - Verify requirements before proceeding
- **Handle errors** - Use troubleshooting sections when issues occur
- **Combine skills** - Chain multiple skills for complex tasks

## Examples

### Example 1: Testing a New Endpoint

```bash
# User request: "Test the /exec endpoint"
# AI should invoke test-endpoint skill and follow its workflow

# 1. Set authentication
export GATEWAY_TOKEN="test-token-123"

# 2. Test basic exec
curl -X POST http://localhost:7860/exec \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello"}'

# 3. Verify response
# Expected: {"success":true,"output":"hello\n",...}
```

### Example 2: Adding a New Feature

```bash
# User request: "Add a new endpoint for listing files"
# AI should invoke add-endpoint skill and follow its pattern

# Skill guides through:
# 1. Creating handler in internal/handlers/
# 2. Registering route in internal/server/server.go
# 3. Adding input validation
# 4. Implementing security checks
# 5. Writing tests
# 6. Documentation
```

### Example 3: Debugging an Issue

```bash
# User request: "The gateway is not responding"
# AI should invoke debug-sandbox skill

# Skill provides systematic checks:
# 1. Check if server is running
# 2. Verify authentication token
# 3. Check network connectivity
# 4. Review logs for errors
# 5. Test with simple command
# 6. Verify system resources
```

## Skill Dependencies

Some skills work better together:

- **deploy-sandbox** → **monitor-sandbox** (deploy then monitor)
- **add-endpoint** → **test-endpoint** (implement then test)
- **integrate-with-ai** → **test-endpoint** (integrate then verify)
- **setup-browser** → **test-endpoint** (setup then test browser endpoints)

## Maintenance

Keep skills up to date:

- **Update when code changes** - If handlers or routes change, update relevant skills
- **Add new troubleshooting** - Document new issues as they're discovered
- **Improve examples** - Enhance with better patterns as they emerge
- **Remove obsolete content** - Delete sections that no longer apply

## Feedback

If a skill doesn't work or could be improved:

1. Test the commands manually
2. Identify what's wrong or missing
3. Update the skill file
4. Commit the improvement
5. Update USAGE.md if workflow changes

## Quick Reference

| Task | Skill | Key Sections |
|------|-------|--------------|
| Test any endpoint | test-endpoint | Quick test, All endpoints |
| Deploy to production | deploy-sandbox | Pre-deployment, Methods |
| Add new feature | add-endpoint | Step-by-step, Patterns |
| Fix an issue | debug-sandbox | Common issues, Quick diagnostic |
| Check performance | benchmark-sandbox | Quick test, Detailed benchmarks |
| Integrate with AI | integrate-with-ai | Client libraries, Patterns |
| Setup browser | setup-browser | Installation, Verification |
| Monitor production | monitor-sandbox | Health checks, Alerts |

## Getting Help

If unsure which skill to use:

1. **Check README.md** - Lists all skills with "Use when" guidance
2. **Read skill descriptions** - Frontmatter describes each skill's purpose
3. **Look at examples** - This file shows common use cases
4. **Try the most relevant** - Skills are safe to explore

## Success Metrics

A skill is successful when:

- ✅ Users can complete the task by following the skill
- ✅ AI assistants can interpret and execute the workflow
- ✅ All code examples work without modification
- ✅ Troubleshooting section resolves common issues
- ✅ The skill saves time compared to manual approach

## Next Steps

1. **For users**: Browse README.md to see available skills
2. **For AI assistants**: Read skills relevant to current task
3. **For contributors**: Create skills for common workflows
4. **For maintainers**: Keep skills synchronized with codebase
