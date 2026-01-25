package main

import (
	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/rl"
)

// Stats gathers various game statistics.
type Stats struct {
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
	Achievements      map[Achievement]int
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
	DrankPotions      int
	TimesPushed       int
	TimesBlinked      int
	TimesBlocked      int
}

// TurnStats computes some turn-specific stats.
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

// LevelStats computes level-specific stats.
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
	g.Stats.DUSpottedPerc[g.Map.Depth] = g.Stats.DUSpotted[g.Map.Depth] * 100 / nmons
}

// Achievement represents an achievement.
type Achievement string

// List of achievements.
const (
	NoAchievement          Achievement = "Pitiful Death"
	AchBananaCollector     Achievement = "Banana Collector"
	AchHarmonistNovice     Achievement = "Harmonist Novice"
	AchHarmonistInitiate   Achievement = "Harmonist Initiate"
	AchHarmonistMaster     Achievement = "Harmonist Master"
	AchNoviceOricCelmist   Achievement = "Oric Celmist Novice"
	AchInitiateOricCelmist Achievement = "Oric Celmist Initiate"
	AchMasterOricCelmist   Achievement = "Oric Celmist Master"
	AchUnstealthy          Achievement = "Unstealthy Gawalt"
	AchStealthNovice       Achievement = "Stealth Novice"
	AchStealthInitiate     Achievement = "Stealth Initiate"
	AchStealthMaster       Achievement = "Stealth Master"
	AchPyromancerNovice    Achievement = "Pyromancer Novice"
	AchPyromancerInitiate  Achievement = "Pyromancer Initiate"
	AchPyromancerMaster    Achievement = "Pyromancer Master"
	AchDestructorNovice    Achievement = "Destructor Novice"
	AchDestructorInitiate  Achievement = "Destructor Initiate"
	AchDestructorMaster    Achievement = "Destructor Master"
	AchTeleport            Achievement = "Oric Teleport Maniac"
	AchCloak               Achievement = "Dressed Gawalt"
	AchAmulet              Achievement = "Protective Charm"
	AchRescuedShaedra      Achievement = "Rescued Shaedra"
	AchRetrievedArtifact   Achievement = "Recovered Gem Portal Artifact"
	AchAcrobat             Achievement = "Acrobat"
	AchTree                Achievement = "Tree Climber"
	AchTable               Achievement = "Under Table Gawalt"
	AchHole                Achievement = "Hole Crawler"
	AchDoors               Achievement = "Door Opener"
	AchBarrels             Achievement = "Barrel Enthousiast"
	AchExtinguisher        Achievement = "Light Extinguisher"
	AchLoreStudent         Achievement = "Lore student"
	AchLoremaster          Achievement = "Loremaster"
	AchNoviceExplorer      Achievement = "Novice Explorer"
	AchInitiateExplorer    Achievement = "Initiate Explorer"
	AchMasterExplorer      Achievement = "Master Explorer"
	AchAssassin            Achievement = "Assassin"
	AchInsomniaNovice      Achievement = "Insomnia Novice"
	AchInsomniaInitiate    Achievement = "Insomnia Initiate"
	AchInsomniaMaster      Achievement = "Insomnia Master"
	AchSleepy              Achievement = "Sleepy Gawalt"
	AchAntimagicNovice     Achievement = "Antimagic Novice"
	AchAntimagicInitiate   Achievement = "Antimagic Initiate"
	AchAntimagicMaster     Achievement = "Antimagic Master"
)

// Get makes you get the achievement.
func (ach Achievement) Get(g *Game) {
	if g.Stats.Achievements[ach] == 0 {
		g.Stats.Achievements[ach] = g.Turn
		g.LogfStyled("Achievement: %s.", logSpecial, ach)
		g.StoryPrintf("Achievement: %s", ach)
	}
}

// AchivementTerrain reports whether reaching that terrain is notable for
// achievement purposes.
func AchivementTerrain(t rl.Cell) bool {
	switch t {
	case TreeCell, TableCell, HoledWallCell, DoorCell, BarrelCell:
		return true
	default:
		return false
	}
}

// Reach handles any achievement for reaching the given position.
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
