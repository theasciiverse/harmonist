# Changes from upstream

This is the Asciiverse packaging of
[Harmonist: Dayoriah Clan Infiltration](https://codeberg.org/anaseto/harmonist)
by anaseto (Yon), a stealth roguelike in Go on the gruid library.
Upstream is the `upstream` remote; this build is upstream's `v1.0.3` tag
(`35159e9`, 2026-08-06). The `asciiverse` branch carries the changes below,
each small enough to offer upstream (UPSTREAM.md).

We ship the terminal version only: tcell, the default build without tags.
No SDL tiles build.

## Licence

`LICENSE` in the repository is an ISC-style permission notice
("Permission to use, copy, modify, and distribute this software for any
purpose with or without fee..."), copyright 2019 Yon. That file is what we
go by, so the catalog lists `ISC`. The itch.io page instead says "Code
license: BSD 2-clause" and "Asset license: Creative Commons Attribution
v4.0 International". Both are permissive; the terminal build ships no
tile images.

## The game

Unchanged: same dungeon, rules, texts and 80x24 screen, drawn at the top
left of a bigger terminal by gruid-tcell, as upstream has it.

## Code

- **State folder** (`io.go`, `DataDir`): the save, config, replays and
  `logs.txt` were kept in `$XDG_DATA_HOME/harmonist` (default
  `~/.local/share/harmonist`). They now live in
  `$XDG_STATE_HOME/harmonist` (default `~/.local/state/harmonist`), the
  XDG folder meant for per-user state such as saves. A folder already in
  the old place is moved across on first use, so nobody loses a game.
  Windows keeps `%LOCALAPPDATA%\harmonist`.

Session logging is upstream's own: every run writes `logs.txt` in that
folder, `-l` adds the game's messages to it, and each game is recorded
as a replay (`harmonist -r _` plays the last one back).

## Packaging

- `.github/workflows/asciiverse-release.yml` (new): av archives for Linux
  (static, x86-64 and ARM64), macOS (Intel and Apple silicon) and Windows
  (x86-64 and ARM64), plus SHA256SUMS, cross-built with cgo off on one
  runner. Upstream has no CI of its own.
- `asciiverse/record-preview.sh`, `asciiverse/drive.py` (new): the
  catalog preview.
- `asciiverse/harmonist.toml`, `asciiverse/harmonist.md`,
  `asciiverse/harmonist.cast` (new): the catalog manifest, store page and
  preview, as drafts for hub.
- `CLAUDE.md`, `PATCHES.md`, `UPSTREAM.md` (new): notes.
