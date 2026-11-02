# Development Workflow (this fork)

The owner gives a rough direction; the agent turns it into a settled spec, builds it, verifies it on this machine, and hands it back for approval. The owner only answers questions, approves the spec, approves the merge, and approves production deploys. Everything runs locally: no GitHub pull requests, GitHub CI, or GitHub-hosted AI review.

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

## 5. Hand back and merge

- Give the owner a short plain-language summary: what changed, the `scripts/ci-local.sh` result, what was not verified, and what to click through on staging.
- Only after the owner approves: `git switch main && git merge --no-ff <branch>`, run `scripts/ci-local.sh` once more on `main`, then `git push github main`. Delete the merged local branch.

## 6. Release and deploy

Release is semi-automatic: one command takes merged `main` to staging; production waits for the owner.

- SSH uses the owner's 1Password agent, which cannot prompt for background processes. If `ssh netcup` hangs or `release` reports no SSH connection, ask the owner to run `scripts/deploy/deploy.sh login` in a visible terminal (one fingerprint, valid 8 hours); later commands reuse that connection.
- After the merge is pushed and the owner approves releasing it, run `scripts/deploy/deploy.sh release ["<notes>"]` in the background (it takes 10-20 minutes). It runs the `Ruoli Publish` workflow (tag, GitHub release, `xbang0222/new-api:<tag>` image build), waits for the image, and deploys it to staging. This is the only GitHub automation this workflow uses; the Docker build needs the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` repository secrets.
- Then `scripts/deploy/deploy.sh tunnel` so the owner can check staging at http://localhost:3041. To redeploy an existing tag to staging only: `scripts/deploy/deploy.sh staging <tag>`. Staging is isolated: its own SQLite database, no access to production MySQL or Redis.
- Production only after the owner explicitly approves that tag: `scripts/deploy/deploy.sh prod <tag>`. The script backs up the database, switches the image, health-checks, and rolls back automatically on failure.
- Manual rollback: `scripts/deploy/deploy.sh prod <previous-tag>`.
- Never read production database contents or credentials outside `deploy.sh`.
