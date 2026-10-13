# Service Layer

Business logic lives here. Handlers call services; services call repositories and optional infra (cache, lock, etc.).

## Guidelines

- Keep business rules in services; keep HTTP binding in handlers
- Inject dependencies via constructors (wired in `internal/app`)
- Prefer returning DTOs for HTTP-facing methods
- Map persistence errors to `internal/errcode` at the service or repository boundary
- Do not set `created_at` / `updated_at` manually when the model implements Bun `BeforeAppendModelHook`

## Sample

`user.go` demonstrates Create / Get / Update / Delete / List with cache-aside on read paths (password never cached).

## Multi API surface

The sample stays flat. When admin and client diverge, split into `service/admin` and `service/client` packages (see AGENTS.md). Shared logic can stay in the root `service` package.
