# Development Workflow (this fork)

The owner gives a rough direction; the agent turns it into a settled spec, builds it, verifies it on this machine, and hands it back for approval. The owner only answers questions, approves the spec, tests the result on this Mac, and approves the merge, which also releases it to production. Everything runs locally: no GitHub pull requests, GitHub CI, or GitHub-hosted AI review.

Before doing anything that leaves a trace on the owner's computer, the server, or GitHub, tell the owner in plain Simplified Chinese what will happen, where, and why. Talk to the owner in Simplified Chinese.

## Shared skills and scope

This workflow applies only to this repository. Reusable skill bodies live under `~/.agents/skills/<name>/SKILL.md`: `grilling`, `to-spec`, `to-tickets`, `tdd`, `implement`, and `code-review`. Read the required file directly with the available file-reading tool, including its relevant references. Vendor-specific skill tools and slash commands are optional. Claude entry points link to or read those same bodies. Explicit-only discovery settings do not require another user command when this workflow already directs the agent to read a skill.

Skills supply methods; this file supplies the repository-specific destinations, commands, approvals, and release policy. Do not put this project's CI, database matrix, or deployment policy into global agent instructions. Resolve project rules before applying generic skill defaults. Never publish a spec or ticket to an external tracker as a side effect of a skill.

User approval of a concrete plan covers its spec, ticket breakdown, and test boundaries. Record that approval locally and proceed; do not request it again when switching skills or resuming a ticket. Ask only for a material new decision or an approval that has not been given. Approval to implement is not approval to merge or deploy.

## 1. Clarify the requirement

- When the owner describes a feature or change direction that is not already a precise spec, use the `grilling` skill before planning or coding. Look up facts in the code yourself; only ask the owner for decisions.
- Do not start implementation until the owner confirms the shared understanding.

## 2. Write the spec and tickets

- Specs and tickets are local markdown under `.scratch/<feature-slug>/` (ignored by Git via `.git/info/exclude`). This repository is public: never publish specs, prices, user data, or business details to GitHub.
- Spec: `.scratch/<feature-slug>/spec.md` (`/to-spec` template). Tickets: `.scratch/<feature-slug>/issues/<NN>-<slug>.md` (`/to-tickets` template), one file per ticket, each with a `Status:` line (`ready-for-agent`, `in-progress`, `done`).
- Show the owner a short plain-language summary of the spec, ticket list, and observable test boundaries. Wait for approval only if they have not already approved that concrete plan. Keep ticket status and verification evidence up to date locally.

## 3. Implement

- Never commit directly to `main`. Create a local branch per feature: `feat/<slug>`, `fix/<slug>`, or `chore/<slug>`.
- Preserve pre-existing uncommitted work. Use an isolated worktree when needed; never include unrelated changes in this branch.
- Work ticket by ticket with the `implement` and `tdd` skills. Commit after each ticket with a conventional message (`feat(scope): ...`).
- Follow every rule in `AGENTS.md`, including the billing, database, authentication, and frontend read gates.

## 4. Verify on this machine

- Run `scripts/ci-local.sh` on the branch. It runs the same checks as `.github/workflows/ci.yml` in a temporary checkout that is deleted afterwards. Use `--backend` or `--web` while iterating; run the full script before handing back.
- Also run `gofmt -l` on changed Go files (must print nothing). If `relaykit/` or `tokenkit/` changed, the script already builds them independently. If database behavior changed, run the three-database matrix required by `AGENTS.md`.
- The CI script checks a committed ref (default `HEAD`), not uncommitted edits. Commit the completed ticket changes before the final run and record the exact checked SHA. Run changed-file lint and the production frontend build separately; the script does not replace those checks or the real three-database matrix.
- Read the shared `code-review` skill and have an independent reviewer inspect the branch diff against `main`. A separate review agent is suitable; a client-native local review is also suitable. Do not use GitHub-hosted review. If no independent reviewer is available, report that gate as unverified rather than claiming self-review is independent.
- Record the review base/head, actionable findings, fixes, and re-review outcome in `.scratch/<feature-slug>/review.md`. Fix every confirmed defect, re-review changed behavior, and re-run affected checks and final full CI. Later code changes invalidate the corresponding earlier review and verification.
- The repository-local pre-push hook may invoke `scripts/ci-local.sh` for the pushed commit. Check `core.hooksPath` and existing hooks before installing with `scripts/ci-local.sh --install-hook`; never overwrite another tool's hook. This is an additional check, not merge/deploy approval.
- The branch must contain the latest `main` (`git merge-base --is-ancestor main HEAD`); otherwise merge `main` into it and re-run the checks, so what the owner tests is what goes live.

## 5. Owner acceptance on this Mac

The owner tests the branch on this Mac before merging; no image is built for it.

- If `web/` changed, rebuild the embedded UI with `cd web && bun run build`.
- Check that the acceptance process uses an isolated local test database/configuration, never production credentials; report the chosen port and stop any process you started after acceptance.
- Tell the owner first, then start `go run main.go` in the background. It serves http://localhost:3000 from the local SQLite file `one-api.db`, which holds no production data.
- Give the owner a short plain-language summary: what changed, the `scripts/ci-local.sh` result, what was not verified, and exactly what to click at http://localhost:3000. Stop the process when the owner is done.
- Flag the change as high-risk when it changes the database schema or migrations, billing, or authentication. High-risk changes also go through staging (section 6).

## 6. Merge, release and deploy

When the owner says to merge (合并) after acceptance, that one approval covers merging, releasing and deploying that branch to production. For a high-risk change it covers only up to staging.

- SSH uses the owner's 1Password agent, which cannot prompt for background processes. Check it first with `ssh -O check -o ControlPath=$HOME/.ssh/cm-netcup netcup`; if it is not running, ask the owner to run `scripts/deploy/deploy.sh login` in a visible terminal (one fingerprint, valid 8 hours).
- `git switch main && git merge --no-ff <branch>`, run `scripts/ci-local.sh` on `main` (no failures beyond those the branch already had), then `git push github main` and delete the merged local branch.
- Run `scripts/deploy/deploy.sh release ["<notes>"]` in the background (10-30 minutes). It runs the `Ruoli Publish` workflow (tag, GitHub release, `xbang0222/new-api:<tag>` image build), waits for the image, and deploys it to production: it backs up the database, switches the image, health-checks, and rolls back automatically on failure. Report the tag and the result. This is the only GitHub automation this workflow uses; the Docker build needs the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` repository secrets.
- High-risk change: run `scripts/deploy/deploy.sh release --staging ["<notes>"]` instead, then `scripts/deploy/deploy.sh tunnel` so the owner can check it at http://localhost:3041. Deploy to production with `scripts/deploy/deploy.sh prod <tag>` only after the owner says to go live (上线). Staging is isolated: its own SQLite database, no access to production MySQL or Redis. To redeploy an existing tag to staging: `scripts/deploy/deploy.sh staging <tag>`.
- Manual rollback: `scripts/deploy/deploy.sh prod <previous-tag>`.
- Never read production database contents or credentials outside `deploy.sh`.
