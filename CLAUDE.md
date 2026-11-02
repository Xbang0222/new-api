# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## MANDATORY: Read AGENTS.md with the Read tool

Do not treat `@AGENTS.md` as loaded. Claude Code does not reliably inline that import.

Before any planning, coding, reviewing, or answering a project question, you MUST call the Read tool on the repo-root file `AGENTS.md` and wait for the full contents. This is the first action of every session and every new task.

Rules:

- Do not start from memory, summaries, or this file alone.
- Do not skip the Read because a previous turn mentioned AGENTS.md.
- Do not replace the Read with a grep, glob, or partial skim.
- After reading, follow every rule in `AGENTS.md` for the rest of the work.
- If the task touches `web/`, also Read `web/AGENTS.md` before editing frontend files.
- If the task touches billing as defined under **Billing rules (mandatory read gate)** in `AGENTS.md`, also Read `.agents/rules/billing.md` in full before planning or editing. Tasks outside that definition may skip it.
- For any feature, fix, or change request, also Read `.agents/rules/workflow.md` and follow it from requirement to deploy.

This file only adds commands and a map of the request path; the rules live in the files above.

## Commands

Backend (repo root). There is no `go.work`; `relaykit/` and `tokenkit/` are separate modules, so run their tests from inside each directory with `GOWORK=off`.

```bash
go run main.go                                        # API + embedded UI on :3000; SQLite one-api.db when SQL_DSN is unset
make test                                             # all three Go modules
go test ./controller -run 'TestIntelligentScheduling' -count=1   # single package / test pattern
cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...
gofmt -l <changed files>                              # must print nothing
```

`main.go` embeds `web/dist` (`//go:embed`), so building or vetting the root `main` package needs `make build-web` (or an existing `web/dist`) first. `make test` deliberately skips the root `main` package for this reason.

Frontend (`web/`, Bun only):

```bash
bun run dev                    # Rsbuild dev server; `make dev-web` serves it on :5173
bun run typecheck              # tsgo -b
bun run lint                   # oxlint (includes the project/intl-locale rule)
bun run test -- src/path/to/file.test.ts   # single vitest file
bun run i18n:sync
```

Full local stack: `make dev` (Docker Compose PostgreSQL + API from `docker-compose.dev.yml`, plus the web dev server). `make reset-setup` clears the first-run setup wizard state.

Pre-merge check (replaces GitHub CI in this fork): `scripts/ci-local.sh` checks a clean temporary checkout of `HEAD`; `--backend` / `--web` run one half while iterating. Deploys go through `scripts/deploy/deploy.sh` as described in `workflow.md`.

## Request path

A relay call crosses several packages; reading them in this order is the fastest way in:

1. `router/relay-router.go` (also `video-router.go`) registers the public endpoints and picks a `types.RelayFormat` per path (OpenAI, Claude, Gemini, Responses, Realtime, ...).
2. `middleware.TokenAuth()` resolves the API token and user; `middleware.Distribute()` (`middleware/distributor.go`) selects a channel for the requested model and user group and stores it in the gin context.
3. `controller/relay.go` `Relay()` builds the `RelayInfo` (`relay/common/relay_info.go`), pre-consumes quota (`service.PreConsumeBilling`), dispatches to the format handler in `relay/*_handler.go`, and loops over channels via `service.RetryParam` when an attempt fails before the response starts.
4. The handler gets the provider via `relay.GetAdaptor(apiType)` (`relay/relay_adaptor.go`). Each provider in `relay/channel/<name>/` implements `channel.Adaptor` (`relay/channel/adapter.go`): `Convert*Request` → `DoRequest` → `DoResponse`, which returns usage.
5. Usage is settled into quota and consume logs by `service/` (see the billing read gate for which files).

Async jobs (video, image, music) go through `relay/relay_task.go` and `relay.GetTaskAdaptor()`, which returns the `channel.TaskAdaptor` in `relay/channel/task/jsplugin/`; that adaptor runs the JavaScript plugin from `plugins/tasks/` via `pkg/jsplugin/`. Background polling and settlement of those tasks is in `service/task_polling.go` and `service/task_billing.go`.

Admin/console APIs follow the plain `router/api-router.go` → `controller/` → `service/` → `model/` layering; runtime options persisted in the `options` table are loaded into `setting/` packages.
