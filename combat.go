// combat utility functions

package main

import (
	"errors"
	"fmt"

	"codeberg.org/anaseto/gruid"
)

// InflictDamage makes the monster inflict 1 damage on the player.
func (m *Monster) InflictDamage(g *Game) {
	g.Stats.ReceivedHits++
	oldHP := g.Player.HP
	g.damagePlayer()
	g.md.WoundedAnimation()
	if oldHP > 1 && g.Player.HP <= 1 {
		g.StoryPrintf("Critical hit by %s (HP: %d)", m.Kind, g.Player.HP)
		g.md.WoundedAnimation() // twice
		g.md.criticalHPWarning()
	} else if g.Player.HP > 0 {
		g.StoryPrintf("Hit by %s (HP: %d)", m.Kind, g.Player.HP)
	} else {
		g.StoryPrintf("Killed by %s", m.Kind)
	}
	if g.Player.HP > 0 && g.Player.Inventory.Body == CloakConversion && g.Player.MP < g.Player.MPMax() {
		g.Player.MP++
	}
}

// damagePlayer handles 1 damage for the player.
func (g *Game) damagePlayer() {
	g.Stats.Damage += 1
	g.Stats.DDamage[g.Map.Depth] += 1
	g.Player.HPbonus -= 1
	if g.Player.HPbonus < 0 {
		g.Player.HP += g.Player.HPbonus
		g.Player.HPbonus = 0
	}
}

func (md *model) criticalHPWarning() {
	if md.mode == modeNormal {
		md.mode = modeHPCritical
	}
	md.critical = true
}

// MakeMonstersAware handles monster awareness of the player.
func (g *Game) MakeMonstersAware() {
	for _, m := range g.Monsters {
		if m.Dead {
			continue
		}
		if g.Player.Sees(m.P) {
			m.MakeAware(g)
			if m.State != Resting {
				m.GatherBand(g)
			}
		}
	}
}

// MakeNoise makes a given amount of noise at the given position.
func (g *Game) MakeNoise(noise int, at gruid.Point) {
	mp := &NoisePath{g: g}
	g.PRnoise.BreadthFirstMap(mp, []gruid.Point{at}, noise)
	for _, m := range g.Monsters {
		if !m.Exists() {
			continue
		}
		if m.State == Hunting {
			continue
		}
		d := g.PRnoise.BreadthFirstMapAt(m.P)
		if d > noise {
			continue
		}
		if m.State == Resting && 3*d > 2*noise || m.Status(MonsExhausted) && m.State == Resting && 3*d > noise {
			continue
		}
		if m.SeesPlayer(g) {
			m.MakeAware(g)
		} else {
			m.MakeWanderAt(at)
		}
		m.GatherBand(g)
	}
}

// LeaveRoomForPlayer moves a peaceful monster to make room for the player.
func (m *Monster) LeaveRoomForPlayer(g *Game) gruid.Point {
	dij := &MonPath{g: g, monster: m}
	nodes := g.PR.DijkstraMap(dij, []gruid.Point{m.P}, 10)
	free := invalidPos
	dist := unreachable
	for _, n := range nodes {
		if !m.CanPass(g, n.P) {
			continue
		}
		if n.P == g.Player.P || n.P == m.P {
			continue
		}
		mons := g.MonsterAt(n.P)
		if mons.Exists() {
			continue
		}
		if distance(n.P, m.P) < dist {
			free = n.P
			dist = distance(n.P, m.P)
		}
	}
	// free should be valid except in really rare cases.
	return free
}

func (g *Game) FindJumpTarget(m *Monster) gruid.Point {
	mp := &JumpPath{g: g}
	nodes := g.PR.BreadthFirstMap(mp, []gruid.Point{m.P}, 10)
	free := invalidPos
	dist := unreachable
	for _, n := range nodes {
		if !g.PlayerCanPass(n.P) {
			continue
		}
		if n.P == g.Player.P || n.P == m.P {
			continue
		}
		mons := g.MonsterAt(n.P)
		if mons.Exists() {
			continue
		}
		if distance(n.P, m.P) < dist {
			free = n.P
			dist = distance(n.P, m.P)
		}
	}
	// free should be valid except in really rare cases
	return free
}

