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

func (m *Map) Cell(p gruid.Point) Cell {
	return Cell(m.Terrain.At(p))
}

func (m *Map) Border(p gruid.Point) bool {
	return p.X == MapWidth-1 || p.Y == MapHeight-1 || p.X == 0 || p.Y == 0
}

func (m *Map) SetCell(p gruid.Point, c Cell) {
	oc := m.Cell(p)
	m.Terrain.Set(p, rl.Cell(c|oc&Explored))
}

func (m *Map) SetExplored(p gruid.Point) {
	oc := m.Cell(p)
	m.Terrain.Set(p, rl.Cell(oc|Explored))
}

const maxIterations = 1000

func (mg *MapGen) WallCell() gruid.Point {
	d := mg.m
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("WallCell")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := d.Cell(p)
		if terrain(c) == WallCell {
			return p
		}
	}
}

func (m *Map) HasFreeNeighbor(nbs *paths.Neighbors, p gruid.Point) bool {
	nb := nbs.Cardinal(p, valid)
	for _, q := range nb {
		if m.Cell(q).IsPassable() {
			return true
		}
	}
	return false
}

func (mg *MapGen) HasTooManyWallNeighbors(p gruid.Point) bool {
	m := mg.m
	nbs := mg.neighbors.All(p, valid)
	count := 8 - len(nbs)
	for _, q := range nbs {
		if !m.Cell(q).IsPassable() {
			count++
		}
	}
	return count > 1
}

func (g *Game) HasNonWallExploredNeighbor(p gruid.Point) bool {
	d := g.Map
	neighbors := g.cardinalNeighbors(p)
	for _, q := range neighbors {
		c := d.Cell(q)
		if t, ok := g.TerrainKnowledge[q]; ok {
			c = t
		}
		if !c.IsWall() && explored(c) {
			return true
		}
	}
	return false
}

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

func (mg *MapGen) ConnectRoomsShortestPath(i, j int) bool {
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
		t := terrain(mg.m.Cell(p))
		if t == WallCell || t == ChasmCell || t == GroundCell || t == FoliageCell {
			mg.m.SetCell(p, GroundCell)
			mg.tunnel[p] = true
		}
	}
	r1.entries[e1i].used = true
	r2.entries[e2i].used = true
	r1.tunnels++
	r2.tunnels++
	return true
}

func (mg *MapGen) NewRoom(rpos gruid.Point, kind string) *room {
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

func (mg *MapGen) PutDoors(g *Game) {
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
			mg.m.SetCell(pl.p, DoorCell)
			r.places = append(r.places, place{p: pl.p, kind: PlaceDoor})
		}
	}
}

func (mg *MapGen) PutHoledWalls(g *Game, n int) {
	candidates := []gruid.Point{}
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if mg.room[p] && g.HoledWallCandidate(p) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return
	}
	for range n {
		p := candidates[mg.rand.IntN(len(candidates))]
		g.Map.SetCell(p, HoledWallCell)
	}
}

func (mg *MapGen) PutWindows(g *Game, n int) {
	candidates := []gruid.Point{}
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if mg.room[p] && g.HoledWallCandidate(p) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return
	}
	for range n {
		p := candidates[mg.rand.IntN(len(candidates))]
		g.Map.SetCell(p, WindowCell)
	}
}

func (g *Game) HoledWallCandidate(p gruid.Point) bool {
	d := g.Map
	if !valid(p) || !d.Cell(p).IsWall() {
		return false
	}
	return valid(p.Add(gruid.Point{-1, 0})) && valid(p.Add(gruid.Point{1, 0})) &&
		d.Cell(p.Add(gruid.Point{-1, 0})).IsWall() && d.Cell(p.Add(gruid.Point{1, 0})).IsWall() &&
		valid(p.Add(gruid.Point{0, -1})) && d.Cell(p.Add(gruid.Point{0, -1})).IsPassable() &&
		valid(p.Add(gruid.Point{0, 1})) && d.Cell(p.Add(gruid.Point{0, 1})).IsPassable() ||
		(valid(p.Add(gruid.Point{-1, 0})) && valid(p.Add(gruid.Point{1, 0})) &&
			d.Cell(p.Add(gruid.Point{-1, 0})).IsPassable() && d.Cell(p.Add(gruid.Point{1, 0})).IsPassable() &&
			valid(p.Add(gruid.Point{0, -1})) && d.Cell(p.Add(gruid.Point{0, -1})).IsWall() &&
			valid(p.Add(gruid.Point{0, 1})) && d.Cell(p.Add(gruid.Point{0, 1})).IsWall())
}

type placement int

const (
	PlacementRandom placement = iota
	PlacementCenter
	PlacementEdge
)

