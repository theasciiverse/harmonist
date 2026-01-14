// This file implements a line of sight algorithm.
//
// It works in a way that can remind of the Dijkstra algorithm, but within each
// cone between a diagonal and an orthogonal line, only movements along those
// two directions are allowed. This allows the algorithm to be a simple pass on
// squares around the player, starting from radius 1 until line of sight range.
//
// Going from a gruid.Point from to a gruid.Point pos has a cost, which depends
// essentially on the type of terrain in from. Some circumstances, such as
// being on top of a tree, can influence the cost of terrains.
//
// The obtained light rays are lines formed using at most two adjacent
// directions: a diagonal and an orthogonal one (for example north east and
// east).

package main

import (
	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/paths"
	"codeberg.org/anaseto/gruid/rl"
)

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
		opaque, hard := g.DiagonalOpaque(from, to)
		if opaque {
			return wallcost
		}
		if rs != TreePlayerRay && hard {
			return wallcost - 1
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
	opaque, hard := g.DiagonalOpaque(from, to)
	if opaque {
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
	if rs != TreePlayerRay && hard {
		cost := wallcost - distance(from, src) - 1
		if cost < 1 {
			cost = 1
		}
		return cost
	}
	if rs == TreePlayerRay && t == WindowCell && distance(src, from) >= DefaultLOSRange {
		return wallcost - distance(src, from) - 1
	}
	return distance(to, from)
}

func (lt *lighter) MaxCost(src gruid.Point) int {
	switch lt.rs {
	case TreePlayerRay:
		return TreeRange + 1
	case MonsterRay:
		return DefaultMonsterLOSRange + 1
	case LightRay:
		return LightRange
	default:
		return DefaultLOSRange + 1
	}
}

func (g *Game) DiagonalOpaque(from, to gruid.Point) (opaque, hard bool) {
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
		return false, false
	}
	opaque = true
	hard = true
	for _, p := range ps {
		_, ok := g.Map.Clouds[p]
		if ok {
			continue
		}
		switch g.Map.Terrain.AtU(p) {
		case WindowCell:
			hard = false
		case WallCell, HoledWallCell:
		case FoliageCell:
			opaque = false
		default:
			return false, false
		}
	}
	return opaque, hard
}

type raystyle int

const (
	NormalPlayerRay raystyle = iota
	MonsterRay
	TreePlayerRay
	LightRay
)

const LightRange = 6

const DefaultLOSRange = 12
const DefaultMonsterLOSRange = 12

func (g *Game) StopAuto() {
	if g.autoexploring && !g.autoHalt {
		g.Print("You stop exploring.")
	} else if g.autoDir != ZP {
		g.Print("You stop.")
	} else if g.autoTarget != invalidPos {
		g.Print("You stop.")
	}
	g.autoHalt = true
	g.autoDir = ZP
	g.autoTarget = invalidPos
}

const TreeRange = 50

func (g *Game) Illuminated(p gruid.Point) bool {
	c, ok := g.LightFOV.At(p)
	return ok && c <= LightRange
}

func (g *Game) blocksSSCLOS(p gruid.Point) bool {
	return g.Map.At(p) != WallCell
}

func (g *Game) ComputeLOS() {
	g.ComputeLights()
	for k := range g.Player.LOS {
		delete(g.Player.LOS, k)
	}
	c := g.Map.At(g.Player.P)
	rs := NormalPlayerRay
	maxDepth := DefaultLOSRange
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
		g.blocksSSCLOS,
		false,
	)
	for _, n := range lnodes {
		if !g.Player.FOV.Visible(n.P) {
			continue
		}
		if n.Cost <= DefaultLOSRange {
			g.Player.LOS[n.P] = true
		} else if c == TreeCell && g.Illuminated(n.P) && n.Cost <= TreeRange {
			if g.Map.At(n.P) == WallCell {
				// this is just an approximation, but ok in practice
				nb := nbs.All(n.P, func(npos gruid.Point) bool {
					if !valid(npos) || !g.Illuminated(npos) || g.Map.At(npos) == WallCell {
						return false
					}
					cost, ok := g.Player.FOV.At(npos)
					return ok && cost < TreeRange
				})

				if len(nb) == 0 {
					continue
				}
			}
			g.Player.LOS[n.P] = true
		}
	}
	for p := range g.Player.LOS {
		if g.Player.Sees(p) {
			g.SeePosition(p)
		}
	}
	for _, mons := range g.Monsters {
		if mons.Exists() && g.Player.Sees(mons.P) {
			mons.ComputeLOS(g) // approximation of what the monster will see for player info purposes
			if mons.Seen {
				g.StopAuto()
				continue
			}
			mons.Seen = true
			g.Printf("You see %s (%v).", mons.Kind.Indefinite(false), mons.State)
			if mons.Kind.Notable() {
				g.StoryPrintf("Saw %s", mons.Kind)
			}
			g.StopAuto()
		}
	}
}

