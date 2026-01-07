package main

//import "log"
import "testing"
import "codeberg.org/anaseto/gruid"

func init() {
	Testing = true
	DisableAnimations = true
}

func TestInitLevel(t *testing.T) {
	for i := 0; i < 50; i++ {
		md := &model{}
		g := &Game{md: md}
		md.g = g
		for depth := 0; depth < MaxDepth; depth++ {
			g.InitLevel()
			if g.Player.P == invalidPos {
				t.Errorf("Player starting cell is not valid")
			}
			if !IsPlayerPassable(g.Map.At(g.Player.P)) {
				t.Errorf("Player starting cell is not passable: %+v", g.TerrainShortDesc(g.Map.At(g.Player.P), g.Player.P))
			}
			if g.Map.At(g.Player.P) != GroundCell {
				t.Errorf("Player starting cell is not ground: %+v", g.TerrainShortDesc(g.Map.At(g.Player.P), g.Player.P))
			}
			if g.Map.Depth == WinDepth {
				if g.Map.At(g.Places.Shaedra) != StoryCell {
					t.Errorf("Shaedra not there: %+v", g.Places.Shaedra)
				}
				if g.Objects.Story[g.Places.Shaedra] != StoryShaedra {
					t.Errorf("bad Shaedra place: %+v", g.Places.Shaedra)
				}
			}
			if g.Map.Depth == MaxDepth {
				if g.Map.At(g.Places.Artifact) != StoryCell {
					t.Errorf("Artifact not there: %+v", g.Places.Artifact)
				}
				if g.Objects.Story[g.Places.Artifact] != StoryArtifactSealed {
					t.Errorf("bad Artifact place: %+v", g.Places.Shaedra)
				}
			}
			if g.Map.Depth == MaxDepth || g.Map.Depth == WinDepth {
				if g.Map.At(g.Places.Marevor) != StoryCell {
					t.Errorf("Marevor not there: %+v", g.Places.Artifact)
				}
				if g.Objects.Story[g.Places.Marevor] != NoStory {
					t.Errorf("bad Marevor place: %+v", g.Places.Shaedra)
				}
			}
			if g.Map.Depth == MaxDepth || g.Map.Depth == WinDepth {
				if g.Map.At(g.Places.Monolith) != StoryCell {
					t.Errorf("Monolith not there: %+v", g.Places.Artifact)
				}
				if g.Objects.Story[g.Places.Monolith] != NoStory {
					t.Errorf("bad Monolith place: %+v", g.Places.Shaedra)
				}
			}
			if g.Map.Depth != WinDepth {
				if len(g.Objects.Magaras) != 1 {
					t.Errorf("bad number of magaras: %+v", g.Objects.Magaras)
				}
			}
			for _, m := range g.Monsters {
				if !Passable(g.Map.At(m.P)) {
					t.Errorf("Not free: %+v", m.P)
				}
			}
			for j := 0; j < 20; j++ {
				g.EndTurn()
				g.PlayerBump(randomPlayerNeighbor(g.Player.P)) // wait if impossible
				if g.Player.HP == 0 {
					g.Player.HP = 10
				}
			}
			g.Map.Depth++
		}
	}
}

func BenchmarkLOS(b *testing.B) {
	g := &Game{}
	g.InitLevel()
	for i := 0; i < b.N; i++ {
		g.ComputeLOS()
	}
}

func BenchmarkLights(b *testing.B) {
	g := &Game{}
	g.InitLevel()
	for i := 0; i < b.N; i++ {
		g.ComputeLights()
	}
}

func BenchmarkInitLevel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		g := &Game{}
		for depth := 0; depth < MaxDepth; depth++ {
			g.InitLevel()
			g.Map.Depth++
		}
	}
}

func BenchmarkEndTurn(b *testing.B) {
	md := &model{}
	g := &Game{md: md}
	md.g = g
	g.InitLevel()
	for i := 0; i < b.N; i++ {
		g.EndTurn()
		if g.Player.HP == 0 {
			g.Player.HP = 10
		}
	}
}

func randomPlayerNeighbor(p gruid.Point) gruid.Point {
	switch RandInt(4) {
	case 0:
		return p.Add(gruid.Point{1, 0})
	case 1:
		return p.Add(gruid.Point{-1, 0})
	case 2:
		return p.Add(gruid.Point{0, 1})
	default:
		return p.Add(gruid.Point{0, -1})
	}
}

func BenchmarkEndTurnPlayer(b *testing.B) {
	md := &model{}
	g := &Game{md: md}
	md.g = g
	g.InitLevel()
	for i := 0; i < b.N; i++ {
		g.EndTurn()
		g.PlayerBump(randomPlayerNeighbor(g.Player.P)) // wait if impossible
		if g.Player.HP == 0 {
			g.Player.HP = 10
		}
	}
}
