package main

import (
	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/rl"
)

type stats struct {
	Story             []string
	Killed            int
	KilledMons        map[monsterKind]int
	Moves             int
	Waits             int
	Jumps             int
	WallJumps         int
	ReceivedHits      int
	Dodges            int
	MagarasUsed       int
	DMagaraUses       [MaxDepth + 1]int
	UsedStones        int
	UsedMagaras       map[magaraKind]int
	Damage            int
	DDamage           [MaxDepth + 1]int
	DExplPerc         [MaxDepth + 1]int
	DSleepingPerc     [MaxDepth + 1]int
	DKilledPerc       [MaxDepth + 1]int
	Burns             int
	Digs              int
	Rest              int
	DRests            [MaxDepth + 1]int
	Turns             int
	TWounded          int
	TMWounded         int
	TMonsFOV          int
	NSpotted          int
	NUSpotted         int
	DSpotted          [MaxDepth + 1]int
	DUSpotted         [MaxDepth + 1]int
	DUSpottedPerc     [MaxDepth + 1]int
	Achievements      map[achievement]int
	AtNotablePos      map[gruid.Point]bool
	HarmonicMagUse    int
	OricMagUse        int
	FireUse           int
	DestructionUse    int
	OricTelUse        int
	ClimbedTree       int
	TableHides        int
	HoledWallsCrawled int
	DoorsOpened       int
	BarrelHides       int
	Extinguishments   int
	Lore              map[int]bool
	Statuses          map[status]int
	StolenBananas     int
	TimesPushed       int
	TimesBlinked      int
	TimesBlocked      int
}

func (g *Game) TurnStats() {
	g.Stats.Turns++
	if g.Player.HP < g.Player.HPMax() {
		g.Stats.TWounded++
	}
	if g.MonsterInFOV() != nil {
		g.Stats.TMonsFOV++
		if g.Player.HP < g.Player.HPMax() {
			g.Stats.TMWounded++
		}
	}
}

func (g *Game) LevelStats() {
	free := 0
	exp := 0
	it := g.Map.Terrain.Iterator()
	for it.Next() {
		t := it.Cell()
		if t == WallCell || t == ChasmCell {
			continue
		}
		free++
		if IsKnown(g.Map.KnownTerrain.At(it.P())) {
			exp++
		}
	}
	g.Stats.DExplPerc[g.Map.Depth] = exp * 100 / free
	if g.Stats.DExplPerc[g.Map.Depth] > 93 {
		AchNoviceExplorer.Get(g)
	}
	if g.Map.Depth >= 5 && g.Stats.DExplPerc[g.Map.Depth] > 93 && g.Stats.DExplPerc[g.Map.Depth-1] > 93 && g.Stats.DExplPerc[g.Map.Depth-2] > 93 {
		AchInitiateExplorer.Get(g)
	}
	if g.Map.Depth >= 8 && g.Stats.DExplPerc[g.Map.Depth] > 93 && g.Stats.DExplPerc[g.Map.Depth-1] > 93 && g.Stats.DExplPerc[g.Map.Depth-2] > 93 &&
		g.Stats.DExplPerc[g.Map.Depth-3] > 93 && g.Stats.DExplPerc[g.Map.Depth-4] > 93 {
		AchMasterExplorer.Get(g)
	}
	//g.Stats.DBurns[g.Depth] = g.Stats.CurBurns // XXX to avoid little dump info leak
	nmons := len(g.Monsters)
	kmons := 0
	smons := 0
	for _, mons := range g.Monsters {
		if !mons.Exists() {
			kmons++
			continue
		}
		if mons.State == Resting {
			smons++
		}
	}
	g.Stats.DSleepingPerc[g.Map.Depth] = smons * 100 / nmons
	g.Stats.DKilledPerc[g.Map.Depth] = kmons * 100 / nmons
	g.Stats.DUSpottedPerc[g.Map.Depth] = g.Stats.DUSpotted[g.Map.Depth] * 100 / nmons
}

type achievement string

