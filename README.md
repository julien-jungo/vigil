# Vigil

AI-powered UI testing framework. An autonomous agent drives a browser via Playwright MCP to verify your application against natural-language specs — or explores it freely to find bugs on its own.

## Overview

Vigil sits between your CI pipeline and your web application. Point it at a URL, give it test specs (or don't), and it will:

1. Launch a Playwright browser via MCP
2. Interpret your specs using a configurable LLM (Claude, OpenAI, or others)
3. Execute the tests autonomously — navigating, interacting, asserting
4. Produce a CLI summary, JUnit XML, and an HTML report with screenshots

In **exploratory mode**, Vigil needs no specs at all. It autonomously navigates your app, tries common workflows, and reports anything that looks broken.

## Architecture

### Components

- **CLI** — Go binary. Parses config, loads specs, orchestrates the agent, writes reports.
- **LLM Agent** — Talks to a configurable LLM provider. Interprets specs into actions, decides what to do next, evaluates outcomes.
- **MCP Client** — Connects to the Playwright MCP server over stdio. Translates agent decisions into browser actions (navigate, click, type, screenshot, assert).
- **Reporter** — Collects results and produces JUnit XML (for CI), an HTML report (for humans), and a CLI summary.

## Test Specs

Specs are markdown files with optional YAML frontmatter for structured hints.

```markdown
---
url: /login
tags: [smoke, auth]
---

# Login with valid credentials

1. Navigate to the login page
2. Enter "testuser@example.com" in the email field
3. Enter "correct-password" in the password field
4. Click the login button
5. Verify the dashboard is displayed with a welcome message
```

The agent interprets these steps in natural language — no selectors or rigid locators required. The YAML frontmatter provides optional hints like starting URL, tags for filtering, and priority.

Specs live in a directory (default: `specs/`) and can be organized in subdirectories. Tags allow filtering which specs to run.

## Modes

### Spec-driven

Run tests defined in spec files:

```sh
vigil run --url https://myapp.example.com --specs ./specs
```

### Exploratory

Let the agent autonomously explore and find issues:

```sh
vigil explore --url https://myapp.example.com
```

The agent navigates the app on its own, tries common user flows (sign up, log in, CRUD operations, form submissions), and reports anomalies: broken pages, error messages, unresponsive elements, unexpected behavior.

## Configuration

Vigil is configured via environment variables or a `vigil.yaml` config file.

```yaml
url: https://myapp.example.com
specs: ./specs

llm:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key: ${ANTHROPIC_API_KEY}

playwright:
  headless: true
  viewport:
    width: 1280
    height: 720

reports:
  junit: ./reports/junit.xml
  html: ./reports/report.html

explore:
  max_steps: 100    # max browser actions in exploratory mode
  focus: []         # optional: paths/areas to focus on
```

## Docker

```sh
docker run --rm \
  -e ANTHROPIC_API_KEY \
  -v $(pwd)/specs:/specs \
  -v $(pwd)/reports:/reports \
  vigil run --url https://myapp.example.com
```

The Docker image bundles the Go binary, Playwright MCP server, and a Chromium browser.

## CI Integration

Vigil produces JUnit XML, so it integrates with any CI system that supports JUnit test results. Example with GitHub Actions:

```yaml
- name: Run UI tests
  run: |
    docker run --rm \
      -e ANTHROPIC_API_KEY=${{ secrets.ANTHROPIC_API_KEY }} \
      -v ${{ github.workspace }}/specs:/specs \
      -v ${{ github.workspace }}/reports:/reports \
      vigil run --url ${{ env.APP_URL }}

- name: Publish test results
  uses: dorny/test-reporter@v1
  if: always()
  with:
    name: Vigil UI Tests
    path: reports/junit.xml
    reporter: java-junit
```

## Project Structure

```
vigil/
├── cmd/vigil/       # CLI entrypoint
├── internal/
│   ├── agent/       # LLM agent logic (planning, execution loop)
│   ├── config/      # Configuration loading and validation
│   ├── mcp/         # MCP client (Playwright communication)
│   ├── provider/    # LLM provider adapters (anthropic, openai, ...)
│   ├── reporter/    # JUnit XML, HTML, CLI report generation
│   └── spec/        # Spec parsing (markdown + YAML frontmatter)
├── specs/           # Example test specs
├── Dockerfile
├── vigil.yaml       # Example configuration
└── README.md
```

## Build & Run

```sh
# Build
go build -o vigil ./cmd/vigil

# Run spec-driven tests
./vigil run --url https://myapp.example.com --specs ./specs

# Run exploratory tests
./vigil explore --url https://myapp.example.com

# Run with config file
./vigil run --config vigil.yaml
```

## License

MIT
