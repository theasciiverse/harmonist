package main

import (
	"codeberg.org/anaseto/gruid"
)

type event interface {
	Handle(*Game)
}

func (g *Game) PushEvent(ev event, r int) {
	g.Events.Push(ev, r)
}

func (g *Game) PushEventD(ev event, delay int) {
	g.Events.Push(ev, g.Turn+delay)
}

func (g *Game) PushEventFirst(ev event, r int) {
	g.Events.PushFirst(ev, r)
}

type endTurnEvent int

const endTurnAction endTurnEvent = 0

type playerEventAction int

const (
	StorySequence playerEventAction = iota
	AbyssFall
)

type playerEvent struct {
	Action playerEventAction
}

func (ev *playerEvent) Handle(g *Game) {
	switch ev.Action {
	case StorySequence:
		g.ComputeLOS()
		g.md.Story()
	case AbyssFall:
		if g.Map.At(g.Player.P) == ChasmCell {
			g.FallAbyss(DescendFall)
		}
	}
}

type statusEvent struct {
	Status status
}

var StatusEndMsgs = [...]string{
	StatusExhausted:     "You no longer feel exhausted.",
	StatusSwift:         "You no longer feel speedy.",
	StatusLignification: "You no longer feel attached to the ground.",
	StatusConfusion:     "You no longer feel confused.",
	StatusDig:           "You no longer feel like an earth dragon.",
	StatusLevitation:    "You no longer levitate.",
	StatusShadows:       "You are no longer surrounded by shadows.",
	StatusIlluminated:   "You are no longer illuminated.",
	StatusTransparent:   "You are no longer transparent.",
	StatusDisguised:     "You are no longer disguised.",
	StatusDispersal:     "You are no longer unstable.",
}

func (sev *statusEvent) Handle(g *Game) {
	st := sev.Status
	g.Player.Statuses[st] -= DurationStatusStep
	if g.Player.Statuses[st] <= 0 {
		g.Player.Statuses[st] = 0
		g.LogStyled(StatusEndMsgs[st], logStatusEnd)
		g.md.StatusEndAnimation()
		switch st {
		case StatusLevitation:
			if g.Map.At(g.Player.P) == ChasmCell {
				g.PushEventFirst(&playerEvent{Action: AbyssFall}, g.Turn)
			}
		case StatusLignification:
			g.Player.HPbonus -= LignificationHPbonus
			if g.Player.HPbonus < 0 {
				g.Player.HPbonus = 0
			}
		}
	} else {
		g.PushEventD(sev, DurationStatusStep)
	}
}

type monsterTurnEvent struct {
	Index int
}

func (mev *monsterTurnEvent) Handle(g *Game) {
	mons := g.Monsters[mev.Index]
	if mons.Exists() {
		mons.HandleTurn(g)
	}
}

type monsterStatusEvent struct {
	Index  int
	Status monsterStatus
}

func (mev *monsterStatusEvent) Handle(g *Game) {
	mons := g.Monsters[mev.Index]
	st := mev.Status
	mons.Statuses[st] -= DurationStatusStep
	if mons.Statuses[st] <= 0 {
		mons.Statuses[st] = 0
		if g.Player.Sees(mons.P) {
			g.Logf("%s is no longer %s.", mons.Kind.Definite(true), st)
		}
		switch st {
		case MonsConfused, MonsLignified:
			mons.Path = mons.APath(g, mons.P, mons.Target)
		}
	} else {
		g.PushEventD(&monsterStatusEvent{Index: mev.Index, Status: st}, DurationStatusStep)
	}
}

type posAction int

const (
	CloudEnd posAction = iota
	ObstructionEnd
	ObstructionProgression
	FireProgression
	NightProgression
	MistProgression
	Earthquake
	DelayedHarmonicNoiseEvent
	DelayedOricExplosionEvent
)

type posEvent struct {
	P      gruid.Point
	Action posAction
	Timer  int
}