// Jump handles jumping over a monster.
func (g *Game) Jump(mons *Monster) (bool, error) {
	if mons.Peaceful(g) && mons.Kind != MonsEarthDragon {
		op := mons.P
		if g.Map.At(op) == ChasmCell && !g.Player.HasStatus(StatusLevitation) {
			return true, g.AbyssJump()
		}
		if !mons.CanPass(g, g.Player.P) {
			p := mons.LeaveRoomForPlayer(g)
			if p != invalidPos {
				mons.MoveTo(g, p)
				mons.Swapped = true
				g.PlacePlayerAt(op)
				return false, nil
			}
			// otherwise (which should not happen in practice), swap anyways
		}
		g.PlacePlayerAt(mons.P)
		return false, nil
	}
	if g.Player.HasStatus(StatusExhausted) {
		return false, errors.New("You cannot jump while exhausted.")
	}
	dir := dirnorm(g.Player.P, mons.P)
	p := g.Player.P
	for {
		p = p.Add(dir)
		if !g.PlayerCanPass(p) {
			break
		}
		m := g.MonsterAt(p)
		if !m.Exists() {
			break
		}
	}
	if !g.PlayerCanPass(p) {
		p = g.FindJumpTarget(mons)
		if !g.PlayerCanPass(p) {
			// should not happen in practice, but better safe than sorry
			g.Teleportation()
			return false, nil
		}
	}
	if !g.Player.HasStatus(StatusSwift) && g.Player.Inventory.Body != CloakAcrobat {
		g.PutStatus(StatusExhausted, DurationExhaustion)
	}
	if mons.Kind == MonsEarthDragon {
		g.PutConfusion()
	}
	g.PlacePlayerAt(p)
	g.Stats.Jumps++
	g.Logf("You jump over %s", mons.Kind.Definite(false))
	g.StoryPrintf("Jumped over %s", mons.Kind)
	if g.Stats.Jumps+g.Stats.WallJumps == 15 {
		AchAcrobat.Get(g)
	}
	return false, nil
}

// WallJump performs a jump by propulsing off a wall.
func (g *Game) WallJump(p gruid.Point) error {
	t := g.Map.At(g.Player.P)
	if IsEnclosing(t) {
		return fmt.Errorf("You cannot jump from %s.", g.TerrainShortDesc(t, g.Player.P))
	}
	if g.Player.HasStatus(StatusExhausted) {
		return errors.New("You cannot jump while exhausted.")
	}
	dir := dirnorm(p, g.Player.P)
	p = g.Player.P
	q := p
	count := 0
	path := []gruid.Point{q}
	for count < 4 {
		p = p.Add(dir)
		if !g.PlayerCanJumpPass(p) {
			break
		}
		count++
		q = p
		path = append(path, q)
		m := g.MonsterAt(p)
		if !m.Exists() && count == 3 {
			break
		}
	}
	m := g.MonsterAt(q)
	if m.Exists() {
		return errors.New("There’s not enough room to jump.")
	}
	if count == 3 && !g.PlayerCanPass(q) {
		q = path[len(path)-2]
		m := g.MonsterAt(q)
		if m.Exists() {
			return errors.New("There’s not enough room to jump.")
		}
		path = path[:len(path)-1]
	}
	if !g.PlayerCanPass(q) || (count != 3 && count != 2) {
		return errors.New("There’s not enough room to jump.")
	}
	if !g.Player.HasStatus(StatusSwift) && g.Player.Inventory.Body != CloakAcrobat {
		g.PutStatus(StatusExhausted, DurationExhaustion)
	}
	g.md.PushAnimation(path)
	g.PlacePlayerAt(q)
	g.Stats.WallJumps++
	g.Log("You jump by propelling yourself against the wall.")
	if g.Stats.Jumps+g.Stats.WallJumps == 15 {
		AchAcrobat.Get(g)
	}
	return nil
}

// HitNoise returns the amount of noise corresponding to a hit.
func (g *Game) HitNoise(clang bool) int {
	noise := BaseHitNoise
	if clang {
		noise += 5
	}
	return noise
}

// HandleKill handles a monster death (like when falling into the abyss or
// drowning).
func (g *Game) HandleKill(mons *Monster) {
	g.Stats.Killed++
	g.Stats.KilledMons[mons.Kind]++
	if g.Player.Sees(mons.P) {
		AchAssassin.Get(g)
	}
	if g.Map.At(mons.P) == DoorCell {
		g.ComputeFOV()
	}
	g.StoryPrintf("Death of %s", mons.Kind.Indefinite(false))
}

// The following constants represents various kinds of noise amounts.
const (
	WallNoise              = 10
	ExplosionNoise         = 14
	BarkNoise              = 10
	MagicCastNoise         = 5
	HarmonicNoise          = 7
	BaseHitNoise           = 2
	QueenStoneNoise        = 10
	SingingNoise           = 12
	EarthquakeNoise        = 35
	QueenRockFootstepNoise = 7
	DelayedHarmonicNoise   = 25
	OricExplosionNoise     = 20
)

// ClangMsg returns text for a noisy hit.
func (g *Game) ClangMsg() (sclang string) {
	return " Smash!"
}
