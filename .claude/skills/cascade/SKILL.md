---
name: cascade
description: >
  Carries the release of changed Go modules through every module depending
  on them across the modmap maps, in a session: checks every repository
  before any change, lets the changes be made, then finds the changed
  modules, plans the update rounds, asks which maps to release, tests each
  dependent against the local code, and, on the user's go-ahead, updates,
  commits, tags, and pushes them one by one. Use when asked to cascade,
  propagate, or roll out module changes to their dependents, or to start a
  cascade session.
argument-hint: "[close | abandon]"
---

# Cascade

## Usage

```
/cascade            open a session, cascade its changes, or resume
/cascade close      close the session once nothing is left to release
/cascade abandon    end the active migration where it stands
```

Run from the modmap checkout. Every `gomake :cascade:*` command takes
`--dir ~/ws/ctx42/modmap/tmp` (`ROOT` below); the open session lives in
`ROOT/session/`, the active migration in `ROOT/cascade/`. Report tersely:
say which step runs, show only what fails; never list the modules that
passed. Run each long test or release loop in the background with a
heartbeat on every module or step change: the module, the step, its elapsed
time, and the count done.

`/cascade` picks the section: no `ROOT/session/` → Open; a session but no
`ROOT/cascade/` → Cascade; `ROOT/cascade/` → Resume. A session whose
`session.json` has no `checked` time goes to Open again.

## Rules

- One module at a time, in plan order (`ROOT/cascade/plan.json`, round by
  round); stop at the first failure. The one exception: a phase 1 build or
  test failure runs the fix loop of Phase 1 step 4: the run tests every
  other module first, and goes on once the failed ones pass.
- Never commit, tag, push, or edit a `go.mod`/`go.sum` in phase 1.
- Edit code only as a fix the user accepted, and only in modules of the plan.
  Between migrations the session is open for any change the user asks for.
- Never roll anything back.
- A module marked `skip` (`modes` in `ROOT/cascade/state.json`) is never
  tested, updated, committed, or pushed; one marked `in-place` is never
  tagged.
- Only four things mark a module: the map pick, an ignore setting, the
  user's choice for a module on its own branch, and the user's choice for a
  dependent failing the preflight. Never offer a mark for anything else.
- Settings of a module (`gomake :cascade:set`) hold across sessions:
  `--ignore` leaves it out of every session, `--branch <b>` lets it sit on
  `<b>`; tags are made only on master, so a release of such a module stops
  for the user's choice.
- Log every step you take outside a target: `gomake :cascade:log --dir ROOT
  -m <module> -s <step> [-v <version>] [-f "<reason>"]`.
  The `:cascade:*` targets log their own steps.

## Open

1. `gomake --list` must show `:cascade:open`; else stop: gomake needs
   rebuilding with gmtool's targets.
2. `go run ./cmd/modmap -c modmap.yaml --modules > <tmp>`.
3. `gomake :cascade:set --dir ROOT` lists the saved settings; show the
   `ignore` and `branch` ones.
4. `gomake :cascade:open --dir ROOT <tmp>`. It rebases every repository
   behind origin; ahead ones keep their commits. For each `fail` line show
   the reason; the user fixes it by hand or ignores the module (`gomake
   :cascade:set --dir ROOT -m <module> --ignore`). A rebase conflict was
   aborted: the repository is as it was. Run it again until it passes; never
   fix a repository yourself.
5. Say the session is open: make the changes, in any module, then run
   `/cascade` to cascade them.

## Cascade

1. `gomake :cascade:roots --dir ROOT` fetches every module and prints the
   changed ones (`root` lines) with why: uncommitted changes, commits not on
   origin, commits on origin after the latest release, or a requirement
   older than the latest release of a session module, or of another major
   version tagged in its repository (a `/v2` pin beside a `/v3` clone); it
   never crosses a major version. `no roots`: offer `/cascade
   close`, and stop.
2. Write the plan to a temporary file: `go run ./cmd/modmap -c modmap.yaml
   --plan <root> [--plan <root>...] --lag > <tmp>`, naming every root. It
   spans every map; each module names its `map`. Show the roots, the rounds,
   and the `excluded` modules, grouped by map. From `lag`, show each
   requirement with `breaking` commits or `missing` set, grouped by required
   module: the pinned version, `latest`, and the breaking subjects.
3. Ask which maps to release: any subset of the maps the rounds name, each
   shown with its module and root count. The changes of the others wait in
   the session for a later cascade.
