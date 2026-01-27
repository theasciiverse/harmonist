package main

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
)

type statusSlice []status

func (sts statusSlice) Len() int           { return len(sts) }
func (sts statusSlice) Swap(i, j int)      { sts[i], sts[j] = sts[j], sts[i] }
func (sts statusSlice) Less(i, j int) bool { return sts[i] < sts[j] }

type monsSlice []monsterKind

func (ms monsSlice) Len() int      { return len(ms) }
func (ms monsSlice) Swap(i, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms monsSlice) Less(i, j int) bool {
	return ms[i].Dangerousness() > ms[j].Dangerousness()
}

func (g *Game) DumpStatuses() string {
	sts := sort.StringSlice{}
	for st, c := range g.Player.Statuses {
		if c > 0 {
			sts = append(sts, st.String())
		}
	}
	sort.Sort(sts)
	if len(sts) == 0 {
		return "You are free of any status effects."
	}
	return "Statuses:\n" + strings.Join(sts, "\n")
}

func (g *Game) SortedKilledMonsters() monsSlice {
	var ms monsSlice
	for mk, p := range g.Stats.KilledMons {
		if p == 0 {
			continue
		}
		ms = append(ms, mk)
	}
	sort.Sort(ms)
	return ms
}

func (g *Game) Dump() string {
	sb := &strings.Builder{}
	fmt.Fprintf(sb, " ♣ Game Summary — Harmonist %s ♣\n\n", Version)
	if g.Wizard != WizardNone {
		fmt.Fprintf(sb, "ENABLED WIZARD MODE.\n")
	}
	if g.Player.HP > 0 && g.Map.Depth == -1 {
		fmt.Fprintf(sb, "You escaped from Dayoriah Clan’s domain alive!\n")
	} else if g.Player.HP <= 0 {
		fmt.Fprintf(sb, "You died while exploring depth %d of Dayoriah Clan’s domain.\n", g.Map.Depth)
	} else {
		fmt.Fprintf(sb, "You are exploring depth %d of Dayoriah Clan’s domain.\n", g.Map.Depth)
	}
	if g.LiberatedShaedra {
		fmt.Fprint(sb, "You rescued Shaedra.\n")
	} else {
		fmt.Fprint(sb, "You did not rescue Shaedra.\n")
	}
	if g.LiberatedArtifact {
		fmt.Fprint(sb, "You recovered the Gem Portal Artifact.\n")
	} else {
		fmt.Fprint(sb, "You did not recover the Gem Portal Artifact.\n")
	}
	fmt.Fprintf(sb, "You spent %d turns in Hareka’s Underground.\n", g.Turn)
	fmt.Fprintf(sb, "\n")
	fmt.Fprintf(sb, "You have %d/%d HP, %d/%d MP and %d/%d bananas.\n", g.Player.HP, g.Player.HPMax(), g.Player.MP, g.Player.MPMax(), g.Player.Bananas, MaxBananas)
	fmt.Fprintf(sb, "\n")
	fmt.Fprint(sb, g.DumpStatuses())
	fmt.Fprintf(sb, "\n")
	fmt.Fprintf(sb, "\nMagaras:\n")
	for _, mag := range g.Player.Magaras {
		if mag.Kind != NoMagara {
			fmt.Fprintf(sb, "- %s (charges: %d)\n", mag, mag.Charges)
		}
	}
	fmt.Fprintf(sb, "\nInventory:\n")
	if g.Player.Inventory.Body.Kind != NoItem {
		fmt.Fprintf(sb, "- %s (body)\n", g.Player.Inventory.Body.ShortDesc(g))
	}
	if g.Player.Inventory.Neck.Kind != NoItem {
		fmt.Fprintf(sb, "- %s (neck)\n", g.Player.Inventory.Neck.ShortDesc(g))
	}
	fmt.Fprintf(sb, "\nLast messages:\n")
	for i := len(g.Logs) - 10; i < len(g.Logs); i++ {
		if i >= 0 {
			fmt.Fprintf(sb, "%s\n", g.Logs[i].dumpString())
		}
	}
	fmt.Fprintf(sb, "\nDungeon:\n")
	fmt.Fprintf(sb, "┌%s┐\n", strings.Repeat("─", MapWidth))
	sb.WriteString(g.DumpDungeon())
	fmt.Fprintf(sb, "└%s┘\n", strings.Repeat("─", MapWidth))
	if g.Stats.Killed > 0 {
		fmt.Fprintf(sb, "\n")
		fmt.Fprint(sb, g.DumpedKilledMonsters())
	}
	if g.Stats.MagarasUsed > 0 {
		fmt.Fprintf(sb, "\nMagara uses:\n")
		magaraUses := []string{}
		for mag, n := range g.Stats.UsedMagaras {
			if mag != NoMagara {
				magaraUses = append(magaraUses, fmt.Sprintf("- %s (used %d %s)\n", mag, n, times(n)))
			}
		}
		sort.Strings(magaraUses)
		for _, s := range magaraUses {
			sb.WriteString(s)
		}
	}
	g.DetailedStatistics(sb)
	fmt.Fprintf(sb, "\nTimeline:\n")
	fmt.Fprint(sb, g.DumpStory())
	return sb.String()
}

