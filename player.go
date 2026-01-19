package main

import (
	"errors"
	"fmt"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/rl"
)

// Player represents the player.
type Player struct {
	HP        int                  // current HP
	HPbonus   int                  // temporary HP bonus (lignification)
	MP        int                  // current MP
	Bananas   int                  // number of carried bananas
	Magaras   []Magara             // equipped magaras
	Statuses  map[status]int       // active status effect durations
	Expire    map[status]int       // turn of expiration for active status effects
	P         gruid.Point          // current position
	AFOV      map[gruid.Point]bool // actual field of view (accounting for darkness)
	FOV       *rl.FOV              // field of view (full range)
	Inventory Inventory            // inventory (cloak, amulet, artifact)
}

// Inventory represents the inventory.
type Inventory struct {
	Body item // cloak
	Neck item // amulet
	Misc item // artifact
}

// DefaultHealth represents the starting max HP.
const DefaultHealth = 5

func (pl *Player) HPMax() int {
	hpmax := DefaultHealth
	if pl.Inventory.Body == CloakVitality {
		hpmax += 2
	}
	if hpmax < 2 {
		hpmax = 2
	}
	return hpmax
}

// DefaultMPmax represents the starting max MP.
const DefaultMPmax = 6

func (pl *Player) MPMax() int {
	return DefaultMPmax
}

func (pl *Player) HasStatus(st status) bool {
	return pl.Statuses[st] > 0
}

func (g *Game) AutoToDir() bool {
	if g.MonsterInFOV() == nil {
		p := g.Player.P.Add(g.autoDir)
		nbs := g.dirNeighbors(p, g.autoDir)
		blocked := g.dirBlocked(p, g.autoDir)
		if g.PlayerCanPass(p) && (g.autoDirNeighbors == nbs || nbs != dirFreeLaterals && blocked ||
			nbs == dirBlockedLaterals && (g.autoDirChanged || g.dirNeighbors(g.Player.P, g.autoDir) == dirBlockedLaterals)) {
			again, err := g.PlayerBump(p)
			if err != nil {
				g.Log(err.Error())
				g.autoDir = ZP
				return false
			}
			if again {
				g.autoDir = ZP
				return false
			}
			// player moved in the direction
			g.autoDirChanged = false
			if blocked && nbs > 0 {
				if g.PlayerCanPass(left(p, g.autoDir)) {
					g.autoDir = left(p, g.autoDir).Sub(p)
					g.autoDirChanged = true
				} else if g.PlayerCanPass(right(p, g.autoDir)) {
					g.autoDir = right(p, g.autoDir).Sub(p)
					g.autoDirChanged = true
				}
				g.autoDirNeighbors = g.dirNeighbors(p, g.autoDir)
			}
			return true
		}
	}
	g.autoDir = ZP
	return false
}

type dirNeighbors int

const (
	dirFreeLaterals dirNeighbors = iota
	dirBlockedLeft
	dirBlockedRight
	dirBlockedLaterals
)

func (g *Game) dirNeighbors(p, dir gruid.Point) (dn dirNeighbors) {
	if !g.PlayerCanPass(left(p, dir)) {
		dn += dirBlockedLeft
	}
	if !g.PlayerCanPass(right(p, dir)) {
		dn += dirBlockedRight
	}
	return dn
}

func (g *Game) dirBlocked(p, dir gruid.Point) bool {
	return !g.PlayerCanPass(p.Add(dir))
}

func (g *Game) GoToDir(dir gruid.Point) (again bool, err error) {
	if g.MonsterInFOV() != nil {
		g.autoDir = ZP
		return again, errors.New("You cannot travel while there are monsters in view.")
	}
	p := g.Player.P.Add(dir)
	if !g.PlayerCanPass(p) {
		return again, errors.New("You cannot move in that direction.")
	}
	again, err = g.PlayerBump(p)
	if err != nil || again {
		return again, err
	}
	g.autoDir = dir
	nbs := g.dirNeighbors(p, dir)
	g.autoDirChanged = false
	blocked := g.dirBlocked(p, dir)
	if blocked && nbs > 0 {
		if g.PlayerCanPass(left(p, dir)) {
			g.autoDir = left(p, dir).Sub(p)
			g.autoDirChanged = true
		} else if g.PlayerCanPass(right(p, dir)) {
			g.autoDir = right(p, dir).Sub(p)
			g.autoDirChanged = true
		}
		g.autoDirNeighbors = g.dirNeighbors(p, g.autoDir)
	} else {
		g.autoDirNeighbors = nbs
	}
	return again, err
}