// Achievements.
const (
	NoAchievement          achievement = "Pitiful Death"
	AchBananaCollector     achievement = "Banana Collector"
	AchHarmonistNovice     achievement = "Harmonist Novice"
	AchHarmonistInitiate   achievement = "Harmonist Initiate"
	AchHarmonistMaster     achievement = "Harmonist Master"
	AchNoviceOricCelmist   achievement = "Oric Celmist Novice"
	AchInitiateOricCelmist achievement = "Oric Celmist Initiate"
	AchMasterOricCelmist   achievement = "Oric Celmist Master"
	AchUnstealthy          achievement = "Unstealthy Gawalt"
	AchStealthNovice       achievement = "Stealth Novice"
	AchStealthInitiate     achievement = "Stealth Initiate"
	AchStealthMaster       achievement = "Stealth Master"
	AchPyromancerNovice    achievement = "Pyromancer Novice"
	AchPyromancerInitiate  achievement = "Pyromancer Initiate"
	AchPyromancerMaster    achievement = "Pyromancer Master"
	AchDestructorNovice    achievement = "Destructor Novice"
	AchDestructorInitiate  achievement = "Destructor Initiate"
	AchDestructorMaster    achievement = "Destructor Master"
	AchTeleport            achievement = "Oric Teleport Maniac"
	AchCloak               achievement = "Dressed Gawalt"
	AchAmulet              achievement = "Protective Charm"
	AchRescuedShaedra      achievement = "Rescued Shaedra"
	AchRetrievedArtifact   achievement = "Recovered Gem Portal Artifact"
	AchAcrobat             achievement = "Acrobat"
	AchTree                achievement = "Tree Climber"
	AchTable               achievement = "Under Table Gawalt"
	AchHole                achievement = "Hole Crawler"
	AchDoors               achievement = "Door Opener"
	AchBarrels             achievement = "Barrel Enthousiast"
	AchExtinguisher        achievement = "Light Extinguisher"
	AchLoreStudent         achievement = "Lore student"
	AchLoremaster          achievement = "Loremaster"
	AchNoviceExplorer      achievement = "Novice Explorer"
	AchInitiateExplorer    achievement = "Initiate Explorer"
	AchMasterExplorer      achievement = "Master Explorer"
	AchAssassin            achievement = "Assassin"
	AchInsomniaNovice      achievement = "Insomnia Novice"
	AchInsomniaInitiate    achievement = "Insomnia Initiate"
	AchInsomniaMaster      achievement = "Insomnia Master"
	AchSleepy              achievement = "Sleepy Gawalt"
	AchAntimagicNovice     achievement = "Antimagic Novice"
	AchAntimagicInitiate   achievement = "Antimagic Initiate"
	AchAntimagicMaster     achievement = "Antimagic Master"
)

func (ach achievement) Get(g *Game) {
	if g.Stats.Achievements[ach] == 0 {
		g.Stats.Achievements[ach] = g.Turn
		g.LogfStyled("Achievement: %s.", logSpecial, ach)
		g.StoryPrintf("Achievement: %s", ach)
	}
}

func ReachNotable(t rl.Cell) bool {
	switch t {
	case TreeCell, TableCell, HoledWallCell, DoorCell, BarrelCell:
		return true
	default:
		return false
	}
}

func (g *Game) Reach(p gruid.Point) {
	if g.Stats.AtNotablePos[p] {
		return
	}
	g.Stats.AtNotablePos[p] = true
	t := g.Map.At(p)
	switch t {
	case TreeCell:
		g.Stats.ClimbedTree++
		if g.Stats.ClimbedTree == 12 {
			AchTree.Get(g)
		}
	case TableCell:
		g.Stats.TableHides++
		if g.Stats.TableHides == 12 {
			AchTable.Get(g)
		}
	case HoledWallCell:
		g.Stats.HoledWallsCrawled++
		if g.Stats.HoledWallsCrawled == 12 {
			AchHole.Get(g)
		}
	case DoorCell:
		g.Stats.DoorsOpened++
		if g.Stats.DoorsOpened == 100 {
			AchDoors.Get(g)
		}
	case BarrelCell:
		g.Stats.BarrelHides++
		if g.Stats.BarrelHides == 20 {
			AchBarrels.Get(g)
		}
	}
}
