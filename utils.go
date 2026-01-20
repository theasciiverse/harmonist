package main

import (
	"iter"
	"math/rand/v2"

	"codeberg.org/anaseto/gruid"
)

// Abs returns the absolute value.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// RandInt is like rand.IntN but returns zero for non-positive integers.
func RandInt(n int) int {
	if n <= 0 {
		return 0
	}
	x := rand.IntN(n)
	return x
}

// Indefinite returns the given text prefixed by an indefinite article
// (uppercased if desired).
func Indefinite(s string, upper bool) (text string) {
	if len(s) > 0 {
		switch s[0] {
		case 'a', 'e', 'i', 'o', 'u':
			if upper {
				text = "An " + s
			} else {
				text = "an " + s
			}
		default:
			if upper {
				text = "A " + s
			} else {
				text = "a " + s
			}
		}
	}
	return text
}

// Neighbors returns an iterator over cardinal neighbors of the given position.
func Neighbors(p gruid.Point) iter.Seq[gruid.Point] {
	return func(yield func(gruid.Point) bool) {
		for i := -1; i <= 1; i += 2 {
			q := p.Shift(i, 0)
			if InMap(q) && !yield(q) {
				return
			}
			q = p.Shift(0, i)
			if InMap(q) && !yield(q) {
				return
			}
		}
	}
}

// AllNeighbors returns an iterator over cardinal and diagonal neighbors of the
// given position.
func AllNeighbors(p gruid.Point) iter.Seq[gruid.Point] {
	return func(yield func(gruid.Point) bool) {
		for y := -1; y <= 1; y++ {
			for x := -1; x <= 1; x++ {
				if x == 0 && y == 0 {
					continue
				}
				q := p.Shift(x, y)
				if InMap(q) && !yield(q) {
					return
				}
			}
		}
	}
}

// NeighborsFunc returns an iterator over cardinal neighbors of the given
// position that statisfy the given predicate.
func NeighborsFunc(p gruid.Point, f func(gruid.Point) bool) iter.Seq[gruid.Point] {
	return func(yield func(gruid.Point) bool) {
		for i := -1; i <= 1; i += 2 {
			q := p.Shift(i, 0)
			if f(q) && !yield(q) {
				return
			}
			q = p.Shift(0, i)
			if f(q) && !yield(q) {
				return
			}
		}
	}
}
