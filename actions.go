package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	//"time"

	"codeberg.org/anaseto/gruid"
	"codeberg.org/anaseto/gruid/ui"
)

type action int

const (
	ActionNone action = iota
	ActionW
	ActionS
	ActionN
	ActionE
	ActionRunW
	ActionRunS
	ActionRunN
	ActionRunE
	ActionWaitTurn
	ActionDescend
	ActionTargetNearestStairs
	ActionExplore
	ActionExamine
	ActionEvoke
	ActionInteract
	ActionInventory
	ActionLogs
	ActionDump
	ActionHelp
	ActionSave
	ActionQuit
	ActionWizard
	ActionWizardMenu
	ActionWizardDescend

	ActionPreviousMonster
	ActionNextMonster
	ActionNextObject
	ActionTarget
	ActionEscape

	ActionSettings
	ActionMenu
	ActionNextStairs

	ActionSetKeys
	ActionInvertLOS
	ActionToggleTiles
	ActionToggleShowNumbers

	ActionWizardInfo
	ActionWizardToggleMode
)

var ConfigurableKeyActions = [...]action{
	ActionW,
	ActionS,
	ActionN,
	ActionE,
	ActionRunW,
	ActionRunS,
	ActionRunN,
	ActionRunE,
	ActionWaitTurn,
	ActionEvoke,
	ActionInteract,
	ActionInventory,
	ActionExamine,
	ActionTargetNearestStairs,
	ActionExplore,
	ActionLogs,
	ActionDump,
	ActionSave,
	ActionQuit,
	ActionMenu,
	ActionPreviousMonster,
	ActionNextMonster,
	ActionNextObject,
	ActionNextStairs,
	ActionTarget}

func (k action) normalModeAction() bool {
	switch k {
	case ActionW, ActionS, ActionN, ActionE,
		ActionRunW, ActionRunS, ActionRunN, ActionRunE,
		ActionWaitTurn,
		ActionDescend,
		ActionTargetNearestStairs,
		ActionExplore,
		ActionExamine,
		ActionEvoke,
		ActionInteract,
		ActionInventory,
		ActionLogs,
		ActionDump,
		ActionHelp,
		ActionMenu,
		ActionSave,
		ActionQuit,
		ActionSettings,
		ActionWizard,
		ActionWizardMenu,
		ActionWizardInfo,
		ActionWizardToggleMode:
		return true
	default:
		return false
	}
}

func (k action) String() (text string) {
	switch k {
	case ActionW:
		text = "Move west"
	case ActionS:
		text = "Move south"
	case ActionN:
		text = "Move north"
	case ActionE:
		text = "Move east"
	case ActionRunW:
		text = "Travel west"
	case ActionRunS:
		text = "Travel south"
	case ActionRunN:
		text = "Travel north"
	case ActionRunE:
		text = "Travel east"
	case ActionWaitTurn:
		text = "Wait a turn"
	case ActionDescend:
		text = "Descend stairs"
	case ActionTargetNearestStairs:
		text = "Target nearest stairs"
	case ActionExplore:
		text = "Autoexplore"
	case ActionExamine:
		text = "Examine"
	case ActionEvoke:
		text = "Evoke card"
	case ActionInteract:
		text = "Interact"
	case ActionInventory:
		text = "Inventory"
	case ActionLogs:
		text = "View messages"
	case ActionDump:
		text = "Dump game statistics"
	case ActionSave:
		text = "Save and quit"
	case ActionQuit:
		text = "Quit (without saving)"
	case ActionHelp:
		text = "Help (default keybindings)"
	case ActionSettings:
		text = "Configure settings"
	case ActionWizard:
		text = "Wizard (debug) mode"
	case ActionWizardMenu:
		text = "Wizard (debug) mode information"
	case ActionMenu:
		text = "Action Menu"
	case ActionSetKeys:
		text = "Change key bindings"
	case ActionInvertLOS:
		text = "Toggle dark/light theme"
	case ActionToggleTiles:
		text = "Toggle tiles/ASCII display"
	case ActionToggleShowNumbers:
		text = "Toggle hearts/numbers"
	case ActionWizardInfo:
		text = "Info"
	case ActionWizardToggleMode:
		text = "toggle normal/map/all wizard mode"
	}
	return text
}

