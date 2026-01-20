package main

import (
	"slices"

	"codeberg.org/anaseto/gruid"
)

// cardinalNeighbors returns cardinal neighbors as a slice.
func (g *Game) cardinalNeighbors(p gruid.Point) []gruid.Point {
	return slices.Collect(Neighbors(p))
}

// playerPassableNeighbors returns all neighbors (including diagonal ones) that
// are passable for the player.
func (g *Game) playerPassableNeighbors(p gruid.Point) []gruid.Point {
	var ps []gruid.Point
	for q := range AllNeighbors(p) {
		if InMap(q) && IsPlayerPassable(g.Map.At(q)) {
			ps = append(ps, q)
		}
	}
	return ps
}

// nonWallNeighbors returns all non-wall cardinal neighbors as a slice.
func (g *Game) nonWallNeighbors(p gruid.Point) []gruid.Point {
	return slices.Collect(NeighborsFunc(p, func(q gruid.Point) bool {
		return InMap(q) && g.Map.At(q) != WallCell
	}))
}

// flammableNeighbors returns all flammable neighbors.
func (g *Game) flammableNeighbors(p gruid.Point) []gruid.Point {
	return slices.Collect(NeighborsFunc(p, func(q gruid.Point) bool {
		return InMap(q) && Flammable(g.Map.At(q))
	}))
}

// randomNeighbor returns a random neighbor with some bias toward horizontal
// directions.
func randomNeighbor(p gruid.Point) gruid.Point {
	switch RandInt(6) {
	case 0, 1:
		return p.Add(gruid.Point{1, 0})
	case 2, 3:
		return p.Add(gruid.Point{-1, 0})
	case 4:
		return p.Add(gruid.Point{0, 1})
	default:
		return p.Add(gruid.Point{0, -1})
	}
}