func (g *Game) DetailedStatistics(sb *strings.Builder) {
	fmt.Fprintf(sb, "\nStatistics:\n")
	fmt.Fprintf(sb, "You evoked magaras %d %s (%d oric, %d harmonic, %d others).\n",
		g.Stats.MagarasUsed, times(g.Stats.MagarasUsed), g.Stats.OricMagUse,
		g.Stats.HarmonicMagUse, g.Stats.MagarasUsed-g.Stats.OricMagUse-g.Stats.HarmonicMagUse)
	fmt.Fprintf(sb, "You activated %s.\n", CountNoun("magical stone", g.Stats.UsedStones))
	fmt.Fprintf(sb, "You rested %d %s.\n", g.Stats.Rest, times(g.Stats.Rest))
	fmt.Fprintf(sb, "You drank %s.\n", CountNoun("potion", g.Stats.DrankPotions))
	fmt.Fprintf(sb, "You were spotted by %s, %d %s.\n", CountNoun("monster", g.Stats.NUSpotted), g.Stats.NSpotted, times(g.Stats.NSpotted))
	fmt.Fprintf(sb, "You endured %d damage.\n", g.Stats.Damage)
	if n := g.Stats.Statuses[StatusConfusion]; n > 0 {
		fmt.Fprintf(sb, "You were confused %d %s.\n", n, times(n))
	}
	if n := g.Stats.Statuses[StatusLignification]; n > 0 {
		fmt.Fprintf(sb, "You were lignified %d %s.\n", n, times(n))
	}
	if n := g.Stats.Statuses[StatusIlluminated]; n > 0 {
		fmt.Fprintf(sb, "You were illuminated by an harmonic celmist %d %s.\n", n, times(n))
	}
	if n := g.Stats.TimesBlocked; n > 0 {
		fmt.Fprintf(sb, "You were blocked by an oric celmist barrier %d %s.\n", n, times(n))
	}
	if n := g.Stats.TimesPushed; n > 0 {
		fmt.Fprintf(sb, "You were pushed %d %s by monsters.\n", n, times(n))
	}
	if n := g.Stats.TimesBlinked; n > 0 {
		fmt.Fprintf(sb, "You were blinked %d %s by blinking frogs.\n", n, times(n))
	}
	if g.Stats.StolenBananas > 0 {
		fmt.Fprintf(sb, "You were stolen %d bananas by harpies.\n", g.Stats.StolenBananas)
	}
	fmt.Fprintf(sb, "You jumped %d %s over monsters.\n", g.Stats.Jumps, times(g.Stats.Jumps))
	fmt.Fprintf(sb, "You jumped %d %s by propelling yourself against walls.\n", g.Stats.WallJumps, times(g.Stats.WallJumps))
	fmt.Fprintf(sb, "You hid in %s.\n", CountNoun("barrel", g.Stats.BarrelHides))
	fmt.Fprintf(sb, "You crawled through %s.\n", CountNoun("holed wall", g.Stats.HoledWallsCrawled))
	fmt.Fprintf(sb, "You climbed %s.\n", CountNoun("tree", g.Stats.ClimbedTree))
	fmt.Fprintf(sb, "You hid under %s.\n", CountNoun("table", g.Stats.TableHides))
	fmt.Fprintf(sb, "You opened %s.\n", CountNoun("door", g.Stats.DoorsOpened))
	fmt.Fprintf(sb, "You moved %d %s.\n", g.Stats.Moves, times(g.Stats.Moves))
	fmt.Fprintf(sb, "You waited %d %s.\n", g.Stats.Waits, times(g.Stats.Waits))
	fmt.Fprintf(sb, "You extinguished %s.\n", CountNoun("campfire", g.Stats.Extinguishments))
	fmt.Fprintf(sb, "You read %d lore messages out of %d.\n", len(g.Stats.Lore), len(g.Params.Lore))
	if g.Stats.Burns > 0 {
		fmt.Fprintf(sb, "There were %s.\n", CountNoun("fire", g.Stats.Burns))
	}
	if g.Stats.Digs > 0 {
		fmt.Fprintf(sb, "There were %s.\n", CountNoun("destroyed walls", g.Stats.Digs))
	}
	if g.Stats.NadreExplosions > 0 {
		if g.Stats.NadreExplosions > 1 {
			fmt.Fprintf(sb, "There were %d nadre explosions.\n", g.Stats.NadreExplosions)
		} else {
			sb.WriteString("There was 1 nadre explosion.\n")
		}
	}
	if g.Stats.Killed > 0 {
		fmt.Fprintf(sb, "You murdered %s.\n", CountNoun("monster", g.Stats.Killed))
	}
	fmt.Fprintf(sb, "You spent %d%% turns wounded.\n", g.Stats.TWounded*100/(g.Stats.Turns+1))
	fmt.Fprintf(sb, "You spent %d%% turns with monsters in sight.\n", g.Stats.TMonsFOV*100/(g.Stats.Turns+1))
	fmt.Fprintf(sb, "You spent %d%% turns wounded with monsters in sight.\n", g.Stats.TMWounded*100/(g.Stats.Turns+1))
	maxDepth := 1 + max(g.Map.Depth-1, g.ExploredLevels)
	if g.Player.HP <= 0 {
		maxDepth++
	}
	if maxDepth >= MaxDepth+1 {
		// should not happen
		maxDepth = 1
	}
	fmt.Fprintf(sb, "\n")
	hfmt := "%-21s"
	perLevel := func(title string, x []int) {
		fmt.Fprintf(sb, hfmt, title)
		for _, n := range x[min(len(x), 1):] {
			fmt.Fprintf(sb, " %4d", n)
		}
		sb.WriteByte('\n')
	}
	fmt.Fprintf(sb, hfmt, "Quantity/Depth")
	for i := 1; i < maxDepth; i++ {
		fmt.Fprintf(sb, " %4d", i)
	}
	sb.WriteByte('\n')
	perLevel("Turns", g.Stats.DTurns[:maxDepth])
	perLevel("Explored (%)", g.Stats.DExplPerc[:maxDepth])
	perLevel("Sleeping monsters (%)", g.Stats.DSleepingPerc[:maxDepth])
	perLevel("Alerted monsters (%)", g.Stats.DUSpottedPerc[:maxDepth])
	perLevel("Total alerts", g.Stats.DSpotted[:maxDepth])
	perLevel("Rests", g.Stats.DRests[:maxDepth])
	perLevel("Received damage", g.Stats.DDamage[:maxDepth])
	perLevel("Magara uses", g.Stats.DMagaraUses[:maxDepth])
	fmt.Fprintf(sb, "\nAchievements:\n")
	achvs := []string{}
	for achv := range g.Stats.Achievements {
		achvs = append(achvs, string(achv))
	}
	sort.Strings(achvs)
	for _, achv := range achvs {
		fmt.Fprintf(sb, "- %s (turn %d)\n", achv, g.Stats.Achievements[Achievement(achv)])
	}
}