func (k action) targetingModeDescription() (text string) {
	switch k {
	case ActionW:
		text = "Move cursor west"
	case ActionS:
		text = "Move cursor south"
	case ActionN:
		text = "Move cursor north"
	case ActionE:
		text = "Move cursor east"
	case ActionRunW:
		text = "Big move cursor west"
	case ActionRunS:
		text = "Big move cursor south"
	case ActionRunN:
		text = "Big move north"
	case ActionRunE:
		text = "Big move east"
	case ActionDescend:
		text = "Target next stair"
	case ActionPreviousMonster:
		text = "Target previous monster"
	case ActionNextMonster:
		text = "Target next monster"
	case ActionNextObject:
		text = "Target next object"
	case ActionNextStairs:
		text = "Target next stairs"
	case ActionTarget:
		text = "Go to"
	case ActionEscape:
		text = "Quit targeting mode"
	}
	return text
}

func (k action) targetingModeAction() bool {
	switch k {
	case ActionW, ActionS, ActionN, ActionE,
		ActionRunW, ActionRunS, ActionRunN, ActionRunE,
		ActionPreviousMonster,
		ActionNextMonster,
		ActionNextObject,
		ActionNextStairs,
		ActionTarget,
		ActionEscape:
		return true
	default:
		return false
	}
}

func (md *model) interact() (string, bool) {
	g := md.g
	c := g.Dungeon.Cell(g.Player.P)
	switch terrain(c) {
	case StairCell:
		if terrain(g.Dungeon.Cell(g.Player.P)) == StairCell && g.Objects.Stairs[g.Player.P] != BlockedStair ||
			terrain(g.Dungeon.Cell(g.Player.P)) == StairCell && g.Objects.Stairs[g.Player.P] == BlockedStair {
			return "descend", true
		}
		return "", false
	case BarrelCell:
		return "rest", true
	case MagaraCell:
		return "equip magara", true
	case StoneCell:
		return "activate stone", true
	case ScrollCell:
		return "read scroll", true
	case ItemCell:
		return "equip item", true
	case LightCell:
		return "extinguish light", true
	case StoryCell:
		if g.Objects.Story[g.Player.P] == StoryArtifact && !g.LiberatedArtifact ||
			g.Objects.Story[g.Player.P] == StoryArtifactSealed {
			return "take artifact", true
		}
		return "", false
	}
	return "", false
}

