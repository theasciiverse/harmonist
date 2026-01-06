// many ideas here from articles found at http://www.roguebasin.com/

package main

import (
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
