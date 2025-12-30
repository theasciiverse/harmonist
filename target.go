package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/ui"
)

var invalidPos = gruid.Point{-1, -1}

type examination struct {
	p            gruid.Point
	nmonster     int
	objects      []gruid.Point
	nobject      int
	sortedStairs []gruid.Point
	stairIndex   int
	info         posInfo
	scroll       bool
}

// HideCursor hides the target cursor.
func (md *model) HideCursor() {
	md.targ.ex.p = invalidPos
}

// SetCursor sets the target cursor.
func (md *model) SetCursor(p gruid.Point) {
	md.targ.ex.p = p
}

// CancelExamine cancels current targeting.
func (md *model) CancelExamine() {
	md.g.Highlight = nil
	md.g.MonsterTargLOS = nil
	*md.targ.ex = examination{}
	md.HideCursor()
	md.targ.kbTargeting = false
}

// Examine targets a given position with the cursor.
func (md *model) Examine(p gruid.Point) {
	if md.targ.ex.p == p {
		return
	}
	md.examine(p)
}

func (md *model) examine(p gruid.Point) {
	if !valid(p) {
		return
	}
	md.SetCursor(p)
	md.computeHighlight()
	m := md.g.MonsterAt(p)
	if m.Exists() && md.g.Player.Sees(p) {
		md.g.ComputeMonsterCone(m)
	} else {
		md.g.MonsterTargLOS = nil
	}
	md.updatePosInfo()
	md.targ.ex.scroll = false
}

func (md *model) targetMonsters() []*Monster {
	tms := []*Monster{}
	for _, m := range md.g.Monsters {
		if !m.Exists() || !md.g.Player.Sees(m.P) {
			continue
		}
		tms = append(tms, m)
	}
	for p, i := range md.g.LastMonsterKnownAt {
		m := md.g.Monsters[i]
		if !m.Exists() || md.g.Player.Sees(m.P) {
			continue
		}
		mons := &Monster{}
		*mons = *m
		mons.P = p
		tms = append(tms, mons)
	}
	sort.Slice(tms, func(i, j int) bool {
		mi := tms[i]
		mj := tms[j]
		if md.g.Player.Sees(mi.P) && !md.g.Player.Sees(mj.P) {
			return true
		}
		if !md.g.Player.Sees(mi.P) && md.g.Player.Sees(mj.P) {
			return false
		}
		return distance(md.g.Player.P, mi.P) < distance(md.g.Player.P, mj.P)
	})
	return tms
}

// KeyboardExamine toggels keyboard examination mode on the player.
func (md *model) KeyboardExamine() {
	if md.targ.kbTargeting {
		md.CancelExamine()
		return
	}
	md.targ.kbTargeting = true
	p := md.g.Player.P
	md.targ.ex.p = p
	md.targ.ex.objects = []gruid.Point{}
	md.examine(p)
}

type posInfo struct {
	P           gruid.Point
	Unknown     bool
	Noise       bool
	Unreachable bool
	Sees        bool
	Player      bool
	Monster     *Monster
	Cell        Cell
	Cloud       string
	Lighted     bool
}