func (g *Game) DumpStory() string {
	return strings.Join(g.Stats.Story, "\n")
}

func (g *Game) DumpDungeon() string {
	buf := bytes.Buffer{}
	it := g.Map.KnownTerrain.Iterator()
	i := 0
	for it.Next() {
		p := it.P()
		kt := it.Cell()
		if i%MapWidth == 0 {
			if i == 0 {
				buf.WriteRune('│')
			} else {
				buf.WriteString("│\n│")
			}
		}
		if !IsKnown(kt) {
			buf.WriteRune(' ')
			if i == MapSize-1 {
				buf.WriteString("│\n")
			}
			i++
			continue
		}
		var r rune
		// XXX this can be simplified
		switch kt {
		case WallCell:
			r = '#'
		default:
			switch p {
			case g.Player.P:
				r = '@'
			default:
				r, _ = g.TerrainStyle(kt, p)
				if _, ok := g.Map.Clouds[p]; ok && g.Player.AFOV[p] {
					r = '≡'
				}
				m := g.MonsterAt(p)
				if m.Exists() && g.Player.AFOV[m.P] {
					r = m.Kind.Letter()
				}
			}
		}
		buf.WriteRune(r)
		if i == MapSize-1 {
			buf.WriteString("│\n")
		}
		i++
	}
	return buf.String()
}