func (g *Game) MoveToTarget() bool {
	if !InMap(g.autoTarget) {
		return false
	}
	path := g.PlayerPath(g.Player.P, g.autoTarget)
	if g.MonsterInFOV() != nil {
		g.autoTarget = invalidPos
	}
	if len(path) < 1 {
		g.autoTarget = invalidPos
		return false
	}
	var err error
	var again bool
	if len(path) > 1 {
		again, err = g.PlayerBump(path[1])
	} else {
		g.WaitTurn()
	}
	if err != nil {
		g.Log(err.Error())
		g.autoTarget = invalidPos
		return false
	}
	if InMap(g.autoTarget) && g.Player.P == g.autoTarget {
		g.autoTarget = invalidPos
	}
	return !again
}

func (g *Game) WaitTurn() {
	g.Stats.Waits++
}

func (g *Game) MonsterCount() (count int) {
	for _, mons := range g.Monsters {
		if mons.Exists() {
			count++
		}
	}
	return count
}

func (g *Game) Rest() error {
	if g.Map.At(g.Player.P) != BarrelCell {
		return fmt.Errorf("This place is not safe for sleeping.")
	}
	if cld, ok := g.Map.Clouds[g.Player.P]; ok && cld == CloudFire {
		return errors.New("You cannot rest on flames.")
	}
	if !g.NeedsRegenRest() && !g.StatusRest() {
		return errors.New("You do not need to rest.")
	}
	if g.Player.Bananas <= 0 {
		return errors.New("You cannot sleep without eating for dinner a banana first.")
	}
	// TODO: animation
	//g.ui.DrawMessage("Resting...")
	g.resting = true
	g.restingTurns = RandInt(5) // you do not wake up when you want
	g.Player.Bananas--
	return nil
}

func (g *Game) StatusRest() bool {
	for st, q := range g.Player.Statuses {
		if st.Info() {
			continue
		}
		if q > 0 {
			return true
		}
	}
	return false
}

func (g *Game) NeedsRegenRest() bool {
	return g.Player.HP < g.Player.HPMax() || g.Player.MP < g.Player.MPMax()
}

func (g *Game) Teleportation() {
	var p gruid.Point
	i := 0
	count := 0
	for {
		count++
		if count > maxIterations {
			panic("Teleportation")
		}
		p = g.FreePassableCell()
		if distance(p, g.Player.P) < 15 && i < maxIterations {
			i++
			continue
		}
		break

	}
	if InMap(p) {
		// should always happen
		opos := g.Player.P
		g.Log("You teleport away.")
		g.md.TeleportAnimation(opos, p, true)
		g.PlacePlayerAt(p)
	} else {
		// should not happen
		g.Log("Something went wrong with the teleportation.")
	}
}

const MaxBananas = 4

func (g *Game) CollectGround() {
	p := g.Player.P
	t := g.Map.At(p)
	if IsNotable(t) {
		g.autoexploreMapRebuild = true
	switchcell:
		switch t {
		case BarrelCell:
			// TODO: move here message
		case BananaCell:
			if g.Player.Bananas >= MaxBananas {
				g.Log("There is a banana, but your pack is already full.")
			} else {
				g.Log("You take a banana.")
				g.Player.Bananas++
				g.StoryPrintf("Found banana (bananas: %d)", g.Player.Bananas)
				g.Map.Set(p, GroundCell)
				delete(g.Objects.Bananas, p)
				if g.Player.Bananas == MaxBananas {
					AchBananaCollector.Get(g)
				}
			}
		case MagaraCell:
			for i, mag := range g.Player.Magaras {
				if mag.Kind != NoMagara {
					continue
				}
				g.Player.Magaras[i] = g.Objects.Magaras[p]
				delete(g.Objects.Magaras, p)
				g.Map.Set(p, GroundCell)
				g.Logf("You take the %s.", g.Player.Magaras[i])
				g.StoryPrintf("Took %s", g.Player.Magaras[i])
				break switchcell
			}
			g.Logf("You stand over %s.", Indefinite(g.Objects.Magaras[p].String(), false))
		case FakeStairCell:
			g.Map.Set(p, GroundCell)
			g.LogStyled("You stand over fake stairs.", logSpecial)
			g.LogStyled("Harmonic illusions!", logSpecial)
			g.StoryPrint("Found harmonic fake stairs!")
			g.md.FoundFakeStairsAnimation()
		case PotionCell:
			g.DrinkPotion(p)
		default:
			g.Logf("You are standing over %s.", g.TerrainShortDesc(t, p))
		}
	} else if t == DoorCell {
		g.Log("You stand at the door.")
	}
	if AchivementTerrain(t) {
		g.Reach(p)
	}
}

