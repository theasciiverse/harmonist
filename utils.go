package main

import (
	"iter"
	"math/rand/v2"

	"codeberg.org/anaseto/gruid"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func RandInt(n int) int {
	if n <= 0 {
		return 0
	}
	x := rand.IntN(n)
	return x
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

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
