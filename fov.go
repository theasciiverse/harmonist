package main

import (
	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/paths"
	"codeberg.org/anaseto/gruid/rl"
)

// These constants represent various kinds of vision/light ranges.
const (
	DefaultFOVRange        = 12
	DefaultMonsterFOVRange = 12
	LightRange             = 6
	TreeRange              = 50
)

// lighter implements rl.Lighter.
type lighter struct {
	rs raystyle
	g  *Game
}

func (lt *lighter) Cost(src, from, to gruid.Point) int {
	g := lt.g
	rs := lt.rs
	wallcost := lt.MaxCost(src)
	// no terrain cost on origin
	if src == from {
		// specific diagonal costs
		vis := g.diagonalVisibility(from, to)
		switch vis {
		case Opaque:
			return wallcost
		case Fuzzy:
			if rs != TreePlayerRay {
				return wallcost - 1
			}
		}
		return distance(to, from)
	}
	// from terrain specific costs
	t := g.Map.Terrain.AtU(from)
	if t == WallCell {
		return wallcost
	}
	if _, ok := g.Map.Clouds[from]; ok {
		return wallcost
	}
	// specific diagonal costs
	vis := g.diagonalVisibility(from, to)
	if vis == Opaque {
		return wallcost
	}
	switch t {
	case DoorCell:
		if from != src {
			mons := g.MonsterAt(from)
			if !mons.Exists() && from != g.Player.P {
				return wallcost
			}
		}
	case FoliageCell, HoledWallCell:
		switch rs {
		case TreePlayerRay:
			if t == FoliageCell {
				break
			}
			fallthrough
		default:
			return wallcost + distance(to, from) - 3
		}
	}
	if rs != TreePlayerRay && vis == Fuzzy {
		cost := max(1, wallcost-distance(from, src)-1)
		return cost
	}
	if rs == TreePlayerRay && t == WindowCell && distance(src, from) >= DefaultFOVRange {
		return wallcost - distance(src, from) - 1
	}
	return distance(to, from)
}

func (lt *lighter) MaxCost(src gruid.Point) int {
	switch lt.rs {
	case TreePlayerRay:
		return TreeRange + 1
	case MonsterRay:
		return DefaultMonsterFOVRange + 1
	case LightRay:
		return LightRange
	default:
		return DefaultFOVRange + 1
	}
}

// visibility represents various kinds of visibility effects of terrain.
type visibility uint32

const (
	Opaque visibility = 0b00
	Fuzzy  visibility = 0b01
	Clear  visibility = 0b11
)

func (g *Game) diagonalVisibility(from, to gruid.Point) visibility {
	// The state uses cardinal movement only, so two diagonal walls should,
	// for example, block line of sight. This is in contrast with the main
	// mechanics of the line of sight algorithm, which for gameplay reasons
	// allows diagonals for light rays in normal circumstances.
	var ps [2]gruid.Point
	delta := to.Sub(from)
	switch delta {
	case gruid.Point{1, -1}:
		ps[0] = from.Shift(1, 0)
		ps[1] = from.Shift(0, -1)
	case gruid.Point{-1, -1}:
		ps[0] = from.Shift(-1, 0)
		ps[1] = from.Shift(0, -1)
	case gruid.Point{-1, 1}:
		ps[0] = from.Shift(-1, 0)
		ps[1] = from.Shift(0, 1)
	case gruid.Point{1, 1}:
		ps[0] = from.Shift(1, 0)
		ps[1] = from.Shift(0, 1)
	default:
		return Clear
	}
	vis := Opaque
	for _, p := range ps {
		_, ok := g.Map.Clouds[p]
		if ok {
			continue
		}
		switch g.Map.Terrain.AtU(p) {
		case WindowCell:
			vis |= Fuzzy
		case WallCell, HoledWallCell:
		case FoliageCell:
			vis |= Fuzzy
		default:
			return Clear
		}
	}
	return vis
}

// raystyle represents the various kinds of vision ray styles.
type raystyle int

const (
	NormalPlayerRay raystyle = iota
	MonsterRay
	TreePlayerRay
	LightRay
)

func (g *Game) StopAuto() {
	if g.autoexploring && !g.autoHalt {
		g.Log("You stop exploring.")
	} else if g.autoDir != ZP {
		g.Log("You stop.")
	} else if g.autoTarget != invalidPos {
		g.Log("You stop.")
	}
	g.autoHalt = true
	g.autoDir = ZP
	g.autoTarget = invalidPos
}

