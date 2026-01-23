package main

import (
	//"log"
	"sort"
	"time"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/paths"
	"codeberg.org/anaseto/gruid/rl"
)

// These constants represent various common durations for animation frames.
const (
	AnimDurExtremelyShort = 15 * time.Millisecond
	AnimDurVeryShort      = 30 * time.Millisecond
	AnimDurShort          = 40 * time.Millisecond
	AnimDurShortMedium    = 60 * time.Millisecond
	AnimDurMedium         = 80 * time.Millisecond
	AnimDurMediumLong     = 100 * time.Millisecond
	AnimDurLong           = 150 * time.Millisecond
)

// Animations provides support for animations.
type Animations struct {
	frames []AnimFrame
	grid   gruid.Grid
	pgrid  gruid.Grid
	idx    int
	draw   bool
}

// AnimFrame represents an animation frame.
type AnimFrame struct {
	Cells    []gruid.FrameCell
	Duration time.Duration
}

type msgAnim int

func (md *model) initAnimations() {
	gd := md.gd.Slice(md.gd.Range().Shift(0, 2, 0, -1))
	max := gd.Size()
	md.anims.grid = gruid.NewGrid(max.X, max.Y)
	md.anims.pgrid = gruid.NewGrid(max.X, max.Y)
	md.anims.grid.Copy(gd)
	md.anims.pgrid.Copy(gd)
}

func (md *model) animNext() gruid.Cmd {
	d := md.anims.frames[0].Duration
	idx := md.anims.idx
	return func() gruid.Msg {
		t := time.NewTimer(d)
		<-t.C
		return msgAnim(idx)
	}
}

func (md *model) animCmd() gruid.Cmd {
	if len(md.anims.frames) == 0 {
		return nil
	}
	idx := md.anims.idx
	return func() gruid.Msg {
		return msgAnim(idx)
	}
}

func (md *model) startAnimSeq() {
	if md.anims.Done() {
		md.resetAnimations()
	}
	it := md.g.Map.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		r, fg, bg := md.positionDrawing(p)
		attrs := AttrInMap
		if md.g.highlight[p] || p == md.targ.ex.p {
			attrs |= AttrReverse
		}
		md.anims.grid.Set(p, gruid.Cell{Rune: r, Style: gruid.Style{Fg: fg, Bg: bg, Attrs: attrs}})
	}
}

func (md *model) resetAnimations() {
	gd := md.gd.Slice(md.gd.Range().Shift(0, 2, 0, -1))
	md.anims.grid.Copy(gd)
	md.anims.pgrid.Copy(gd)
}

// Finish clears the frame queue.
func (a *Animations) Finish() {
	a.idx++
	a.frames = nil
}

// Done reports whether there are no more animations in the queue.
func (a *Animations) Done() bool {
	return len(a.frames) == 0
}

// Draw sets a given rune and foreground color at a given position.
func (a *Animations) Draw(p gruid.Point, r rune, fg, bg gruid.Color) {
	c := a.grid.At(p)
	c.Rune = r
	c.Style.Fg = fg
	c.Style.Bg = bg
	a.grid.Set(p, c)
}

func (a *Animations) DrawReverse(p gruid.Point, r rune, fg, bg gruid.Color) {
	c := a.grid.At(p)
	c.Rune = r
	c.Style.Fg = fg
	c.Style.Bg = bg
	c.Style.Attrs |= AttrReverse
	a.grid.Set(p, c)
}

// Frame emits a new animation frame with given duration.
func (a *Animations) Frame(d time.Duration) {
	frame := AnimFrame{}
	frame.Duration = d
	it := a.grid.Iterator()
	itp := a.pgrid.Iterator()
	for it.Next() && itp.Next() {
		if it.Cell() == itp.Cell() {
			continue
		}
		frame.Cells = append(frame.Cells, gruid.FrameCell{P: it.P(), Cell: it.Cell()})
	}
	a.frames = append(a.frames, frame)
	a.pgrid.Copy(a.grid)
}

// AnimationFrameFast updates the map and waits very briefly.
func (md *model) AnimationFrameFast() {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	md.anims.Frame(AnimDurExtremelyShort)
}

