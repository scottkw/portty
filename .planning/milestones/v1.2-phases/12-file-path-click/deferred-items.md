# Phase 12 deferred items

## From 12-03 (found at its gate, not caused by it)

- `internal/daemon` `TestCLILink_SurvivesZshLoginPathHelper` and `TestCLILink_SurvivesZshLoginPathHelperWithShim` fail on this host: `command -v portty` prints `/usr/local/bin/portty`, not the test's temp per-user bin dir. `/usr/local/bin/portty` is a symlink to `/Applications/Portty.app/Contents/MacOS/Portty`, created Nov 21 17:11 (an installed Portty on the host). macOS `path_helper` puts `/usr/local/bin` ahead of the test's bin dir, so an installed Portty shadows the fixture. The tests (Phase 11-03 and 12-02) assume no `portty` on the login PATH's system dirs. 12-03 touches no file in `internal/daemon`. Fix belongs to a test-hygiene change (look the system dirs up with `PATH=/usr/bin:/bin` only, or skip when `/usr/local/bin/portty` exists), not to this plan.
  - **Resolved (orchestrator, after 12-03):** both tests now expect the first executable `portty` on the login PATH (`firstPortty`), so an installed copy earlier on the PATH is accepted; the PATH-order assertion (bin dir after `/usr/bin`) is unchanged. Product behavior is correct: an installed `/usr/local/bin/portty` should win.
  status: acknowledged
