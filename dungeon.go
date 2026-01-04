// many ideas here from articles found at http://www.roguebasin.com/

package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"sort"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/paths"
	"codeberg.org/anaseto/gruid/rl"
)

// Map represents a map level.
type Map struct {
	Terrain rl.Grid
}

// At returns the terrain cell at the given point.
func (m *Map) At(p gruid.Point) Cell {
	return Cell(m.Terrain.At(p))
}

// Border reports whether the given point is at map boundaries.
func (m *Map) Border(p gruid.Point) bool {
	return p.X == MapWidth-1 || p.Y == MapHeight-1 || p.X == 0 || p.Y == 0
}

// Set updates terrain at given map point.
func (m *Map) Set(p gruid.Point, c Cell) {
	oc := m.At(p)
	m.Terrain.Set(p, rl.Cell(c|oc&Explored))
}

// SetExplored marks a position as explored.
func (m *Map) SetExplored(p gruid.Point) {
	oc := m.At(p)
	m.Terrain.Set(p, rl.Cell(oc|Explored))
}

// maxIterations is used for loops that theoretically may not finish but do in
// practice, just in case.
const maxIterations = 1000

// WallCell returns a random wall position.
func (mg *MapGen) WallCell() gruid.Point {
	d := mg.m
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := d.At(p)
		if terrain(c) == WallCell {
			return p
		}
	}
}

// HasFreeNeighbor reports whether the given position has a free neighbor.
func (m *Map) HasFreeNeighbor(nbs *paths.Neighbors, p gruid.Point) bool {
	nb := nbs.Cardinal(p, valid)
	for _, q := range nb {
		if m.At(q).IsPassable() {
			return true
		}
	}
	return false
}

// HasNonWallExploredNeighbor reports whether the given point is at the
// exploration frontier.
func (g *Game) HasNonWallExploredNeighbor(p gruid.Point) bool {
	d := g.Map
	neighbors := g.cardinalNeighbors(p)
	for _, q := range neighbors {
		c := d.At(q)
		if t, ok := g.TerrainKnowledge[q]; ok {
			c = t
		}
		if !c.IsWall() && explored(c) {
			return true
		}
	}
	return false
}

// MapGen is used for map generation.
type MapGen struct {
	m         *Map
	tunnel    map[gruid.Point]bool
	room      map[gruid.Point]bool
	rooms     []*room
	spl       places
	special   specialRoom
	layout    maplayout
	PR        *paths.PathRange
	rand      *rand.Rand
	neighbors paths.Neighbors
}

// genMap generates the map using the given layout.
func (g *Game) genMap(ml maplayout) {
	mg := MapGen{}
	mg.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	mg.PR = paths.NewPathRange(gruid.NewRange(0, 0, MapWidth, MapHeight))
	mg.layout = ml
	m := &Map{}
	m.Terrain = rl.NewGrid(MapWidth, MapHeight)
	mg.m = m
	mg.tunnel = make(map[gruid.Point]bool)
	mg.room = make(map[gruid.Point]bool)
	mg.rooms = []*room{}
	switch ml {
	case AutomataCave:
		mg.genCellularAutomataCaveMap()
	case RandomWalkCave:
		mg.genCaveMap(21 * 40)
	case RandomWalkTreeCave:
		mg.genTreeCaveMap()
	case RandomSmallWalkCaveUrbanised:
		mg.genCaveMap(20 * 10)
	case NaturalCave:
		if mg.rand.IntN(3) == 0 {
			mg.genCellularAutomataCaveMap()
		} else {
			mg.genCaveMap(21 * 47)
		}
	}
	var places []gruid.Point
	var nspecial = 4
	if sr := g.Params.Special[g.Depth]; sr != noSpecialRoom {
		nspecial--
		pl := PlacementEdge
		if mg.rand.IntN(3) == 0 {
			pl = PlacementCenter
		}
		mg.special = sr
		var ok bool
		count := 0
		for {
			places, ok = mg.genRooms(sr.Templates(), 1, pl)
			count++
			if count > 150 {
				if g.Depth == WinDepth || g.Depth == MaxDepth {
					panic("special room")
				}
				break
			}
			if ok {
				break
			}
		}
	}
	switch g.Depth {
	case WinDepth:
		mg.genShaedraCell(g)
		nspecial--
	case MaxDepth:
		mg.genArtifactPlace(g)
		nspecial--
	}
	normal := 5
	if g.Depth < 3 {
		nspecial--
		normal--
	}
	switch ml {
	case RandomWalkCave:
		mg.genRooms(roomBigTemplates, nspecial-1, PlacementRandom)
		mg.genRooms(roomNormalTemplates, normal, PlacementRandom)
	case RandomWalkTreeCave:
		mg.genRooms(roomBigTemplates, nspecial+1, PlacementRandom)
		mg.genRooms(roomNormalTemplates, normal+2, PlacementRandom)
	case RandomSmallWalkCaveUrbanised:
		nspecial += 3
		mg.genRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.genRooms(roomNormalTemplates, normal+5, PlacementRandom)
	case NaturalCave:
		nspecial++
		if g.Depth == WinDepth {
			nspecial += 2
		}
		mg.genRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.genRooms(roomNormalTemplates, normal-3, PlacementRandom)
	default:
		mg.genRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.genRooms(roomNormalTemplates, normal+2, PlacementRandom)
	}
	mg.connectRooms()
	g.Map = m
	mg.putDoors()
	mg.playerStart(g, places)
	mg.clearUnconnected(g)
	if mg.rand.IntN(10) > 0 {
		var c Cell
		if mg.rand.IntN(5) > 1 {
			c = ChasmCell
		} else {
			c = WaterCell
		}
		mg.genLake(c)
		if mg.rand.IntN(5) == 0 {
			mg.genLake(c)
		}
	}
	if g.Depth < MaxDepth {
		if g.Params.Blocked[g.Depth] {
			mg.genStairs(g, SealedStair)
		} else {
			mg.genStairs(g, NormalStair)
		}
		mg.genFakeStairs(g)
	}
	for i := 0; i < 4+mg.rand.IntN(2); i++ {
		mg.genBarrel(g)
	}
	mg.addSpecialFeatures(g, ml)
	mg.PR.CCMapAll(newPather(func(p gruid.Point) bool {
		return valid(p) && g.Map.At(p).IsPassable()
	}))
	mg.genMonsters(g)
	mg.putCavernCells()
	if mg.rand.IntN(2) == 0 {
		mg.genQueenRock()
	}
}

// placement represents various kinds of vault placements.
type placement int

const (
	PlacementRandom placement = iota
	PlacementCenter
	PlacementEdge
)

// genRooms generates the vaults.
func (mg *MapGen) genRooms(templates []string, n int, pl placement) (ps []gruid.Point, ok bool) {
	if len(templates) == 0 {
		return nil, false
	}
	ok = true
	for range n {
		var r *room
		count := 500
		var p gruid.Point
		var tpl string
		for r == nil && count > 0 {
			count--
			switch pl {
			case PlacementRandom:
				p = gruid.Point{mg.rand.IntN(MapWidth - 1), mg.rand.IntN(MapHeight - 1)}
			case PlacementCenter:
				p = gruid.Point{MapWidth/2 - 4 + mg.rand.IntN(5), MapHeight/2 - 3 + mg.rand.IntN(4)}
			case PlacementEdge:
				if mg.rand.IntN(2) == 0 {
					p = gruid.Point{mg.rand.IntN(MapWidth / 4), mg.rand.IntN(MapHeight - 1)}
				} else {
					p = gruid.Point{3*MapWidth/4 + mg.rand.IntN(MapWidth/4) - 1, mg.rand.IntN(MapHeight - 1)}
				}
			}
			tpl = templates[mg.rand.IntN(len(templates))]
			r = mg.newRoom(p, tpl)
		}
		if r != nil {
			switch pl {
			case PlacementCenter, PlacementEdge:
				r.special = true
			}
			mg.rooms = append(mg.rooms, r)
			ps = append(ps, p)
		} else {
			ok = false
		}
	}
	return ps, ok
}