func (md *model) SwappingAnimation(mp, pp gruid.Point) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	_, fgm, bgColorm := md.positionDrawing(mp)
	_, _, bgColorp := md.positionDrawing(pp)
	md.anims.Draw(mp, 'Φ', fgm, bgColorp)
	md.anims.Draw(pp, 'Φ', ColorBlue, bgColorm)
	md.anims.Frame(AnimDurMedium)
	md.anims.Draw(mp, 'Φ', ColorBlue, bgColorp)
	md.anims.Draw(pp, 'Φ', fgm, bgColorm)
	md.anims.Frame(AnimDurMedium)
}

func (md *model) TeleportAnimation(from, to gruid.Point, showto bool) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	_, _, bgColorf := md.positionDrawing(from)
	_, _, bgColort := md.positionDrawing(to)
	if showto {
		md.anims.Draw(from, 'Φ', ColorBlue, bgColorf)
		md.anims.Draw(to, 'Φ', ColorCyan, bgColort)
		md.anims.Frame(AnimDurMediumLong)
	} else {
		md.anims.Draw(from, 'Φ', ColorCyan, bgColorf)
		md.anims.Frame(AnimDurMediumLong)
	}
}

type explosionStyle int

const (
	FireExplosion explosionStyle = iota
	WallExplosion
	AroundWallExplosion
)

func (md *model) MonsterProjectileAnimation(ray []gruid.Point, r rune, fg gruid.Color) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	for i := range len(ray) {
		p := ray[i]
		or, fgColor, bgColor := md.positionDrawing(p)
		md.anims.Draw(p, r, fg, bgColor)
		md.anims.Frame(AnimDurVeryShort)
		md.anims.Draw(p, or, fgColor, bgColor)
	}
}

func (md *model) WaveDrawAt(p gruid.Point, fg gruid.Color) {
	r, _, bgColor := md.positionDrawing(p)
	md.anims.DrawReverse(p, r, fg, bgColor)
}

func (md *model) ExplosionDrawAt(p gruid.Point, fg gruid.Color) {
	g := md.g
	_, _, bgColor := md.positionDrawing(p)
	mons := g.MonsterAt(p)
	r := ';'
	switch RandInt(9) {
	case 0, 6:
		r = ','
	case 1:
		r = '}'
	case 2:
		r = '%'
	case 3, 7:
		r = ':'
	case 4:
		r = '\\'
	case 5:
		r = '~'
	}
	if mons.Exists() || g.Player.P == p {
		r = '√'
	}
	md.anims.DrawReverse(p, r, fg, bgColor)
}

func (md *model) NoiseAnimation(noises []gruid.Point) {
	if DisableAnimations {
		return
	}
	md.FOVWavesAnimation(DefaultFOVRange, WaveHarmonicNoise, md.g.Player.P)
	colors := []gruid.Color{ColorViolet, ColorCyan}
	for i := range 2 {
		for _, p := range noises {
			r := '♫'
			_, _, bgColor := md.positionDrawing(p)
			md.anims.DrawReverse(p, r, colors[i], bgColor)
		}
		_, _, bgColor := md.positionDrawing(md.g.Player.P)
		md.anims.DrawReverse(md.g.Player.P, '@', colors[i], bgColor)
		md.anims.Frame(AnimDurShort)
	}

}

func (md *model) ExplosionAnimation(es explosionStyle, p gruid.Point) {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	colors := [2]gruid.Color{ColorYellow, ColorOrange}
	if es == WallExplosion || es == AroundWallExplosion {
		colors[0] = ColorViolet
		colors[1] = ColorMagenta
	}
	for range 3 {
		nb := g.PlayerPassableNeighbors(p)
		if es != AroundWallExplosion {
			nb = append(nb, p)
		}
		for _, q := range nb {
			fg := colors[RandInt(2)]
			if !g.Player.AFOV[q] {
				continue
			}
			md.ExplosionDrawAt(q, fg)
		}
		md.anims.Frame(AnimDurMediumLong)
	}
}