func (cev *posEvent) Handle(g *Game) {
	switch cev.Action {
	case CloudEnd:
		delete(g.Map.Clouds, cev.P)
		g.ComputeLOS()
	case ObstructionEnd:
		if t, ok := g.Map.MagicalBarriers[cev.P]; ok {
			g.Map.Set(cev.P, t)
		}
	case ObstructionProgression:
		p := g.FreePassableCell()
		g.MagicalBarrierAt(p)
		if g.Player.Sees(p) {
			g.Logf("You see an oric barrier appear out of thin air.")
			g.StopAuto()
		}
		g.PushEvent(&posEvent{Action: ObstructionProgression},
			g.Turn+DurationObstructionProgression+RandInt(DurationObstructionProgression/4))
	case FireProgression:
		if _, ok := g.Map.Clouds[cev.P]; !ok {
			break
		}
		for _, p := range g.playerPassableNeighbors(cev.P) {
			if RandInt(10) == 0 {
				continue
			}
			g.Burn(p)
		}
		delete(g.Map.Clouds, cev.P)
		g.NightFog(cev.P, 1)
		g.ComputeLOS()
	case NightProgression:
		if _, ok := g.Map.Clouds[cev.P]; !ok {
			break
		}
		if cev.Timer <= 0 {
			delete(g.Map.Clouds, cev.P)
			g.ComputeLOS()
			break
		}
		g.MakeCreatureSleep(cev.P)
		cev.Timer--
		g.PushEventD(cev, DurationTurn)
	case MistProgression:
		p := g.FreePassableCell()
		g.Fog(p, 1)
		g.PushEvent(&posEvent{Action: MistProgression},
			g.Turn+DurationMistProgression+RandInt(DurationMistProgression/4))
	case Earthquake:
		g.LogStyled("The earth suddenly shakes with force!", logSpecial)
		g.LogStyled("Craack!", logSpecial)
		g.StoryPrint("Special event: earthquake!")
		g.MakeNoise(EarthquakeNoise, cev.P)
		g.Map.NoiseIllusion[cev.P] = true
		it := g.Map.Terrain.Iterator()
		for it.Next() {
			p := it.P()
			t := it.Cell()
			if !IsDiggable(t) || !g.Map.HasFreeNeighbor(&g.nbs, p) {
				continue
			}
			if distance(cev.P, p) > RandInt(35) || RandInt(2) == 0 {
				continue
			}
			g.Map.Set(p, RubbleCell)
			g.Fog(p, 1)
		}
	case DelayedHarmonicNoiseEvent:
		if cev.Timer <= 1 {
			g.Player.Statuses[StatusDelay] = 0
			g.Log("Pop!")
			g.Map.NoiseIllusion[cev.P] = true
			g.MakeNoise(DelayedHarmonicNoise, cev.P)
		} else {
			cev.Timer--
			g.Player.Statuses[StatusDelay] = cev.Timer
			g.PushEventD(cev, DurationTurn)
		}
	case DelayedOricExplosionEvent:
		if cev.Timer <= 1 {
			g.OricExplosion(cev.P)
		} else {
			cev.Timer--
			g.Player.Statuses[StatusDelay] = cev.Timer
			g.PushEventD(cev, DurationTurn)
		}
	}
}

// OricExplosion produces an oric explosion centered around the given position.
func (g *Game) OricExplosion(at gruid.Point) {
	g.Player.Statuses[StatusDelay] = 0
	g.Log(g.CrackSound())
	g.Map.NoiseIllusion[at] = true
	dij := &gridPath{dungeon: g.Map}
	g.MakeNoise(OricExplosionNoise, at)
	nodes := g.PR.DijkstraMap(dij, []gruid.Point{at}, 7)
	fogs := []gruid.Point{}
	for _, n := range nodes {
		t := g.Map.At(n.P)
		if !IsDiggable(t) {
			continue
		}
		g.Map.Set(n.P, RubbleCell)
		g.Stats.Digs++
		if g.Player.Sees(n.P) {
			g.md.WallExplosionAnimation(n.P)
		}
		fogs = append(fogs, n.P)
		if IsKnown(g.Map.KnownTerrain.At(n.P)) {
			g.Map.KnownTerrain.Set(n.P, RubbleCell)
		}
	}
	for _, p := range fogs {
		g.Fog(p, 1)
	}
	g.ComputeLOS()
}