func (md *model) normalModeAction(action action) (again bool, eff gruid.Effect, err error) {
	g := md.g
	switch action {
	case ActionNone:
		again = true
		err = actionErrorUnknown
	case ActionW, ActionS, ActionN, ActionE:
		if !md.targ.kbTargeting {
			again, err = g.PlayerBump(g.Player.P.Add(keyToDir(action)))
		} else {
			p := md.targ.ex.p.Add(keyToDir(action))
			if valid(p) {
				md.Examine(p)
			}
			again = true
		}
	case ActionRunW, ActionRunS, ActionRunN, ActionRunE:
		if !md.targ.kbTargeting {
			again, err = g.GoToDir(keyToDir(action))
		} else {
			q := invalidPos
			p := md.targ.ex.p
			for range 5 {
				p = p.Add(keyToDir(action))
				if !valid(p) {
					break
				}
				q = p
			}
			if q != invalidPos {
				md.Examine(q)
			}
			again = true
		}
	case ActionPreviousMonster:
		again = true
		md.nextMonster("-", md.targ.ex)
	case ActionNextMonster:
		again = true
		md.nextMonster("+", md.targ.ex)
	case ActionNextObject:
		again = true
		md.nextObject(md.targ.ex.p, md.targ.ex)
	case ActionNextStairs:
		again = true
		md.nextStair(md.targ.ex)
	case ActionTarget:
		again = true
		err = md.target()
		if err != nil {
			break
		}
		if g.MoveToTarget() {
			again = false
		}
	case ActionEscape:
		again = true
		md.CancelExamine()
	case ActionWaitTurn:
		g.WaitTurn()
	case ActionTargetNearestStairs:
		again = true
		stairs := g.StairsSlice()
		sortedStairs := g.SortedNearestTo(stairs, g.Player.P)
		if len(sortedStairs) > 0 {
			stair := sortedStairs[0]
			if g.Player.P == stair {
				err = errors.New("You are already on the stairs.")
				break
			}
			if valid(stair) {
				md.targ.kbTargeting = true
				md.Examine(stair)
			}
		} else {
			err = errors.New("You cannot target any stairs.")
		}
	case ActionInteract:
		c := g.Dungeon.Cell(g.Player.P)
		switch terrain(c) {
		case StairCell:
			if terrain(g.Dungeon.Cell(g.Player.P)) == StairCell && g.Objects.Stairs[g.Player.P] != BlockedStair {
				strt := g.Objects.Stairs[g.Player.P]
				err = md.checkShaedra(strt)
				if err != nil {
					break
				}
				again = true
				if g.Descend(DescendNormal) {
					md.win()
				}
			} else if terrain(g.Dungeon.Cell(g.Player.P)) == StairCell && g.Objects.Stairs[g.Player.P] == BlockedStair {
				err = errors.New("The stairs are blocked by a magical stone barrier energies.")
			} else {
				err = errors.New("No stairs here.")
			}
		case BarrelCell:
			err = g.Rest()
		case MagaraCell:
			again = true
			md.equipMagaraMenu()
		case StoneCell:
			err = g.ActivateStone()
		case ScrollCell:
			again = true
			md.readScroll()
		case ItemCell:
			err = md.g.EquipItem()
		case LightCell:
			err = g.ExtinguishFire()
		case StoryCell:
			if g.Objects.Story[g.Player.P] == StoryArtifact && !g.LiberatedArtifact {
				g.PushEventFirst(&playerEvent{Action: StorySequence}, g.Turn)
				g.LiberatedArtifact = true
			} else if g.Objects.Story[g.Player.P] == StoryArtifactSealed {
				err = errors.New("The artifact is protected by a magical stone barrier.")
			} else {
				err = errors.New("You cannot interact with anything here.")
			}
		default:
			err = errors.New("You cannot interact with anything here.")
		}
	case ActionEvoke:
		again = true
		md.evokeMagaraMenu()
	case ActionInventory:
		again = true
		md.openIventory()
	case ActionExplore:
		again, err = g.Autoexplore()
	case ActionExamine:
		again = true
		md.KeyboardExamine()
	case ActionHelp:
		again = true
		md.KeysHelp()
	case ActionMenu:
		again = true
		md.openMenu()
	case ActionLogs:
		again = true
		if len(md.logs) > 0 {
			md.logs = md.logs[:len(md.logs)-1]
		}
		for _, e := range g.Log[len(md.logs):] {
			md.logs = append(md.logs, md.pagerMarkup.WithText(e.MText))
		}
		md.pager.SetLines(md.logs)
		md.pager.SetCursor(gruid.Point{0, len(md.logs)})
		md.pager.SetBox(&ui.Box{Title: ui.Text("Messages").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
		md.mode = modePager
		md.pagerMode = modeLogs
	case ActionSave:
		again = true
		g.checks()
		errsave := g.Save()
		if errsave != nil {
			g.PrintfStyled("Error: %v", logError, errsave)
			g.PrintStyled("Could not save state.", logError)
			md.mode = modeNormal
		} else {
			md.mode = modeQuit
			eff = gruid.End()
		}
	case ActionDump:
		again = true
		errdump := g.WriteDump()
		if errdump != nil {
			g.PrintfStyled("Error: %v", logError, errdump)
		} else {
			dataDir, _ := DataDir()
			if dataDir != "" {
				g.Printf("Game statistics written to %s.", filepath.Join(dataDir, "dump.txt"))
			} else {
				g.Print("Game statistics written.")
			}
		}
		md.mode = modeNormal
	case ActionWizardMenu:
		if g.Wizard {
			again = true
			md.openWizardMenu()
		} else {
			err = actionErrorUnknown
		}
	case ActionWizardDescend:
		if g.Wizard && g.Depth == WinDepth && !g.LiberatedShaedra {
			g.RescuedShaedra()
		}
		if g.Wizard && g.Depth < MaxDepth {
			g.StoryPrint("Descended wizardly")
			again = true
			if g.Descend(DescendNormal) {
				md.win()
			}
		} else {
			err = actionErrorUnknown
		}
	case ActionWizard:
		again = true
		if !md.g.Wizard {
			md.g.PrintStyled("Do you really want to enter wizard mode (irreversible)? [y/N]", logConfirm)
			md.mode = modeWizardConfirmation
		}
	case ActionQuit:
		again = true
		md.Quit()
	case ActionSettings:
		again = true
		md.openSettings()
	case ActionSetKeys:
		again = true
		md.openKeyBindings()
	case ActionInvertLOS:
		again = true
		GameConfig.DarkLOS = !GameConfig.DarkLOS
		err := SaveConfig()
		if err != nil {
			g.Print(err.Error())
		}
		clearCache()
		eff = gruid.Cmd(func() gruid.Msg { return gruid.MsgScreen{} })
		md.mode = modeNormal
	case ActionToggleTiles:
		again = true
		md.ApplyToggleTiles()
		eff = gruid.Cmd(func() gruid.Msg { return gruid.MsgScreen{} })
		err := SaveConfig()
		if err != nil {
			g.Print(err.Error())
		}
		md.mode = modeNormal
	case ActionToggleShowNumbers:
		again = true
		GameConfig.ShowNumbers = !GameConfig.ShowNumbers
		err := SaveConfig()
		if err != nil {
			g.Print(err.Error())
		}
		md.updateStatusInfo()
		md.mode = modeNormal
	case ActionWizardInfo:
		again = true
		md.wizardInfo()
	case ActionWizardToggleMode:
		switch g.WizardMode {
		case WizardNormal:
			g.WizardMode = WizardMap
		case WizardMap:
			g.WizardMode = WizardSeeAll
		case WizardSeeAll:
			g.WizardMode = WizardNormal
		}
		md.mode = modeNormal
	default:
		err = actionErrorUnknown
	}
	if err != nil {
		again = true
	}
	return again, eff, err
}

func (md *model) wizardInfo() {
	md.mode = modeSmallPager
	st := gruid.Style{}
	md.smallPager.SetBox(&ui.Box{Title: ui.Text("Wizard Info").WithStyle(st.WithFg(ColorCyan))})
	stts := []ui.StyledText{}
	stts = append(stts, ui.Textf("Monsters: %d \n", len(md.g.Monsters)))
	md.smallPager.SetLines(stts)
}

func (md *model) readScroll() {
	sc, ok := md.g.Objects.Scrolls[md.g.Player.P]
	if !ok {
		md.g.PrintStyled("Error while reading message.", logError)
		return
	}
	md.g.Print("You read the message.")
	md.mode = modeSmallPager
	st := gruid.Style{}
	switch sc {
	case ScrollLore:
		md.smallPager.SetBox(&ui.Box{Title: ui.Text("Lore Message").WithStyle(st.WithFg(ColorCyan))})
		stts := []ui.StyledText{}
		text := ui.Text(sc.Text(md.g)).Format(56)
		for _, s := range strings.Split(text.Text(), "\n") {
			stts = append(stts, ui.Text(s))
		}
		md.smallPager.SetLines(stts)
		md.smallPager.SetCursor(gruid.Point{0, 0})
		if !md.g.Stats.Lore[md.g.Depth] {
			md.g.StoryPrint("Read lore message")
		}
		md.g.Stats.Lore[md.g.Depth] = true
		if len(md.g.Stats.Lore) == 4 {
			AchLoreStudent.Get(md.g)
		}
		if len(md.g.Stats.Lore) == len(md.g.Params.Lore) {
			AchLoremaster.Get(md.g)
		}
	default:
		md.smallPager.SetBox(&ui.Box{Title: ui.Text("Story Message").WithStyle(st.WithFg(ColorCyan))})
		stts := []ui.StyledText{}
		text := ui.Text(sc.Text(md.g)).Format(56)
		for _, s := range strings.Split(text.Text(), "\n") {
			stts = append(stts, ui.Text(s))
		}
		md.smallPager.SetLines(stts)
		md.smallPager.SetCursor(gruid.Point{0, 0})
	}
}

type actionError int

const (
	actionErrorUnknown actionError = iota
)

func (e actionError) Error() string {
	switch e {
	case actionErrorUnknown:
		return "unknown action"
	}
	return ""
}

func altBgEntries(entries []ui.MenuEntry) {
	for i := range entries {
		if i%2 == 1 {
			st := entries[i].Text.Style()
			entries[i].Text = entries[i].Text.WithStyle(st.WithBg(ColorBackgroundSecondary))
		}
	}
}

var settingsActions = []action{
	ActionSetKeys,
	ActionInvertLOS,
	ActionToggleShowNumbers,
}

func (md *model) openSettings() {
	entries := []ui.MenuEntry{}
	r := 'a'
	for _, it := range settingsActions {
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s", r, it),
			Keys: []gruid.Key{gruid.Key(r)},
		})
		r++
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Settings").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeSettings
}

func (md *model) keysForAction(a action) string {
	keys := []gruid.Key{}
	for k, action := range md.keysNormal {
		if a == action && !k.In(keys) {
			keys = append(keys, k)
		}
	}
	for k, action := range md.keysTarget {
		if a == action && !k.In(keys) {
			keys = append(keys, k)
		}
	}
	ks := make([]string, len(keys))
	for i := range ks {
		ks[i] = string(keys[i])
	}
	sort.Strings(ks)
	return strings.Join(ks, " or ")
}

func (md *model) openKeyBindings() {
	entries := []ui.MenuEntry{}
	r := 'a'
	for _, it := range ConfigurableKeyActions {
		desc := it.String()
		if !it.normalModeAction() {
			desc = it.targetingModeDescription()
		}
		desc = fmt.Sprintf(" %-36s %s", desc, md.keysForAction(it))
		entries = append(entries, ui.MenuEntry{
			Text: ui.Text(desc),
		})
		r++
	}
	altBgEntries(entries)
	md.keysMenu.SetBox(&ui.Box{Title: ui.Text("Key Bindings").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.keysMenu.SetEntries(entries)
	md.keysMenu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeKeys
}

var menuActions = []action{
	ActionLogs,
	ActionHelp,
	ActionDump,
	ActionSettings,
	ActionSave,
	ActionQuit,
}

var menuKeys = []rune{'m', '?', '#', 'C', 'S', 'Q'}

func (md *model) openMenu() {
	hstyle := gruid.Style{}.WithFg(ColorCyan)
	entries := []ui.MenuEntry{}
	for i, it := range menuActions {
		r := menuKeys[i]
		switch r {
		case 'm':
			entries = append(entries, ui.MenuEntry{
				Text:     ui.Text("Gameplay Info").WithStyle(hstyle),
				Disabled: true,
			})
		case 'C':
			entries = append(entries, ui.MenuEntry{
				Text:     ui.Text("Other Actions").WithStyle(hstyle),
				Disabled: true,
			})
		}
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s", r, it),
			Keys: []gruid.Key{gruid.Key(r)},
		})
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Menu").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeGameMenu
}

func (md *model) openIventory() {
	entries := []ui.MenuEntry{}
	items := []item{md.g.Player.Inventory.Body, md.g.Player.Inventory.Neck, md.g.Player.Inventory.Misc}
	parts := []string{"body", "neck", "backpack"}
	r := 'a'
	for i, it := range items {
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s (%s)", r, it.ShortDesc(md.g), parts[i]),
			Keys: []gruid.Key{gruid.Key(r)},
		})
		r++
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Inventory").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeInventory
	it := items[0]
	md.description.Content = ui.Text(it.Desc(md.g)).Format(UIWidth/2 - 1 - 2)
	md.description.Box = &ui.Box{Title: ui.Text(it.String())}
}

func (md *model) evokeMagaraMenu() {
	entries := []ui.MenuEntry{}
	items := md.g.Player.Magaras
	r := 'a'
	for _, it := range items {
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s ", r, it.ShortDesc()),
			Keys: []gruid.Key{gruid.Key(r)},
		})
		r++
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Evoke Magara").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeEvocation
	it := items[0]
	md.description.Content = ui.Text(it.Desc(md.g)).Format(UIWidth/2 - 1 - 2)
	md.description.Box = &ui.Box{Title: ui.Text(it.String())}
}

