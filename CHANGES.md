# v? ?

This release polishes various gameplay aspects and brings many UI improvements,
most of them inspired from Shamogu.

Improved and reworked content:

* Magical stones have been reworked to have a strategical aspect in addition to
  the tactical one, like menhirs in Shamogu. The mapping stone is gone, the
  fog stone was replaced by a new earth stone, which destroys nearby walls
  (producing dust) and reveals interior walls, and the queenstone has been
  merged with the sensing stone. The other stones are still the same but now
  provide some kind of partial information about the map, in addition to their
  usual tactical effect (revealing barrels for barrel stone, lights for night
  stone, magaras for teleport stone, stairs for seal stone, trees and partially
  foliage for tree stone). To compensate for the extra strategic buff, there
  are somewhat less stones on the maps than before.
* The tile where a magical stone is present is now always lighted, due to
  magical stones producing a very small light. This change mostly intends to
  make stone coloring independent from lighting and not require the player to
  manually examine the stone to see if the tile is lighted. It's a quite
  thematic and minor change that may help further make magical stones more
  interesting and different from other kinds of items.
* Monsters now always lose a turn noticing the player. Previously, some actions
  (like throwing a javelin) were disabled on the turn a monster noticed the
  player, but the monster would still move or even attack you (if adjacent).
  That wasn't very intuitive and could lead to instant surprise-attacks behind
  doors. To compensate for this buff, the player now has 1 less maximum HP and
  MP, though you get +1 MP after liberating Shaedra when returning Marevor's
  artifact in exchange for a new special magara.
* Monster noise visual marking now uses different colors for different kinds of
  noises, like in Shamogu. Also, it has been adjusted so that different kinds
  of noises are heard at varied ranges. Also, cloak of hearing should now be
  more interesting, as it helps hear in-place footsteps of watching monsters
  (except satowalga plants, which make no noise at all), and even resting
  monsters at a short range.
* The “cloak of magic” has been removed, as it was a bit redundant with respect
  to the “cloak of conversion”. The latter is a bit more interesting and less
  quantitative.
* Mirror specter monsters now absorb the player's mana at a somewhat slower
  rate than before.

UI, graphics, QoL:

* Update to latest Gruid version and require Go 1.24 or later, bringing various
  improvements (including SDL2 version improvements, see further below).
* Simplified controls taking inspiration from Shamogu. In particular, there's
  no longer a separate *targeting mode help*. Non-movement targeting keys now
  work in normal mode, too, without any conflicts.
* New help menu providing help about various topics: basics of core stealth
  mechanics, items, statuses, and various tips.
* Various small improvements in magara and item descriptions.
* Change color of out-of-view monster memory to light grey unless the monster
  was resting, to make it clearer that it's out of view (inspired from a
  similar change in Shamogu).
* Some tile drawings have been improved, including some improvements from
  Shamogu, like a more visible `?` sign on the exploration frontier.
* Many other small UI improvements and fixes inspired from Shamogu, including
  status bar and menu improvements, as well as updated/fixed titles in messages
  and end of game summary pager windows.
* ASCII version (gruid-tcell): new `-t` option for terminals supporting “true
  color” to use the same selenized color palette as the tiles version.
* SDL2 version (gruid-sdl): it now enables graphic acceleration and uses new
  custom bindings for SDL2 that are much smaller and compile much faster. There
  are also now new command-line `-h` and `-w` options for adjusting the
  scaling, and fullscreen `-F` should work better.
* In native builds, rename game statistic's dump file to `dump.txt`
  so that it's clearer that the file is text, unlike the other ones (save and
  replay are stored compactly as compressed binary files).

Fixes:

* Fix map drawing refresh issue when interrupting chained animations early.
* Fix possible cases of unreachable rubble after earthquake.
* Fix old regression that broke the fire magara.
* Prevent satowalga plants from ever being generated on top of a banana (that
  made swapping and teleport the only ways to get such banana, which was kind
  of fun, but it was a bit confusing).
* Minor fix in story sequence when liberating Shaedra: half of the story
  sequence animation could happen out of view if a nixe or some other monsters
  moved you away during the same turn you reached Shaedra. Now the story
  sequence only starts if you end up at the right place after the whole turn
  ends.

# v0.6.0 2025-03-05

* Fix v0.4 regression when falling in abyss after levitation.
* Make G enter examine mode and select nearest stairs. Previous behavior with G
  was buggy and not suited for Harmonist's stealth gameplay.
* Disallow swapping from a cramped place (like a table).

# v0.5.0 2023-06-16

* Add detailed descriptions in examine mode.
* Fix a bug in which critical HP warning could interrupt story sequence.
* Fix some descriptions and typos.
* Updated minimal version of some dependencies.

# v0.4.1 2021-03-05

This is a bugfix release mainly based on helpful feedback by players.

* Fixed the tiles SDL version on windows.
* Improvements in the SDL version: window title and icon.
* Remove an unfriendly error message when save is from a previous version and
  thus cannot be loaded.
* Mention zoom functionality for SDL version in key bindings help.
* Fix a bug in pathfinding on map's left and right edges.

# v0.4 2021-02-22

Highligths:

* A new field of view algorithm (symmetric shadow casting combined with the
  previous custom one) that should offer a better stealthy experience thanks to
  expanding shadows and less permissive vision in general.