func (mg *MapGen) GenRooms(templates []string, n int, pl placement) (ps []gruid.Point, ok bool) {
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
			r = mg.NewRoom(p, tpl)
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

func (mg *MapGen) ConnectRooms() {
	sort.Sort(roomSlice(mg.rooms))
	for i, r := range mg.rooms {
		if i == 0 {
			continue
		}
		if r.tunnels > 0 {
			continue
		}
		nearest := mg.nearestConnectedRoom(i)
		ok := mg.ConnectRoomsShortestPath(nearest, i)
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
		mg.ConnectRoomsShortestPath(i, mg.nearRoom(i))
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
		mg.ConnectRoomsShortestPath(i, mg.nearRoom(i))
	}
}

type maplayout int

const (
	AutomataCave maplayout = iota
	RandomWalkCave
	RandomWalkTreeCave
	RandomSmallWalkCaveUrbanised
	NaturalCave
)

func (mg *MapGen) GenShaedraCell(g *Game) {
	g.Objects.Story = map[gruid.Point]story{}
	g.Places.Shaedra = mg.spl.Shaedra
	g.Objects.Story[g.Places.Shaedra] = StoryShaedra
	g.Places.Monolith = mg.spl.Monolith
	g.Objects.Story[g.Places.Monolith] = NoStory
	g.Places.Marevor = mg.spl.Marevor
	g.Objects.Story[g.Places.Marevor] = NoStory
}

func (mg *MapGen) GenArtifactPlace(g *Game) {
	g.Objects.Story = map[gruid.Point]story{}
	g.Places.Artifact = mg.spl.Artifact
	g.Objects.Story[g.Places.Artifact] = StoryArtifactSealed
	g.Places.Monolith = mg.spl.Monolith
	g.Objects.Story[g.Places.Monolith] = NoStory
	g.Places.Marevor = mg.spl.Marevor
	g.Objects.Story[g.Places.Marevor] = NoStory
}

func (g *Game) GenRoomTunnels(ml maplayout) {
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
		mg.GenCellularAutomataCaveMap()
	case RandomWalkCave:
		mg.GenCaveMap(21 * 40)
	case RandomWalkTreeCave:
		mg.GenTreeCaveMap()
	case RandomSmallWalkCaveUrbanised:
		mg.GenCaveMap(20 * 10)
	case NaturalCave:
		if mg.rand.IntN(3) == 0 {
			mg.GenCellularAutomataCaveMap()
		} else {
			mg.GenCaveMap(21 * 47)
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
			places, ok = mg.GenRooms(sr.Templates(), 1, pl)
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
	if g.Depth == WinDepth {
		mg.GenShaedraCell(g)
		nspecial--
	} else if g.Depth == MaxDepth {
		mg.GenArtifactPlace(g)
		nspecial--
	}
	normal := 5
	if g.Depth < 3 {
		nspecial--
		normal--
	}
	switch ml {
	case RandomWalkCave:
		mg.GenRooms(roomBigTemplates, nspecial-1, PlacementRandom)
		mg.GenRooms(roomNormalTemplates, normal, PlacementRandom)
	case RandomWalkTreeCave:
		mg.GenRooms(roomBigTemplates, nspecial+1, PlacementRandom)
		mg.GenRooms(roomNormalTemplates, normal+2, PlacementRandom)
	case RandomSmallWalkCaveUrbanised:
		nspecial += 3
		mg.GenRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.GenRooms(roomNormalTemplates, normal+5, PlacementRandom)
	case NaturalCave:
		nspecial++
		if g.Depth == WinDepth {
			nspecial += 2
		}
		mg.GenRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.GenRooms(roomNormalTemplates, normal-3, PlacementRandom)
	default:
		mg.GenRooms(roomBigTemplates, nspecial, PlacementRandom)
		mg.GenRooms(roomNormalTemplates, normal+2, PlacementRandom)
	}
	mg.ConnectRooms()
	g.Map = m
	mg.PutDoors(g)
	mg.PlayerStartCell(g, places)
	mg.ClearUnconnected(g)
	if mg.rand.IntN(10) > 0 {
		var c Cell
		if mg.rand.IntN(5) > 1 {
			c = ChasmCell
		} else {
			c = WaterCell
		}
		mg.GenLake(c)
		if mg.rand.IntN(5) == 0 {
			mg.GenLake(c)
		}
	}
	if g.Depth < MaxDepth {
		if g.Params.Blocked[g.Depth] {
			mg.GenStairs(g, BlockedStair)
		} else {
			mg.GenStairs(g, NormalStair)
		}
		mg.GenFakeStairs(g)
	}
	for i := 0; i < 4+mg.rand.IntN(2); i++ {
		mg.GenBarrel(g)
	}
	mg.AddSpecial(g, ml)
	mg.PR.CCMapAll(newPather(func(p gruid.Point) bool {
		return valid(p) && g.Map.Cell(p).IsPassable()
	}))
	mg.GenMonsters(g)
	mg.PutCavernCells(g)
	if mg.rand.IntN(2) == 0 {
		mg.GenQueenRock()
	}
}

func (mg *MapGen) PutCavernCells(g *Game) {
	d := mg.m
	// TODO: improve handling and placement of this
	it := mg.m.Terrain.Iterator()
	for it.Next() {
		p := it.P()
		if terrain(Cell(it.Cell())) == GroundCell && !mg.room[p] && !mg.tunnel[p] {
			d.SetCell(p, CavernCell)
		}
	}
}

func (dg *MapGen) ClearUnconnected(g *Game) {
	d := dg.m
	sp := newPather(func(p gruid.Point) bool { return d.Cell(p).IsPlayerPassable() })
	dg.PR.CCMap(sp, g.Player.P)
	mg := rl.MapGen{Grid: dg.m.Terrain}
	mg.KeepCC(dg.PR, g.Player.P, rl.Cell(WallCell))
}

func (mg *MapGen) AddSpecial(g *Game, ml maplayout) {
	g.Objects.Stones = map[gruid.Point]stone{}
	if g.Params.Blocked[g.Depth] || g.Depth == MaxDepth {
		mg.GenBarrierStone(g)
	}
	bananas := 1
	bananas += g.Params.ExtraBanana[g.Depth]
	for i := 0; i < bananas; i++ {
		mg.GenBanana(g)
	}
	if !g.Params.NoMagara[g.Depth] {
		mg.GenMagara(g)
	}
	mg.GenItem(g)
	mg.GenPotion(g, MagicPotion)
	if g.Params.HealthPotion[g.Depth] {
		mg.GenPotion(g, HealthPotion)
	}
	mg.GenStones(g)
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
		mg.GenTable(g)
	}
	mg.GenLight(g)
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
		mg.GenTree(g)
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
	mg.PutHoledWalls(g, nhw)
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
	mg.PutWindows(g, nwin)
	if g.Params.Lore[g.Depth] {
		mg.PutLore(g)
	}
}

func (mg *MapGen) PutLore(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 2000 {
			panic("PutLore1")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	count = 0
	for {
		count++
		if count > maxIterations {
			panic("PutLore2")
		}
		i := mg.rand.IntN(len(LoreMessages))
		if g.GeneratedLore[i] {
			continue
		}
		g.GeneratedLore[i] = true
		g.Objects.Lore[p] = i
		g.Objects.Scrolls[p] = ScrollLore
		g.Map.SetCell(p, ScrollCell)
		break
	}
}

func (mg *MapGen) GenLight(g *Game) {
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
		p := mg.OutsideGroundCell(g)
		g.Map.SetCell(p, LightCell)
	}
	for i := 0; i < ni; i++ {
		p := mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
		if p != invalidPos {
			g.Map.SetCell(p, LightCell)
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

func (mg *MapGen) PlayerStartCell(g *Game, places []gruid.Point) {
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
		c := g.Map.Cell(npos)
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
	g.Map.SetCell(itpos, ScrollCell)
	switch g.Depth {
	case 1:
		g.Objects.Scrolls[itpos] = ScrollStory
	case 4:
		g.Objects.Scrolls[itpos] = ScrollDayoriahMessage
	}
}

func (mg *MapGen) GenBanana(g *Game) {
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("GenBanana")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := mg.m.Cell(p)
		if terrain(c) == GroundCell && !mg.room[p] {
			mg.m.SetCell(p, BananaCell)
			g.Objects.Bananas[p] = true
			break
		}
	}
}

func (mg *MapGen) GenPotion(g *Game, ptn potion) {
	count := 0
	p := invalidPos
	for p == invalidPos {
		count++
		if count > maxIterations {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	mg.m.SetCell(p, PotionCell)
	g.Objects.Potions[p] = ptn
}

func (mg *MapGen) OutsideGroundCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("OutsideGroundCell")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.Cell(p)
		if terrain(c) == GroundCell && !mg.room[p] {
			return p
		}
	}
}

func (mg *MapGen) OutsideCavernMiddleCell(g *Game) gruid.Point {
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
		c := mg.m.Cell(p)
		if terrain(c) == GroundCell && count < 400 || terrain(c) == FoliageCell && count < 350 {
			continue
		}
		if (c.IsGround() || terrain(c) == FoliageCell) && !mg.room[p] && !mg.HasTooManyWallNeighbors(p) {
			return p
		}
	}
}

func (mg *MapGen) SatowalgaCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 2000 {
			return g.FreeCellForMonster()
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.Cell(p)
		if terrain(c) == GroundCell && count < 400 {
			continue
		}
		if c.IsGround() && !mg.room[p] && !mg.HasTooManyWallNeighbors(p) {
			return p
		}
	}
}

func (mg *MapGen) FoliageCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			return mg.OutsideCell(g)
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
		c := mg.m.Cell(p)
		if terrain(c) == FoliageCell {
			return p
		}
	}
}