// Newroom creates a new room at the given position of the given kind.
func (mg *MapGen) newRoom(rpos gruid.Point, kind string) *room {
	r := &room{p: rpos, vault: &rl.Vault{}}
	err := r.vault.Parse(kind)
	if err != nil {
		log.Printf("bad vault:%v", err)
		return nil
	}
	r.w = r.vault.Size().X
	r.h = r.vault.Size().Y
	drev := 2
	if r.w > r.h+2 {
		drev += r.w - r.h - 2
		if drev > 4 {
			drev = 4
		}
	}
	if mg.rand.IntN(drev) == 0 {
		switch mg.rand.IntN(2) {
		case 0:
			r.vault.Reflect()
			r.vault.Rotate(1 + 2*mg.rand.IntN(2))
		default:
			r.vault.Rotate(1 + 2*mg.rand.IntN(2))
		}
	} else {
		switch mg.rand.IntN(2) {
		case 0:
			r.vault.Reflect()
			r.vault.Rotate(2 * mg.rand.IntN(2))
		default:
			r.vault.Rotate(2 * mg.rand.IntN(2))
		}
	}
	r.w = r.vault.Size().X
	r.h = r.vault.Size().Y
	if !r.HasSpace(mg) {
		return nil
	}
	r.Dig(mg)
	if r.w == 0 || r.h == 0 {
		log.Printf("bad vault size: %v", r.vault.Size())
	}
	return r
}

// connectRooms connects the rooms through tunnels to ensure connexity.
func (mg *MapGen) connectRooms() {
	sort.Sort(roomSlice(mg.rooms))
	for i, r := range mg.rooms {
		if i == 0 {
			continue
		}
		if r.tunnels > 0 {
			continue
		}
		nearest := mg.nearestConnectedRoom(i)
		ok := mg.connectRoomsShortestPath(nearest, i)
		for !ok {
			panic("connect rooms")
		}
	}
	extraTunnels := 6
	switch mg.layout {
	case RandomSmallWalkCaveUrbanised:
		extraTunnels = 7
	case NaturalCave:
		extraTunnels = 4
	}
	count := 0
	for i, r := range mg.rooms {
		if count >= extraTunnels {
			break
		}
		if r.tunnels > 1 {
			continue
		}
		count++
		mg.connectRoomsShortestPath(i, mg.nearRoom(i))
	}
	if count >= extraTunnels {
		return
	}
	for i, r := range mg.rooms {
		if count >= extraTunnels {
			break
		}
		if r.tunnels > 2 {
			continue
		}
		count++
		mg.connectRoomsShortestPath(i, mg.nearRoom(i))
	}
}

// connectRoomsShortestPath connects two rooms with a paved ground tunnel.
func (mg *MapGen) connectRoomsShortestPath(i, j int) bool {
	if i == j {
		return false
	}
	r1 := mg.rooms[i]
	r2 := mg.rooms[j]
	var e1pos, e2pos gruid.Point
	var e1i, e2i int
	e1i = r1.UnusedEntry()
	e1pos = r1.entries[e1i].p
	e2i = r2.UnusedEntry()
	e2pos = r2.entries[e2i].p
	tp := &tunnelPath{dg: mg}
	path := mg.PR.AstarPath(tp, e1pos, e2pos)
	if len(path) == 0 {
		log.Printf("no path from %v to %v", e1pos, e2pos)
		return false
	}
	for _, p := range path {
		if !valid(p) {
			panic(fmt.Sprintf("gruid.Point %v from %v to %v", p, e1pos, e2pos))
		}
		t := terrain(mg.m.At(p))
		if t == WallCell || t == ChasmCell || t == GroundCell || t == FoliageCell {
			mg.m.Set(p, GroundCell)
			mg.tunnel[p] = true
		}
	}
	r1.entries[e1i].used = true
	r2.entries[e2i].used = true
	r1.tunnels++
	r2.tunnels++
	return true
}

// roomSlice is used for sorting vaults/rooms.
type roomSlice []*room

func (rs roomSlice) Len() int      { return len(rs) }
func (rs roomSlice) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }
func (rs roomSlice) Less(i, j int) bool {
	center := gruid.Point{MapWidth / 2, MapHeight / 2}
	ipos := rs[i].p
	ipos.X += rs[i].w / 2
	ipos.Y += rs[i].h / 2
	jpos := rs[j].p
	jpos.X += rs[j].w / 2
	jpos.Y += rs[j].h / 2
	return rs[i].special || !rs[j].special && distance(ipos, center) <= distance(jpos, center)
}

func (mg *MapGen) nearestConnectedRoom(i int) (k int) {
	r := mg.rooms[i]
	d := unreachable
	for j, nextRoom := range mg.rooms[:i] {
		if j == i {
			continue
		}
		nd := roomDistance(r, nextRoom)
		if nd < d {
			d = nd
			k = j
		}
	}
	return k
}

func (mg *MapGen) nearRoom(i int) (k int) {
	r := mg.rooms[i]
	d := unreachable
	for j, nextRoom := range mg.rooms {
		if j == i {
			continue
		}
		nd := roomDistance(r, nextRoom)
		if nd < d {
			n := mg.rand.IntN(5)
			if n > 0 {
				d = nd
				k = j
			}
		}
	}
	return k
}

// putDoors places doors.
func (mg *MapGen) putDoors() {
	for _, r := range mg.rooms {
		for _, e := range r.entries {
			if e.used && !e.virtual {
				r.places = append(r.places, place{p: e.p, kind: PlaceDoor})
			}
		}
		for _, pl := range r.places {
			if pl.kind != PlaceDoor {
				continue
			}
			mg.m.Set(pl.p, DoorCell)
			r.places = append(r.places, place{p: pl.p, kind: PlaceDoor})
		}
	}
}

// putHoledWalls adds n holed walls (if possible).
func (mg *MapGen) putHoledWalls(g *Game, n int) {
	candidates := []gruid.Point{}
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if mg.room[p] && g.holedWallCandidate(p) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return
	}
	for range n {
		p := candidates[mg.rand.IntN(len(candidates))]
		g.Map.Set(p, HoledWallCell)
	}
}

// putWindows adds n windows (if possible).
func (mg *MapGen) putWindows(g *Game, n int) {
	candidates := []gruid.Point{}
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if mg.room[p] && g.holedWallCandidate(p) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return
	}
	for range n {
		p := candidates[mg.rand.IntN(len(candidates))]
		g.Map.Set(p, WindowCell)
	}
}

// holedWallCandidate reports whether the given position is suitable for a
// holed wall.
func (g *Game) holedWallCandidate(p gruid.Point) bool {
	d := g.Map
	if !valid(p) || !d.At(p).IsWall() {
		return false
	}
	return valid(p.Add(gruid.Point{-1, 0})) && valid(p.Add(gruid.Point{1, 0})) &&
		d.At(p.Add(gruid.Point{-1, 0})).IsWall() && d.At(p.Add(gruid.Point{1, 0})).IsWall() &&
		valid(p.Add(gruid.Point{0, -1})) && d.At(p.Add(gruid.Point{0, -1})).IsPassable() &&
		valid(p.Add(gruid.Point{0, 1})) && d.At(p.Add(gruid.Point{0, 1})).IsPassable() ||
		(valid(p.Add(gruid.Point{-1, 0})) && valid(p.Add(gruid.Point{1, 0})) &&
			d.At(p.Add(gruid.Point{-1, 0})).IsPassable() && d.At(p.Add(gruid.Point{1, 0})).IsPassable() &&
			valid(p.Add(gruid.Point{0, -1})) && d.At(p.Add(gruid.Point{0, -1})).IsWall() &&
			valid(p.Add(gruid.Point{0, 1})) && d.At(p.Add(gruid.Point{0, 1})).IsWall())
}

// genShaedraCell places Shaedra, Marevor, and the monolith.
func (mg *MapGen) genShaedraCell(g *Game) {
	g.Objects.Story = map[gruid.Point]story{}
	g.Places.Shaedra = mg.spl.Shaedra
	g.Objects.Story[g.Places.Shaedra] = StoryShaedra
	g.Places.Monolith = mg.spl.Monolith
	g.Objects.Story[g.Places.Monolith] = NoStory
	g.Places.Marevor = mg.spl.Marevor
	g.Objects.Story[g.Places.Marevor] = NoStory
}

// genArtifactPlace places the artifact and the monolith.
func (mg *MapGen) genArtifactPlace(g *Game) {
	g.Objects.Story = map[gruid.Point]story{}
	g.Places.Artifact = mg.spl.Artifact
	g.Objects.Story[g.Places.Artifact] = StoryArtifactSealed
	g.Places.Monolith = mg.spl.Monolith
	g.Objects.Story[g.Places.Monolith] = NoStory
	g.Places.Marevor = mg.spl.Marevor
	g.Objects.Story[g.Places.Marevor] = NoStory
}

