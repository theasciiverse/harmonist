package main

import (
	"math/rand/v2"
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