func (g *Game) NightFog(at gruid.Point, radius int) {
	dij := &noisePath{g: g}
	nodes := g.PR.DijkstraMap(dij, []gruid.Point{at}, radius)
	for _, n := range nodes {
		_, ok := g.Map.Clouds[n.P]
		if !ok {
			g.Map.Clouds[n.P] = CloudNight
			g.PushEventD(&posEvent{Action: NightProgression,
				P: n.P, Timer: DurationNightFog}, DurationCloudProgression)

			g.MakeCreatureSleep(n.P)
		}
	}
	g.ComputeLOS()
}

func (g *Game) MakeCreatureSleep(p gruid.Point) {
	if p == g.Player.P {
		if g.PutStatus(StatusConfusion, DurationConfusionPlayer) {
			g.Log("The clouds of night confuse you.")
		}
		return
	}
	mons := g.MonsterAt(p)
	if !mons.Exists() || (RandInt(2) == 0 && mons.Status(MonsExhausted)) {
		// do not always make already exhausted monsters sleep (they were probably awaken)
		return
	}
	mons.EnterConfusion(g)
	if mons.State != Resting && g.Player.Sees(mons.P) {
		g.Logf("%s falls asleep.", mons.Kind.Definite(true))
	}
	mons.State = Resting
	mons.Dir = ZP
	mons.ExhaustTime(g, 4+RandInt(2))
}

func (g *Game) Burn(p gruid.Point) {
	if _, ok := g.Map.Clouds[p]; ok {
		return
	}
	t := g.Map.At(p)
	if !Flammable(t) {
		return
	}
	g.Stats.Burns++
	switch t {
	case DoorCell:
		g.Log("The door vanishes in magical flames.")
	case TableCell:
		g.Log("The table vanishes in magical flames.")
	case BarrelCell:
		g.Log("The barrel vanishes in magical flames.")
		delete(g.Objects.Barrels, p)
	case TreeCell:
		g.Log("The tree vanishes in magical flames.")
	}
	g.Map.Set(p, GroundCell)
	g.Map.Clouds[p] = CloudFire
	if g.Player.Sees(p) {
		g.ComputeLOS()
	}
	g.PushEventD(&posEvent{P: p, Action: FireProgression}, DurationCloudProgression)
}

const (
	DurationSwiftness              = 4
	DurationShadows                = 15
	DurationLevitation             = 18
	DurationShortSwiftness         = 3
	DurationDigging                = 8
	DurationParalysisMonster       = 6
	DurationCloudProgression       = 1
	DurationFog                    = 15
	DurationExhaustion             = 5
	DurationConfusionMonster       = 12
	DurationConfusionPlayer        = 5
	DurationLignificationMonster   = 15
	DurationLignificationPlayer    = 4
	DurationMagicalBarrier         = 15
	DurationObstructionProgression = 15
	DurationMistProgression        = 12
	DurationSmokingCloakFog        = 2
	DurationExhaustionMonster      = 10
	DurationSatiationMonster       = 40
	DurationIlluminated            = 7
	DurationTransparency           = 15
	DurationDisguise               = 9
	DurationDispersal              = 12
	DurationHarmonicNoiseDelay     = 13
	DurationOricExplosionDelay     = 8
	DurationTurn                   = 1
	DurationStatusStep             = 1
	DurationNightFog               = 15
)