// mapLayout represents the various kinds of map layouts.
type maplayout int

const (
	AutomataCave maplayout = iota
	RandomWalkCave
	RandomWalkTreeCave
	RandomSmallWalkCaveUrbanised
	NaturalCave
)

// putCavernCells ensures distinction between paved ground and natural cave
// ground (for patrol purposes).
func (mg *MapGen) putCavernCells() {
	d := mg.m
	// TODO: improve handling and placement of this
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if terrain(Cell(it.Cell())) == GroundCell && !mg.room[p] && !mg.tunnel[p] {
			d.Set(p, CavernCell)
		}
	}
}

// clearUnconnected ensures that no unconnected terrain remains.
func (dg *MapGen) clearUnconnected(g *Game) {
	d := dg.m
	sp := newPather(func(p gruid.Point) bool { return d.At(p).IsPlayerPassable() })
	dg.PR.CCMap(sp, g.Player.P)
	mg := rl.MapGen{Grid: dg.m.Terrain}
	mg.KeepCC(dg.PR, g.Player.P, rl.Cell(WallCell))
}

// addSpecialFeatures handles placement of stones, bananas, potions, tables,
// lights, trees, holed walls, windows, and lore.
func (mg *MapGen) addSpecialFeatures(g *Game, ml maplayout) {
	g.Objects.Stones = map[gruid.Point]stone{}
	if g.Params.Blocked[g.Depth] || g.Depth == MaxDepth {
		mg.genSealStone(g)
	}
	bananas := 1
	bananas += g.Params.ExtraBanana[g.Depth]
	for i := 0; i < bananas; i++ {
		mg.genBanana(g)
	}
	if !g.Params.NoMagara[g.Depth] {
		mg.genMagara(g)
	}
	mg.genItem(g)
	mg.genPotion(g, MagicPotion)
	if g.Params.HealthPotion[g.Depth] {
		mg.genPotion(g, HealthPotion)
	}
	mg.genStones(g)
	ntables := 4
	switch ml {
	case AutomataCave, RandomWalkCave, NaturalCave:
		if mg.rand.IntN(3) == 0 {
			ntables++
		} else if mg.rand.IntN(10) == 0 {
			ntables--
		}
	case RandomWalkTreeCave:
		if mg.rand.IntN(4) > 0 {
			ntables++
		}
		if mg.rand.IntN(4) > 0 {
			ntables++
		}
	case RandomSmallWalkCaveUrbanised:
		ntables += 2
		if mg.rand.IntN(4) > 0 {
			ntables++
		}
	}
	if g.Params.Tables[g.Depth] {
		ntables += 2 + mg.rand.IntN(2)
	}
	for i := 0; i < ntables; i++ {
		mg.genTable(g)
	}
	mg.genLights(g)
	ntrees := 1
	switch ml {
	case AutomataCave:
		if mg.rand.IntN(4) == 0 {
			ntrees++
		} else if mg.rand.IntN(8) == 0 {
			ntrees--
		}
	case RandomWalkCave:
		if mg.rand.IntN(4) > 0 {
			ntrees++
		}
		if mg.rand.IntN(8) == 0 {
			ntrees++
		}
	case NaturalCave:
		ntrees++
		if mg.rand.IntN(2) > 0 {
			ntrees++
		}
	case RandomWalkTreeCave, RandomSmallWalkCaveUrbanised:
		if mg.rand.IntN(2) == 0 {
			ntrees--
		}
	}
	if g.Params.Trees[g.Depth] {
		ntrees += 2 + mg.rand.IntN(2)
	}
	for i := 0; i < ntrees; i++ {
		mg.genTree(g)
	}
	nhw := 1
	if mg.rand.IntN(3) > 0 {
		nhw++
	}
	if g.Params.Holes[g.Depth] {
		nhw += 3 + mg.rand.IntN(2)
	}
	switch ml {
	case RandomSmallWalkCaveUrbanised:
		if mg.rand.IntN(4) > 0 {
			nhw++
		}
	}
	mg.putHoledWalls(g, nhw)
	nwin := 1
	if nhw == 1 {
		nwin++
	}
	if g.Params.Windows[g.Depth] {
		nwin += 4 + mg.rand.IntN(3)
	}
	switch ml {
	case RandomSmallWalkCaveUrbanised:
		if mg.rand.IntN(4) > 0 {
			nwin++
		}
	}
	mg.putWindows(g, nwin)
	if g.Params.Lore[g.Depth] {
		mg.putLore(g)
	}
}

// putLore generates a lore message.
func (mg *MapGen) putLore(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 2000 {
			panic("infinite loop")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	count = 0
	for {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		i := mg.rand.IntN(len(LoreMessages))
		if g.GeneratedLore[i] {
			continue
		}
		g.GeneratedLore[i] = true
		g.Objects.Lore[p] = i
		g.Objects.Scrolls[p] = ScrollLore
		g.Map.Set(p, ScrollCell)
		break
	}
}

// genLights generates lights.
func (mg *MapGen) genLights(g *Game) {
	no := 2
	ni := 8
	switch mg.layout {
	case NaturalCave:
		no += mg.rand.IntN(2)
		ni += mg.rand.IntN(3)
	case AutomataCave, RandomWalkCave:
		ni += mg.rand.IntN(4)
	case RandomWalkTreeCave:
		no--
		ni += mg.rand.IntN(4)
	case RandomSmallWalkCaveUrbanised:
		no--
		no -= mg.rand.IntN(2)
		ni += 2
		ni += mg.rand.IntN(4)
	}
	for i := 0; i < no; i++ {
		p := mg.outsideGroundCell(g)
		g.Map.Set(p, LightCell)
	}
	for i := 0; i < ni; i++ {
		p := mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
		if p != invalidPos {
			g.Map.Set(p, LightCell)
		} else if mg.rand.IntN(10) > 0 {
			i--
		}
	}
	it := g.Map.Terrain.Iterator()
	for it.Next() {
		if it.Cell() == rl.Cell(LightCell) {
			g.Objects.Lights[it.P()] = true
		}
	}
	g.ComputeLights()
}

// playerStart generates the player's starting room features.
func (mg *MapGen) playerStart(g *Game, places []gruid.Point) {
	const far = 30
	r := mg.rooms[len(mg.rooms)-1]
loop:
	for i := len(mg.rooms) - 2; i >= 0; i-- {
		for _, p := range places {
			if distance(r.p, p) < far && distance(mg.rooms[i].p, p) >= far {
				r = mg.rooms[i]
				mg.rooms[len(mg.rooms)-1], mg.rooms[i] = mg.rooms[i], mg.rooms[len(mg.rooms)-1]
				break loop
			}
		}
	}
	g.Player.P = r.RandomPlace(PlacePatrol)
	switch g.Depth {
	case 1, 4:
	default:
		return
	}
	itpos := invalidPos
	neighbors := g.playerPassableNeighbors(g.Player.P)
	for i := range len(neighbors) {
		j := RandInt(len(neighbors) - i)
		neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
	}
loopnb:
	for _, npos := range neighbors {
		c := g.Map.At(npos)
		if c.IsGround() {
			for _, pl := range r.places {
				if npos == pl.p {
					continue loopnb
				}
			}
			itpos = npos
			break
		}
	}
	if itpos == invalidPos {
		itpos = r.RandomPlace(PlaceItem)
	}
	if itpos == invalidPos {
		itpos = r.RandomPlaces(PlaceSpecialOrStatic)
		if itpos == invalidPos {
			panic("no item")
		}
	}
	g.Map.Set(itpos, ScrollCell)
	switch g.Depth {
	case 1:
		g.Objects.Scrolls[itpos] = ScrollStory
	case 4:
		g.Objects.Scrolls[itpos] = ScrollDayoriahMessage
	}
}

// genBanana places a banana at a suitable location.
func (mg *MapGen) genBanana(g *Game) {
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := mg.m.At(p)
		if terrain(c) == GroundCell && !mg.room[p] {
			mg.m.Set(p, BananaCell)
			g.Objects.Bananas[p] = true
			break
		}
	}
}

// genPotion places a potion at a suitable vault location.
func (mg *MapGen) genPotion(g *Game, ptn potion) {
	count := 0
	p := invalidPos
	for p == invalidPos {
		count++
		if count > maxIterations {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	mg.m.Set(p, PotionCell)
	g.Objects.Potions[p] = ptn
}

// outsideGroundCell returns a random non-vault ground position.
func (mg *MapGen) outsideGroundCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.At(p)
		if terrain(c) == GroundCell && !mg.room[p] {
			return p
		}
	}
}