4. `gomake :cascade:start --dir ROOT <tmp>`; it fails while a migration is
   active — stop and say so: `/cascade` resumes it, `/cascade abandon` ends
   it. Ignored modules are marked `skip`. For each `mark` line, a module on
   its own branch, ask the user now, before anything is tested: skip it,
   update it in place, or stop; record it with `gomake :cascade:mark --dir
   ROOT -m <module> --skip` or `--in-place`.
5. Unless every map was picked: `gomake :cascade:pick --dir ROOT <map>...`
   naming the picked maps. `picked changes need changes of unpicked maps:`
   names the roots a picked root requires: show them, and ask for a pick
   adding their maps, or a stop.
6. `gomake :cascade:sync --dir ROOT` rebases every plan module behind origin,
   its uncommitted changes stashed around the rebase. On a `fail` line show
   the reason and stop: the conflict is the user's to resolve, the stash or
   the rebase left as git left it. Run it again once resolved.
7. Go to Phase 1.

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
   It then lifts every requirement of each copy with `go get -u -t ./...`
   and puts back a lift of a module with breaking commits, or one raising
   the `go` directive. Show the `hold`, `down`, and `go` lines; a `lift`
   line only counts what was raised (listed in `lift.txt` beside the copy).
   Phase 2 raises each module to what its copy requires.
3. `gomake :cascade:run --dir ROOT` tests every module of the plan not
   skipped, in its directory, against the local code of the modules before
   it: `go test -race -count=1` with `-timeout 30m` and
   `-modfile=ROOT/cascade/_mod/<m>/go.mod`, where `<m>` is the module path
   with `/` replaced by `_`. It logs each `test`. A module whose files (but
   `go.mod`/`go.sum`), the files of the plan modules it requires, and its
   modfile match its last pass in the session logs `reuse` and is not tested
   again. A failing module does not stop the run: the modules requiring it
   log `blocked`, the rest are tested, and the run ends with `STOPPED
   <first>: some modules failed: <module>: tests failed: see <log>; ...`.
   Run it in the background and report `gomake :cascade:status --dir ROOT`
   as the heartbeat. Leave `GOWORK` alone: the go commands tests run inherit
   it. Record what the user accepts for a module with `gomake :cascade:set
   --dir ROOT -m <module>`: a test flag (`--flag=-p=1`), a variable (`--env
   TZ=UTC`, or `--env PATH=<dir>:$PATH`), or retries (`-r 1`, which rerun
   only the failed packages); without `-m` it holds for every module, and it
   holds in both phases. Settings are saved in `ROOT/settings.json` and
   every later migration starts with them: list them with `gomake
   :cascade:set --dir ROOT` before the first run and add only what is
   missing.
4. On build or test failures, offer only a fix or a stop; never offer
   `skip` or `in-place`:
   1. Stop and explain every failure from the output, all in one report;
      ask about all of them at once.
   2. Propose the fix one change at a time; apply only the changes the user
      accepts. Log `-s fix`, never with `-f`: `-f` marks the step failed.
      When the failure comes from a plan module changing behaviour, not
      only its API, name that module's commit; a fix that drops a test
      accepts the behaviour change, so say so. A failure from a lifted
      requirement names the lift (`lift.txt`).
   3. Run `gomake :cascade:run --dir ROOT` again: it reuses the modules
      unchanged since their pass, so it tests only the fixed, failed, and
      blocked ones; repeat until it passes or the user stops. For one
      module alone: `-m <module> --only`.
   4. Show each fixed module's `git diff` and wait for the user's review;
      log `-s review` for each.
5. Find what phase 1 cannot test: the modfiles replace plan modules by
   local directories, so released versions are never resolved.
   - `gomake :cascade:images --dir ROOT` lists the Docker images whose
     new tag the release needs, and the modules using each; phase 2 builds
     and pushes each right after its module's release.
   - A pin on a plan module at a pseudo-version of a commit gone from it is
     a preflight `note` line; `:cascade:update` writes the release in its
     place, so it needs nothing.
   - Look for any other test or config tying a plan module's `go.mod`
     version to an artifact, such as a `modkit.Ver` lookup.

   Settle each with the user before the gate: a fix as in step 4.