func (g *Game) Waves(maxCost int, ws wavestyle, center gruid.Point) (dists []int, cdists map[int][]int) {
	var mp paths.Pather
	switch ws {
	case WaveHarmonicNoise:
		mp = &GridPath{}
	default:
		mp = &NoisePath{g: g}
	}
	nodes := g.PR.BreadthFirstMap(mp, []gruid.Point{center}, maxCost)
	cdists = make(map[int][]int)
	for _, n := range nodes {
		cdists[n.Cost] = append(cdists[n.Cost], Pos2Idx(n.P))
	}
	for dist := range cdists {
		dists = append(dists, dist)
	}
	sort.Ints(dists)
	return dists, cdists
}

func (md *model) FOVWavesAnimation(r int, ws wavestyle, center gruid.Point) {
	dists, cdists := md.g.Waves(r, ws, center)
	for _, d := range dists {
		wave := cdists[d]
		if len(wave) == 0 {
			break
		}
		md.WaveAnimation(wave, ws)
	}
}

type wavestyle int

const (
	WaveHarmonicNoise wavestyle = iota
	WaveNoise
	WaveConfusion
	WaveDaze
	WaveTree
	WaveSleeping
)

func (md *model) WaveAnimation(wave []int, ws wavestyle) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	for _, i := range wave {
		p := Idx2Pos(i)
		switch ws {
		case WaveConfusion:
			fg := ColorGreen
			if md.g.Player.Sees(p) {
				md.WaveDrawAt(p, fg)
			}
		case WaveSleeping:
			fg := ColorViolet
			if md.g.Player.Sees(p) {
				md.WaveDrawAt(p, fg)
			}
		case WaveDaze:
			fg := ColorCyan
			if md.g.Player.Sees(p) {
				md.WaveDrawAt(p, fg)
			}
		case WaveTree:
			fg := ColorYellow
			if md.g.Player.Sees(p) {
				md.WaveDrawAt(p, fg)
			}
		case WaveNoise:
			fg := ColorOrange
			if md.g.Player.Sees(p) {
				md.WaveDrawAt(p, fg)
			}
		case WaveHarmonicNoise:
			fg := ColorCyan
			md.WaveDrawAt(p, fg)
		}
	}
	md.anims.Frame(AnimDurVeryShort)
}

func (md *model) WallExplosionAnimation(p gruid.Point) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	colors := [2]gruid.Color{ColorViolet, ColorMagenta}
	for _, fg := range colors {
		_, _, bgColor := md.positionDrawing(p)
		md.anims.DrawReverse(p, '%', fg, bgColor)
		md.anims.Frame(AnimDurVeryShort)
	}
}

type beamstyle int

const (
	BeamSleeping beamstyle = iota
	BeamLignification
	BeamObstruction
)

func (md *model) BeamsAnimation(ray []gruid.Point, bs beamstyle) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	md.anims.Frame(AnimDurVeryShort)
	// change colors depending on effect
	var fg gruid.Color
	switch bs {
	//	case BeamSleeping:
	//		fg = ColorFgSleepingMonster
	case BeamLignification:
		fg = ColorYellow
	case BeamObstruction:
		fg = ColorCyan
	}
	for range 3 {
		for i := len(ray) - 1; i >= 0; i-- {
			p := ray[i]
			_, _, bgColor := md.positionDrawing(p)
			r := '*'
			if RandInt(2) == 0 {
				r = '×'
			}
			md.anims.DrawReverse(p, r, fg, bgColor)
		}
		md.anims.Frame(AnimDurShort)
	}
}

func (md *model) SlowingMagaraAnimation(ray []gruid.Point) {
	if DisableAnimations {
		return
	}
	md.startAnimSeq()
	colors := [2]gruid.Color{ColorGreen, ColorCyan}
	for range 3 {
		for i := len(ray) - 1; i >= 0; i-- {
			fg := colors[RandInt(2)]
			p := ray[i]
			_, _, bgColor := md.positionDrawing(p)
			r := '*'
			if RandInt(2) == 0 {
				r = '×'
			}
			md.anims.DrawReverse(p, r, fg, bgColor)
		}
		md.anims.Frame(AnimDurShortMedium)
	}
}

func (md *model) ProjectileSymbol(dir gruid.Point) (r rune) {
	switch dir {
	case gruid.Point{1, 0}, gruid.Point{-1, 0}:
		r = '—'
	case gruid.Point{1, -1}, gruid.Point{-1, 1}:
		r = '/'
	case gruid.Point{0, 1}, gruid.Point{0, -1}:
		r = '|'
	case gruid.Point{1, 1}, gruid.Point{-1, -1}:
		r = '\\'
	}
	return r
}

