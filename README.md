# Mixxx Mode for Push 3

DJ with [Mixxx](https://mixxx.org) on an Ableton Push 3 in standalone mode.

Switching to Mixxx mode stops Push3 and Live, starts Mixxx on the Push's own
audio outputs, and maps two decks to the pads, buttons and encoders. The Push
screen shows a 960x160 Mixxx skin. Hold SETTINGS for 1.5 seconds to go back to
Push mode.

Requires push-manager from [ableton-push-hack](https://github.com/federico-pepe/ableton-push-hack)
with `POST /api/display/direct` (it drives the screen while Push3 is stopped).

## Install

Install **Mixxx Mode** from Push Hack Catalog. The download is large because
Mixxx and the libraries it needs come with it. Nothing is installed outside
`/data/push-hack/hacks/mixxx/`. Updates keep your Mixxx library and settings
(`settings/` in that folder). **Remove** in the catalog deletes that whole folder,
including your library, playlists, cue points and audio settings. To keep
them, copy `settings/` somewhere else first. The music copied from USB sticks
(`/data/Music/DJ/USB/`) stays.

## Switching

Open **Mixxx Mode** from push-manager's web page and use **Switch to Mixxx** /
**Back to Push**.
On the Push itself, open the Shadow UI (Shift+Set), go to the **MIXXX** tab
and press **START** (first button under the screen). This needs a
push-manager that closes the Shadow UI when direct mode turns on.
On the Push, hold SETTINGS to go back.

## Controls

| Control | Left side (deck 1 or 3) | Right side (deck 2 or 4) |
|---|---|---|
| Play / cue | Play / Record | 1/4 / 1/4t |
| Master volume | Main, then turn the jog wheel | |
| Tempo of the master deck (press = original tempo) | Tempo encoder | Tempo encoder |
| Nudge (beatmatch) | Volume encoder (above tempo) | Jog wheel (decks shown) |
| Pull-up (spin the track back) | Shift + volume encoder | Shift + jog wheel |
| Volume | Pad column 4, bottom 6 pads (bottom = 0, top = full) | Pad column 5, bottom 6 pads |
| Key lock | Pad column 4, second pad from the top | Pad column 5, same |
| Headphone cue | Lock = deck 1, Stop Clips = deck 2 | Mute = deck 3, Solo = deck 4 |
| Switch deck | Undo (deck 1 <-> 3) | Save (deck 2 <-> 4) |
| Encoders, EQ mode | 1-4: High, Mid, Low, Filter | 5-8: High, Mid, Low, Filter |
| Encoders, FX mode | 1-4: FX 1, FX 2, FX 3, FX mix | 5-8: same |
| Encoders, GRID mode | 1-3: grid position, BPM, waveform zoom | 5-7: same |
| Encoders, GAIN mode | 1: deck gain (0 dB at center) | 5: same |
| Pads | Columns 1-3 | Columns 6-8 |

Pull-up: hold Shift and turn the jog wheel (or the volume encoder for
deck 1). This holds the track like a record and moves it, also while it
plays. Stop turning or let go of Shift and it plays on. `PULLUP_TICKS` in the
mapping sets how far each tick moves (fewer = further).

Pads per deck (left columns 1-3, right 6-8), top row first: hot cues 1-3;
hot cues 4-6; hot cues 7-8; loop in, loop out, reloop / exit loop; loop 4, 8,
16 beats; halve, double, slip; (empty row); jump back, (empty), jump forward.
The inner column (column 4 left, column 5 right): sync on the top pad, key
lock below it, then the 6-pad volume fader. Quantize is the Quantize button
(all decks). Headphone cue is on the buttons left of the pads, one per deck,
top down: Lock = deck 1, Stop Clips = deck 2, Mute = deck 3, Solo = deck 4.
They stay on those decks when Undo / Save switch the sides.

Pad colors: each hot cue has its own color once set (red, orange, yellow,
green, sky blue, blue, violet, pink), near-black while empty. Loop pads are
dark brown, amber while a loop plays. Both jumps are blue. Slip is dark red,
bright red while on.
Toggles are the darkest shade off and the brightest on: sync red, key lock
violet. Hold
Delete and press a hot cue to clear it.

Volume faders: 6 pads, one level each: bottom = silent, then 20%, 40%, 60%,
80%, top = full. Pads light up to the level.

Undo and Save switch which deck each side of the Push controls. Their LEDs
are lit while a side is on deck 3 or 4. Pads, encoders, transport, nudge and
the volume fader all follow, and so do the labels on the screen.

The top row of the screen shows what each encoder controls and where it is
set. EQ and filter stop at center and stay there while you keep turning;
pause briefly (1/8 s), then turn again to go past it. If the knob is only a
little off center (less than a tenth of its range), it turns straight past
center without stopping (`DETENT_NEAR` in the mapping). Hold Delete and touch an
encoder to reset it: EQ to 0 dB, filter to center, effect off, FX mix to
full, grid and BPM undo the last grid change, zoom goes back to default,
gain to 0 dB. Deck gain also stops at 0 dB, like EQ.

Buttons above the screen: in FX mode, the button over an effect loads the next
effect (hold Delete for the previous one). In GRID mode, the button over GRID
puts the grid's beat at the play position. In GAIN mode, the button over AUTO
(2 or 6) turns auto gain on or off for all decks; AUTO is lit while it is on.
With auto gain off, deck gain is the only level control. While the library is open they
sort the track list: #, title, artist, BPM, key, time, date added, genre.
Press the same one again to reverse the order.

Library search: with the library open, press button 1 below the screen
(SEARCH) and the top 4 pad rows become a
keyboard (A-Z, space, - and ', DEL, CLEAR; hold Shift for 1-0 and . & ( ) /
!). The key map shows over the bottom of the screen. Letters go into the
search box above the library tree; turn the jog wheel to go back to the
track list. Press SEARCH again to hide the keyboard; closing the library
hides it too.

Import a Rekordbox playlist to the Push: open the library, select the
playlist in the tree under Rekordbox (click the jog wheel left, turn it), and
press button 2 below the screen (IMPORT). Mixxx makes a playlist with the
same name and order, with the Rekordbox hot cues and beatgrid. The Push then
copies the tracks from the drive to its own storage; the bottom line of the
screen shows how far it is. From the next Mixxx start the playlist plays
without the drive. Importing again copies only what is missing.

Mixxx sometimes opens a window (for example "Couldn't load track."). It
shows on the Push screen while it is open. Turn the jog wheel to go to the
next button, press the jog wheel to click it.

Help = this manual on the Push screen; turn the jog wheel to scroll, press
Help again to close it.

Master level: the button column right of the pads, from 1/8 (bottom) up to
1/32t, lights green, yellow and red with the master output level. Main lights
red while the master output clips.

Master volume: press Main, then turn the jog wheel. It stops at 0 dB like the
EQ (pause, then turn again to go past). The top row shows MAIN VOLUME with a
bar and the level in dB, and Main lights white. Press Main again, or stop
turning for 3 s, and the jog goes back to what it did before.

Colors like a CDJ: play is green, solid while playing and blinking while
stopped. Cue is yellow and turns orange: solid at the cue point, blinking
(in step with play) when paused somewhere else. The decks use Mixxx's Pioneer
cue mode for this. The headphone cue buttons (Lock, Stop Clips, Mute, Solo)
are yellow off and orange on.

Session (left of the pads) = open / close the library. Layout (above Accent
and Repeat) = next layout: side by side,
top/bottom (waveforms meet in the middle), four decks. In the four-deck
layout the decks the Push controls have colored numbers, the others grey.

Buttons below the screen: 1 = SEARCH and 2 = IMPORT (library open),
3 = swap the master deck, 5 = GAIN mode,
6 = GRID mode, 7 = EQ mode, 8 = FX mode. Hold Shift for FX mode while you
hold it; let go and the encoders go back to the mode before. Quantize button
= quantize on all decks. Touch strip = crossfader; let go and it goes back
to the center.

Accent = reverb at 75% on both decks the Push controls. Press again to turn
it off. Accent is lit while either deck has reverb on. Reverb is effect 1;
if you loaded another effect there, Accent turns that one on instead.

The master deck is marked MASTER on the screen. The tempo encoder changes its
tempo. With SYNC on, it is Mixxx's sync leader, so the other deck follows.
All decks start with SYNC and key lock on, deck 1 as master.

Each effect unit starts with Reverb, Echo and Flanger. An effect's encoder is
its amount. Turn it to 0 (or press Accent again) and Reverb and Echo ring out
like in Rekordbox: nothing new goes in, but the tail keeps playing. The
effect turns off 6 s later. FX MIX starts at full and fades all three.
Mixxx is stopped without saving, so effect changes last until Mixxx mode ends.

The deck screen stacks both decks like Rekordbox: deck 1's info line and
overview on top, then both scrolling waveforms next to each other in the
middle (zoom is shared, so beats line up), then deck 2's overview and info
line. Button LEDs are off unless the button does something in Mixxx mode.

The label rows (encoder names on top, button names below) show while you
touch an encoder or press a button above or below the screen, and hide 1.5 s
after, so the waveforms get the full height. They slide open and shut (about
0.1 s) and are 10 px tall each. They stay while the library is
open.

EQ: Mixxx's Bessel8 LV-Mix isolator (full kill, like Rekordbox's isolator
mode) with crossovers at 265 Hz and 3.6 kHz (band centers about 70 Hz, 1 kHz,
13 kHz, like a Pioneer DJM).

The track list shows #, artist, title, BPM, key and time. The bridge sets
these columns every time Mixxx mode starts. Waveforms are
3-band, like Rekordbox (low blue, mid amber, high white), and auto gain
(ReplayGain) is on, with no extra boost (`InitialReplayGainBoost` 0 in
`src/mode.go`). ReplayGain aims at -18 LUFS, so club tracks play 7-12 dB
quieter than without it. Turn up the master or a deck's gain if needed.

Library: the jog wheel scrolls and press opens. Click the jog wheel left to go
to the library tree, right to go to the track list. Add loads the
selected track on the left deck, Swap on the right deck, and closes the library.
If the list ever stops moving, click the jog wheel right (or left): that gives
the list (or tree) focus again. The jog also takes the track list by itself
when neither pane has focus, for example after a load closed the library.

## Music

Mixxx reads `/data/Music/DJ` (rescanned every time Mixxx mode starts). Copy
tracks there with Push Manager.

Rekordbox USB drives show up under **Rekordbox** in the library tree. Plugged
a drive in while Mixxx is running? Select **Rekordbox** and press the jog wheel
to search for drives again.

The Push mounts FAT drives with a character set that has no room for names
like `Dārin` or `ROSALÍA`, so Mixxx could not find those tracks. In Mixxx
mode the bridge mounts FAT drives again with UTF-8 names. Upper and lower
case still match, so a track Rekordbox lists under `Bakey` is found in a
folder named `BAKEY`. This also applies to drives you plug in while Mixxx
runs. Back in Push mode it mounts them the way the Push did.

Sometimes the Push does not mount a stick it has seen before. In Mixxx mode
the bridge notices within a few seconds and asks the Push to mount it again.

A track that still can't be loaded (file missing) leaves the deck empty and
Mixxx shows an error window; click OK with the jog wheel.

Audio: set the buffer size and the outputs on the Mixxx Mode web page
(**Audio**). By default the master plays on outputs 1/2 and the cue on the
headphone jack (outputs 3/4), with a 256-frame buffer (5.8 ms). If you hear clicks, pick a
bigger buffer. Changes apply the next time Mixxx starts. The settings live in
`settings/audio.json`, so updates keep them. The bridge also lets Mixxx's
audio thread run with realtime priority. Mixxx's own default buffer is
23 ms, about 50 ms from pressing a button to hearing it.

Imported playlists are copied to `/data/Music/DJ/USB/`, with the same
folders as on the drive, so a track in two playlists is copied once. When the
copy is done, Mixxx switches those tracks to the copies right away, so you
can unplug the drive. Import needs the patched Mixxx build:
`patches/sidebar-import-playlist.patch` adds the `[Library],import_playlist`
control, and `patches/import-relocate.patch` lets the bridge tell Mixxx
about the copies (`settings/import-ready.json`). If Mixxx misses one, the
bridge fixes it at the next Mixxx start. A track that is in two imported playlists is copied
once and stays one track in both playlists.

Remove a track from the Mixxx library and its copy goes too: every time Mixxx
starts or stops, the bridge deletes copies in `/data/Music/DJ/USB/` that no
library track uses any more. This only touches the `USB` folder, never your
own tracks.

## Layout

| Path | What |
|---|---|
| `src/` | `mixxx-bridge`: mode switch, screen copy, exit button |
| `mapping/` | Mixxx controller mapping (XML + JS) |
| `skin/PushHack/` | 960x160 Mixxx skin |
| `patches/` | Patches for Mixxx 2.5.6 (Rekordbox import, live relocation) |
| `scripts/build-mixxx.sh` | Builds the patched Mixxx package (Docker) |
| `scripts/bundle-mixxx.sh` | Builds the Mixxx bundle (Mixxx 2.5 from the Mixxx PPA, Ubuntu 22.04) |
| `scripts/release.sh` | Packages a release for Push Hack Catalog |

## Building a release

Everything is built on a computer with Docker, never on the Push. On an
Apple Silicon Mac the Mixxx build runs under emulation and takes a few hours
the first time. The `mixxx-build` volume keeps the objects, so later builds
only compile what changed.

1. Install Docker Desktop and give it at least 8 GB of memory
   (Settings > Resources).
2. Build the patched Mixxx package:
   `docker run --rm --platform linux/amd64 -v "$PWD":/work -v mixxx-build:/build -e PARALLEL=2 ubuntu:22.04 /work/scripts/build-mixxx.sh`
   Each compile job needs 1 to 1.5 GB. Raise `PARALLEL` only if Docker has
   the memory for it.
3. Build the bundle:
   `docker run --rm --platform linux/amd64 -v "$PWD":/work ubuntu:22.04 /work/scripts/bundle-mixxx.sh`
4. Bump `version` in `hack.json`, then run `scripts/release.sh`.
5. `gh release create v<version> dist/mixxx.tar.gz dist/mixxx-source.tar.gz`,
   then commit and push the new `release.json`. The catalog reads it from
   `main`.

## License

GPL-2.0-or-later, see `LICENSE`. The release bundles Mixxx (GPL-2.0-or-later)
built with the patches in `patches/`. Each release has the matching Mixxx
source attached.