func visionRange(p gruid.Point, radius int) gruid.Range {
	drg := gruid.NewRange(0, 0, MapWidth, MapHeight)
	delta := gruid.Point{radius, radius}
	return drg.Intersect(gruid.Range{Min: p.Sub(delta), Max: p.Add(delta).Shift(1, 1)})
}

func (g *Game) mfovSetCenter(p gruid.Point) {
	if g.mfov == nil {
		g.mfov = rl.NewFOV(visionRange(p, DefaultMonsterLOSRange))
	} else {
		g.mfov.SetRange(visionRange(p, DefaultMonsterLOSRange))
	}
}

func (m *Monster) ComputeLOS(g *Game) {
	if m.Kind.Peaceful() {
		return
	}
	for k := range m.LOS {
		delete(m.LOS, k)
	}
	g.mfovSetCenter(m.P)
	lt := &lighter{rs: MonsterRay, g: g}
	lnodes := g.mfov.VisionMap(lt, m.P)
	g.mfov.SSCVisionMap(
		m.P, DefaultMonsterLOSRange,
		g.blocksSSCLOS,
		false,
	)
	for _, n := range lnodes {
		if !g.mfov.Visible(n.P) {
			continue
		}
		if n.P == m.P {
			m.LOS[n.P] = true
			continue
		}
		if n.Cost <= DefaultMonsterLOSRange && g.Map.At(n.P) != BarrelCell {
			pnode, ok := g.mfov.From(lt, n.P)
			if !ok || !Hides(g.Map.At(pnode.P)) {
				m.LOS[n.P] = true
			}
		}
	}
}