* Overhaul of the user interface and a lot of polishing. Among most noticeable
  changes for players, description for items and places is now displayed just
  by examining them, without requiring an additional keypress, and using the
  mouse does not force examination mode for keyboard anymore.
* Improved Replay (arrow up and down moves 1 minute forward/backward).
* New SDL driver that replaces the Tk one as graphical native driver.
* Many minor bug fixes (including some minor gameplay tweaks).

Packaging notes: the game now uses the gruid library. This means that for the
terminal version it will use the tcell go library, and for graphical
applications the SDL library.

# v0.3 2019-12-10

Highligths:

* Improved monster AI: now they remember you!
* Limited number of charges for magaras: you'll have to adapt your playstyle.
* New core mechanic: jump by propulsing against walls!
* Improved map generation and special events (mist and earthquake)
* 2 new terrain features: fake stairs and queen rock that amplifies sound.
* 4 new monsters, a new cloak, 6 new magaras and 2 old ones reworked, and
  potions.

More detailed list:

* Improve monster AI: now they behave differently after they have spotted you
  at least once. Exact behavior depends on the type of monster, but they all
  have a chance of exploring around the last place they saw you at, in addition
  to their normal duties.
* Magaras now have a maximum number of charges, to avoid spamming of a single
  magara.
* You can now jump 2 or 3 tiles away by propulsing yourself against walls,
  which leaves you exhausted for a few turns, as the usual over monster jumps.
  A significant buff for acrobatics cloak!
* Greatly improve and diversify map generation, in particular the tunnel
  generation algorithm used for all kind of maps.
* New special events: mist level and earthquake.
* New special map layouts more or less urbanised than the common ones.
* Many new special rooms.
* New terrain feature: fake stairs (harmonic illusions).
* New terrain feature: queen rock, special terrain that amplifies sound, so
  that even so you're usually very silent, your footsteps on such rocks will be
  heard. One more use for levitation magara!
* New unique monster: Crazy Imp that sings and plays guitar, attracting
  unwanted attention.
* New monster: haze cat, with very good night vision.
* New monster: spiders, with panoramic vision and confusing bite.
* New monster: acid mounds, whose bite corrodes magara charges.
* Satowalga plants acidic projectiles may now corrode your magara charges,
  instead of slowing you.
* New cloak of conversion, that generates MP from lost HP.
* New magara of energy that replenishes HP and MP, but only has one charge.
* New magara of transparency that makes you transparent (visible only to
  adjacent monsters) when standing of a lighted cell.
* New magara of delayed noise: it will produce harmonic noise in your current
  position after a certain number of turns.
* New magara of disguise: it will disguise you with illusions into a guard, so
  that most monsters that are not already chasing you will be friendly (except
  some monsters with good flair).
* New special magara of dispersal that makes monster hitting you blink away.
* New special magara of delayed oric explosion that generates a big oric
  explosion that destroys walls in a wide area.
* Remove player and monster speed: a turn is now always 1, no half turns.
* Replace magara of slowing with magara of paralysis, a stronger but shorter
  variation on lignification (may change in the future).
* Rework magara of swiftness to give several “free” moves, which is more
  intuitive than previous behavior.
* New potion items: they have an on-drink effect when you move onto them. You
  cannot carry them.
* Some monsters now have resistances/counter against some effects: satowalga
  plants and tree mushrooms cannot be lignified, blinking frogs partially
  reflect teleportation effects (they teleport but you too), haze cats have
  shallow sleep (they wake up very fast after falling asleep).
* Improved magical stone placement, and chances of getting a particular kind of
  stone may depend on whether it's inside or outside a building.
* Display number of remaining turns for monsters statuses too.
* Various balance adjustments, like 1 more HP and MP, but less bananas,
  encouraging better strategic play.
* Fix/improve many animations.
* Fix bug which could lead to negative number of bananas when jumping into
  chasm.
* Fix rare bug when blinking to the same place.
* Make “x” key work in key settings menu too.
* Minor UI/messages/stats improvements and other miscellaneous bug fixes.

# v0.2 2019-07-22

* Many new special room shapes.
* Much improved story timeline in character game statistics.
* Add effect duration in magara description when possible.
* Improved fullscreen support with F11 in browser version.
* New tile for extinguished lights, and improved tile for doors.
* Handle window closing gracefully in Tk version.

# v0.1 2019-05-11

First release of Harmonist: Dayoriah Clan Infiltration.

Features:

* Short coffee-break runs (around half an hour).
* 8 levels deep dungeon (+ 3 optional levels), 3 distinct map generators, 19
  monsters, 6 magical cloaks, 6 magical amulets, and 16 magaras (evokable
  magical items).
* Both graphical tiles (web or Tk) and terminal ASCII versions. Animations.
  Simple controls.  Mouse support.  Mouse-friendly 100x26 layout, and
  traditional compact 80x24 layout.  Automatic recording for later replay.
* Light and noise stealth mechanics. Monster footsteps can be heared.
* A main story narrative, and many short lore texts.
* No XP, no upstairs, no automatic regeneration, no grinding/farming.
* Fully destructible terrain (wall destruction, foliage fire).
* Simplified inventory management (e.g. no more than one cloak or amulet).
* Many terrain features: dense foliage, doors, tables, barrels, windows, holes
  in the walls, trees, chasm, …
* 9 distinct magical stones that may be activated once with some special
  magical effect.
