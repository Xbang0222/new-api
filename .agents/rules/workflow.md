# Development Workflow (this fork)

The owner gives a rough direction; the agent turns it into a settled spec, builds it, verifies it, and hands back a PR. The owner only answers questions, approves the spec, merges the PR, and approves production deploys.

## 1. Clarify the requirement

- When the owner describes a feature or change direction that is not already a precise spec, use the `grilling` skill before planning or coding. Look up facts in the code yourself; only ask the owner for decisions.
- Do not start implementation until the owner confirms the shared understanding.

## 2. Write the spec and tickets

- Specs and tickets are local markdown under `.scratch/<feature-slug>/` (ignored by Git via `.git/info/exclude`). This repository is public: never publish specs, prices, user data, or business details to GitHub issues.
- Spec: `.scratch/<feature-slug>/spec.md` (`/to-spec` template). Tickets: `.scratch/<feature-slug>/issues/<NN>-<slug>.md` (`/to-tickets` template), one file per ticket, each with a `Status:` line (`ready-for-agent`, `in-progress`, `done`).
- Show the owner a short plain-language summary of the spec and the ticket list, and wait for approval.

## 3. Implement

- Never commit directly to `main`. Create a branch per feature: `feat/<slug>`, `fix/<slug>`, or `chore/<slug>`.
- Work ticket by ticket with the `tdd` skill. Commit after each ticket with a conventional message (`feat(scope): ...`).
- Follow every rule in `AGENTS.md`, including the billing, database, authentication, and frontend read gates.

## 4. Verify before handing back

Run and record the actual results of every check that applies:

- Backend: `gofmt -l` on changed Go files (must print nothing), `go vet ./...`, and `make test` (needs `web/dist/index.html`; create a placeholder if `web/dist` is missing).
- `relaykit/` or `tokenkit/` touched: `GOWORK=off go build ./...` inside that module.
- `web/` touched: `bun run typecheck` and `bun run test` in `web/`.
- Database behavior touched: the three-database matrix required by `AGENTS.md`.
- Then run the `/code-review` skill on the branch diff against `main`, fix every confirmed finding, and re-run the checks.

## 5. Hand back

- Push the branch and open a PR to `Xbang0222/new-api` `main` following the PR rules in `AGENTS.md`. Wait for the `CI` workflow to pass; fix failures on the branch.
- Give the owner a short plain-language summary: what changed, what was verified, what was not, and anything they should click through on staging.
- The owner merges. Do not merge PRs yourself unless the owner explicitly asks.

## 6. Release and deploy

- Release: the owner runs the `Ruoli Publish` workflow (or asks the agent to run `gh workflow run ruoli-publish.yml`), which builds the `xbang0222/new-api:<tag>` image.
- SSH uses the owner's 1Password agent, which cannot prompt for background processes. If `ssh netcup` hangs, ask the owner to run `scripts/deploy/deploy.sh login` in a visible terminal (one fingerprint, valid 8 hours); later commands reuse that connection.
- Staging first: `scripts/deploy/deploy.sh staging <tag>`, then `scripts/deploy/deploy.sh tunnel` so the owner can check it at http://localhost:3041. Staging is isolated: its own SQLite database, no access to production MySQL or Redis.
- Never read production database contents or credentials outside `deploy.sh`.
- Production only after the owner explicitly approves that tag: `scripts/deploy/deploy.sh prod <tag>`. The script backs up the database, switches the image, health-checks, and rolls back automatically on failure.
- Manual rollback: `scripts/deploy/deploy.sh prod <previous-tag>`.