func (mg *MapGen) OutsideCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("OutsideCell")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		mons := g.MonsterAt(p)
		if mons.Exists() {
			continue
		}
		c := mg.m.Cell(p)
		if !mg.room[p] && (terrain(c) == FoliageCell || terrain(c) == GroundCell) {
			return p
		}
	}
}

func (mg *MapGen) InsideCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > 1500 {
			panic("InsideCell")
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
		c := mg.m.Cell(p)
		if mg.room[p] && (terrain(c) == FoliageCell || terrain(c) == GroundCell) {
			return p
		}
	}
}

func (mg *MapGen) GenItem(g *Game) {
	plan := g.GenPlan[g.Depth]
	if plan != GenAmulet && plan != GenCloak {
		return
	}
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("GenItem")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	g.Map.SetCell(p, ItemCell)
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

func (mg *MapGen) GenBarrierStone(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("GenBarrierStone")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
	}
	g.Map.SetCell(p, StoneCell)
	g.Objects.Stones[p] = SealStone
}

func (mg *MapGen) GenMagara(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > maxIterations {
			panic("GenMagara")
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceItem)
	}
	g.Map.SetCell(p, MagaraCell)
	mag := g.RandomMagara()
	g.Objects.Magaras[p] = mag
	g.GeneratedMagaras = append(g.GeneratedMagaras, mag.Kind)
}

func (mg *MapGen) GenStairs(g *Game, st stair) {
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
	g.Map.SetCell(p, StairCell)
	g.Objects.Stairs[p] = st
}