func (g *Game) FallAbyss(style descendstyle) {
	if g.Player.HasStatus(StatusLevitation) {
		return
	}
	g.Player.HP -= 2
	if g.Player.HP <= 0 {
		g.Player.HP = 1
	}
	g.Player.MP -= 2
	if g.Player.MP < 0 {
		g.Player.MP = 0
	}
	if g.Player.Bananas > 0 {
		g.Player.Bananas--
	}
	if style == DescendFall && g.Map.Depth == MaxDepth || g.Map.Depth == WinDepth {
		g.Player.HP = 0
		return
	}
	g.Descend(style)
}

func (g *Game) AbyssJumpConfirmation() {
	g.LogStyled("Do you really want to jump into the abyss? (DANGEROUS) [y/N]", logConfirm)
	g.md.mode = modeJumpConfirmation
}

func (g *Game) DeepChasmDepth() bool {
	return g.Map.Depth == WinDepth || g.Map.Depth == MaxDepth
}

func (g *Game) AbyssJump() error {
	if g.DeepChasmDepth() {
		return errors.New("You cannot jump into deep chasm.")
	}
	g.AbyssJumpConfirmation()
	return nil
}

func (g *Game) PlayerBump(p gruid.Point) (again bool, err error) {
	if !InMap(p) {
		return again, errors.New("You cannot move there.")
	}
	t := g.Map.At(p)
	switch {
	case t == BarrierCell && !g.Player.HasStatus(StatusLevitation):
		return again, errors.New("You cannot move into a magical barrier.")
	case t == WindowCell && !g.Player.HasStatus(StatusDig):
		return again, errors.New("You cannot pass through the closed window.")
	case t == BarrelCell && g.MonsterFOV[g.Player.P]:
		return again, errors.New("You cannot enter a barrel while seen.")
	}
	mons := g.MonsterAt(p)
	if IsJumpPropulsion(t) && !g.Player.HasStatus(StatusDig) {
		err := g.WallJump(p)
		if err != nil {
			return again, err
		}
	} else if !mons.Exists() {
		if g.Player.HasStatus(StatusLignification) {
			return again, errors.New("You cannot move while lignified.")
		}
		if t == ChasmCell && !g.Player.HasStatus(StatusLevitation) {
			again = true
			return again, g.AbyssJump()
		}
		if t == BarrelCell {
			g.Log("You hide yourself inside the barrel.")
		} else if t == TableCell {
			g.Log("You hide yourself under the table.")
		} else if t == TreeCell {
			g.Log("You climb to the top.")
		} else if t == HoledWallCell {
			g.Log("You crawl under the wall.")
		}
		if IsDiggable(t) && t != HoledWallCell {
			g.Map.Set(p, RubbleCell)
			g.MakeNoise(WallNoise, p)
			g.Log(g.CrackSound())
			g.Fog(p, 1)
			g.Stats.Digs++
			g.Stats.DestructionUse++
			if g.Stats.DestructionUse == 20 {
				AchDestructorNovice.Get(g)
			}
			if g.Stats.DestructionUse == 40 {
				AchDestructorInitiate.Get(g)
			}
			if g.Stats.DestructionUse == 60 {
				AchDestructorMaster.Get(g)
			}
		}
		if g.Player.Inventory.Body == CloakSmoke {
			_, ok := g.Map.Clouds[g.Player.P]
			if !ok && AllowsFog(g.Map.At(g.Player.P)) {
				g.Map.Clouds[g.Player.P] = CloudPlain
				g.PushEventD(&PosEvent{P: g.Player.P, Action: CloudEnd}, DurationSmokingCloakFog)
			}
		}
		//}
		g.Stats.Moves++
		g.PlacePlayerAt(p)
	} else if again, err = g.Jump(mons); err != nil {
		return again, err
	}
	if g.Player.HasStatus(StatusSwift) {
		again = true
		g.Player.Statuses[StatusSwift]--
		if !g.Player.HasStatus(StatusSwift) {
			g.Log("You no longer feel swift.")
		}
		g.md.updateMapInfo()
		return again, nil
	}
	return again, nil
}