// outsideCavernMiddleCell returns a random non-vault ground or foliage
// position (with various biases).
func (mg *MapGen) outsideCavernMiddleCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 2500 {
			return invalidPos
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.At(p)
		if terrain(c) == GroundCell && count < 400 || terrain(c) == FoliageCell && count < 350 {
			continue
		}
		if (c.IsGround() || terrain(c) == FoliageCell) && !mg.room[p] && !mg.hasTooManyWallNeighbors(p) {
			return p
		}
	}
}

// hasTooManyWallNeighbors reports whether the given position has more than 2
// wall neighbors (excluding boundaries).
func (mg *MapGen) hasTooManyWallNeighbors(p gruid.Point) bool {
	m := mg.m
	nbs := mg.neighbors.All(p, valid)
	count := 8 - len(nbs)
	for _, q := range nbs {
		if !m.At(q).IsPassable() {
			count++
		}
	}
	return count > 1
}

// genItem generates a magical equipment item (cloak or amulet).
func (mg *MapGen) genItem(g *Game) {
	plan := g.GenPlan[g.Depth]
	if plan != GenAmulet && plan != GenCloak {
		return
	}
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	g.Map.Set(p, ItemCell)
	var it item
	switch plan {
	case GenCloak:
		it = g.RandomCloak()
		g.GeneratedCloaks = append(g.GeneratedCloaks, it)
	case GenAmulet:
		it = g.RandomAmulet()
		g.GeneratedAmulets = append(g.GeneratedAmulets, it)
	}
	g.Objects.Items[p] = it
}

// genSealStone handles steal stone generation.
func (mg *MapGen) genSealStone(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
	}
	g.Map.Set(p, StoneCell)
	g.Objects.Stones[p] = SealStone
}

// genMagara handles magara generation.
func (mg *MapGen) genMagara(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	g.Map.Set(p, MagaraCell)
	mag := g.RandomMagara()
	g.Objects.Magaras[p] = mag
	g.GeneratedMagaras = append(g.GeneratedMagaras, mag.Kind)
}

// genStairs generates real stairs (possibly sealed).
func (mg *MapGen) genStairs(g *Game, st stair) {
	var ri, pj int
	best := 0
	for i, r := range mg.rooms {
		for j, pl := range r.places {
			score := distance(pl.p, g.Player.P) + mg.rand.IntN(20)
			if !pl.used && pl.kind == PlaceSpecialStatic && score > best {
				ri = i
				pj = j
				best = distance(pl.p, g.Player.P)
			}
		}
	}
	r := mg.rooms[ri]
	r.places[pj].used = true
	r.places[pj].used = true
	p := r.places[pj].p
	g.Map.Set(p, StairCell)
	g.Objects.Stairs[p] = st
}

// genFakeStairs generates fake stairs.
func (mg *MapGen) genFakeStairs(g *Game) {
	if !g.Params.FakeStair[g.Depth] {
		return
	}
	var ri, pj int
	best := 0
loop:
	for i, r := range mg.rooms {
		for _, pl := range r.places {
			if terrain(mg.m.At(pl.p)) == StairCell {
				continue loop
			}
		}
		for j, pl := range r.places {
			score := distance(pl.p, g.Player.P) + mg.rand.IntN(20)
			if !pl.used && pl.kind == PlaceSpecialStatic && score > best {
				ri = i
				pj = j
				best = distance(pl.p, g.Player.P)
			}
		}
	}
	r := mg.rooms[ri]
	r.places[pj].used = true
	r.places[pj].used = true
	p := r.places[pj].p
	g.Map.Set(p, FakeStairCell)
	g.Objects.FakeStairs[p] = true
}

// genBarrel places a barrel in a free suitable location.
func (mg *MapGen) genBarrel(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 500 {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceSpecialStatic)
	}
	g.Map.Set(p, BarrelCell)
	g.Objects.Barrels[p] = true
}

// genTable places a table in a free suitable location.
func (mg *MapGen) genTable(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 500 {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
	}
	g.Map.Set(p, TableCell)
}

// genTree places a tree in a free suitable outside location.
func (mg *MapGen) genTree(g *Game) {
	p := mg.outsideCavernMiddleCell(g)
	if p != invalidPos {
		g.Map.Set(p, TreeCell)
	}
}

// genStones places magical stones.
func (mg *MapGen) genStones(g *Game) {
	nstones := 3
	switch mg.rand.IntN(8) {
	case 1, 2, 3, 4, 5:
		nstones++
	case 6, 7:
		nstones += 2
	}
	inroom := 2
	if g.Params.Stones[g.Depth] {
		nstones += 4 + mg.rand.IntN(3)
		inroom += 2
	}
	if mg.layout == RandomSmallWalkCaveUrbanised {
		inroom++
	}
	for i := 0; i < nstones; i++ {
		p := invalidPos
		var st stone
		if i < inroom {
			count := 0
			for p == invalidPos {
				count++
				if count > 1500 {
					p = mg.caveGroundCell()
					break
				}
				p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceStatic)
			}
			st = mg.randomInStone(g)
		} else {
			p = mg.caveGroundCell()
			st = mg.randomOutStone()
		}
		g.Objects.Stones[p] = st
		g.Map.Set(p, StoneCell)
	}
}

func (mg *MapGen) caveGroundCell() gruid.Point {
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := mg.m.At(p)
		if (terrain(c) == GroundCell || terrain(c) == CavernCell || terrain(c) == QueenRockCell) && !mg.room[p] {
			return p
		}
	}
}

func (mg *MapGen) randomInStone(g *Game) stone {
	if g.Params.MappingStone[g.Depth] {
		g.Params.MappingStone[g.Depth] = false
		return MappingStone
	}
	instones := []stone{
		BarrelStone,
		QueenStone,
		TreeStone,
		TeleportStone,
		SensingStone,
	}
	if mg.rand.IntN(2) == 0 {
		// fog stone less often inside
		instones = append(instones, FogStone)
	}
	return instones[RandInt(len(instones))]
}

func (mg *MapGen) randomOutStone() stone {
	instones := []stone{
		BarrelStone,
		FogStone,
		QueenStone,
		NightStone,
		TreeStone,
		TeleportStone,
	}
	if mg.rand.IntN(2) == 0 {
		// sensing stone less often outside
		instones = append(instones, SensingStone)
	}
	return instones[mg.rand.IntN(len(instones))]
}

func (mg *MapGen) genCellularAutomataCaveMap() {
	mg.runCellularAutomataCave()
	mg.foliage(false)
}

func (dg *MapGen) runCellularAutomataCave() bool {
	rules := []rl.CellularAutomataRule{
		{WCutoff1: 5, WCutoff2: 2, Reps: 4, WallsOutOfRange: true},
		{WCutoff1: 5, WCutoff2: 25, Reps: 3, WallsOutOfRange: true},
	}
	mg := rl.MapGen{Rand: dg.rand, Grid: dg.m.Terrain}
	if dg.rand.IntN(2) == 0 {
		mg.CellularAutomataCave(rl.Cell(WallCell), rl.Cell(GroundCell), 0.47, rules)
	} else {
		mg.CellularAutomataCave(rl.Cell(WallCell), rl.Cell(GroundCell), 0.43, rules)
	}
	return true
}

// genLake generates a lake using the given terrain for filling.
func (mg *MapGen) genLake(c Cell) {
	walls := []gruid.Point{}
	xshift := 10 + mg.rand.IntN(5)
	yshift := 5 + mg.rand.IntN(3)
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if p.X < xshift || p.Y < yshift || p.X > MapWidth-xshift || p.Y > MapHeight-yshift {
			continue
		}
		c := Cell(it.Cell())
		if terrain(c) == WallCell && !mg.room[p] {
			walls = append(walls, p)
		}
	}
	count := 0
	var bestpos = walls[RandInt(len(walls))]
	var bestsize int
	d := mg.m
	passable := func(p gruid.Point) func(q gruid.Point) bool {
		return func(q gruid.Point) bool {
			return valid(q) && terrain(mg.m.At(q)) == WallCell && !mg.room[q] && distance(p, q) < 10+mg.rand.IntN(10)
		}
	}
	for {
		p := walls[RandInt(len(walls))]
		sp := newPather(passable(p))
		size := len(mg.PR.CCMap(sp, p))
		count++
		if abs(bestsize-90) > abs(size-90) {
			bestsize = size
			bestpos = p
		}
		if count > 15 || abs(size-90) < 25 {
			break
		}
	}
	sp := newPather(passable(bestpos))
	for _, p := range mg.PR.CCMap(sp, bestpos) {
		d.Set(p, c)
	}
}