func (mg *MapGen) GenFakeStairs(g *Game) {
	if !g.Params.FakeStair[g.Depth] {
		return
	}
	var ri, pj int
	best := 0
loop:
	for i, r := range mg.rooms {
		for _, pl := range r.places {
			if terrain(mg.m.Cell(pl.p)) == StairCell {
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
	g.Map.SetCell(p, FakeStairCell)
	g.Objects.FakeStairs[p] = true
}

func (mg *MapGen) GenBarrel(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 500 {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceSpecialStatic)
	}
	g.Map.SetCell(p, BarrelCell)
	g.Objects.Barrels[p] = true
}

func (mg *MapGen) GenTable(g *Game) {
	p := invalidPos
	count := 0
	for p == invalidPos {
		count++
		if count > 500 {
			return
		}
		p = mg.rooms[RandInt(len(mg.rooms))].RandomPlaces(PlaceSpecialOrStatic)
	}
	g.Map.SetCell(p, TableCell)
}

func (mg *MapGen) GenTree(g *Game) {
	p := mg.OutsideCavernMiddleCell(g)
	if p != invalidPos {
		g.Map.SetCell(p, TreeCell)
	}
}

func (mg *MapGen) CaveGroundCell(g *Game) gruid.Point {
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("CaveGroundCell")
		}
		x := mg.rand.IntN(MapWidth)
		y := mg.rand.IntN(MapHeight)
		p := gruid.Point{x, y}
		c := mg.m.Cell(p)
		if (terrain(c) == GroundCell || terrain(c) == CavernCell || terrain(c) == QueenRockCell) && !mg.room[p] {
			return p
		}
	}
}

func (mg *MapGen) RandomInStone(g *Game) stone {
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

func (mg *MapGen) RandomOutStone(g *Game) stone {
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

func (mg *MapGen) GenStones(g *Game) {
	// Magical Stones
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
					p = mg.CaveGroundCell(g)
					break
				}
				p = mg.rooms[RandInt(len(mg.rooms))].RandomPlace(PlaceStatic)
			}
			st = mg.RandomInStone(g)
		} else {
			p = mg.CaveGroundCell(g)
			st = mg.RandomOutStone(g)
		}
		g.Objects.Stones[p] = st
		g.Map.SetCell(p, StoneCell)
	}
}

func (mg *MapGen) GenCellularAutomataCaveMap() {
	mg.RunCellularAutomataCave()
	mg.Foliage(false)
}

func (dg *MapGen) RunCellularAutomataCave() bool {
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

func (mg *MapGen) GenLake(c Cell) {
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
			return valid(q) && terrain(mg.m.Cell(q)) == WallCell && !mg.room[q] && distance(p, q) < 10+mg.rand.IntN(10)
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
		d.SetCell(p, c)
	}
}

func (mg *MapGen) GenQueenRock() {
	cavern := []gruid.Point{}
	for i := range DungeonNCells {
		p := idxtopos(i)
		c := mg.m.Cell(p)
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
			return valid(q) && terrain(mg.m.Cell(q)) == CavernCell && distance(q, p) < 15+mg.rand.IntN(5)
		}
		cp := newPather(passable)
		for _, q := range mg.PR.CCMap(cp, p) {
			mg.m.SetCell(q, QueenRockCell)
		}
	}

}

func (dg *MapGen) Foliage(less bool) {
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

func (dg *MapGen) GenCaveMap(size int) {
	mg := rl.MapGen{
		Rand: dg.rand,
		Grid: dg.m.Terrain,
	}
	wlk := walker{rand: dg.rand}
	mg.RandomWalkCave(wlk, rl.Cell(GroundCell), float64(size)/float64(DungeonNCells), 8)
	dg.Foliage(false)
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
			panic("DigBlock")
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

func (mg *MapGen) GenTreeCaveMap() {
	d := mg.m
	center := gruid.Point{40, 10}
	d.SetCell(center, GroundCell)
	d.SetCell(center.Add(gruid.Point{1, 0}), GroundCell)
	d.SetCell(center.Add(gruid.Point{1, -1}), GroundCell)
	d.SetCell(center.Add(gruid.Point{0, 1}), GroundCell)
	d.SetCell(center.Add(gruid.Point{1, 1}), GroundCell)
	d.SetCell(center.Add(gruid.Point{0, -1}), GroundCell)
	d.SetCell(center.Add(gruid.Point{-1, -1}), GroundCell)
	d.SetCell(center.Add(gruid.Point{-1, 0}), GroundCell)
	d.SetCell(center.Add(gruid.Point{-1, 1}), GroundCell)
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
			if terrain(d.Cell(p)) != GroundCell {
				d.SetCell(p, GroundCell)
				cells++
			}
		}
	}
	mg.Foliage(true)
}

// monster generation