func (g *Game) DumpedKilledMonsters() string {
	buf := &bytes.Buffer{}
	fmt.Fprint(buf, "Killed Monsters:\n")
	ms := g.SortedKilledMonsters()
	for _, mk := range ms {
		fmt.Fprintf(buf, "- %s: %d\n", mk, g.Stats.KilledMons[mk])
	}
	return buf.String()
}

func (g *Game) SimplifedDump(msg string, err error) string {
	buf := &strings.Builder{}
	fmt.Fprintf(buf, " ♣ Game Summary — Harmonist %s ♣\n\n", Version)
	if g.Wizard != WizardNone {
		fmt.Fprintf(buf, "ENABLED WIZARD MODE.\n")
	}
	if g.Player.HP > 0 && g.Map.Depth == -1 {
		fmt.Fprintf(buf, "You escaped from Dayoriah Clan’s domain alive!\n")
	} else if g.Player.HP <= 0 {
		fmt.Fprintf(buf, "You died while exploring depth %d of Dayoriah Clan’s domain.\n", g.Map.Depth)
	} else {
		fmt.Fprintf(buf, "You are exploring depth %d of Dayoriah Clan’s domain.\n", g.Map.Depth)
	}
	if g.LiberatedShaedra {
		fmt.Fprint(buf, "You rescued Shaedra.\n")
	} else {
		fmt.Fprint(buf, "You did not rescue Shaedra.\n")
	}
	if g.LiberatedArtifact {
		fmt.Fprint(buf, "You recovered the Gem Portal Artifact.\n")
	} else {
		fmt.Fprint(buf, "You did not recover the Gem Portal Artifact.\n")
	}
	fmt.Fprintf(buf, "You spent %d turns in Hareka’s Underground.\n", g.Turn)
	fmt.Fprintf(buf, "\n")
	if err != nil {
		fmt.Fprintf(buf, "Error writing dump: %v.\n", err)
		log.Printf("error writing dump: %v", err)
	} else {
		fmt.Fprintln(buf, msg)
	}
	return buf.String()
}

func times(n int) string {
	if n == 1 {
		return "time"
	}
	return "times"
}
