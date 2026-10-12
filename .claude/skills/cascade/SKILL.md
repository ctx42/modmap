---
name: cascade
description: >
  Carries the release of a changed Go module through every module depending
  on it across the modmap maps: plans the update rounds, asks which maps to
  release, checks the repositories, tests each dependent against the local
  code, then, on the user's go-ahead, updates, commits, tags, and pushes them
  one by one. Use when asked to cascade, propagate, or roll out a module
  change to its dependents.
argument-hint: "[<module> | abandon]"
---

# Cascade

## Usage

```
/cascade <module>   start a migration of <module> across every map
/cascade            resume the active migration
/cascade abandon    end the active migration where it stands
```

Run from the modmap checkout. Every `gomake :cascade:*` command takes
`--dir ~/ws/ctx42/modmap/tmp` (`ROOT` below); the active migration lives in
`ROOT/cascade/`. Report tersely: say which step runs, show only what fails;
never list the modules that passed. Run each long test or release loop in
the background with a heartbeat on every module or step change: the module,
the step, its elapsed time, and the count done.

## Rules

- One module at a time, in plan order (`ROOT/cascade/plan.json`, round by
  round); stop at the first failure. The one exception: a phase 1 build or
  test failure runs the fix loop of Phase 1 step 4, and the run goes on
  once the module passes.
- Never commit, tag, push, or edit a `go.mod`/`go.sum` in phase 1.
- Edit code only as a fix the user accepted, and only in modules of the plan.
- Never roll anything back.
- A module marked `skip` (`modes` in `ROOT/cascade/state.json`) is never
  tested, updated, committed, or pushed; one marked `in-place` is never
  tagged.
- Only two things mark a module: the map pick, and the user's choice for a
  dependent failing the preflight. Never offer a mark for anything else.
- Log every step you take outside a target: `gomake :cascade:log --dir ROOT
  -m <module> -s <step> [-v <version>] [-f "<reason>"]`.
  The `:cascade:*` targets log their own steps.

## Start

1. `gomake --list` must show `:cascade:start`; else stop: gomake needs
   rebuilding with gmtool's targets.
2. Write the plan to a temporary file:
   `go run ./cmd/modmap -c modmap.yaml --plan <module> --lag > <tmp>`. It
   spans every map; each module names its `map`. Show the rounds and the
   `excluded` modules, grouped by map. From `lag`, show each requirement
   with `breaking` commits or `missing` set, grouped by required module:
   the pinned version, `latest`, and the breaking subjects. Those are the
   changes phase 1 will meet beyond the changed module.
3. Ask which maps to release: any subset of the maps the rounds name, each
   shown with its module count. Every module of a map left out is marked
   `skip`; the changed module is always released.
4. `gomake :cascade:start --dir ROOT <tmp>`; it fails while a migration is
   active — stop and say so: `/cascade` resumes it, `/cascade abandon` ends
   it.
5. Unless every map was picked: `gomake :cascade:pick --dir ROOT <map>...`
   naming the picked maps; with none picked, name no map.

## Phase 1

1. `gomake :cascade:preflight --dir ROOT`. For each failing dependent, show
   the reason and ask the user to choose:
   - skip it: left as it is, on its branch, clean and not behind the origin
     branch of the same name if there is one;
   - update it in place: updated on its branch, pushed there without a tag;
     needs that branch on origin;
   - stop, to fix it by hand.

   Record a choice with `gomake :cascade:mark --dir ROOT -m <module>
   --skip` or `--in-place`, adding `--recheck` when the preflight ran with
   it; when it refuses in place, offer skip or stop. The modules of an
   unpicked map are not checked; the preflight lists them as `skip ...
   (unpicked)`. A failing changed module, or any stop, ends the run with the
   migration active. Run the preflight again until it passes.
2. `gomake :cascade:modfiles --dir ROOT` writes, per module, a copy of its
   `go.mod` replacing the other plan modules by their local directories,
   but the marked ones: their dependents test against their last release.