func (g *Game) GenBand(band monsterBand) []monsterKind {
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

func (mg *MapGen) BandInfoGuard(g *Game, band monsterBand, pl placeKind) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
loop:
	for p == invalidPos {
		count++
		if count > maxIterations {
			p = mg.InsideCell(g)
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

func (mg *MapGen) BandInfoGuardSpecial(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for _, r := range mg.rooms {
		count++
		if count > 1 {
			log.Print("unavailable special guard position")
			p = mg.InsideCell(g)
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

func (mg *MapGen) BandInfoPatrol(g *Game, band monsterBand, pl placeKind) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for p == invalidPos || g.MonsterAt(p).Exists() {
		count++
		if count > 4000 {
			p = mg.InsideCell(g)
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
			target = mg.InsideCell(g)
			break
		}
		target = mg.rooms[RandInt(len(mg.rooms)-1)].RandomPlace(pl)
	}
	bandinfo.Path = append(bandinfo.Path, p)
	bandinfo.Path = append(bandinfo.Path, target)
	bandinfo.Beh = BehPatrol
	return bandinfo
}

func (mg *MapGen) BandInfoPatrolSpecial(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	p := invalidPos
	count := 0
	for _, r := range mg.rooms {
		count++
		if count > 1 {
			log.Print("unavailable special patrol position")
			p = mg.InsideCell(g)
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
			p = mg.InsideCell(g)
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

func (mg *MapGen) BandInfoOutsideGround(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.OutsideGroundCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) BandInfoSatowalga(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.SatowalgaCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) BandInfoOutside(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.OutsideCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (mg *MapGen) BandInfoOutsideExplore(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.OutsideCell(g))
	for range 4 {
		for range 100 {
			p := mg.OutsideCell(g)
			if mg.PR.CCMapAt(p) == mg.PR.CCMapAt(bandinfo.Path[0]) {
				bandinfo.Path = append(bandinfo.Path, p)
				break
			}
		}
	}
	bandinfo.Beh = BehExplore
	return bandinfo
}

func (mg *MapGen) BandInfoOutsideExploreButterfly(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.OutsideCell(g))
	for range 9 {
		for range 100 {
			p := mg.OutsideCell(g)
			if mg.PR.CCMapAt(p) == mg.PR.CCMapAt(bandinfo.Path[0]) {
				bandinfo.Path = append(bandinfo.Path, p)
				break
			}
		}
	}
	bandinfo.Beh = BehExplore
	return bandinfo
}

func (mg *MapGen) BandInfoFoliage(g *Game, band monsterBand) bandInfo {
	bandinfo := bandInfo{Kind: monsterBand(band)}
	bandinfo.Path = append(bandinfo.Path, mg.FoliageCell(g))
	bandinfo.Beh = BehWander
	return bandinfo
}

func (g *Game) FreeCellForMonster() gruid.Point {
	d := g.Map
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("FreeCellForMonster")
		}
		x := RandInt(MapWidth)
		y := RandInt(MapHeight)
		p := gruid.Point{x, y}
		c := d.Cell(p)
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

func (g *Game) FreeCellForBandMonster(p gruid.Point) gruid.Point {
	count := 0
	for {
		count++
		if count > maxIterations {
			return g.FreeCellForMonster()
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
		if mons.Exists() || !g.Map.Cell(p).IsPassable() {
			continue
		}
		return p
	}
}

func (mg *MapGen) PutMonsterBand(g *Game, band monsterBand) bool {
	monsters := g.GenBand(band)
	if monsters == nil {
		return false
	}
	awake := mg.rand.IntN(5) > 0
	var bdinf bandInfo
	switch band {
	case LoneYack, LoneWorm, PairYack:
		bdinf = mg.BandInfoFoliage(g, band)
	case LoneDog, LoneHarpy:
		bdinf = mg.BandInfoOutsideGround(g, band)
	case LoneBlinkingFrog, LoneExplosiveNadre, PairExplosiveNadre:
		bdinf = mg.BandInfoOutside(g, band)
	case LoneMirrorSpecter, LoneWingedMilfid, LoneVampire, PairWingedMilfid, LoneEarthDragon, LoneHazeCat, LoneSpider:
		bdinf = mg.BandInfoOutsideExplore(g, band)
	case LoneButterfly:
		bdinf = mg.BandInfoOutsideExploreButterfly(g, band)
	case LoneTreeMushroom, LoneAcidMound:
		bdinf = mg.BandInfoOutside(g, band)
	case LoneHighGuard:
		bdinf = mg.BandInfoGuard(g, band, PlacePatrol)
	case LoneSatowalgaPlant:
		bdinf = mg.BandInfoSatowalga(g, band)
	case SpecialLoneVampire, SpecialLoneNixe, SpecialLoneMilfid, SpecialLoneOricCelmist, SpecialLoneHarmonicCelmist, SpecialLoneHighGuard,
		SpecialLoneHarpy, SpecialLoneTreeMushroom, SpecialLoneMirrorSpecter, SpecialLoneHazeCat, SpecialLoneSpider, SpecialLoneAcidMound, SpecialLoneDog, SpecialLoneExplosiveNadre, SpecialLoneYack, SpecialLoneBlinkingFrog:
		if mg.rand.IntN(5) > 0 {
			bdinf = mg.BandInfoPatrolSpecial(g, band)
		} else {
			bdinf = mg.BandInfoGuardSpecial(g, band)
		}
		if !awake && mg.rand.IntN(2) > 0 {
			awake = true
		}
	case UniqueCrazyImp:
		bdinf = mg.BandInfoOutside(g, band)
		bdinf.Beh = BehCrazyImp
	default:
		bdinf = mg.BandInfoPatrol(g, band, PlacePatrol)
	}
	g.Bands = append(g.Bands, bdinf)
	var p gruid.Point
	if len(bdinf.Path) == 0 {
		// should not happen now
		p = g.FreeCellForMonster()
	} else {
		p = bdinf.Path[0]
		if g.MonsterAt(p).Exists() {
			log.Printf("already monster at %v mons %v no place for %v", p, g.MonsterAt(p).Kind, monsters)
			p = g.FreeCellForMonster()
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
			p = g.FreeCellForBandMonster(p)
		}
	}
	return true
}

func (mg *MapGen) PutRandomBand(g *Game, bands []monsterBand) bool {
	return mg.PutMonsterBand(g, bands[RandInt(len(bands))])
}

func (mg *MapGen) PutRandomBandN(g *Game, bands []monsterBand, n int) {
	for range n {
		mg.PutMonsterBand(g, bands[RandInt(len(bands))])
	}
}

func (mg *MapGen) GenMonsters(g *Game) {
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
			mg.PutRandomBandN(g, []monsterBand{SpecialLoneVampire}, 2)
		case roomNixes:
			mg.PutRandomBandN(g, []monsterBand{SpecialLoneNixe}, 2)
		case roomFrogs:
			mg.PutRandomBandN(g, []monsterBand{SpecialLoneBlinkingFrog}, 2)
		case roomMilfids:
			switch mg.rand.IntN(6) {
			case 0:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneYack}, 2)
			case 1:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneDog}, 2)
			default:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneMilfid}, 2)
			}
		case roomCelmists:
			switch mg.rand.IntN(3) {
			case 0:
				bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
				mg.PutRandomBandN(g, bandOricCelmists, 2)
			case 1:
				bandHarmonicCelmists := []monsterBand{SpecialLoneHarmonicCelmist}
				mg.PutRandomBandN(g, bandHarmonicCelmists, 2)
			case 2:
				bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
				bandHarmonicCelmists := []monsterBand{SpecialLoneHarmonicCelmist}
				mg.PutRandomBandN(g, bandHarmonicCelmists, 1)
				mg.PutRandomBandN(g, bandOricCelmists, 1)
			}
		case roomHarpies:
			if mg.rand.IntN(3) > 0 {
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneHarpy}, 2)
			} else {
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneSpider}, 2)
			}
		case roomTreeMushrooms:
			if mg.rand.IntN(3) > 0 {
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneTreeMushroom}, 2)
			} else {
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneHazeCat}, 2)
			}
		case roomMirrorSpecters:
			switch mg.rand.IntN(6) {
			case 0:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneAcidMound}, 2)
			case 1:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneExplosiveNadre}, 2)
			default:
				mg.PutRandomBandN(g, []monsterBand{SpecialLoneMirrorSpecter}, 2)
			}
		case roomShaedra:
			if mg.rand.IntN(3) > 0 {
				mg.PutRandomBand(g, []monsterBand{SpecialLoneHighGuard})
			} else {
				mg.PutRandomBand(g, []monsterBand{SpecialLoneOricCelmist})
			}
		case roomArtifact:
			switch mg.rand.IntN(3) {
			case 0:
				mg.PutRandomBand(g, []monsterBand{SpecialLoneHarmonicCelmist})
			case 1:
				mg.PutRandomBand(g, []monsterBand{SpecialLoneOricCelmist})
			default:
				mg.PutRandomBand(g, []monsterBand{SpecialLoneHighGuard})
			}
		default:
			// XXX not used now
			bandOricCelmists := []monsterBand{SpecialLoneOricCelmist}
			mg.PutRandomBandN(g, bandOricCelmists, 2)
		}
	}
	if g.Depth == g.Params.CrazyImp {
		mg.PutRandomBand(g, []monsterBand{UniqueCrazyImp})
	}
	mg.PutRandomBandN(g, bandsButterfly, 2)
	if mg.layout == RandomSmallWalkCaveUrbanised {
		mg.PutRandomBandN(g, bandsGuard, 1+(g.Depth+1)/4)
	}
	switch g.Depth {
	case 1:
		// 8-9
		if mg.rand.IntN(2) == 0 {
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandsGuard, 5)
			} else {
				mg.PutRandomBandN(g, bandsGuard, 4)
				mg.PutRandomBandN(g, bandsBipeds, 1)
			}
			mg.PutRandomBandN(g, bandsAnimals, 3)
		} else {
			mg.PutRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(5) > 0 {
				mg.PutRandomBandN(g, bandsAnimals, 5)
			} else {
				mg.PutRandomBandN(g, bandsAnimals, 3)
				mg.PutRandomBandN(g, bandsRare, 1)
			}
		}
	case 2:
		// 10-11
		mg.PutRandomBandN(g, bandsGuard, 3)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 7
			mg.PutRandomBandN(g, bandsBipeds, 1)
			mg.PutRandomBandN(g, bandsAnimals, 4)
			mg.PutRandomBandN(g, bandsPlants, 2)
		case 2, 3:
			// 8
			mg.PutRandomBandN(g, bandsAnimals, 3)
			mg.PutRandomBandN(g, bandsRare, 1)
			mg.PutRandomBandN(g, bandsButterfly, 2)
			mg.PutRandomBandN(g, bandsPlants, 2)
		case 4:
			// 8
			mg.PutRandomBandN(g, bandsPlants, 3)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandFrog, 5)
			} else {
				mg.PutRandomBandN(g, bandYack, 5)
			}
		}
	case 3:
		// 11-12
		mg.PutRandomBandN(g, bandsHighGuard, 2)
		mg.PutRandomBandN(g, bandsGuard, 4)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 5
			if mg.rand.IntN(3) == 0 {
				mg.PutRandomBandN(g, bandDog, 3)
			} else {
				mg.PutRandomBandN(g, bandsAnimals, 3)
			}
			mg.PutRandomBandN(g, bandsPlants, 2)
		case 2, 3:
			// 5
			mg.PutRandomBandN(g, bandsAnimals, 2)
			mg.PutRandomBandN(g, bandsPlants, 1)
			mg.PutRandomBandN(g, bandsBipeds, 2)
		case 4:
			// 6
			mg.PutRandomBandN(g, bandsPlants, 1)
			mg.PutRandomBandN(g, bandNadre, 5)
		}
	case 4:
		// 12-13
		mg.PutRandomBandN(g, bandsHighGuard, 2)
		switch mg.rand.IntN(5) {
		case 0, 1:
			// 10
			mg.PutRandomBandN(g, bandsGuard, 4)
			mg.PutRandomBandN(g, bandsRare, 2)
			mg.PutRandomBandN(g, bandGuardPair, 1)
			mg.PutRandomBandN(g, bandsBipeds, 1)
			mg.PutRandomBandN(g, bandsPlants, 1)
		case 2, 3:
			// 11
			mg.PutRandomBandN(g, bandsGuard, 7)
			mg.PutRandomBandN(g, bandGuardPair, 1)
			mg.PutRandomBandN(g, bandsAnimals, 1)
			mg.PutRandomBandN(g, bandsPlants, 1)
		case 4:
			mg.PutRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandOricCelmist, 4)
			} else {
				mg.PutRandomBandN(g, bandHarmonicCelmist, 4)
			}
			mg.PutRandomBandN(g, bandsPlants, 1)
		}
	case 5:
		// 13-14
		mg.PutRandomBandN(g, bandsHighGuard, 2)
		if mg.rand.IntN(2) == 0 {
			// 11
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandsGuard, 2)
				mg.PutRandomBandN(g, bandGuardPair, 1)
			} else {
				mg.PutRandomBandN(g, bandsGuard, 4)
			}
			mg.PutRandomBandN(g, bandsAnimals, 1)
			mg.PutRandomBandN(g, bandsRare, 2)
			mg.PutRandomBandN(g, bandsBipeds, 3)
			mg.PutRandomBandN(g, bandsPlants, 1)
		} else {
			// 12
			mg.PutRandomBandN(g, bandsGuard, 2)
			mg.PutRandomBandN(g, bandsAnimals, 3)
			mg.PutRandomBandN(g, bandsBipeds, 2)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandOricCelmistPair, 1)
			} else {
				mg.PutRandomBandN(g, bandHarmonicCelmistPair, 1)
			}
			mg.PutRandomBandN(g, bandsRare, 1)
			mg.PutRandomBandN(g, bandsPlants, 2)
		}
	case 6:
		// 15-17
		mg.PutRandomBandN(g, bandsHighGuard, 1)
		if mg.rand.IntN(2) == 0 {
			// 14
			mg.PutRandomBandN(g, bandsGuard, 3)
			mg.PutRandomBandN(g, bandsAnimals, 2)
			mg.PutRandomBandN(g, bandsRare, 3)
			mg.PutRandomBandN(g, bandsBipeds, 1)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandYackPair, 1)
			} else {
				mg.PutRandomBandN(g, bandWingedMilfidPair, 1)
			}
			mg.PutRandomBandN(g, bandsPlants, 3)
		} else {
			// 16
			mg.PutRandomBandN(g, bandsGuard, 2)
			if mg.rand.IntN(2) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, bandYack, 8)
				} else {
					mg.PutRandomBandN(g, bandFrog, 8)
				}
			} else {
				mg.PutRandomBandN(g, bandsRare, 2)
				if mg.rand.IntN(3) == 0 {
					mg.PutRandomBandN(g, bandsAnimals, 4)
					mg.PutRandomBandN(g, []monsterBand{PairWorm}, 1)
				} else {
					mg.PutRandomBandN(g, bandsAnimals, 6)
				}
			}
			mg.PutRandomBandN(g, bandsButterfly, 1)
			mg.PutRandomBandN(g, bandsPlants, 5)
		}
	case 7:
		// 19
		mg.PutRandomBandN(g, bandsHighGuard, 1)
		if mg.rand.IntN(2) == 0 {
			// 18
			mg.PutRandomBandN(g, bandsGuard, 4)
			if mg.rand.IntN(3) == 0 {
				mg.PutRandomBandN(g, bandDog, 4)
				mg.PutRandomBandN(g, bandsAnimals, 2)
			} else {
				mg.PutRandomBandN(g, bandsAnimals, 6)
			}
			mg.PutRandomBandN(g, bandsButterfly, 1)
			mg.PutRandomBandN(g, bandsRare, 2)
			mg.PutRandomBandN(g, bandsBipeds, 3)
			mg.PutRandomBandN(g, bandsPlants, 2)
		} else {
			// 18
			mg.PutRandomBandN(g, bandsGuard, 1)
			mg.PutRandomBandN(g, bandsRare, 4)
			mg.PutRandomBandN(g, bandsButterfly, 1)
			if mg.rand.IntN(3) == 0 {
				mg.PutRandomBandN(g, bandNadre, 7)
			} else {
				mg.PutRandomBandN(g, bandsAnimals, 5)
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, []monsterBand{PairFrog}, 1)
				} else {
					mg.PutRandomBandN(g, []monsterBand{PairDog}, 1)
				}
			}
			mg.PutRandomBandN(g, bandsPlants, 5)
		}
	case 8:
		// 18-19
		mg.PutRandomBandN(g, bandsHighGuard, 4)
		if mg.rand.IntN(2) == 0 {
			// 14
			mg.PutRandomBandN(g, bandsGuard, 5)
			mg.PutRandomBandN(g, bandsRare, 1)
			if mg.rand.IntN(3) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, bandOricCelmist, 6)
				} else {
					mg.PutRandomBandN(g, bandMadNixe, 6)
				}
				mg.PutRandomBandN(g, bandsBipeds, 2)
			} else {
				mg.PutRandomBandN(g, bandsBipeds, 8)
			}
		} else {
			// 15
			mg.PutRandomBandN(g, bandsGuard, 5)
			mg.PutRandomBandN(g, bandsAnimals, 2)
			mg.PutRandomBandN(g, bandsRare, 1)
			mg.PutRandomBandN(g, bandHarmonicCelmistPair, 1)
			mg.PutRandomBandN(g, bandsBipeds, 4)
			mg.PutRandomBandN(g, bandsPlants, 1)
		}
	case 9:
		// 20-24
		mg.PutRandomBandN(g, bandsHighGuard, 2)
		if mg.rand.IntN(2) == 0 {
			// 18
			mg.PutRandomBandN(g, bandsGuard, 3)
			if mg.rand.IntN(2) == 0 {
				switch mg.rand.IntN(4) {
				case 0:
					mg.PutRandomBandN(g, bandTreeMushroom, 4)
					mg.PutRandomBandN(g, []monsterBand{PairTreeMushroom}, 1)
				case 1:
					mg.PutRandomBandN(g, bandDragon, 6)
					mg.PutRandomBandN(g, bandsAnimals, 2)
				case 2:
					mg.PutRandomBandN(g, bandHazeCat, 4)
					mg.PutRandomBandN(g, []monsterBand{PairHazeCat}, 1)
				case 3:
					mg.PutRandomBandN(g, bandSpider, 4)
					mg.PutRandomBandN(g, []monsterBand{PairSpider}, 1)
				}
			} else {
				mg.PutRandomBandN(g, bandsRare, 4)
				mg.PutRandomBandN(g, []monsterBand{PairTreeMushroom}, 1)
			}
			mg.PutRandomBandN(g, bandsRare, 4)
			mg.PutRandomBandN(g, bandsAnimals, 1)
			mg.PutRandomBandN(g, bandsBipeds, 2)
			mg.PutRandomBandN(g, bandsPlants, 2)
		} else {
			// 22+2
			mg.PutRandomBandN(g, bandsButterfly, 2)
			mg.PutRandomBandN(g, bandsGuard, 2)
			mg.PutRandomBandN(g, bandsAnimals, 8)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandExplosiveNadrePair, 2)
			} else {
				mg.PutRandomBandN(g, bandYackPair, 2)
			}
			mg.PutRandomBandN(g, bandsRare, 3)
			mg.PutRandomBandN(g, bandsPlants, 3)
			mg.PutRandomBandN(g, []monsterBand{PairSatowalga}, 1)
		}
	case 10:
		// 22
		mg.PutRandomBandN(g, bandsHighGuard, 3)
		if mg.rand.IntN(2) == 0 {
			// 19
			mg.PutRandomBandN(g, bandsGuard, 7)
			mg.PutRandomBandN(g, bandGuardPair, 1)
			mg.PutRandomBandN(g, bandsRare, 2)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandsBipeds, 8)
			} else {
				mg.PutRandomBandN(g, bandsBipeds, 4)
				mg.PutRandomBandN(g, bandMirrorSpecter, 4)
			}
		} else {
			// 19
			mg.PutRandomBandN(g, bandGuardPair, 1)
			if mg.rand.IntN(3) == 0 {
				mg.PutRandomBandN(g, bandsGuard, 4)
				mg.PutRandomBandN(g, bandVampire, 4)
				mg.PutRandomBandN(g, []monsterBand{PairVampire}, 1)
				mg.PutRandomBandN(g, bandsRare, 2)
			} else {
				mg.PutRandomBandN(g, bandsGuard, 6)
				mg.PutRandomBandN(g, bandsBipeds, 3)
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, []monsterBand{PairNixe}, 1)
				} else {
					mg.PutRandomBandN(g, []monsterBand{PairOricCelmist}, 1)
				}
				mg.PutRandomBandN(g, bandsAnimals, 2)
			}
			mg.PutRandomBandN(g, bandsAnimals, 2)
			mg.PutRandomBandN(g, bandsPlants, 1)
			mg.PutRandomBandN(g, bandsRare, 1)
		}
	case 11:
		// 26
		mg.PutRandomBandN(g, bandsHighGuard, 5)
		if mg.rand.IntN(2) == 0 {
			// 21
			mg.PutRandomBandN(g, bandsGuard, 5)
			mg.PutRandomBandN(g, bandsRare, 2)
			if mg.rand.IntN(3) == 0 {
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, bandOricCelmist, 5)
				} else {
					mg.PutRandomBandN(g, bandHarmonicCelmist, 5)
				}
				mg.PutRandomBandN(g, bandsBipeds, 5)
			} else {
				mg.PutRandomBandN(g, bandsBipeds, 10)
			}
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandVampirePair, 1)
			} else {
				if mg.rand.IntN(2) == 0 {
					mg.PutRandomBandN(g, bandOricCelmistPair, 1)
				} else {
					mg.PutRandomBandN(g, bandHarmonicCelmistPair, 1)
				}
			}
			mg.PutRandomBandN(g, bandsAnimals, 2)
		} else {
			// 21
			mg.PutRandomBandN(g, bandsGuard, 5)
			mg.PutRandomBandN(g, bandOricCelmistPair, 1)
			mg.PutRandomBandN(g, []monsterBand{PairGuard}, 1)
			mg.PutRandomBandN(g, bandsRare, 1)
			mg.PutRandomBandN(g, bandsBipeds, 7)
			if mg.rand.IntN(2) == 0 {
				mg.PutRandomBandN(g, bandHarmonicCelmistPair, 1)
			} else {
				mg.PutRandomBandN(g, bandNixePair, 1)
			}
			mg.PutRandomBandN(g, bandsAnimals, 1)
			mg.PutRandomBandN(g, bandsPlants, 1)
		}
	}
}