func (g *Game) SeeNotable(t rl.Cell, p gruid.Point) {
	switch t {
	case MagaraCell:
		mag := g.Objects.Magaras[p]
		dp := &mappingPath{g: g}
		path := g.PR.AstarPath(dp, g.Player.P, p)
		if len(path) > 0 {
			g.StoryPrintf("Spotted %s (distance: %d)", mag, len(path))
		} else {
			g.StoryPrintf("Spotted %s", mag)
		}
	case ItemCell:
		it := g.Objects.Items[p]
		dp := &mappingPath{g: g}
		path := g.PR.AstarPath(dp, g.Player.P, p)
		if len(path) > 0 {
			g.StoryPrintf("Spotted %s (distance: %d)", it.ShortDesc(g), len(path))
		} else {
			g.StoryPrintf("Spotted %s", it.ShortDesc(g))
		}
	case StairCell:
		st := g.Objects.Stairs[p]
		dp := &mappingPath{g: g}
		path := g.PR.AstarPath(dp, g.Player.P, p)
		if len(path) > 0 {
			g.StoryPrintf("Discovered %s (distance: %d)", st, len(path))
		} else {
			g.StoryPrintf("Discovered %s", st)
		}
	case FakeStairCell:
		dp := &mappingPath{g: g}
		path := g.PR.AstarPath(dp, g.Player.P, p)
		if len(path) > 0 {
			g.StoryPrintf("Discovered %s (distance: %d)", normalStairShortDesc, len(path))
		} else {
			g.StoryPrintf("Discovered %s", normalStairShortDesc)
		}
	case StoryCell:
		st := g.Objects.Story[p]
		if st == StoryArtifactSealed {
			dp := &mappingPath{g: g}
			path := g.PR.AstarPath(dp, g.Player.P, p)
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
	if g.Objects.Story[p] == StoryShaedra && !g.LiberatedShaedra &&
		(distance(g.Player.P, p) <= 1 ||
			distance(g.Player.P, g.Places.Marevor) <= 1 ||
			distance(g.Player.P, g.Places.Monolith) <= 1) &&
		g.Player.P != g.Places.Marevor &&
		g.Player.P != g.Places.Monolith {
		g.PushEventFirst(&playerEvent{Action: StorySequence}, g.Turn)
		g.LiberatedShaedra = true
	}
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
			g.PrintfStyled("You %s %s.", logNotable, see, g.TerrainShortDesc(t, p))
			g.StopAuto()
			g.SeeNotable(t, p)
		}
	default:
		// XXX this can be improved to handle more terrain types changes
		if t == WallCell && kt != WallCell {
			g.Printf("There is no longer a wall there.")
			g.StopAuto()
			g.autoexploreMapRebuild = true
		} else if cld, ok := g.Map.Clouds[p]; ok && cld == CloudFire && t != kt && Flammable(kt) {
			g.Printf("There are flames there.")
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

func (g *Game) ComputeNoise() {
	dij := &noisePath{g: g}
	rg := DefaultLOSRange
	nodes := g.PR.BreadthFirstMap(dij, []gruid.Point{g.Player.P}, rg)
	count := 0
	for _, mons := range g.Monsters {
		if mons != nil {
			mons.Noise = false
		}
	}
	rmax := 2
	goodhearing := g.Player.Inventory.Body == CloakHear
	for _, n := range nodes {
		if g.Player.Sees(n.P) {
			continue
		}
		mons := g.MonsterAt(n.P)
		if mons.Exists() && mons.State != Resting && mons.State != Watching &&
			(RandInt(rmax) > 0 || goodhearing || g.Map.At(mons.P) == QueenRockCell) {
			switch mons.Kind {
			case MonsSatowalgaPlant, MonsButterfly:
				// Satowalga plants and kerejats don't make any
				// noise.
			case MonsMirrorSpecter:
				if mons.Kind == MonsMirrorSpecter && goodhearing {
					mons.Noise = true
					g.Print("You hear an imperceptible air movement.")
					count++
				}
			case MonsWingedMilfid, MonsTinyHarpy:
				mons.Noise = true
				g.Print("You hear the flapping of wings.")
				count++
			case MonsEarthDragon, MonsTreeMushroom, MonsYack:
				mons.Noise = true
				g.Print("You hear heavy footsteps.")
				count++
			case MonsWorm, MonsAcidMound:
				mons.Noise = true
				g.Print("You hear a creep noise.")
				count++
			case MonsDog, MonsBlinkingFrog, MonsHazeCat, MonsCrazyImp, MonsSpider:
				mons.Noise = true
				g.Print("You hear light footsteps.")
				count++
			default:
				mons.Noise = true
				g.Print("You hear footsteps.")
				count++
			}
		}
	}
	if count > 0 {
		g.StopAuto()
	}
}

func (pl *Player) Sees(p gruid.Point) bool {
	return pl.LOS[p]
}

func (m *Monster) SeesPlayer(g *Game) bool {
	return m.Sees(g, g.Player.P) && g.Player.Sees(m.P)
}

func (m *Monster) SeesLight(g *Game, p gruid.Point) bool {
	if !(m.LOS[p] && inViewCone(m.Dir, m.P, p)) {
		return false
	}
	if m.State == Resting && distance(m.P, p) > 1 {
		return false
	}
	return true
}

func (m *Monster) Sees(g *Game, p gruid.Point) bool {
	var darkRange = 4
	if m.Kind == MonsHazeCat {
		darkRange = DefaultMonsterLOSRange
	}
	if g.Player.Inventory.Body == CloakShadows {
		darkRange--
	}
	if g.Player.HasStatus(StatusShadows) {
		darkRange = 1
	}
	const tableRange = 1
	if !(m.LOS[p] && (inViewCone(m.Dir, m.P, p) || m.Kind == MonsSpider)) {
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

func (g *Game) ComputeMonsterLOS() {
	for k := range g.MonsterLOS {
		delete(g.MonsterLOS, k)
	}
	for _, mons := range g.Monsters {
		if !mons.Exists() || !g.Player.Sees(mons.P) {
			continue
		}
		for p := range g.Player.LOS {
			if !g.Player.Sees(p) {
				continue
			}
			if mons.Sees(g, p) {
				g.MonsterLOS[p] = true
			}
		}
	}
	if g.MonsterLOS[g.Player.P] {
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
		if distance(lpos, g.Player.P) > DefaultLOSRange+LightRange && pt != TreeCell {
			continue
		}
		sources = append(sources, lpos)
	}
	for _, mons := range g.Monsters {
		if !mons.Exists() || mons.Kind != MonsButterfly || mons.Status(MonsConfused) || mons.Status(MonsParalysed) {
			continue
		}
		if distance(mons.P, g.Player.P) > DefaultLOSRange+LightRange && pt != TreeCell {
			continue
		}
		sources = append(sources, mons.P)
	}
	lt := &lighter{rs: LightRay, g: g}
	g.LightFOV.LightMap(lt, sources)
}

func (g *Game) ComputeMonsterCone(m *Monster) {
	g.MonsterTargLOS = make(map[gruid.Point]bool)
	for p := range g.Player.LOS {
		if !g.Player.Sees(p) {
			continue
		}
		if m.Sees(g, p) {
			g.MonsterTargLOS[p] = true
		}
	}
}

// UpdateKnowledge updates player's knowledge about the given monster.
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
