"""Plays Harmonist for the catalog preview (asciiverse/record-preview.sh).

Reads the tmux pane and presses keys at a watchable pace: through the
intro, then auto-explore (o). When someone is in view, the monkey keeps
out of the way: it waits a turn or two in the shadows, then steps away
from the nearest creature, and explores again once the way is clear.
When a level is explored it travels to the stairs and goes down (e).
Quits without saving (Q) after about 45 seconds.
    python3 -I drive.py <tmux socket> <session> [debug]
"""
import subprocess
import sys
import time

SOCK, SESS = sys.argv[1], sys.argv[2]
DEBUG = len(sys.argv) > 3
LENGTH = 45.0
T0 = time.time()
KEYS = {(-1, 0): "h", (1, 0): "l", (0, -1): "k", (0, 1): "j"}
MAP_TOP, MAP_BOTTOM = 2, 23        # rows of the 80x24 screen holding the map


def tmux(*args):
    return subprocess.run(["tmux", "-L", SOCK, *args], capture_output=True,
                          text=True).stdout


def screen():
    return tmux("capture-pane", "-p", "-t", SESS).split("\n")


def key(k, wait=0.45):
    if DEBUG:
        print(round(time.time() - T0, 1), k, file=sys.stderr)
    tmux("send-keys", "-t", SESS, k)
    time.sleep(wait)


def find(rows, test):
    out = []
    for y in range(MAP_TOP, MAP_BOTTOM):
        row = rows[y][:80] if y < len(rows) else ""
        for x, c in enumerate(row):
            if test(c):
                out.append((x, y))
    return out


BLOCK = set("#¤ Θ♫≈◊")


def cell(rows, x, y):
    if not (MAP_TOP <= y < MAP_BOTTOM) or not (0 <= x < 80) or x >= len(rows[y]):
        return " "
    return rows[y][x]


def flee_step(rows, hero, foes):
    """The step that keeps furthest from the creatures, into foliage if tied."""
    def score(d):
        x, y = hero[0] + d[0], hero[1] + d[1]
        far = min(abs(x - fx) + abs(y - fy) for fx, fy in foes)
        return (far, cell(rows, x, y) == '"')
    moves = [d for d in KEYS if (c := cell(rows, hero[0] + d[0], hero[1] + d[1])) not in BLOCK
             and not c.isalpha()]
    return max(moves, key=score, default=None)


def frontier_step(rows, hero, foes, stuck):
    """First step of the shortest walk to a seen cell next to unseen space."""
    near = {(fx + dx, fy + dy) for fx, fy in foes for dx in (-1, 0, 1) for dy in (-1, 0, 1)}
    seen = {hero: None}
    queue = [hero]
    for pos in queue:
        x, y = pos
        if pos != hero and any(cell(rows, x + dx, y + dy) == " " for dx, dy in KEYS):
            while seen[pos] != hero:
                pos = seen[pos]
            return (pos[0] - hero[0], pos[1] - hero[1])
        for dx, dy in KEYS:
            nxt = (x + dx, y + dy)
            c = cell(rows, *nxt)
            if nxt in seen or c in BLOCK or c.isalpha() or nxt in near or nxt in stuck:
                continue
            seen[nxt] = pos
            queue.append(nxt)
    return None


time.sleep(2.5)                    # title screen
key("Space", 2.0)                  # the story
start = time.time()
waited = 0
last = None
stuck = set()
while time.time() - start < LENGTH:
    rows = screen()
    text = "\n".join(rows[:2])
    if "to continue]" in "\n".join(rows[:3]):
        key("Escape", 0.8)         # a warning waits for a key
        continue
    hero = find(rows, lambda c: c == "@")
    if not hero:                   # a menu or the end screen
        key("Escape", 0.6)
        if not find(screen(), lambda c: c == "@"):
            if DEBUG:
                print("\n".join(screen()), file=sys.stderr)
            break
        continue
    hx, hy = hero[0]
    foes = find(rows, lambda c: c.isalpha())
    if "monsters in view" in text or ("You see" in text and foes):
        # auto-explore refuses while someone is in view: run from anyone
        # close, else walk on by hand towards the nearest unexplored edge
        close = [f for f in foes if abs(f[0] - hx) + abs(f[1] - hy) <= 5]
        if close:
            step = flee_step(rows, (hx, hy), close)
        elif waited < 1:
            step = None
        else:
            step = frontier_step(rows, (hx, hy), foes, stuck)
        key(KEYS[step] if step else ".", 0.45)
        if step and find(screen(), lambda c: c == "@") == hero:
            stuck.add((hx + step[0], hy + step[1]))
        waited += 1
        key("o", 0.6)
        continue
    waited = 0
    if rows == last:               # nothing left to explore: go down
        key("x", 0.4)
        key(">", 0.4)
        key(".", 2.0)
        key("e", 1.5)
    else:
        key("o", 1.0)
    last = rows
key("Escape", 0.5)
key("Q", 1.0)
key("y", 2.0)
for _ in range(6):                 # the end screens, until the game exits
    if tmux("list-sessions").strip() == "":
        break
    key("Escape", 1.2)