func (md *model) equipMagaraMenu() {
	entries := []ui.MenuEntry{}
	items := md.g.Player.Magaras
	r := 'a'
	for _, it := range items {
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s ", r, it.ShortDesc()),
			Keys: []gruid.Key{gruid.Key(r)},
		})
		r++
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Replace which magara?").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeEquip
	it := items[0]
	md.description.Content = ui.Text(it.Desc(md.g)).Format(UIWidth/2 - 1 - 2)
	md.description.Box = &ui.Box{Title: ui.Textf("%s (equipped)", it.String())}
}

var wizardActions = []action{
	ActionWizardInfo,
	ActionWizardToggleMode,
}

func (md *model) openWizardMenu() {
	entries := []ui.MenuEntry{}
	r := 'a'
	for _, it := range wizardActions {
		entries = append(entries, ui.MenuEntry{
			Text: ui.Textf("%c - %s", r, it),
			Keys: []gruid.Key{gruid.Key(r)},
		})
		r++
	}
	altBgEntries(entries)
	md.menu.SetBox(&ui.Box{Title: ui.Text("Wizard Menu").WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	md.menu.SetEntries(entries)
	md.menu.SetActiveInvokable(0)
	md.mode = modeMenu
	md.menuMode = modeWizard
}

func (md *model) updateKeysDescription(title string, actions []string) {
	md.pagerMode = modeHelpKeys
	md.mode = modePager
	if CustomKeys {
		title = fmt.Sprintf(" Default %s ", title)
	} else {
		title = fmt.Sprintf(" %s ", title)
	}
	md.pager.SetBox(&ui.Box{Title: ui.Text(title).WithStyle(gruid.Style{}.WithFg(ColorYellow))})
	lines := []ui.StyledText{}
	for i := 0; i < len(actions)-1; i += 2 {
		stt := ui.StyledText{}
		if actions[i+1] != "" {
			stt = stt.WithTextf(" %-36s %s", actions[i], actions[i+1])
		} else {
			stt = stt.WithTextf(" %s ", actions[i]).WithStyle(gruid.Style{}.WithFg(ColorCyan))
		}
		if i%4 == 2 {
			stt = stt.WithStyle(stt.Style().WithBg(ColorBackgroundSecondary))
		}
		lines = append(lines, stt)
	}
	md.pager.SetLines(lines)
	md.pager.SetCursor(gruid.Point{0, 0})
}

func (md *model) KeysHelp() {
	entries := []string{
		"Basic Game Actions", "",
		"Move/Jump", "arrows or hjkl or mouse left",
		"Wait a turn", "“.” or 5 or enter or mouse left on @",
		"Interact (Equip/Descend/Rest...)", "e",
		"Evoke/Zap magara", "v or z",
		"Inventory", "i",
		"Toggle keyboard examine mode", "x",
		"Open menu", "SPACE or mouse right",
		"Close menu, inventory...", "SPACE or ESC or mouse left outside",
		"Advanced Game Actions", "",
		"View previous messages", "m",
		"Examine previous/next monster", "- +",
		"Examine next stair/object", "> n",
		"Run (auto-move in direction)", "shift+arrows or HJKL",
		"Travel (auto-move to destination)", "“.” or ENTER in keyboard examine mode",
		"Autoexplore (use with caution)", "o",
		"Other Common Actions", "",
		"Save and quit", "S",
		"Quit (without saving)", "Q",
	}
	md.updateKeysDescription("Keybindings (default values)", entries)
}

func (md *model) checkShaedra(st stair) (err error) {
	g := md.g
	if g.Depth == WinDepth && st == NormalStair && terrain(g.Dungeon.Cell(g.Places.Shaedra)) == StoryCell {
		err = errors.New("You have to rescue Shaedra first!")
	}
	return err

}

// Quit enters confirmation mode for quit without saving.
func (md *model) Quit() {
	md.g.PrintStyled("Do you really want to quit without saving? [y/N]", logConfirm)
	md.mode = modeQuitConfirmation
}