func (g *Game) Illuminated(p gruid.Point) bool {
	c, ok := g.LightFOV.At(p)
	return ok && c <= LightRange || g.Map.Terrain.At(p) == StoneCell
}

func (g *Game) blocksSSCFOV(p gruid.Point) bool {
	return g.Map.At(p) != WallCell
}

// ComputeFOV updates the field of view of both player and monsters (so that
// the player can tell what monsters will see).
func (g *Game) ComputeFOV() {
	g.ComputeLights()
	for k := range g.Player.AFOV {
		delete(g.Player.AFOV, k)
	}
	c := g.Map.At(g.Player.P)
	rs := NormalPlayerRay
	maxDepth := DefaultFOVRange
	if c == TreeCell {
		rs = TreePlayerRay
		maxDepth = TreeRange
	}
	lt := &lighter{rs: rs, g: g}
	g.Player.FOV.SetRange(visionRange(g.Player.P, maxDepth))
	lnodes := g.Player.FOV.VisionMap(lt, g.Player.P)
	nbs := paths.Neighbors{}
	g.Player.FOV.SSCVisionMap(
		g.Player.P, maxDepth,
		g.blocksSSCFOV,
		false,
	)
	for _, n := range lnodes {
		if !g.Player.FOV.Visible(n.P) {
			continue
		}
		if n.Cost <= DefaultFOVRange {
			g.Player.AFOV[n.P] = true
		} else if c == TreeCell && g.Illuminated(n.P) && n.Cost <= TreeRange {
			if g.Map.At(n.P) == WallCell {
				// this is just an approximation, but ok in practice
				nb := nbs.All(n.P, func(npos gruid.Point) bool {
					if !InMap(npos) || !g.Illuminated(npos) || g.Map.At(npos) == WallCell {
						return false
					}
					cost, ok := g.Player.FOV.At(npos)
					return ok && cost < TreeRange
				})

				if len(nb) == 0 {
					continue
				}
			}
			g.Player.AFOV[n.P] = true
		}
	}
	for p := range g.Player.AFOV {
		if g.Player.Sees(p) {
			g.SeePosition(p)
		}
	}
	for _, mons := range g.Monsters {
		if mons.Exists() && g.Player.Sees(mons.P) {
			mons.ComputeFOV(g) // approximation of what the monster will see for player info purposes
			mons.UpdateKnowledge(g, mons.P)
			if mons.Seen {
				g.StopAuto()
				continue
			}
			g.SeeMonster(mons, "see")
			g.StopAuto()
		}
	}
}

// SeeMonster makes you see or sense a monster.
func (g *Game) SeeMonster(mons *Monster, verb string) {
	if mons.Seen {
		return
	}
	mons.Seen = true
	g.Logf("You %s %s (%v).", verb, mons.Kind.Indefinite(false), mons.State)
	if mons.Kind.Notable() {
		g.StoryPrintf("Saw %s", mons.Kind)
	}
}

// visionRange returns the vision range from the given position.
func visionRange(p gruid.Point, radius int) gruid.Range {
	drg := gruid.NewRange(0, 0, MapWidth, MapHeight)
	delta := gruid.Point{radius, radius}
	return drg.Intersect(gruid.Range{Min: p.Sub(delta), Max: p.Add(delta).Shift(1, 1)})
}

// ComputeFOV updates the field of view of the monster.
func (m *Monster) ComputeFOV(g *Game) {
	if m.Kind.Peaceful() {
		return
	}
	for k := range m.FOV {
		delete(m.FOV, k)
	}
	if g.mfov == nil {
		g.mfov = rl.NewFOV(visionRange(m.P, DefaultMonsterFOVRange))
	} else {
		g.mfov.SetRange(visionRange(m.P, DefaultMonsterFOVRange))
	}
	lt := &lighter{rs: MonsterRay, g: g}
	lnodes := g.mfov.VisionMap(lt, m.P)
	g.mfov.SSCVisionMap(
		m.P, DefaultMonsterFOVRange,
		g.blocksSSCFOV,
		false,
	)
	for _, n := range lnodes {
		if !g.mfov.Visible(n.P) {
			continue
		}
		if n.P == m.P {
			m.FOV[n.P] = true
			continue
		}
		if n.Cost <= DefaultMonsterFOVRange && g.Map.At(n.P) != BarrelCell {
			pnode, ok := g.mfov.From(lt, n.P)
			if !ok || !Hides(g.Map.At(pnode.P)) {
				m.FOV[n.P] = true
			}
		}
	}
}