3. `gomake :cascade:run --dir ROOT` tests every module of the plan not
   skipped, in its directory, against the local code of the modules before
   it: `gomake :go:test` with `-timeout 30m` and
   `-modfile=ROOT/cascade/_mod/<m>/go.mod`, where `<m>` is the module path
   with `/` replaced by `_`. It logs each `test` and stops at the first
   failure: `STOPPED <module>: tests failed: see <log>`. Run it in the
   background and report `gomake :cascade:status --dir ROOT` as the
   heartbeat. Leave `GOWORK` alone: the go commands tests run inherit it.
   Record what the user accepts for a module with `gomake :cascade:set
   --dir ROOT -m <module>`: a test flag (`--flag=-p=1`), a variable
   (`--env TZ=UTC`, or `--env PATH=<dir>:$PATH`), or retries (`-r 1`);
   without `-m` it holds for every module, and it holds in both phases.
4. On a build or test failure, offer only a fix or a stop; never offer
   `skip` or `in-place`:
   1. Stop and explain the failure from the output.
   2. Propose the fix one change at a time; apply only the changes the user
      accepts. Log `-s fix`, never with `-f`: `-f` marks the step failed.
      When the failure comes from a plan module changing behaviour, not
      only its API, name that module's commit; a fix that drops a test
      accepts the behaviour change, so say so.
   3. Run `gomake :cascade:run --dir ROOT -m <module> --only` again; repeat
      until it passes or the user stops.
   4. Show the module's `git diff` and wait for the user's review; log
      `-s review`, then go on with `gomake :cascade:run --dir ROOT -m
      <next module>`.
5. Find what phase 1 cannot test: the modfiles replace plan modules by
   local directories, so released versions are never resolved.
   - `gomake :cascade:images --dir ROOT` lists the Docker images whose
     new tag the release needs, and the modules using each; a test or
     config tying the image tag to the `go.mod` version fails until it is
     published.
   - A pin on a plan module at a pseudo-version of a commit gone from it is
     a preflight `note` line; `:cascade:update` writes the release in its
     place, so it needs nothing.
   - Look for any other test or config tying a plan module's `go.mod`
     version to an artifact, such as a `modkit.Ver` lookup.

   Settle each with the user before the gate: a fix as in step 4, or an
   image the user publishes during phase 2.
6. When every module passes, write the commit message of every module
   carrying uncommitted changes, the changed module included:
   `ROOT/cascade/msg/<m>.txt`, its first line `# files:` and the files the
   module's diff touches (`go.mod` and `go.sum` may be left out: phase 2
   adds them), then the message `/cm mini` writes for that diff, the
   dependency update included. `gomake :cascade:messages --dir ROOT` must
   report none `missing` or `invalid`. A module whose only changes phase 2
   makes needs none: it is committed as `chore(deps): update
   dependencies`.
7. Report the modules that carry fixes and the findings of step 5, and go
   to the gate. Nothing has been committed or released.

## Gate

1. Ask the user for the go-ahead to release; phase 2 commits, tags, and
   pushes every module of the plan not marked, and commits and pushes the
   in-place ones without a tag. Show the commit messages `gomake
   :cascade:messages --dir ROOT` prints with the question, so the go-ahead
   covers them; make a change the user asks for in the message file, and
   ask again. Anything but an explicit yes stops here with the migration
   active.
2. On yes: `gomake :cascade:approve --dir ROOT`. It repeats the sync check,
   letting dependents carry their fixes, and moves the migration to phase 2
   only when every module passes. On failure stop and hand back the failing
   modules: nothing was released, and the migration stays in phase 1.

## Phase 2

`gomake :cascade:run --dir ROOT` releases every module of the plan not marked
`skip`, applications included, pushing the `in-place` ones without a tag. It
starts at the first module not released, runs each one's steps in its
directory, logs them, and stops at the first failure with `STOPPED <module>:
<reason>`; roll nothing back. Run it in the background with the heartbeat of
`gomake :cascade:status --dir ROOT`. A `gomake :bump -u` with nothing to
release counts, for every module: its current tag is the release.