func (md *model) MonsterJavelinAnimation(ray []gruid.Point, hit bool) {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	for i := range len(ray) {
		p := ray[i]
		r, fgColor, bgColor := md.positionDrawing(p)
		md.anims.Draw(p, md.ProjectileSymbol(DirNorm(g.Player.P, p)), ColorRed, bgColor)
		md.anims.Frame(AnimDurVeryShort)
		md.anims.Draw(p, r, fgColor, bgColor)
	}
}

func (md *model) WoundedAnimation() {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	r, _, bg := md.positionDrawing(g.Player.P)
	md.anims.Draw(g.Player.P, r, ColorYellow, bg)
	md.anims.Frame(AnimDurShortMedium)
	if g.Player.HP <= 15 {
		md.anims.Draw(g.Player.P, r, ColorRed, bg)
		md.anims.Frame(AnimDurShortMedium)
	}
}

func (md *model) PlayerGoodEffectAnimation() {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	r, _, bg := md.positionDrawing(g.Player.P)
	md.anims.Draw(g.Player.P, r, ColorGreen, bg)
	md.anims.Frame(AnimDurShortMedium)
	md.anims.Draw(g.Player.P, r, ColorYellow, bg)
	md.anims.Frame(AnimDurShortMedium)
}

func (md *model) StatusEndAnimation() {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	r, _, bg := md.positionDrawing(g.Player.P)
	md.anims.Draw(g.Player.P, r, ColorViolet, bg)
	md.anims.Frame(AnimDurShortMedium)
}

func (md *model) EffectAtPPAnimation() {
	if DisableAnimations {
		return
	}
	g := md.g
	md.startAnimSeq()
	r, _, bg := md.positionDrawing(g.Player.P)
	md.anims.Draw(g.Player.P, r, ColorCyan, bg)
	md.anims.Frame(AnimDurShortMedium)
}

func (md *model) FoundFakeStairsAnimation() {
	if DisableAnimations {
		return
	}
	g := md.g
	r, _, bg := md.positionDrawing(g.Player.P)
	md.anims.Draw(g.Player.P, r, ColorMagenta, bg)
	md.anims.Frame(AnimDurMediumLong)
}

func (md *model) PushAnimation(path []gruid.Point) {
	if DisableAnimations {
		return
	}
	if len(path) == 0 {
		// should not happen
		return
	}
	md.startAnimSeq()
	_, _, bg := md.positionDrawing(path[0])
	for _, p := range path[:len(path)-1] {
		md.anims.Draw(p, '×', ColorBlue, bg)
	}
	md.anims.Draw(path[len(path)-1], '@', ColorBlue, bg)
	md.anims.Frame(AnimDurMediumLong)
}

func (md *model) AbyssFallAnimation() {
	if DisableAnimations {
		return
	}
	gd := rl.NewGrid(MapWidth, MapHeight)
	n := 5
	gd.FillFunc(func() rl.Cell {
		return rl.Cell(RandInt(n))
	})
	for i := range n {
		it := gd.Iterator()
		for it.Next() {
			if it.Cell() == rl.Cell(i) {
				md.anims.Draw(it.P(), ' ', ColorForeground, ColorBackground)
			}
		}
		md.anims.Frame(AnimDurVeryShort)
	}
}

// NextStoryPart makes a story sequence progress.
func (md *model) NextStoryPart() {
	switch md.g.Map.Depth {
	case WinDepth:
		md.FreeingShaedra()
	case MaxDepth:
		md.TakingArtifact()
	default:
		md.mode = modeNormal // should not happen
	}
}

