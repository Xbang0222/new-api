# Development Workflow (this fork)

The owner gives a rough direction; the agent turns it into a settled spec, builds it, verifies it on this machine, and hands it back for approval. The owner only answers questions, approves the spec, tests the result on this Mac, and approves the merge, which also releases it to production. Everything runs locally: no GitHub pull requests, GitHub CI, or GitHub-hosted AI review.

Before doing anything that leaves a trace on the owner's computer, the server, or GitHub, tell the owner in plain Simplified Chinese what will happen, where, and why. Talk to the owner in Simplified Chinese.

## 1. Clarify the requirement

- When the owner describes a feature or change direction that is not already a precise spec, use the `grilling` skill before planning or coding. Look up facts in the code yourself; only ask the owner for decisions.
- Do not start implementation until the owner confirms the shared understanding.

## 2. Write the spec and tickets

- Specs and tickets are local markdown under `.scratch/<feature-slug>/` (ignored by Git via `.git/info/exclude`). This repository is public: never publish specs, prices, user data, or business details to GitHub.
- Spec: `.scratch/<feature-slug>/spec.md` (`/to-spec` template). Tickets: `.scratch/<feature-slug>/issues/<NN>-<slug>.md` (`/to-tickets` template), one file per ticket, each with a `Status:` line (`ready-for-agent`, `in-progress`, `done`).
- Show the owner a short plain-language summary of the spec and the ticket list, and wait for approval.

## 3. Implement

- Never commit directly to `main`. Create a local branch per feature: `feat/<slug>`, `fix/<slug>`, or `chore/<slug>`.
- Work ticket by ticket with the `tdd` skill. Commit after each ticket with a conventional message (`feat(scope): ...`).
- Follow every rule in `AGENTS.md`, including the billing, database, authentication, and frontend read gates.

## 4. Verify on this machine

- Run `scripts/ci-local.sh` on the branch. It runs the same checks as `.github/workflows/ci.yml` in a temporary checkout that is deleted afterwards. Use `--backend` or `--web` while iterating; run the full script before handing back.
- Also run `gofmt -l` on changed Go files (must print nothing). If `relaykit/` or `tokenkit/` changed, the script already builds them independently. If database behavior changed, run the three-database matrix required by `AGENTS.md`.
- Then run the `/code-review` skill on the branch diff against `main`, fix every confirmed finding, and re-run the checks.
- The branch must contain the latest `main` (`git merge-base --is-ancestor main HEAD`); otherwise merge `main` into it and re-run the checks, so what the owner tests is what goes live.

## 5. Owner acceptance on this Mac

The owner tests the branch on this Mac before merging; no image is built for it.

- If `web/` changed, rebuild the embedded UI with `cd web && bun run build`.
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