// genQueenRock places queenrock tiles.
func (mg *MapGen) genQueenRock() {
	cavern := []gruid.Point{}
	for i := range DungeonNCells {
		p := idxtopos(i)
		c := mg.m.At(p)
		if terrain(c) == CavernCell {
			cavern = append(cavern, p)
		}
	}
	if len(cavern) == 0 {
		return
	}
	for i := 0; i < 1+mg.rand.IntN(2); i++ {
		p := cavern[RandInt(len(cavern))]
		passable := func(q gruid.Point) bool {
			return valid(q) && terrain(mg.m.At(q)) == CavernCell && distance(q, p) < 15+mg.rand.IntN(5)
		}
		cp := newPather(passable)
		for _, q := range mg.PR.CCMap(cp, p) {
			mg.m.Set(q, QueenRockCell)
		}
	}

}

// foliage generates foliage.
func (dg *MapGen) foliage(less bool) {
	gd := rl.NewGrid(MapWidth, MapHeight)
	rules := []rl.CellularAutomataRule{
		{WCutoff1: 5, WCutoff2: 2, Reps: 4, WallsOutOfRange: true},
		{WCutoff1: 5, WCutoff2: 25, Reps: 2, WallsOutOfRange: true},
	}
	mg := rl.MapGen{Rand: dg.rand, Grid: gd}
	winit := 0.55
	if less {
		winit = 0.53
	}
	mg.CellularAutomataCave(rl.Cell(WallCell), rl.Cell(FoliageCell), winit, rules)
	it := dg.m.Terrain.Iterator()
	itgd := gd.Iterator()
	for it.Next() && itgd.Next() {
		if terrain(Cell(it.Cell())) == GroundCell && Cell(itgd.Cell()) == FoliageCell {
			it.SetCell(rl.Cell(FoliageCell))
		}
	}
}

// walker implements rl.RandomWalker.
type walker struct {
	rand *rand.Rand
}

// Neighbor returns a random neighbor position, favoring horizontal directions
// (because the maps we use are longer in that direction).
func (w walker) Neighbor(p gruid.Point) gruid.Point {
	switch w.rand.IntN(6) {
	case 0, 1:
		return p.Shift(1, 0)
	case 2, 3:
		return p.Shift(-1, 0)
	case 4:
		return p.Shift(0, 1)
	default:
		return p.Shift(0, -1)
	}
}

func (dg *MapGen) genCaveMap(size int) {
	mg := rl.MapGen{
		Rand: dg.rand,
		Grid: dg.m.Terrain,
	}
	wlk := walker{rand: dg.rand}
	mg.RandomWalkCave(wlk, rl.Cell(GroundCell), float64(size)/float64(DungeonNCells), 8)
	dg.foliage(false)
}

func (mg *MapGen) DigBlock(block []gruid.Point) []gruid.Point {
	d := mg.m
	p := mg.WallCell()
	block = block[:0]
	count := 0
	for {
		count++
		if count > 3000 && count%500 == 0 {
			p = mg.WallCell()
			block = block[:0]
		}
		if count > 10000 {
			panic("infinite loop")
		}
		block = append(block, p)
		if d.HasFreeNeighbor(&mg.neighbors, p) {
			break
		}
		p = randomNeighbor(p)
		if !valid(p) {
			block = block[:0]
			p = mg.WallCell()
			continue
		}
		if !valid(p) {
			return nil
		}
	}
	return block
}

func (mg *MapGen) genTreeCaveMap() {
	d := mg.m
	center := gruid.Point{40, 10}
	d.Set(center, GroundCell)
	d.Set(center.Add(gruid.Point{1, 0}), GroundCell)
	d.Set(center.Add(gruid.Point{1, -1}), GroundCell)
	d.Set(center.Add(gruid.Point{0, 1}), GroundCell)
	d.Set(center.Add(gruid.Point{1, 1}), GroundCell)
	d.Set(center.Add(gruid.Point{0, -1}), GroundCell)
	d.Set(center.Add(gruid.Point{-1, -1}), GroundCell)
	d.Set(center.Add(gruid.Point{-1, 0}), GroundCell)
	d.Set(center.Add(gruid.Point{-1, 1}), GroundCell)
	max := 21 * 21
	cells := 1
	block := make([]gruid.Point, 0, 64)
loop:
	for cells < max {
		block = mg.DigBlock(block)
		if len(block) == 0 {
			continue loop
		}
		for _, p := range block {
			if terrain(d.At(p)) != GroundCell {
				d.Set(p, GroundCell)
				cells++
			}
		}
	}
	mg.foliage(true)
}

// monster generation

