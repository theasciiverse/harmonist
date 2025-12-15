package main

import "codeberg.org/anaseto/gruid"

func (g *Game) cardinalNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, valid)
}

func (g *Game) playerPassableNeighbors(p gruid.Point) []gruid.Point {
	d := g.Dungeon
	return g.nbs.All(p, func(q gruid.Point) bool {
		return valid(q) && d.Cell(q).IsPlayerPassable()
	})
}

func (g *Game) nonWallNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, func(q gruid.Point) bool {
		return valid(q) && terrain(g.Dungeon.Cell(q)) != WallCell
	})
}

func (g *Game) flammableNeighbors(p gruid.Point) []gruid.Point {
	return g.nbs.Cardinal(p, func(q gruid.Point) bool {
		return valid(q) && g.Dungeon.Cell(q).Flammable()
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
