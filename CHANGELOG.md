## v0.4.2 (Sat, 10 Oct 2026 19:36:09 UTC)
- chore: drop GOWORK=off from the cascade release test env.

## v0.4.1 (Sat, 10 Oct 2026 19:26:44 UTC)
- chore: add a skill cascading a module change to its dependents.
- chore: release the dependents in the cascade skill.
- chore: test cascade dependents against per-module go.mod copies.
- build(deps): update 5 ctx42 dependencies.

## v0.4.0 (Sat, 10 Oct 2026 12:07:46 UTC)
- build: bump testkit and golang.org/x dependencies.
- feat: print the update plan of a changed module.

## v0.3.0 (Fri, 09 Oct 2026 19:45:01 UTC)
- build: bump ctx42 dependencies.
- fix(mod): keep the pattern error in ValidateGlob errors.
- test: assert error causes rather than wrapper prefixes.
- fix(mod): follow a scan root that is a symbolic link.
- feat(mod)!: stop a scan when its context is done.
- fix(cli): let Ctrl-C interrupt the wide map prompt.
- fix(cli): report a failed answer read even after partial input.
- fix(cli): ask about a wide map only on a real terminal.
- test(cli): exercise the wide map question on a pseudo-terminal.
- fix(mod): honour replace directives when resolving.
- fix(view): keep the map pages in a directory only the user can reach.
- fix(view): open the page by its absolute path.
- fix(mod): skip the directories the go command ignores while scanning.
- fix(mod): include the go command's message in query errors.
- fix(mod): report the context error when a query is interrupted.
- fix(mod): run the go command with the process environment for a nil env.
- fix(mod): tolerate go.mod directives this parser does not know.
- fix(conf)!: reject unknown keys in the configuration file.
- fix(conf)!: reject maps sharing an output file or listing an empty dir.
- fix(conf): name the file and map in configuration validation errors.
- test(conf): cover absPath directly.
- docs(graph): name *CycleError as the cycle error type.
- test(graph): compare cycle errors as *CycleError values.
- fix(graph): treat a nil module as one without dependencies.
- fix(cli): keep the previous map when writing a new one fails.
- test(cli): cover option parsing, map generation and writing directly.
- test(conf): cover Config.resolve directly.
- test(svg): cover the map document writers directly.
- test(view): cover privateDir and replaceFile directly.
- test(graph): cover the graph building steps directly.
- test(mod): cover the resolver steps and the directory walk directly.
- build: bump golang.org/x/term to v0.46.0.
- build: bump golang.org/x/sys to v0.49.0 and merge the require blocks.
- feat(mod): add AnyOf to keep modules any of several filters keep.
- feat: draw the selected maps side by side in one image.
- build: drop stale golang.org/x/sys v0.48.0 sums from go.sum.
- docs: lead with go install and correct stale usage notes.

## v0.2.0 (Tue, 29 Sep 2026 07:01:28 UTC)
- feat(web)!: open the map from a file instead of serving it.
- ci: run the tests on every push and pull request.

## v0.1.0 (Fri, 18 Sep 2026 09:11:06 UTC)
- feat: add modmap, a Go module dependency map.
- docs: add the project README.
- docs: license the project under MIT.
- feat: number the modules in the order they must be updated.
- feat: serve the map in a browser with --web.
- docs: describe --web where the options are documented.