// SeeNotable makes the player look at any notable features at the given
// position.
func (g *Game) SeeNotable(t rl.Cell, p gruid.Point) {
	switch t {
	case MagaraCell:
		mag := g.Objects.Magaras[p]
		path := g.PR.JPSPath(nil, g.Player.P, p, InMap, false)
		if len(path) > 0 {
			g.StoryPrintf("Spotted %s (distance: %d)", mag, len(path))
		} else {
			g.StoryPrintf("Spotted %s", mag)
		}
	case ItemCell:
		it := g.Objects.Items[p]
		path := g.PR.JPSPath(nil, g.Player.P, p, InMap, false)
		if len(path) > 0 {
			g.StoryPrintf("Spotted %s (distance: %d)", it.ShortDesc(g), len(path))
		} else {
			g.StoryPrintf("Spotted %s", it.ShortDesc(g))
		}
	case StairCell:
		st := g.Objects.Stairs[p]
		path := g.PR.JPSPath(nil, g.Player.P, p, InMap, false)
		if len(path) > 0 {
			g.StoryPrintf("Discovered %s (distance: %d)", st, len(path))
		} else {
			g.StoryPrintf("Discovered %s", st)
		}
	case FakeStairCell:
		path := g.PR.JPSPath(nil, g.Player.P, p, InMap, false)
		if len(path) > 0 {
			g.StoryPrintf("Discovered %s (distance: %d)", normalStairShortDesc, len(path))
		} else {
			g.StoryPrintf("Discovered %s", normalStairShortDesc)
		}
	case StoryCell:
		st := g.Objects.Story[p]
		if st == StoryArtifactSealed {
			path := g.PR.JPSPath(nil, g.Player.P, p, InMap, false)
			if len(path) > 0 {
				g.StoryPrintf("Discovered Portal Moon Gem Artifact (distance: %d)", len(path))
			} else {
				g.StoryPrint("Discovered Portal Moon Gem Artifact")
			}
		}
	}
}

// SeePosition fully reveals position p to the player.
func (g *Game) SeePosition(p gruid.Point) {
	g.SeeTerrainAt(p)
	if mons := g.KnownMonsterAt(p); !mons.Exists() || mons.P != p {
		delete(g.KnownMonster, p)
		if mons.Exists() {
			mons.LastKnownPos = invalidPos
		}
	}
	delete(g.Map.NoiseIllusion, p)
}

// SeeTerrainAt reveals terrain at position p to the player.
func (g *Game) SeeTerrainAt(p gruid.Point) {
	t := g.Map.Terrain.At(p)
	kt := g.Map.KnownTerrain.At(p)
	switch kt {
	case Unknown, UnknownPassable:
		see := "see"
		if !g.Player.Sees(p) {
			see = "sense"
		}
		if IsNotable(t) {
			g.LogfStyled("You %s %s.", logNotable, see, g.TerrainShortDesc(t, p))
			g.StopAuto()
			g.SeeNotable(t, p)
		}
	default:
		// XXX this can be improved to handle more terrain types changes
		if t == WallCell && kt != WallCell {
			g.Logf("There is no longer a wall there.")
			g.StopAuto()
			g.autoexploreMapRebuild = true
		} else if cld, ok := g.Map.Clouds[p]; ok && cld == CloudFire && t != kt && Flammable(kt) {
			g.Logf("There are flames there.")
			g.StopAuto()
			g.autoexploreMapRebuild = true
		}
	}
	if kt != t {
		g.autoexploreMapRebuild = true
		g.Map.KnownTerrain.Set(p, t)
	}
	if t != BarrierCell {
		delete(g.Map.MagicalBarriers, p)
	}
}

// Ray returns a light ray to the given position (from the player's).
func (g *Game) Ray(p gruid.Point) []gruid.Point {
	t := g.Map.At(g.Player.P)
	rs := NormalPlayerRay
	if t == TreeCell {
		rs = TreePlayerRay
	}
	lt := &lighter{rs: rs, g: g}
	lnodes := g.Player.FOV.Ray(lt, p)
	ps := []gruid.Point{}
	for i := len(lnodes) - 1; i > 0; i-- {
		ps = append(ps, lnodes[i].P)
	}
	return ps
}