Rebase, never merge: `git fetch origin` before a module's first step and
again before its bump or push; the run does both and stops with `behind
origin`. Then `git pull --rebase` onto it and run the module again from its
first step; a local tag on an unpushed commit stops the run instead. Never
bump or push a tree behind origin: the bump pushes its tag even when origin
rejects the branch.

After each bump, the release must be the highest version tag on origin
(`git ls-remote --tags origin`); a higher one stops the run, handed back
with both versions. A bump failing at its push may have pushed the tag:
say which refs landed. A test failing here that passed in phase 1 is shown
and rerun unchanged only on the user's word. `:cascade:update` failing with
`image not in the registry` changed nothing: name the image, wait for the
user to publish it, then resume at that module.

1. `gomake :cascade:run --dir ROOT`. Per module:
   1. Unless it is the changed module, `:cascade:update`, then the tests
      with `ENV`: `gomake :cascade:env --dir ROOT` prints the go
      environment; its `KEY=VALUE` lines but `GOWORK=off` are `ENV` below.
      Leave `GOWORK` alone: the go commands tests run inherit it. A test
      failing under `ENV` stops the run like any other failure; never
      re-run it with `GOWORK` or `ENV` changed.
   2. Stage everything (`git add -A`): the phase 1 fixes with `go.mod` and
      `go.sum`. Commit them as one, with the message reviewed at the gate.
   3. Marked `in-place`: `git push origin HEAD`; log `-s push`; never bump.
      Otherwise: `gomake :bump -u`; log `-s bump -v <version>`.
2. On a stop, settle the reason, then run `gomake :cascade:run --dir ROOT -m
   <module>`:
   - `commit message needed`, `commit message lists other files`: write or
     fix the module's message file and show it to the user before the run;
   - `behind origin`: rebase as above;
   - `tests failed`: show the log; rerun only on the user's word;
   - `image not in the registry`: wait for the user to publish it.
3. `gomake :cascade:progress --dir ROOT` must end with `complete`; else stop
   and show it. Then `gomake :cascade:finish --dir ROOT` and report each
   module with the version released.

## Resume

Never plan again: the recorded `ROOT/cascade/plan.json` holds for the whole
migration. Without `ROOT/cascade/` there is nothing to resume; say so.

1. `gomake --list` must show `:cascade:start`, as in Start.
2. Read `phase` from `ROOT/cascade/state.json`.
3. Phase 1: show the picked maps (`unpicked` in `state.json` lists the
   others) and offer to change the pick; on a change run
   `gomake :cascade:pick --dir ROOT <map>...` naming every map to release.
   Then run Phase 1 again from step 1, passing `--recheck` to the preflight
   when `ROOT/cascade/run.log` has a `test` line and not otherwise: only
   once testing began may dependents carry fixes. Test every module again,
   passed ones included: the code may have changed since.
4. Phase 2: the pick is fixed; never offer to change it.
   `gomake :cascade:progress --dir ROOT`. On `complete` go to
   Phase 2 step 3. Otherwise `gomake :cascade:run --dir ROOT`: it
   continues at the module `next:` names, from its first step even when
   some steps already ran: `:cascade:update` with nothing to
   update is a no-op, its tests run again, and nothing staged commits
   nothing. When that module's `pending` line says `release not on origin`,
   stop and hand back: the release is local and needs pushing.

## Abandon

Run `gomake :cascade:abandon --dir ROOT` and report the archive path it
prints. Nothing in the modules is rolled back; say which modules carry
uncommitted or unreleased work.

## Self-learning

Obey this skill's lessons when it has any: read a sibling `LESSONS.md`. Most
runs have none. On a correction or self-caught mistake, append a one-line
rule there — general, naming no module of the run.