func (g *Game) SwiftFog() {
	mp := &NoisePath{g: g}
	nodes := g.PR.BreadthFirstMap(mp, []gruid.Point{g.Player.P}, 2)
	for _, n := range nodes {
		_, ok := g.Map.Clouds[n.P]
		if !ok && AllowsFog(g.Map.At(n.P)) {
			g.Map.Clouds[n.P] = CloudPlain
			g.PushEvent(&PosEvent{P: n.P, Action: CloudEnd}, g.Turn+DurationFog+RandInt(DurationFog/2))
		}
	}
	g.PutStatus(StatusSwift, DurationShortSwiftness)
	g.ComputeFOV()
	g.Log("You feel an energy burst and smoke comes out from you.")
}

func (g *Game) Confusion() {
	if g.PutStatus(StatusConfusion, DurationConfusionPlayer) {
		g.Log("You feel confused.")
	}
}

// PlacePlayerAt moves the player to a given position, swapping positions with
// a monster if necessary, and handles FOV and monsters awareness update,
// ground collecting, and footsteps noise.
func (g *Game) PlacePlayerAt(p gruid.Point) {
	if p == g.Player.P {
		return
	}
	m := g.MonsterAt(p)
	ppos := g.Player.P
	g.Player.P = p
	if m.Exists() {
		m.MoveTo(g, ppos)
		m.Swapped = true
	}
	if g.Map.At(g.Player.P) == QueenRockCell && !g.Player.HasStatus(StatusLevitation) {
		g.MakeNoise(QueenRockFootstepNoise, g.Player.P)
		g.Log("Tap-tap.")
	}
	g.CollectGround()
	g.ComputeFOV()
	g.MakeMonstersAware()
}

const LignificationHPbonus = 4

func (g *Game) EnterLignification() {
	if g.PutStatus(StatusLignification, DurationLignificationPlayer) {
		g.Log("You feel rooted to the ground.")
		g.Player.HPbonus += LignificationHPbonus
	}
}

func (g *Game) ExtinguishFire() error {
	g.Map.Set(g.Player.P, ExtinguishedLightCell)
	g.Objects.Lights[g.Player.P] = false
	g.Stats.Extinguishments++
	if g.Stats.Extinguishments >= 15 {
		AchExtinguisher.Get(g)
	}
	g.Log("You extinguish the fire.")
	return nil
}

func (g *Game) PutStatus(st status, duration int) bool {
	if g.Player.Statuses[st] != 0 {
		return false
	}
	g.Player.Statuses[st] += duration
	g.PushEventD(&StatusEvent{Status: st}, DurationStatusStep)
	g.Stats.Statuses[st]++
	if st.Good() {
		g.Player.Expire[st] = g.Turn + duration
	}
	return true
}

func (g *Game) PutFakeStatus(st status, duration int) bool {
	if g.Player.Statuses[st] != 0 {
		return false
	}
	g.Player.Statuses[st] += duration
	g.Stats.Statuses[st]++
	if st.Good() {
		g.Player.Expire[st] = g.Turn + duration
	}
	return true
}

func (g *Game) PlayerCanPass(p gruid.Point) bool {
	if !InMap(p) {
		return false
	}
	t := g.Map.At(p)
	return IsPlayerPassable(t) ||
		g.Player.HasStatus(StatusLevitation) && (t == BarrierCell || IsLevitatePassable(t)) ||
		g.Player.HasStatus(StatusDig) && IsDiggable(t)
}

func (g *Game) PlayerCanJumpPass(p gruid.Point) bool {
	if !InMap(p) {
		return false
	}
	t := g.Map.At(p)
	return IsJumpPassable(t) ||
		g.Player.HasStatus(StatusLevitation) && t == BarrierCell
}
