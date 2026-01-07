package main

import "codeberg.org/anaseto/gruid"

func (g *Game) cardinalNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, valid)
}

func (g *Game) playerPassableNeighbors(p gruid.Point) []gruid.Point {
	d := g.Map
	return g.nbs.All(p, func(q gruid.Point) bool {
		return valid(q) && IsPlayerPassable(d.At(q))
	})
}

func (g *Game) nonWallNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, func(q gruid.Point) bool {
		return valid(q) && g.Map.At(q) != WallCell
	})
}

func (g *Game) flammableNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, func(q gruid.Point) bool {
		return valid(q) && Flammable(g.Map.At(q))
	})
}

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
