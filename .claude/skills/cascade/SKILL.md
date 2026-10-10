---
name: cascade
description: >
  Carries the release of a changed Go module through every module of a modmap
  map that depends on it: plans the update rounds, checks the repositories,
  tests each dependent against the local code, then, on the user's go-ahead,
  updates, commits, tags, and pushes them one by one. Use when asked to
  cascade, propagate, or roll out a module change to its dependents.
argument-hint: "[<module> <map> | abandon]"
---

# Cascade

## Usage

```
/cascade <module> <map>   start a migration of <module> limited to <map>
/cascade                  resume the active migration
/cascade abandon          end the active migration where it stands
```

Run from the modmap checkout. Every `gomake :cascade:*` command takes
`--dir ~/ws/ctx42/modmap/tmp` (`ROOT` below); the active migration lives in
`ROOT/cascade/`. Report tersely: say which step runs, show only what fails.

## Rules

- One module at a time, in plan order (`ROOT/cascade/plan.json`, round by
  round); stop at the first failure.
- Never commit, tag, push, or edit a `go.mod`/`go.sum` in phase 1.
- Edit code only as a fix the user accepted, and only in modules of the plan.
- Never roll anything back.
- A module marked `skip` (`modes` in `ROOT/cascade/state.json`) is never
  tested, updated, committed, or pushed; one marked `in-place` is never
  tagged.
- Log every step you take outside a target: `gomake :cascade:log --dir ROOT
  -m <module> -s <step> [-v <version>] [-f "<reason>"]`.
  The `:cascade:*` targets log their own steps.

## Start

1. `gomake --list` must show `:cascade:start`; else stop: gomake needs
   rebuilding with gmtool's targets.
2. Write the plan to a temporary file:
   `go run ./cmd/modmap -c modmap.yaml --plan <module> <map> > <tmp>`.
   Show the rounds and the `excluded` modules.
3. `gomake :cascade:start --dir ROOT <tmp>`; it fails while a migration is
   active — stop and say so: `/cascade` resumes it, `/cascade abandon` ends
   it.

## Phase 1

1. `gomake :cascade:preflight --dir ROOT`. For each failing dependent, show
   the reason and ask the user to choose:
   - skip it: left as it is, on its branch, clean and not behind the origin
     branch of the same name if there is one;
   - update it in place: updated on its branch, pushed there without a tag;
     needs that branch on origin;
   - stop, to fix it by hand.

   Record a choice with `gomake :cascade:mark --dir ROOT -m <module>
   --skip` or `--in-place`; when it refuses in place, offer skip or stop.
   A failing changed module, or any stop, ends the run with the migration
   active. Run the preflight again until it passes.
2. `gomake :cascade:modfiles --dir ROOT` writes, per module, a copy of its
   `go.mod` replacing the other plan modules by their local directories.
3. For each module of the plan not skipped, in its directory:
   `gomake :go:test --dir ROOT/cascade/tests/<m> -- -mod=mod
   -modfile=ROOT/cascade/mod/<m>/go.mod`, where `<m>` is the module path
   with `/` replaced by `_` (create the tests directory first). Leave
   `GOWORK` alone: the go commands tests run inherit it. Log `-s test`,
   with `-f` on failure.
4. On a build or test failure:
   1. Stop and explain the failure from the output.
   2. Propose the fix one change at a time; apply only the changes the user
      accepts. Log `-s fix`.
   3. Run the module's tests again; repeat until they pass or the user stops.
   4. Show the module's `git diff` and wait for the user's review; log
      `-s review`, then go on to the next module.
5. When every module passes, report the modules that carry fixes and go to
   the gate. Nothing has been committed or released.

## Gate

1. Ask the user for the go-ahead to release; phase 2 commits, tags, and
   pushes every module of the plan not marked, and commits and pushes the
   in-place ones without a tag. Anything but an explicit yes stops here with
   the migration active.
2. On yes: `gomake :cascade:approve --dir ROOT`. It repeats the sync check,
   letting dependents carry their fixes, and moves the migration to phase 2
   only when every module passes. On failure stop and hand back the failing
   modules: nothing was released, and the migration stays in phase 1.

## Phase 2

Run every command of a module in its directory. Release every module of the
plan not marked, applications included; stop at the first failure, roll
nothing back, and hand back with the failing step logged `-f`.

1. Run `gomake :cascade:env --dir ROOT` once; its `KEY=VALUE` lines are `ENV`
   below.
2. Round 1, the changed module:
   1. With uncommitted changes, stage them all (`git add -A`) and commit
      with `/cm mini apply`; log `-s commit`. Its unpushed commits stay.
   2. `gomake :bump -u`; log `-s bump -v <version>` with the version it
      printed. A bump with nothing to release counts: its current tag is the
      release.
3. Every other module not skipped, in plan order:
   1. `gomake :cascade:update --dir ROOT -m <module>`.
   2. `ENV gomake :go:test --dir ROOT/cascade/tests/<module>`; log `-s test`.
   3. Stage everything (`git add -A`): the phase 1 fixes with `go.mod` and
      `go.sum`. Commit them as one with `/cm mini apply`; log `-s commit`.
   4. `gomake :bump -u`; log `-s bump -v <version>`. In place instead:
      `git push origin HEAD`; log `-s push`.
4. `gomake :cascade:progress --dir ROOT` must end with `complete`; else stop
   and show it. Then `gomake :cascade:finish --dir ROOT` and report each
   module with the version released.

## Resume

Never plan again: the recorded `ROOT/cascade/plan.json` holds for the whole
migration. Without `ROOT/cascade/` there is nothing to resume; say so.

1. `gomake --list` must show `:cascade:start`, as in Start.
2. Read `phase` from `ROOT/cascade/state.json`.
3. Phase 1: run Phase 1 again from step 1. Use `--recheck` on the
   preflight only once testing began (`ROOT/cascade/run.log` has a `test`
   line): only then may dependents carry fixes. Test every module again,
   passed ones included: the code may have changed since.
4. Phase 2: `gomake :cascade:progress --dir ROOT`. On `complete` go to
   Phase 2 step 4. Otherwise run Phase 2 step 1, then continue at the module
   `next:` names, from its first step; skip `/cm` when there is nothing to
   commit. When that module's `pending` line says `release not on origin`,
   stop and hand back: the release is local and needs pushing.

## Abandon

Run `gomake :cascade:abandon --dir ROOT` and report the archive path it
prints. Nothing in the modules is rolled back; say which modules carry
uncommitted or unreleased work.

## Self-learning

Obey this skill's lessons when it has any: read a sibling `LESSONS.md`. Most
runs have none. On a correction or self-caught mistake, append a one-line
rule there — general, naming no module of the run.