// genMonsters generates and places monsters.
func (mg *MapGen) genMonsters(g *Game) {
	g.Monsters = []*Monster{}
	g.Bands = []bandInfo{}
	// common bands
	bandsGuard := []monsterBand{LoneGuard}
	bandsButterfly := []monsterBand{LoneButterfly}
	bandsHighGuard := []monsterBand{LoneHighGuard}
	bandsAnimals := []monsterBand{LoneYack, LoneWorm, LoneDog, LoneBlinkingFrog, LoneExplosiveNadre, LoneHarpy, LoneAcidMound}
	bandsPlants := []monsterBand{LoneSatowalgaPlant}
	bandsBipeds := []monsterBand{LoneOricCelmist, LoneMirrorSpecter, LoneWingedMilfid, LoneMadNixe, LoneVampire, LoneHarmonicCelmist}
	bandsRare := []monsterBand{LoneTreeMushroom, LoneEarthDragon, LoneHazeCat, LoneSpider}
	// monster specific bands
	bandNadre := []monsterBand{LoneExplosiveNadre}
	bandFrog := []monsterBand{LoneBlinkingFrog}
	bandDog := []monsterBand{LoneDog}
	bandYack := []monsterBand{LoneYack}
	bandVampire := []monsterBand{LoneVampire}
	bandOricCelmist := []monsterBand{LoneOricCelmist}
	bandHarmonicCelmist := []monsterBand{LoneHarmonicCelmist}
	bandMadNixe := []monsterBand{LoneMadNixe}
	bandMirrorSpecter := []monsterBand{LoneMirrorSpecter}
	bandTreeMushroom := []monsterBand{LoneTreeMushroom}
	bandHazeCat := []monsterBand{LoneHazeCat}
	bandSpider := []monsterBand{LoneSpider}
	bandDragon := []monsterBand{LoneEarthDragon}
	bandGuardPair := []monsterBand{PairGuard}
	bandYackPair := []monsterBand{PairYack}
	bandExplosiveNadrePair := []monsterBand{PairExplosiveNadre}
	bandWingedMilfidPair := []monsterBand{PairWingedMilfid}
	bandNixePair := []monsterBand{PairNixe}
	bandVampirePair := []monsterBand{PairVampire}
	bandOricCelmistPair := []monsterBand{PairOricCelmist}
	bandHarmonicCelmistPair := []monsterBand{PairHarmonicCelmist}
	// special bands
	if g.Params.Special[g.Depth] != noSpecialRoom {
		switch mg.special {
		case roomVampires:
			mg.putRandomBandN(g, []monsterBand{SpecialLoneVampire}, 2)
		case roomNixes:
			mg.putRandomBandN(g, []monsterBand{SpecialLoneNixe}, 2)
		case roomFrogs:
			mg.putRandomBandN(g, []monsterBand{SpecialLoneBlinkingFrog}, 2)
		case roomMilfids:
			switch mg.rand.IntN(6) {
			case 0:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneYack}, 2)
			case 1:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneDog}, 2)
			default:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneMilfid}, 2)
			}
		case roomCelmists:
			switch mg.rand.IntN(3) {
			case 0:
				bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
				mg.putRandomBandN(g, bandOricCelmists, 2)
			case 1:
				bandHarmonicCelmists := []monsterBand{SpecialLoneHarmonicCelmist}
				mg.putRandomBandN(g, bandHarmonicCelmists, 2)
			case 2:
				bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
				bandHarmonicCelmists := []monsterBand{SpecialLoneHarmonicCelmist}
				mg.putRandomBandN(g, bandHarmonicCelmists, 1)
				mg.putRandomBandN(g, bandOricCelmists, 1)
			}
		case roomHarpies:
			if mg.rand.IntN(3) > 0 {
				mg.putRandomBandN(g, []monsterBand{SpecialLoneHarpy}, 2)
			} else {
				mg.putRandomBandN(g, []monsterBand{SpecialLoneSpider}, 2)
			}
		case roomTreeMushrooms:
			if mg.rand.IntN(3) > 0 {
				mg.putRandomBandN(g, []monsterBand{SpecialLoneTreeMushroom}, 2)
			} else {
				mg.putRandomBandN(g, []monsterBand{SpecialLoneHazeCat}, 2)
			}
		case roomMirrorSpecters:
			switch mg.rand.IntN(6) {
			case 0:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneAcidMound}, 2)
			case 1:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneExplosiveNadre}, 2)
			default:
				mg.putRandomBandN(g, []monsterBand{SpecialLoneMirrorSpecter}, 2)
			}
		case roomShaedra:
			if mg.rand.IntN(3) > 0 {
				mg.putRandomBand(g, []monsterBand{SpecialLoneHighGuard})
			} else {
				mg.putRandomBand(g, []monsterBand{SpecialLoneOricCelmist})
			}
		case roomArtifact:
			switch mg.rand.IntN(3) {
			case 0:
				mg.putRandomBand(g, []monsterBand{SpecialLoneHarmonicCelmist})
			case 1:
				mg.putRandomBand(g, []monsterBand{SpecialLoneOricCelmist})
			default:
				mg.putRandomBand(g, []monsterBand{SpecialLoneHighGuard})
			}
		default:
			// XXX not used now
			bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
			mg.putRandomBandN(g, bandOricCelmists, 2)
		}
	}
	if g.Depth == g.Params.CrazyImp {
		mg.putRandomBand(g, []monsterBand{UniqueCrazyImp})
	}
	mg.putRandomBandN(g, bandsButterfly, 2)
	if mg.layout == RandomSmallWalkCaveUrbanised {
		mg.putRandomBandN(g, bandsGuard, 1+(g.Depth+1)/4)
	}
	switch g.Depth {
	case 1:
		// 8-9
		if mg.rand.IntN(2) == 0 {
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandsGuard, 5)
			} else {
				mg.putRandomBandN(g, bandsGuard, 4)
				mg.putRandomBandN(g, bandsBipeds, 1)
			}
			mg.putRandomBandN(g, bandsAnimals, 3)
		} else {
			mg.putRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(5) > 0 {
				mg.putRandomBandN(g, bandsAnimals, 5)
			} else {
				mg.putRandomBandN(g, bandsAnimals, 3)
				mg.putRandomBandN(g, bandsRare, 1)
			}
		}
	case 2:
		// 10-11
		mg.putRandomBandN(g, bandsGuard, 3)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 7
			mg.putRandomBandN(g, bandsBipeds, 1)
			mg.putRandomBandN(g, bandsAnimals, 4)
			mg.putRandomBandN(g, bandsPlants, 2)
		case 2, 3:
			// 8
			mg.putRandomBandN(g, bandsAnimals, 3)
			mg.putRandomBandN(g, bandsRare, 1)
			mg.putRandomBandN(g, bandsButterfly, 2)
			mg.putRandomBandN(g, bandsPlants, 2)
		case 4:
			// 8
			mg.putRandomBandN(g, bandsPlants, 3)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandFrog, 5)
			} else {
				mg.putRandomBandN(g, bandYack, 5)
			}
		}
	case 3:
		// 11-12
		mg.putRandomBandN(g, bandsHighGuard, 2)
		mg.putRandomBandN(g, bandsGuard, 4)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 5
			if mg.rand.IntN(3) == 0 {
				mg.putRandomBandN(g, bandDog, 3)
			} else {
				mg.putRandomBandN(g, bandsAnimals, 3)
			}
			mg.putRandomBandN(g, bandsPlants, 2)
		case 2, 3:
			// 5
			mg.putRandomBandN(g, bandsAnimals, 2)
			mg.putRandomBandN(g, bandsPlants, 1)
			mg.putRandomBandN(g, bandsBipeds, 2)
		case 4:
			// 6
			mg.putRandomBandN(g, bandsPlants, 1)
			mg.putRandomBandN(g, bandNadre, 5)
		}
	case 4:
		// 12-13
		mg.putRandomBandN(g, bandsHighGuard, 2)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 10
			mg.putRandomBandN(g, bandsGuard, 4)
			mg.putRandomBandN(g, bandsRare, 2)
			mg.putRandomBandN(g, bandGuardPair, 1)
			mg.putRandomBandN(g, bandsBipeds, 1)
			mg.putRandomBandN(g, bandsPlants, 1)
		case 2, 3:
			// 11
			mg.putRandomBandN(g, bandsGuard, 7)
			mg.putRandomBandN(g, bandGuardPair, 1)
			mg.putRandomBandN(g, bandsAnimals, 1)
			mg.putRandomBandN(g, bandsPlants, 1)
		case 4:
			mg.putRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandOricCelmist, 4)
			} else {
				mg.putRandomBandN(g, bandHarmonicCelmist, 4)
			}
			mg.putRandomBandN(g, bandsPlants, 1)
		}
	case 5:
		// 13-14
		mg.putRandomBandN(g, bandsHighGuard, 2)
		if mg.rand.IntN(2) == 0 {
			// 11
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandsGuard, 2)
				mg.putRandomBandN(g, bandGuardPair, 1)
			} else {
				mg.putRandomBandN(g, bandsGuard, 4)
			}
			mg.putRandomBandN(g, bandsAnimals, 1)
			mg.putRandomBandN(g, bandsRare, 2)
			mg.putRandomBandN(g, bandsBipeds, 3)
			mg.putRandomBandN(g, bandsPlants, 1)
		} else {
			// 12
			mg.putRandomBandN(g, bandsGuard, 2)
			mg.putRandomBandN(g, bandsAnimals, 3)
			mg.putRandomBandN(g, bandsBipeds, 2)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandOricCelmistPair, 1)
			} else {
				mg.putRandomBandN(g, bandHarmonicCelmistPair, 1)
			}
			mg.putRandomBandN(g, bandsRare, 1)
			mg.putRandomBandN(g, bandsPlants, 2)
		}
	case 6:
		// 15-17
		mg.putRandomBandN(g, bandsHighGuard, 1)
		if mg.rand.IntN(2) == 0 {
			// 14
			mg.putRandomBandN(g, bandsGuard, 3)
			mg.putRandomBandN(g, bandsAnimals, 2)
			mg.putRandomBandN(g, bandsRare, 3)
			mg.putRandomBandN(g, bandsBipeds, 1)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandYackPair, 1)
			} else {
				mg.putRandomBandN(g, bandWingedMilfidPair, 1)
			}
			mg.putRandomBandN(g, bandsPlants, 3)
		} else {
			// 16
			mg.putRandomBandN(g, bandsGuard, 2)
			if mg.rand.IntN(2) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, bandYack, 8)
				} else {
					mg.putRandomBandN(g, bandFrog, 8)
				}
			} else {
				mg.putRandomBandN(g, bandsRare, 2)
				if mg.rand.IntN(3) == 0 {
					mg.putRandomBandN(g, bandsAnimals, 4)
					mg.putRandomBandN(g, []monsterBand{PairWorm}, 1)
				} else {
					mg.putRandomBandN(g, bandsAnimals, 6)
				}
			}
			mg.putRandomBandN(g, bandsButterfly, 1)
			mg.putRandomBandN(g, bandsPlants, 5)
		}
	case 7:
		// 19
		mg.putRandomBandN(g, bandsHighGuard, 1)
		if mg.rand.IntN(2) == 0 {
			// 18
			mg.putRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(3) == 0 {
				mg.putRandomBandN(g, bandDog, 4)
				mg.putRandomBandN(g, bandsAnimals, 2)
			} else {
				mg.putRandomBandN(g, bandsAnimals, 6)
			}
			mg.putRandomBandN(g, bandsButterfly, 1)
			mg.putRandomBandN(g, bandsRare, 2)
			mg.putRandomBandN(g, bandsBipeds, 3)
			mg.putRandomBandN(g, bandsPlants, 2)
		} else {
			// 18
			mg.putRandomBandN(g, bandsGuard, 1)
			mg.putRandomBandN(g, bandsRare, 4)
			mg.putRandomBandN(g, bandsButterfly, 1)
			if mg.rand.IntN(3) == 0 {
				mg.putRandomBandN(g, bandNadre, 7)
			} else {
				mg.putRandomBandN(g, bandsAnimals, 5)
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, []monsterBand{PairFrog}, 1)
				} else {
					mg.putRandomBandN(g, []monsterBand{PairDog}, 1)
				}
			}
			mg.putRandomBandN(g, bandsPlants, 5)
		}
	case 8:
		// 18-19
		mg.putRandomBandN(g, bandsHighGuard, 4)
		if mg.rand.IntN(2) == 0 {
			// 14
			mg.putRandomBandN(g, bandsGuard, 5)
			mg.putRandomBandN(g, bandsRare, 1)
			if mg.rand.IntN(3) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, bandOricCelmist, 6)
				} else {
					mg.putRandomBandN(g, bandMadNixe, 6)
				}
				mg.putRandomBandN(g, bandsBipeds, 2)
			} else {
				mg.putRandomBandN(g, bandsBipeds, 8)
			}
		} else {
			// 15
			mg.putRandomBandN(g, bandsGuard, 5)
			mg.putRandomBandN(g, bandsAnimals, 2)
			mg.putRandomBandN(g, bandsRare, 1)
			mg.putRandomBandN(g, bandHarmonicCelmistPair, 1)
			mg.putRandomBandN(g, bandsBipeds, 4)
			mg.putRandomBandN(g, bandsPlants, 1)
		}
	case 9:
		// 20-24
		mg.putRandomBandN(g, bandsHighGuard, 2)
		if mg.rand.IntN(2) == 0 {
			// 18
			mg.putRandomBandN(g, bandsGuard, 3)
			if mg.rand.IntN(2) == 0 {
				switch mg.rand.IntN(4) {
				case 0:
					mg.putRandomBandN(g, bandTreeMushroom, 4)
					mg.putRandomBandN(g, []monsterBand{PairTreeMushroom}, 1)
				case 1:
					mg.putRandomBandN(g, bandDragon, 6)
					mg.putRandomBandN(g, bandsAnimals, 2)
				case 2:
					mg.putRandomBandN(g, bandHazeCat, 4)
					mg.putRandomBandN(g, []monsterBand{PairHazeCat}, 1)
				case 3:
					mg.putRandomBandN(g, bandSpider, 4)
					mg.putRandomBandN(g, []monsterBand{PairSpider}, 1)
				}
			} else {
				mg.putRandomBandN(g, bandsRare, 4)
				mg.putRandomBandN(g, []monsterBand{PairTreeMushroom}, 1)
			}
			mg.putRandomBandN(g, bandsRare, 4)
			mg.putRandomBandN(g, bandsAnimals, 1)
			mg.putRandomBandN(g, bandsBipeds, 2)
			mg.putRandomBandN(g, bandsPlants, 2)
		} else {
			// 22+2
			mg.putRandomBandN(g, bandsButterfly, 2)
			mg.putRandomBandN(g, bandsGuard, 2)
			mg.putRandomBandN(g, bandsAnimals, 8)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandExplosiveNadrePair, 2)
			} else {
				mg.putRandomBandN(g, bandYackPair, 2)
			}
			mg.putRandomBandN(g, bandsRare, 3)
			mg.putRandomBandN(g, bandsPlants, 3)
			mg.putRandomBandN(g, []monsterBand{PairSatowalga}, 1)
		}
	case 10:
		// 22
		mg.putRandomBandN(g, bandsHighGuard, 3)
		if mg.rand.IntN(2) == 0 {
			// 19
			mg.putRandomBandN(g, bandsGuard, 7)
			mg.putRandomBandN(g, bandGuardPair, 1)
			mg.putRandomBandN(g, bandsRare, 2)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandsBipeds, 8)
			} else {
				mg.putRandomBandN(g, bandsBipeds, 4)
				mg.putRandomBandN(g, bandMirrorSpecter, 4)
			}
		} else {
			// 19
			mg.putRandomBandN(g, bandGuardPair, 1)
			if mg.rand.IntN(3) == 0 {
				mg.putRandomBandN(g, bandsGuard, 4)
				mg.putRandomBandN(g, bandVampire, 4)
				mg.putRandomBandN(g, []monsterBand{PairVampire}, 1)
				mg.putRandomBandN(g, bandsRare, 2)
			} else {
				mg.putRandomBandN(g, bandsGuard, 6)
				mg.putRandomBandN(g, bandsBipeds, 3)
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, []monsterBand{PairNixe}, 1)
				} else {
					mg.putRandomBandN(g, []monsterBand{PairOricCelmist}, 1)
				}
				mg.putRandomBandN(g, bandsAnimals, 2)
			}
			mg.putRandomBandN(g, bandsAnimals, 2)
			mg.putRandomBandN(g, bandsPlants, 1)
			mg.putRandomBandN(g, bandsRare, 1)
		}
	case 11:
		// 26
		mg.putRandomBandN(g, bandsHighGuard, 5)
		if mg.rand.IntN(2) == 0 {
			// 21
			mg.putRandomBandN(g, bandsGuard, 5)
			mg.putRandomBandN(g, bandsRare, 2)
			if mg.rand.IntN(3) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, bandOricCelmist, 5)
				} else {
					mg.putRandomBandN(g, bandHarmonicCelmist, 5)
				}
				mg.putRandomBandN(g, bandsBipeds, 5)
			} else {
				mg.putRandomBandN(g, bandsBipeds, 10)
			}
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandVampirePair, 1)
			} else {
				if mg.rand.IntN(2) == 0 {
					mg.putRandomBandN(g, bandOricCelmistPair, 1)
				} else {
					mg.putRandomBandN(g, bandHarmonicCelmistPair, 1)
				}
			}
			mg.putRandomBandN(g, bandsAnimals, 2)
		} else {
			// 21
			mg.putRandomBandN(g, bandsGuard, 5)
			mg.putRandomBandN(g, bandOricCelmistPair, 1)
			mg.putRandomBandN(g, []monsterBand{PairGuard}, 1)
			mg.putRandomBandN(g, bandsRare, 1)
			mg.putRandomBandN(g, bandsBipeds, 7)
			if mg.rand.IntN(2) == 0 {
				mg.putRandomBandN(g, bandHarmonicCelmistPair, 1)
			} else {
				mg.putRandomBandN(g, bandNixePair, 1)
			}
			mg.putRandomBandN(g, bandsAnimals, 1)
			mg.putRandomBandN(g, bandsPlants, 1)
		}
	}
}

