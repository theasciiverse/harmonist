# Harmonist (Asciiverse packaging)

anaseto's Harmonist: Dayoriah Clan Infiltration (Go, gruid, ISC per the
repository's LICENSE), packaged for the Asciiverse catalog (id
`harmonist`, kind `game`, genre `roguelike`). Not our code: the game
stays as its author made it; list every change in PATCHES.md and keep
each one offerable upstream (UPSTREAM.md).

- `upstream` is codeberg.org/anaseto/harmonist (branch `master`); ours is
  branch `asciiverse` (from the `v1.0.3` tag), `origin`
  github.com/theasciiverse/harmonist (private).
- Build: `CGO_ENABLED=0 go build -trimpath .` (terminal version, tcell;
  never `-tags sdl`). Go from go.mod, 1.24+.
- Release: tag `v<upstream version>-av.<n>` (first `v1.0.3-av.1`) and push
  it; `.github/workflows/asciiverse-release.yml` cross-builds Linux, macOS
  and Windows on one runner.
- Save, config, replays and `logs.txt` live in `$XDG_STATE_HOME/harmonist`.
  Tests and scripted runs always set scratch `HOME`, `XDG_STATE_HOME` and
  `XDG_DATA_HOME` in the same command (the old data folder is moved on
  first use), under `systemd-run --user --scope -p MemoryMax=4G`.
- The screen is 80x24, drawn at the top left of bigger terminals.
- Preview: `asciiverse/record-preview.sh ./harmonist`.
- Catalog drafts: `asciiverse/harmonist.toml` and `.md`; hub pushes are
  done from beast.
