# Coding Guidelines

## Language & Style

- Don't add comments unless absolutely necessary — code should be self-explanatory
- Prefer functional style and composition over inheritance — use interfaces to abstract implementation details
- Write type-safe and robust code
- Prefer immutability (e.g. record classes in Java, readonly in TypeScript)
- Keep functions, classes, and files short and focused
- Use Go-like naming conventions (e.g. URL not Url, ID not Id)
- Prefer robust languages: TypeScript over JavaScript, Python with type hints over Bash
- Prefer early returns, keep nesting to a minimum
- Avoid magic numbers or strings — use named constants
- Enforce formatting automatically: Prettier for TypeScript, `gofmt` for Go

## Error Handling

- Fail fast — don't silently swallow errors
- Use explicit, typed result/error types (e.g. Result in Kotlin) — no unchecked runtime exceptions
- Define custom error types when it improves readability
- Log errors with context (relevant state/identifiers, not just the message)
- Use structured logging (JSON) with appropriate log levels
- Prefer idempotent operations
- Use correct HTTP status codes

## Testing

- Ensure high test coverage (80%+) with adequate branch coverage
- Test on all levels: unit, integration, API, and UI
- Avoid flaky tests — keep them deterministic, independent, and fast
- Keep test names descriptive and consistent
- Follow Arrange-Act-Assert pattern
- Test edge cases and error paths, not just happy paths
- Use mocks for external dependencies in unit tests
- Use test containers for integration tests

## Dependencies

- Prefer stdlib where possible, otherwise use battle-tested libraries compatible with existing dependencies
- Vet dependencies before adding (maintenance, security, size)
- Keep dependencies up to date but pin versions — don't use latest
- Automate dependency updates with Renovate — group patch updates, review minor/major individually
- Avoid duplicate dependencies that overlap with ones already in use

## Code Organization

- Use modular architecture — keep related logic together (Go-like structure)
- Maintain a clear, consistent project structure
- Keep public API surface small — only expose what's needed
- Separate configuration from code — use environment variables or config files
- Follow 12-factor app principles closely
- Projects should be Kubernetes-ready from the start — include a dedicated local dev Kubernetes config using Skaffold

## API Design

- Prefer REST for CRUD-heavy APIs, gRPC for internal high-throughput services
- Version APIs from day one — use URL path versioning (`/v1/`) for REST
- Use cursor-based pagination for unbounded collections
- Return problem detail objects for errors (RFC 9457)

## Async & Concurrency

- Prefer async/await over raw threads or callbacks
- Avoid shared mutable state — communicate via channels or message queues
- Always set timeouts and cancellation contexts on I/O operations
- Document thread-safety assumptions on any shared data structure

## Observability

- Use OpenTelemetry for distributed tracing — instrument at service boundaries
- Expose a `/metrics` endpoint (Prometheus-compatible) for all services
- Include correlation/trace IDs in log entries to link logs across services
- Define SLOs and alert on them, not on raw resource metrics

## Git

- Use conventional commits — include GitHub issue as scope if applicable (e.g. `feat(#100): add foo`)
- Keep commits small and atomic with meaningful, short messages
- Name branches `feat_foo` or `feat/100_foo` (with GitHub issue)

## Security

- Never hardcode secrets — use environment variables in local dev, Vault in all other environments
- Validate all external input
- Sanitize outputs to prevent XSS/injection
- Use parameterized queries — never concatenate SQL
- Use HTTPS by default
- Follow least privilege principle
- Keep dependencies patched for CVEs

## Documentation

- Keep READMEs short and maintainable — include: description, architecture, build & run commands, configuration, public APIs
- Include at minimum a component overview and project structure in the architecture section
- Use ADRs for important architectural decisions

## CI/CD

- Run the pipeline in sequence: lint, test, build, release, push to container registry
- Generate CHANGELOG from conventional commits
- Lint, test, and build locally before pushing
- Use container-based builds with multi-stage Dockerfiles
- Scan container images for vulnerabilities