func (g *Game) genBand(band monsterBand) []monsterKind {
	mbd := MonsBands[band]
	if !mbd.Band {
		return []monsterKind{mbd.Monster}
	}
	bandMonsters := []monsterKind{}
	for m, n := range mbd.Distribution {
		for range n {
			bandMonsters = append(bandMonsters, m)
		}
	}
	return bandMonsters
}

func (mg *MapGen) bandInfoGuard(g *Game, band monsterBand, pl placeKind) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
loop:
	for p == invalidPos {
		count++
		if count > maxIterations {
			p = mg.insideCell(g)
			break
		}
		for range 20 {
			r := mg.rooms[RandInt(len(mg.rooms)-1)]
			for _, e := range r.places {
				if e.kind == PlaceSpecialStatic {
					p = r.RandomPlace(pl)
					break
				}
			}
			if p != invalidPos && !g.MonsterAt(p).Exists() {
				break loop
			}
		}
		r := mg.rooms[RandInt(len(mg.rooms)-1)]
		p = r.RandomPlace(pl)
	}
	bandinfo.Path = append(bandinfo.Path, p)
	bandinfo.Beh = BehGuard
	return bandinfo
}

func (mg *MapGen) bandInfoGuardSpecial(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for _, r := range mg.rooms {
		count++
		if count > 1 {
			log.Print("unavailable special guard position")
			p = mg.insideCell(g)
			break
		}
		p = r.RandomPlace(PlacePatrolSpecial)
		if p != invalidPos && !g.MonsterAt(p).Exists() {
			break
		}
	}
	bandinfo.Path = append(bandinfo.Path, p)
	bandinfo.Beh = BehGuard
	return bandinfo
}

