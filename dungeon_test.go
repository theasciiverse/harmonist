package main

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/paths"
)

var Rounds = 40

func (m *Map) FreePassableCell() gruid.Point {
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("FreeCell")
		}
		x := RandInt(MapWidth)
		y := RandInt(MapHeight)
		p := gruid.Point{x, y}
		c := m.Cell(p)
		if c.IsPassable() {
			return p
		}
	}
}

func (m *Map) connex(pr *paths.PathRange) bool {
	pos := m.FreePassableCell()
	passable := func(p gruid.Point) bool {
		return terrain(m.Cell(p)) != WallCell
	}
	cp := newPather(passable)
	pr.CCMap(cp, pos)
	it := m.Terrain.Iterator()
	for it.Next() {
		if Cell(it.Cell()).IsPassable() && pr.CCMapAt(it.P()) == -1 {
			return false
		}
	}
	return true
}

func (m *Map) String() string {
	b := &bytes.Buffer{}
	it := m.Terrain.Iterator()
	for i := 0; it.Next(); i++ {
		c := Cell(it.Cell())
		if i > 0 && i%MapWidth == 0 {
			fmt.Fprint(b, "\n")
		}
		if terrain(c) == WallCell {
			fmt.Fprint(b, "#")
		} else {
			fmt.Fprint(b, ".")
		}
	}
	return b.String()
}

func TestAutomataCave(t *testing.T) {
	for i := 0; i < Rounds; i++ {
		g := &Game{}
		g.initrand()
		g.InitFirstLevel()
		g.InitLevelStructures()
		g.GenRoomTunnels(AutomataCave)
		if !g.Map.connex(g.PR) {
			t.Errorf("Not connex:\n%s\n", g.Map.String())
		}
	}
}

func TestRandomWalkCave(t *testing.T) {
	for i := 0; i < Rounds; i++ {
		g := &Game{}
		g.initrand()
		g.InitFirstLevel()
		g.InitLevelStructures()
		g.GenRoomTunnels(RandomWalkCave)
		if !g.Map.connex(g.PR) {
			t.Errorf("Not connex:\n%s\n", g.Map.String())
		}
	}
}

func TestRandomWalkTreeCave(t *testing.T) {
	for i := 0; i < Rounds; i++ {
		g := &Game{}
		g.initrand()
		g.InitFirstLevel()
		g.InitLevelStructures()
		g.GenRoomTunnels(RandomWalkTreeCave)
		if !g.Map.connex(g.PR) {
			t.Errorf("Not connex:\n%s\n", g.Map.String())
		}
	}
}

func TestRandomSmallWalkCaveUrbanised(t *testing.T) {
	for i := 0; i < Rounds; i++ {
		g := &Game{}
		g.initrand()
		g.InitFirstLevel()
		g.InitLevelStructures()
		g.GenRoomTunnels(RandomSmallWalkCaveUrbanised)
		if !g.Map.connex(g.PR) {
			t.Errorf("Not connex:\n%s\n", g.Map.String())
		}
	}
}

func (g *Game) initrand() {
	if g.rand == nil {
		g.rand = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
}

func TestNaturalCave(t *testing.T) {
	for i := 0; i < Rounds; i++ {
		g := &Game{}
		g.initrand()
		g.InitFirstLevel()
		g.InitLevelStructures()
		g.GenRoomTunnels(NaturalCave)
		if !g.Map.connex(g.PR) {
			t.Errorf("Not connex:\n%s\n", g.Map.String())
		}
	}
}
