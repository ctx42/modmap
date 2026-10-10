---
name: cascade
description: >
  Carries the release of a changed Go module through every module of a modmap
  map that depends on it: plans the update rounds, checks the repositories,
  and tests each dependent against the local code before anything is
  released. Use when asked to cascade, propagate, or roll out a module change
  to its dependents.
argument-hint: "<module> <map>"
---

# Cascade

## Usage

```
/cascade <module> <map>   start a migration of <module> limited to <map>
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
- Log every step you take outside a target:
  `gomake :cascade:log --dir ROOT -m <module> -s <step> [-f "<reason>"]`.
  The `:cascade:*` targets log their own steps.

## Start

1. `gomake --list` must show `:cascade:start`; else stop: gomake needs
   rebuilding with gmtool's targets.
2. Write the plan to a temporary file:
   `go run ./cmd/modmap -c modmap.yaml --plan <module> <map> > <tmp>`.
   Show the rounds and the `excluded` modules.
3. `gomake :cascade:start --dir ROOT <tmp>`; it fails while a migration is
   active — stop and say so.

## Phase 1

1. `gomake :cascade:preflight --dir ROOT`. On failure stop and hand back the
   failing modules; the migration stays active.
2. `gomake :cascade:workspace --dir ROOT` prints the workspace path `WORK`.
3. For each module of the plan, in its directory:
   `GOWORK=WORK gomake :go:test --dir ROOT/cascade/tests/<module>`, where
   `<module>` is the module path with `/` replaced by `_` (create the
   directory first). Log `-s test`, with `-f` on failure.
4. On a build or test failure:
   1. Stop and explain the failure from the output.
   2. Propose the fix one change at a time; apply only the changes the user
      accepts. Log `-s fix`.
   3. Run the module's tests again; repeat until they pass or the user stops.
   4. Show the module's `git diff` and wait for the user's review; log
      `-s review`, then go on to the next module.
5. When every module passes, report the modules that carry fixes and stop.
   Phase 1 is done; nothing has been committed or released.

## Self-learning

Obey this skill's lessons when it has any: read a sibling `LESSONS.md`. Most
runs have none. On a correction or self-caught mistake, append a one-line
rule there — general, naming no module of the run.
