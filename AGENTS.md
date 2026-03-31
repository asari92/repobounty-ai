# AGENTS.md

Guidance for agentic coding agents working in this repository.

## Project Overview

RepoBounty AI — Solana hackathon MVP. Sponsors fund public GitHub repos; after a deadline, the system fetches contributor data, runs AI-based reward allocation, and finalizes on-chain. Three components: Go backend, React frontend, Anchor/Solana program.

## Build & Run Commands

### Backend (Go 1.25)

```bash
cd backend
go mod tidy
go run ./cmd/api              # Start API server on :8080
go build -o main ./cmd/api    # Build binary
```

There are no test files currently. To run tests when added:
```bash
go test ./...                          # All tests
go test ./internal/store/...           # Single package
go test -run TestName ./internal/...   # Single test
```

### Frontend (React 18 + Vite + TypeScript)

```bash
cd frontend
npm install
npm run dev              # Dev server on :3000 (proxies /api to :8080)
npm run build            # Type-check + production build
npm run lint             # ESLint check
npm run lint:fix         # ESLint auto-fix
npm run format           # Prettier write
npm run format:check     # Prettier check
```

No test runner is configured. When adding tests, prefer Vitest (`npm install -D vitest`).

### Solana Program (Anchor 0.30.1)

```bash
cd program
yarn install
anchor build              # Build program (use --no-idl in Docker)
anchor test               # Run all tests on localnet
anchor deploy --provider.cluster devnet
```

Single test file: `yarn run ts-mocha -p ./tsconfig.json -t 1000000 tests/repobounty.ts`

### Full Stack (Docker)

```bash
docker compose up --build
# Frontend: http://localhost:5173  Backend: http://localhost:8080
```

## Code Style — Backend (Go)

### Imports

Three groups, separated by blank lines:
1. Standard library (`context`, `fmt`, `net/http`, etc.)
2. Third-party (`github.com/go-chi/chi/v5`, `go.uber.org/zap`, etc.)
3. Internal (`github.com/repobounty/repobounty-ai/internal/...`)

Use alias for package name conflicts: `handler "github.com/repobounty/repobounty-ai/internal/http"`

### Formatting & Naming

- `gofmt` standard: tabs for indentation, no trailing whitespace.
- Exported names: PascalCase. Unexported: camelCase.
- Acronyms stay uppercase: `rpcClient`, `TxSignature`, `PDA`.
- JSON field tags: `snake_case` (e.g., `json:"campaign_id"`).
- `omitempty` on optional response fields (e.g., `FinalizedAt`, `TxSignature`).

### Types & Structs

- Type aliases for domain concepts: `type CampaignState string` with typed constants.
- Use `uint64` for pool amounts/lamports, `uint16` for basis points, `int64` for Unix timestamps.
- All API types defined in `internal/models/` — separate request, response, and domain types.
- Allocations always use basis points (10000 = 100%).

### Error Handling

- Return `fmt.Errorf("context: %w", err)` for wrapped errors.
- Sentinel errors as package-level `var` values: `var ErrNotFound = errors.New("...")`.
- Use `errors.Is(err, store.ErrNotFound)` for error checking, not string comparison.
- Handlers use `writeError(w, status, msg)` — never expose internal error details to clients.
- Log internal errors with `log.Printf` or `zap`; return generic messages in HTTP responses.

### Concurrency

- `sync.RWMutex` for shared state (see `store/memory.go`).
- Lock for writes (`Lock`/`Unlock`), RLock for reads (`RLock`/`RUnlock`).
- Return defensive copies from store methods (see `copycamp` helper).

### Architecture

- Entry point: `cmd/api/main.go` — wires dependencies, starts server with graceful shutdown.
- Services in `internal/` packages: `config`, `models`, `store`, `github`, `ai`, `solana`, `http`.
- Handler pattern: `Handlers` struct holds service interfaces; methods are Chi endpoint handlers.
- Middleware in `internal/http/middleware.go`: CORS, rate limiting, structured logging (zap), recovery.
- Graceful degradation: GitHub/AI/Solana services fall back to mock/deterministic modes when unconfigured.

## Code Style — Frontend (React + TypeScript)

### Formatting (Prettier)

- Single quotes, semicolons, trailing commas (es5), 100 char print width, 2-space indent.
- Arrow parens always: `(x) => x`.

### TypeScript

- Strict mode enabled (`strict: true`).
- `noUnusedLocals` and `noUnusedParameters` — no dead code.
- Use `interface` for data types (see `src/types/index.ts`).
- Use `import type { ... }` for type-only imports.
- Union string types for state: `"created" | "finalized"`.
- JSON fields use `snake_case` to match backend API responses.

### Components & Pages

- One component per file. Default export for page components.
- File structure: `src/api/` (fetch client), `src/components/` (reusable), `src/pages/` (routes), `src/types/` (shared interfaces), `src/hooks/` (custom hooks), `src/idl/` (Solana IDL).
- Tailwind CSS with Solana theme colors: `solana-purple`, `solana-green`, `solana-dark`, `solana-card`, `solana-border`.

### API Client

- Centralized in `src/api/client.ts` using a generic `request<T>` fetch wrapper.
- All API calls go through `api` exported object — no raw `fetch` in components.

## Code Style — Solana Program (Rust + Anchor)

- Anchor 0.30.1 with `declare_id!` macro.
- Constants at top: `MAX_REPO_LEN`, `MAX_ALLOCATIONS`, `BPS_100`.
- Structured in sections: Program, Accounts, State, Errors (with `// ---` section headers).
- Error codes use `#[error_code]` with descriptive `#[msg("...")]` strings.
- Account validation via `#[derive(Accounts)]` with constraints (`has_one`, `seeds`, `bump`).
- Space calculation as an explicit `space()` method — always account for discriminator (8 bytes).
- Tests in `tests/` using ts-mocha + Chai + `@coral-xyz/anchor`.

## Key Domain Rules

- Allocations use basis points: percentages always sum to 10000 (100%).
- Max 10 allocations per campaign. No duplicate contributors in a single allocation set.
- Campaign PDA seeds: `["campaign", authority_pubkey, campaign_id_bytes]`.
- Program ID: `97t3t188wnRoogkD8SoZKWaWbP9qDdN9gUwS4Bdw7Qdo`.
- Campaign states: `created` → `finalized` (one-way transition).
- The backend works without external keys — mock data and deterministic allocation kick in automatically.

## Environment

See `backend/.env.example`. Key variables: `PORT`, `GITHUB_TOKEN`, `OPENROUTER_API_KEY`, `MODEL`, `SOLANA_RPC_URL`, `SOLANA_PRIVATE_KEY`, `PROGRAM_ID`.