// ComputeNoise updates monster noise information.
func (g *Game) ComputeNoise() {
	mp := &NoisePath{g: g}
	maxdist := DefaultFOVRange + 1
	nodes := g.PRnoise.BreadthFirstMap(mp, []gruid.Point{g.Player.P}, maxdist)
	count := 0
	for _, mons := range g.Monsters {
		if mons != nil {
			mons.Noise = false
		}
	}
	goodhearing := g.Player.Inventory.Body == CloakHear
	for _, n := range nodes {
		if g.Player.Sees(n.P) {
			continue
		}
		mons := g.MonsterAt(n.P)
		if !mons.Exists() {
			// Dead monsters make no perceptible noise.
			continue
		}
		dist := g.PRnoise.BreadthFirstMapAt(n.P)
		if dist > maxdist {
			continue
		}
		queenrock := g.Map.At(mons.P) == QueenRockCell
		if queenrock {
			// Monster noise over queen rocks is easier to hear.
			dist -= 4
		}
		if mons.State == Resting {
			if !goodhearing {
				// Resting monsters cannot be heard without
				// cloak of hearing.
				continue
			}
			// Even with good hearing, resting monsters are
			// difficult to perceive from afar.
			dist += 10
		}
		switch {
		case goodhearing:
			if mons.State == Watching {
				if RandInt(3+dist/4) > 1 {
					continue
				}
			} else {
				dist -= 2
			}
		case !queenrock && RandInt(3+dist/4) > 1 || mons.State == Watching:
			// Without cloak of hearing, hearing is unreliable, and
			// it doesn't catch small in-place footsteps from
			// monsters watching around.
			continue
		}
		switch mons.Kind {
		case MonsSatowalgaPlant, MonsButterfly:
			// Satowalga plants and kerejats don't make any
			// noise.
		case MonsMirrorSpecter, MonsSpider:
			// Only cloak of hearing allows for hearing mirror
			// specters and spiders.
			if goodhearing && dist <= maxdist-4 {
				mons.Noise = true
			}
		case MonsEarthDragon, MonsTreeMushroom, MonsYack:
			// Heavy steps are easy to hear at full range.
			mons.Noise = true
		case MonsWorm, MonsAcidMound, MonsDog, MonsBlinkingFrog, MonsHazeCat, MonsCrazyImp:
			// Creeping noises and light footsteps are a bit harder
			// to hear from afar.
			if dist <= maxdist-4 {
				mons.Noise = true
			}
		default:
			if dist <= maxdist-2 {
				mons.Noise = true
			}
		}
		if mons.Noise {
			count++
			if mons.State == Resting {
				g.Log("You hear some breathing.")
			} else {
				g.Logf("You hear %s.", monsterNoise(mons.Kind))
			}
		}
	}
	if count > 0 {
		g.StopAuto()
	}
}

func monsterNoise(mk monsterKind) string {
	switch mk {
	case MonsSatowalgaPlant, MonsButterfly:
		// Should not happen.
		return "a silent noise"
	case MonsMirrorSpecter:
		return "an imperceptible air movement"
	case MonsWingedMilfid, MonsTinyHarpy:
		return "the flapping of wings"
	case MonsEarthDragon, MonsTreeMushroom, MonsYack:
		return "heavy footsteps"
	case MonsWorm, MonsAcidMound:
		return "a creep noise"
	case MonsDog, MonsBlinkingFrog, MonsHazeCat, MonsCrazyImp, MonsSpider:
		return "light footsteps"
	default:
		return "footsteps"
	}
}

// Sees reports whether the player sees the given position.
func (pl *Player) Sees(p gruid.Point) bool {
	return pl.AFOV[p]
}

// SeesPlayer reports whether the monster can see the player.
func (m *Monster) SeesPlayer(g *Game) bool {
	return m.Sees(g, g.Player.P) && g.Player.Sees(m.P)
}

// SeesLight reports whether the monster (a guard in practice) would see a
// campfire at the given position.
func (m *Monster) SeesLight(g *Game, p gruid.Point) bool {
	if !(m.FOV[p] && inViewCone(m.Dir, m.P, p)) {
		return false
	}
	if m.State == Resting && distance(m.P, p) > 1 {
		return false
	}
	return true
}