func (md *model) drawPosInfo() {
	g := md.g
	p := gruid.Point{}
	if md.targ.ex.p.X < MapWidth/2 {
		p.X += MapWidth/2 + 1
	}
	info := md.targ.ex.info

	y := 2
	stt := ui.StyledText{}.WithMarkups(map[rune]gruid.Style{
		'h': gruid.Style{}.WithFg(ColorCyan),
		's': gruid.Style{}.WithFg(ColorGreen),
	})
	formatBox := func(title, s string, fg gruid.Color) {
		md.description.Content = stt.WithText(s).Format(MapWidth/2 - 3)
		if md.description.Content.Size().Y+2 > 2+MapHeight {
			md.description.Box = &ui.Box{Title: ui.NewStyledText(title, gruid.Style{}.WithFg(fg)),
				Footer: ui.Text("scroll/page down for more...")}
		} else {
			md.description.Box = &ui.Box{Title: ui.NewStyledText(title, gruid.Style{}.WithFg(fg))}
		}
		y += md.description.Draw(md.gd.Slice(gruid.NewRange(0, y, MapWidth/2-1, 2+MapHeight).Add(p))).Size().Y
	}

	features := []string{}
	if !info.Unknown {
		features = append(features, info.Cell.ShortString(g, info.P))
		if info.Cloud != "" && info.Sees {
			features = append(features, info.Cloud)
		}
		if info.Lighted && info.Sees {
			features = append(features, "lighted")
		}
	} else {
		features = append(features, "unknown")
	}
	if info.Noise {
		features = append(features, "noise")
	}
	if info.Unreachable {
		features = append(features, "unreachable")
	}
	if !info.Sees && !info.Unknown {
		features = append(features, "seen")
	} else if info.Unknown {
		features = append(features, "unexplored")
	}
	t := features[0]
	if len(features) > 1 {
		t += " (" + strings.Join(features[1:], ", ") + ")"
	}
	fg := ColorFg
	if info.Unreachable {
		fg = ColorOrange
	}
	desc := ""
	if info.Unknown {
		desc = "You do not know what is in there."
	} else {
		desc = info.Cell.Desc(g, info.P)
	}

	if info.Player {
		if !md.targ.ex.scroll {
			formatBox(t, desc, fg)
		}
		formatBox("Syu", "This is you, the monkey named Syu.", ColorBlue)
		return
	}

	mons := info.Monster
	if !mons.Exists() {
		formatBox(t, desc, fg)
		return
	}
	title := fmt.Sprintf("%s (%s %s)", mons.Kind, mons.State, dirString(mons.Dir))
	if !info.Sees {
		title = fmt.Sprintf("%s (seen)", mons.Kind)
	}
	var mfg gruid.Color
	if g.Player.Sees(mons.P) {
		mfg = mons.color(g)
	} else {
		_, mfg = mons.StyleKnowledge()
	}
	var mdesc []string

	mdesc = append(mdesc, mons.Kind.Desc())
	if info.Sees {
		statuses := mons.statusesText()
		if statuses != "" {
			mdesc = append(mdesc, fmt.Sprintf("@hStatuses:@N %s", statuses))
		}
	}
	mdesc = append(mdesc, "@hTraits:@N "+mons.traits())
	if !md.targ.ex.scroll {
		formatBox(t, desc, fg)
	}
	if !mons.Seen {
		formatBox("unknown monster", "You sensed a monster there.", mfg)
	} else {
		formatBox(title, strings.Join(mdesc, "\n"), mfg)
	}
}

func (m *Monster) color(g *Game) gruid.Color {
	var fg gruid.Color
	if m.Status(MonsLignified) {
		fg = ColorFgLignifiedMonster
	} else if m.Status(MonsConfused) {
		fg = ColorFgConfusedMonster
	} else if m.Status(MonsParalysed) {
		fg = ColorFgParalysedMonster
	} else if m.State == Resting {
		fg = ColorFgSleepingMonster
	} else if m.State == Hunting {
		fg = ColorFgMonster
	} else if m.Peaceful(g) {
		fg = ColorFgPlayer
	} else {
		fg = ColorFgWanderingMonster
	}
	return fg
}

func (m *Monster) statusesText() string {
	infos := []string{}
	for st, i := range m.Statuses {
		if i > 0 {
			infos = append(infos, fmt.Sprintf("%s(@s%d@N)", monsterStatus(st), m.Statuses[monsterStatus(st)]))
		}
	}
	return strings.Join(infos, ", ")
}

func (m *Monster) traits() string {
	var info string
	info += fmt.Sprintf("Their size is %s.", m.Kind.Size())
	if m.Kind.Peaceful() {
		info += " " + "They are peaceful."
	}
	if m.Kind.CanOpenDoors() {
		info += " " + "They can open doors."
	}
	if m.Kind.CanFly() {
		info += " " + "They can fly."
	}
	if m.Kind.CanSwim() {
		info += " " + "They can swim."
	}
	if m.Kind.ShallowSleep() {
		info += " " + "They have very shallow sleep."
	}
	if m.Kind.ResistsLignification() {
		info += " " + "They are unaffected by lignification."
	}
	if m.Kind.ReflectsTeleport() {
		info += " " + "They partially reflect back oric teleport magic."
	}
	if m.Kind.GoodSmell() {
		info += " " + "They have good sense of smell."
	}
	return info
}

