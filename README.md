# Xbox Trainer Parser

Reads old Xbox trainers (`.etm` and `.xbtf`) and tells you what's in them: what game
they target, the cheats they have, the scroller text, all the structure. There's a
CLI version and a little desktop app (Trainer Viewer) that lists a folder of them with
box art.

## What's a trainer

If you modded an Xbox back in the day, trainers were the cheat packs you'd load over a
game. Infinite health, max money, that sort of thing. Under the hood it's mostly x86
code that pokes the running game, with a small header holding the game ID, the cheat
names, and a scroller line. `.etm` is the plain format, `.xbtf` is the same thing but
scrambled.

## What it does

Parses both formats, including unscrambling the `.xbtf` ones (that bit's lifted from
XBMC). Pulls the game ID, the cheat labels, the scroller, the offsets. The CLI prints a
boxed readout, and the desktop app does the same but pretty, with box art pulled from
MobCat's title list.

## Why it exists

Honestly, I built this as a preview tool and went way overboard making it look fancy. I
just wanted to browse a folder of trainers and have them render nice. Started as the
boxed terminal output, then turned into a whole Wails desktop app with icons. Total
side quest, and then I shelved it.

A few years later the bones turned out to matter. A trainer is basically a little map of
where the good stuff lives in a game's memory, money, lives, timers, the values cheats
flip. Which is exactly what you need to build achievements for these games. So I'm
pulling it back out to feed the Xbox Achievements thing I'm working on:

https://github.com/MrMilenko/xbox-achievements

Next step is ripping the actual addresses out of the trainer code itself. Then it stops
being just a viewer and becomes a real shortcut for authoring achievement sets.

## Running it

CLI:

```
go build
./xboxtrainerparser "Some Trainer (NTSC) +4.etm"
```

Desktop app (needs Wails and Node):

```
cd trainerui
wails build
```

## Credits

- MobCat for the format notes and the game ID / box art lookup (mobcat.zip/XboxIDs)
- the `.xbtf` unscramble is straight out of XBMC
- the original 2023 version is still in the git history if you want to see where it started