// Sees reports whether the monster can see the given position.
func (m *Monster) Sees(g *Game, p gruid.Point) bool {
	var darkRange = 4
	if m.Kind == MonsHazeCat {
		darkRange = DefaultMonsterFOVRange
	}
	if g.Player.Inventory.Body == CloakShadows {
		darkRange--
	}
	if g.Player.HasStatus(StatusShadows) {
		darkRange = 1
	}
	const tableRange = 1
	if !(m.FOV[p] && (inViewCone(m.Dir, m.P, p) || m.Kind == MonsSpider)) {
		return false
	}
	if m.State == Resting && distance(m.P, p) > 1 {
		return false
	}
	t := g.Map.At(p)
	if (!g.Illuminated(p) && !g.Player.HasStatus(StatusIlluminated) || !IsIlluminable(t)) && distance(m.P, p) > darkRange {
		return false
	}
	if t == TableCell && distance(m.P, p) > tableRange {
		return false
	}
	if g.Player.HasStatus(StatusTransparent) && g.Illuminated(p) && distance(m.P, p) > 1 {
		return false
	}
	return true
}

// ComputeMonsterFOV updates the field of view of all monsters.
func (g *Game) ComputeMonsterFOV() {
	for k := range g.MonsterFOV {
		delete(g.MonsterFOV, k)
	}
	for _, mons := range g.Monsters {
		if !mons.Exists() || !g.Player.Sees(mons.P) {
			continue
		}
		for p := range g.Player.AFOV {
			if !g.Player.Sees(p) {
				continue
			}
			if mons.Sees(g, p) {
				g.MonsterFOV[p] = true
			}
		}
	}
	if g.MonsterFOV[g.Player.P] {
		g.Player.Statuses[StatusUnhidden] = 1
		g.Player.Statuses[StatusHidden] = 0
	} else {
		g.Player.Statuses[StatusUnhidden] = 0
		g.Player.Statuses[StatusHidden] = 1
	}
	if g.Illuminated(g.Player.P) && IsIlluminable(g.Map.At(g.Player.P)) {
		g.Player.Statuses[StatusLight] = 1
	} else {
		g.Player.Statuses[StatusLight] = 0
	}
}

// ComputeLights updates information about lighted terrain.
func (g *Game) ComputeLights() {
	if g.LightFOV == nil {
		g.LightFOV = rl.NewFOV(gruid.NewRange(0, 0, MapWidth, MapHeight))
	}
	sources := []gruid.Point{}
	pt := g.Map.At(g.Player.P)
	for lpos, on := range g.Objects.Lights {
		if !on {
			continue
		}
		if distance(lpos, g.Player.P) > DefaultFOVRange+LightRange && pt != TreeCell {
			continue
		}
		sources = append(sources, lpos)
	}
	for _, mons := range g.Monsters {
		if !mons.Exists() || mons.Kind != MonsButterfly || mons.Status(MonsConfused) || mons.Status(MonsParalysed) {
			continue
		}
		if distance(mons.P, g.Player.P) > DefaultFOVRange+LightRange && pt != TreeCell {
			continue
		}
		sources = append(sources, mons.P)
	}
	lt := &lighter{rs: LightRay, g: g}
	g.LightFOV.LightMap(lt, sources)
}

// ComputeMonsterCone updates information about a specific monster's cone of
// view (for display purposes when examining a monster).
func (g *Game) ComputeMonsterCone(m *Monster) {
	g.MonsterTargFOV = make(map[gruid.Point]bool)
	for p := range g.Player.AFOV {
		if !g.Player.Sees(p) {
			continue
		}
		if m.Sees(g, p) {
			g.MonsterTargFOV[p] = true
		}
	}
}

// UpdateKnowledge updates player's position knowledge about the given monster.
func (m *Monster) UpdateKnowledge(g *Game, p gruid.Point) {
	if mons := g.KnownMonsterAt(p); mons.Exists() {
		if mons.Index != m.Index {
			mons.LastKnownPos = invalidPos
		}
	}
	if m.LastKnownPos != invalidPos {
		delete(g.KnownMonster, m.LastKnownPos)
	}
	g.KnownMonster[p] = m.Index
	m.LastKnownState = m.State
	m.LastKnownPos = p
}

// KnownMonsterAt returns the memory of a monster at the given position, or nil
// otherwise.
func (g *Game) KnownMonsterAt(p gruid.Point) *Monster {
	idx, ok := g.KnownMonster[p]
	if ok {
		return g.Monsters[idx]
	}
	return nil
}