func (md *model) updatePosInfo() {
	g := md.g
	pi := posInfo{}
	p := md.targ.ex.p
	pi.P = p
	switch {
	case !explored(g.Map.Cell(p)):
		pi.Unknown = true
		if g.Noise[p] || g.NoiseIllusion[p] {
			pi.Noise = true
		}
		if mons := g.lastMonsterKnownAt(p); mons.Exists() {
			pi.Monster = mons
		}
		md.targ.ex.info = pi
		return
		//case !targ.Reachable(g, pos):
		//pi.Unreachable = true
		//return
	}
	if p == g.Player.P {
		pi.Player = true
	}
	if g.Player.Sees(p) {
		pi.Sees = true
	}
	c := g.Map.Cell(p)
	if t, ok := g.TerrainKnowledge[p]; ok {
		c = t | c&Explored
	}
	if mons := g.MonsterAt(p); mons.Exists() && g.Player.Sees(p) {
		pi.Monster = mons
	} else if mons := g.lastMonsterKnownAt(p); mons.Exists() {
		pi.Monster = mons
	}
	if cld, ok := g.Clouds[p]; ok && g.Player.Sees(p) {
		pi.Cloud = cld.String()
	}
	pi.Cell = c
	if g.Illuminated(p) && c.IsIlluminable() && g.Player.Sees(p) {
		pi.Lighted = true
	}
	if g.Noise[p] || g.NoiseIllusion[p] {
		pi.Noise = true
	}
	md.targ.ex.info = pi
}

func (md *model) computeHighlight() {
	md.g.computePathHighlight(md.targ.ex.p)
}

func (g *Game) computePathHighlight(p gruid.Point) {
	path := g.PlayerPath(g.Player.P, p)
	g.Highlight = map[gruid.Point]bool{}
	for _, p := range path {
		g.Highlight[p] = true
	}
}

func (md *model) target() error {
	g := md.g
	p := md.targ.ex.p
	if !explored(g.Map.Cell(p)) {
		return errors.New("You do not know this place.")
	}
	if terrain(g.Map.Cell(p)) == WallCell && !g.Player.HasStatus(StatusDig) {
		return errors.New("You cannot travel into a wall.")
	}
	path := g.PlayerPath(g.Player.P, p)
	if len(path) == 0 {
		return errors.New("There is no safe path to this place.")
	}
	if c := g.Map.Cell(p); explored(c) && terrain(c) != WallCell {
		g.autoTarget = p
		return nil
	}
	return errors.New("Invalid destination.")
}

func (md *model) nextMonster(key gruid.Key, data *examination) {
	ms := md.targetMonsters()
	if len(ms) == 0 {
		return
	}
	if !md.targ.kbTargeting {
		md.targ.kbTargeting = true
		data.nmonster = -1
	}
	nmonster := data.nmonster
	if key == "+" {
		nmonster++
	} else {
		nmonster--
	}
	if nmonster > len(ms)-1 {
		nmonster = 0
	} else if nmonster < 0 {
		nmonster = len(ms) - 1
	}
	mons := ms[nmonster]
	p := mons.P
	data.nmonster = nmonster
	md.Examine(p)
}

func (md *model) nextStair(data *examination) {
	g := md.g
	if data.sortedStairs == nil {
		stairs := g.StairsSlice()
		data.sortedStairs = g.SortedNearestTo(stairs, g.Player.P)
	}
	if len(data.sortedStairs) == 0 {
		return
	}
	if !md.targ.kbTargeting {
		md.targ.kbTargeting = true
		data.stairIndex = 0
	}
	md.targ.kbTargeting = true
	if data.stairIndex >= len(data.sortedStairs) {
		data.stairIndex = 0
	}
	if len(data.sortedStairs) > 0 {
		md.Examine(data.sortedStairs[data.stairIndex])
		data.stairIndex++
	}
}

func (md *model) nextObject(np gruid.Point, data *examination) {
	g := md.g
	nobject := data.nobject
	if len(data.objects) == 0 {
		for p := range g.Objects.Stones {
			data.objects = append(data.objects, p)
		}
		for p := range g.Objects.Magaras {
			data.objects = append(data.objects, p)
		}
		for p := range g.Objects.Items {
			data.objects = append(data.objects, p)
		}
		data.objects = g.SortedNearestTo(data.objects, g.Player.P)
	}
	for i := 0; i < len(data.objects); i++ {
		p := data.objects[nobject]
		nobject++
		if nobject > len(data.objects)-1 {
			nobject = 0
		}
		if explored(g.Map.Cell(p)) {
			np = p
			break
		}
	}
	if len(data.objects) == 0 {
		return
	}
	if !md.targ.kbTargeting {
		md.targ.kbTargeting = true
	}
	data.nobject = nobject
	md.Examine(np)
}