// FreeingShaedra handles Shaedra's liberation story sequence.
func (md *model) FreeingShaedra() {
	g := md.g
	switch md.story {
	case 1:
		if !DisableAnimations {
			md.startAnimSeq()
			_, _, bg := md.positionDrawing(g.Places.Monolith)
			md.anims.Draw(g.Places.Monolith, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Objects.Stairs[g.Places.Monolith] = WinStair
		g.Map.Set(g.Places.Monolith, StairCell)
		g.Map.KnownTerrain.Set(g.Places.Monolith, StairCell)
		if !DisableAnimations {
			md.startAnimSeq()
			md.anims.Frame(AnimDurMediumLong)
			_, _, bg := md.positionDrawing(g.Places.Marevor)
			md.anims.Draw(g.Places.Marevor, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Objects.Story[g.Places.Marevor] = StoryMarevor
		g.LogStyled("Marevor: “And what about the mission? Take that magara!”", logSpecial)
		g.LogStyled("Shaedra: “Pff, don’t be reckless!”", logSpecial)
		g.LogStyled("[SPACE/ESC to continue]", logConfirm)
		// Progress to next story part.
		md.story++
	default:
		if !DisableAnimations {
			md.startAnimSeq()
			_, _, bg := md.positionDrawing(g.Places.Marevor)
			md.anims.Draw(g.Places.Marevor, 'Φ', ColorCyan, bg)
			md.anims.Draw(g.Places.Shaedra, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Map.Set(g.Places.Shaedra, GroundCell)
		g.Map.KnownTerrain.Set(g.Places.Shaedra, GroundCell)
		g.Map.Set(g.Places.Marevor, ScrollCell)
		g.Map.KnownTerrain.Set(g.Places.Marevor, ScrollCell)
		g.Objects.Scrolls[g.Places.Marevor] = ScrollExtended
		g.RescuedShaedra()
		// We're done with the story sequence.
		md.story = 0
		md.mode = modeNormal
	}
}

// RescuedShaedra finalizes Shaedra's liberation, including Marevor giving the
// player a special magara.
func (g *Game) RescuedShaedra() {
	g.Player.Magaras = append(g.Player.Magaras, Magara{})
	g.Player.MP++ // gain one MP point when obtaining the new magara slot and returning the artifact
	g.Player.Inventory.Misc = NoItem
	g.LogStyled("You equip the new magara in the artifact’s old place.", logSpecial)
	if RandInt(2) == 0 {
		g.Player.Magaras[len(g.Player.Magaras)-1] = Magara{Kind: DispersalMagara, Charges: DispersalMagara.DefaultCharges()}
	} else {
		g.Player.Magaras[len(g.Player.Magaras)-1] = Magara{Kind: DelayedOricExplosionMagara, Charges: DelayedOricExplosionMagara.DefaultCharges()}
	}
	AchRescuedShaedra.Get(g)
}

// TakingArtifact handles the artifact's activation final story sequence.
func (md *model) TakingArtifact() {
	g := md.g
	switch md.story {
	case 1:
		g.Map.Set(g.Places.Artifact, GroundCell)
		g.Map.KnownTerrain.Set(g.Places.Artifact, GroundCell)
		if !DisableAnimations {
			md.startAnimSeq()
			_, _, bg := md.positionDrawing(g.Places.Monolith)
			md.anims.Draw(g.Places.Monolith, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Objects.Stairs[g.Places.Monolith] = WinStair
		g.Map.Set(g.Places.Monolith, StairCell)
		g.Map.KnownTerrain.Set(g.Places.Monolith, StairCell)
		if !DisableAnimations {
			md.startAnimSeq()
			_, _, bg := md.positionDrawing(g.Places.Marevor)
			md.anims.Draw(g.Places.Marevor, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Objects.Story[g.Places.Marevor] = StoryMarevor
		g.Map.KnownTerrain.Set(g.Places.Marevor, StoryCell)
		g.LogStyled("Marevor: “Great! Let’s escape and find some bones to celebrate!”", logSpecial)
		g.LogStyled("Syu: “Sorry, but I prefer bananas!”", logSpecial)
		g.LogStyled("[SPACE/ESC to continue]", logConfirm)
		// Progress to next story part.
		md.story++
	default:
		if !DisableAnimations {
			md.startAnimSeq()
			_, _, bg := md.positionDrawing(g.Places.Marevor)
			md.anims.Draw(g.Places.Marevor, 'Φ', ColorCyan, bg)
			md.anims.Frame(AnimDurMediumLong)
		}
		g.Map.Set(g.Places.Marevor, GroundCell)
		g.Map.KnownTerrain.Set(g.Places.Marevor, GroundCell)
		AchRetrievedArtifact.Get(g)
		// We're done with the story sequence.
		md.story = 0
		md.mode = modeNormal
	}
}