6. When every module passes, write the commit message of every module
   carrying uncommitted changes, the changed modules included:
   `ROOT/cascade/msg/<m>.txt`, its first line `# files:` and the files the
   module's diff touches (`go.mod` and `go.sum` may be left out: phase 2
   adds them), then the message `/cm mini` writes for that diff, the
   dependency update included. `gomake :cascade:messages --dir ROOT` must
   report none `missing` or `invalid`. A module whose only changes phase 2
   makes needs none: it is committed as `chore(deps): update
   dependencies`.
7. Report the modules that carry fixes, the holds of step 2, and the
   findings of step 5, and go to the gate. Nothing has been committed or
   released.

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
and rerun unchanged only on the user's word.

1. `gomake :cascade:run --dir ROOT`. Per module:
   1. `:cascade:update`, raising its plan requirements to their releases
      and the rest to its phase 1 modfile, then the tests with `ENV`:
      `gomake :cascade:env --dir ROOT` prints the go environment; its
      `KEY=VALUE` lines but `GOWORK=off` are `ENV` below. A module whose
      files and those of the plan modules it requires match its phase 1
      pass is only built, its tests compiled but not run: a `build` step.
      Leave `GOWORK` alone: the go commands tests run inherit it. A test
      failing under `ENV` stops the run like any other failure; never
      re-run it with `GOWORK` or `ENV` changed.
   2. Stage everything (`git add -A`): the phase 1 fixes with `go.mod` and
      `go.sum`. Commit them as one, with the message reviewed at the gate.
   3. Marked `in-place`: `git push origin HEAD`; log `-s push`; never bump.
      Otherwise: `gomake :bump -u`; log `-s bump -v <version>`.
   4. A module whose image the release needs: `gomake :docker:image:build`
      and `gomake :docker:image:push` at the release tag, `gomake
      :docker:login` once on a refused push; logs `-s image -v <version>`.
2. On a stop, settle the reason, then run `gomake :cascade:run --dir ROOT -m
   <module>`:
   - `commit message needed`, `commit message lists other files`: write or
     fix the module's message file and show it to the user before the run;
   - `behind origin`: rebase as above;
   - `tests failed`: show the log; rerun only on the user's word;
   - `modfile changed since phase one passed`: the migration needs phase 1
     again; hand back;
   - `image ...`: show the log; a failed push left the release without its
     image: hand back;
   - `image not in the registry`: the image of a released module is
     missing: hand back.
3. `gomake :cascade:progress --dir ROOT` must end with `complete`; else stop
   and show it. Then `gomake :cascade:finish --dir ROOT` and report each
   module with the version released. The session stays open: the changes
   of unpicked maps wait in it, and the user may change more; `/cascade`
   cascades them.

## Resume

Never plan again: the recorded `ROOT/cascade/plan.json` holds for the whole
migration. Without `ROOT/cascade/` there is nothing to resume; go to Open or
Cascade.

1. `gomake --list` must show `:cascade:start`, as in Open.
2. Read `phase` from `ROOT/cascade/state.json`.
3. Phase 1: show the picked maps (`unpicked` in `state.json` lists the
   others) and offer to change the pick; on a change run
   `gomake :cascade:pick --dir ROOT <map>...` naming every map to release.
   Then run Phase 1 again from step 1, passing `--recheck` to the preflight
   when `ROOT/cascade/run.log` has a `test` line and not otherwise: only
   once testing began may dependents carry fixes. Test every module again,
   passed ones included: the run reuses a module only while its files and
   modfile match its last pass, so changed code is always tested again.
4. Phase 2: the pick is fixed; never offer to change it.
   `gomake :cascade:progress --dir ROOT`. On `complete` go to
   Phase 2 step 3. Otherwise `gomake :cascade:run --dir ROOT`: it
   continues at the module `next:` names, from its first step even when
   some steps already ran: `:cascade:update` with nothing to
   update is a no-op, its tests run again, and nothing staged commits
   nothing. When that module's `pending` line says `release not on origin`,
   stop and hand back: the release is local and needs pushing.

## Close

`gomake :cascade:close --dir ROOT` closes the session once `:cascade:roots`
finds nothing left, and prints the archive path. `changed modules left:`
names the roots still waiting: show them and offer `/cascade`. It refuses
while a migration is active.

## Abandon

Run `gomake :cascade:abandon --dir ROOT` and report the archive path it
prints. Nothing in the modules is rolled back; say which modules carry
uncommitted or unreleased work. The session stays open.

## Self-learning

Obey this skill's lessons when it has any: read a sibling `LESSONS.md`. Most
runs have none. On a correction or self-caught mistake, append a one-line
rule there — general, naming no module of the run.