func (mg *MapGen) bandInfoPatrol(g *Game, band monsterBand, pl placeKind) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for p == invalidPos || g.MonsterAt(p).Exists() {
		count++
		if count > 4000 {
			p = mg.insideCell(g)
			break
		}
		p = mg.rooms[RandInt(len(mg.rooms)-1)].RandomPlace(pl)
	}
	target := invalidPos
	count = 0
	for target == invalidPos {
		// TODO: only find place in other room?
		count++
		if count > 4000 {
			target = mg.insideCell(g)
			break
		}
		target = mg.rooms[RandInt(len(mg.rooms)-1)].RandomPlace(pl)
	}
	bandinfo.Path = append(bandinfo.Path, p)
	bandinfo.Path = append(bandinfo.Path, target)
	bandinfo.Beh = BehPatrol
	return bandinfo
}

func (mg *MapGen) bandInfoPatrolSpecial(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for _, r := range mg.rooms {
		count++
		if count > 1 {
			log.Print("unavailable special patrol position")
			p = mg.insideCell(g)
			break
		}
		p = r.RandomPlace(PlacePatrolSpecial)
		if p != invalidPos && !g.MonsterAt(p).Exists() {
			break
		}
	}
	target := invalidPos
	count = 0
	for _, r := range mg.rooms {
		count++
		if count > 1 {
			log.Print("unavailable special second patrol position")
			p = mg.insideCell(g)
			break
		}
		target = r.RandomPlace(PlacePatrolSpecial)
		if target != invalidPos {
			break
		}
	}
	bandinfo.Path = append(bandinfo.Path, p)
	bandinfo.Path = append(bandinfo.Path, target)
	bandinfo.Beh = BehPatrol
	return bandinfo
}

// insideCell returns a free room cell.
func (mg *MapGen) insideCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		if distance(p, g.Player.P) < DefaultLOSRange {
			continue
		}
		c := mg.m.At(p)
		if mg.room[p] && (terrain(c) == FoliageCell || terrain(c) == GroundCell) {
			return p
		}
	}
}

func (mg *MapGen) bandInfoOutsideGround(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.outsideGroundCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) bandInfoSatowalga(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.satowalgaCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) satowalgaCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 2000 {
			return g.freeCellForMonster()
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.At(p)
		if terrain(c) == GroundCell && count < 400 {
			continue
		}
		if c.IsGround() && !mg.room[p] && !mg.hasTooManyWallNeighbors(p) {
			return p
		}
	}
}

func (mg *MapGen) bandInfoOutside(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.outsideCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) bandInfoOutsideExplore(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.outsideCell(g))
	for range 4 {
		for range 100 {
			p := mg.outsideCell(g)
			if mg.PR.CCMapAt(p) == mg.PR.CCMapAt(bandinfo.Path[0]) {
				bandinfo.Path = append(bandinfo.Path, p)
				break
			}
		}
	}
	bandinfo.Beh = BehExplore
	return bandinfo
}

func (mg *MapGen) bandInfoOutsideExploreButterfly(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.outsideCell(g))
	for range 9 {
		for range 100 {
			p := mg.outsideCell(g)
			if mg.PR.CCMapAt(p) == mg.PR.CCMapAt(bandinfo.Path[0]) {
				bandinfo.Path = append(bandinfo.Path, p)
				break
			}
		}
	}
	bandinfo.Beh = BehExplore
	return bandinfo
}

// outsideCell returns a free non-vault position.
func (mg *MapGen) outsideCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("infinite loop")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.At(p)
		if !mg.room[p] && (terrain(c) == FoliageCell || terrain(c) == GroundCell) {
			return p
		}
	}
}

func (mg *MapGen) bandInfoFoliage(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.foliageCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

// foliageCell returns a free foliage position.
func (mg *MapGen) foliageCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			return mg.outsideCell(g)
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		if distance(p, g.Player.P) < DefaultLOSRange {
			continue
		}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.At(p)
		if terrain(c) == FoliageCell {
			return p
		}
	}
}

// freeCellForMonster returns a free position for monster placement.
func (g *Game) freeCellForMonster() gruid.Point {
	d := g.Map
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("infinite loop")
		}
		x := RandInt(MapWidth)
		y := RandInt(MapHeight)
		p := gruid.Point{x, y}
		c := d.At(p)
		if !c.IsPassable() {
			continue
		}
		if g.Player != nil && distance(g.Player.P, p) < 8 {
			continue
		}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		return p
	}
}

func (g *Game) freeCellForBandMonster(p gruid.Point) gruid.Point {
	count := 0
	for {
		count++
		if count > maxIterations {
			return g.freeCellForMonster()
		}
		neighbors := g.playerPassableNeighbors(p)
		if len(neighbors) == 0 {
			// should not happen
			log.Printf("no neighbors for %v", p)
			count = maxIterations + 1
			continue
		}
		r := RandInt(len(neighbors))
		p = neighbors[r]
		if g.Player != nil && distance(g.Player.P, p) < 8 {
			continue
		}
		mons := g.MonsterAt(p)
		if mons.Exists() || !g.Map.At(p).IsPassable() {
			continue
		}
		return p
	}
}

// putMonsterBand generates the given monster band.
func (mg *MapGen) putMonsterBand(g *Game, band monsterBand) bool {
	monsters := g.genBand(band)
	if monsters == nil {
		return false
	}
	awake := mg.rand.IntN(5) > 0
	var bdinf bandInfo
	switch band {
	case LoneYack, LoneWorm, PairYack:
		bdinf = mg.bandInfoFoliage(g, band)
	case LoneDog, LoneHarpy:
		bdinf = mg.bandInfoOutsideGround(g, band)
	case LoneBlinkingFrog, LoneExplosiveNadre, PairExplosiveNadre:
		bdinf = mg.bandInfoOutside(g, band)
	case LoneMirrorSpecter, LoneWingedMilfid, LoneVampire, PairWingedMilfid, LoneEarthDragon, LoneHazeCat, LoneSpider:
		bdinf = mg.bandInfoOutsideExplore(g, band)
	case LoneButterfly:
		bdinf = mg.bandInfoOutsideExploreButterfly(g, band)
	case LoneTreeMushroom, LoneAcidMound:
		bdinf = mg.bandInfoOutside(g, band)
	case LoneHighGuard:
		bdinf = mg.bandInfoGuard(g, band, PlacePatrol)
	case LoneSatowalgaPlant:
		bdinf = mg.bandInfoSatowalga(g, band)
	case SpecialLoneVampire, SpecialLoneNixe, SpecialLoneMilfid, SpecialLoneOricCelmist, SpecialLoneHarmonicCelmist, SpecialLoneHighGuard,
		SpecialLoneHarpy, SpecialLoneTreeMushroom, SpecialLoneMirrorSpecter, SpecialLoneHazeCat, SpecialLoneSpider, SpecialLoneAcidMound, SpecialLoneDog, SpecialLoneExplosiveNadre, SpecialLoneYack, SpecialLoneBlinkingFrog:
		if mg.rand.IntN(5) > 0 {
			bdinf = mg.bandInfoPatrolSpecial(g, band)
		} else {
			bdinf = mg.bandInfoGuardSpecial(g, band)
		}
		if !awake && mg.rand.IntN(2) > 0 {
			awake = true
		}
	case UniqueCrazyImp:
		bdinf = mg.bandInfoOutside(g, band)
		bdinf.Beh = BehCrazyImp
	default:
		bdinf = mg.bandInfoPatrol(g, band, PlacePatrol)
	}
	g.Bands = append(g.Bands, bdinf)
	var p gruid.Point
	if len(bdinf.Path) == 0 {
		// should not happen now
		p = g.freeCellForMonster()
	} else {
		p = bdinf.Path[0]
		if g.MonsterAt(p).Exists() {
			log.Printf("already monster at %v mons %v no place for %v", p, g.MonsterAt(p).Kind, monsters)
			p = g.freeCellForMonster()
		}
	}
	for i, mk := range monsters {
		mons := &Monster{Kind: mk}
		if awake {
			mons.State = Wandering
		}
		g.Monsters = append(g.Monsters, mons)
		mons.Init()
		mons.Index = len(g.Monsters) - 1
		mons.Band = len(g.Bands) - 1
		mons.PlaceAtStart(g, p)
		mons.Target = mons.NextTarget(g)
		if i < len(monsters)-1 {
			p = g.freeCellForBandMonster(p)
		}
	}
	return true
}

func (mg *MapGen) putRandomBand(g *Game, bands []monsterBand) bool {
	return mg.putMonsterBand(g, bands[RandInt(len(bands))])
}

func (mg *MapGen) putRandomBandN(g *Game, bands []monsterBand, n int) {
	for range n {
		mg.putMonsterBand(g, bands[RandInt(len(bands))])
	}
}
