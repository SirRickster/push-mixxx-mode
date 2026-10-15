// node mapping/check.js — stubs mixxx's engine/midi, checks the encoder logic.
const handlers = {}; // status * 256 + data1 -> fn, so checks can send midi
global.midi = { makeInputHandler(st, d1, fn) { handlers[st * 256 + d1] = fn; }, sendShortMsg() {} };
const send = (st, d1, d2) => handlers[st * 256 + d1](st & 15, d1, d2, st);
const v = {};
const watchers = {}; // group + key -> makeConnection callback (last one wins)
const timers = {}; // id -> fn
let nextTimer = 1;
global.engine = {
    beginTimer(ms, fn, oneShot) { const id = nextTimer++; timers[id] = oneShot ? () => { delete timers[id]; fn(); } : fn; return id; },
    stopTimer(id) { delete timers[id]; },
    scratching: {}, // deck -> ticks while scratch is on
    scratchEnable(d) { this.scratching[d] = 0; },
    scratchTick(d, x) { this.scratching[d] += x; },
    scratchDisable(d) { delete this.scratching[d]; },
    setValue(g, k, x) { v[g + k] = x; }, getValue(g, k) { return v[g + k] || 0; },
    getParameter(g, k) { return v[g + k] ?? 0.5; }, setParameter(g, k, x) { v[g + k] = x; },
    // like mixxx 2.5: [Master] has no connectable controls (vu_meter is [Main]),
    // makeConnection returns undefined for them.
    makeConnection(g, k, fn) {
        if (g === "[Master]") { return undefined; }
        watchers[g + k] = fn;
        return { trigger() {}, disconnect() {} };
    },
};
global.script = { toggleControl() {} };
eval(require("fs").readFileSync(__dirname + "/Ableton-Push-3.js", "utf8") + ";global.PUSH3=PUSH3");
const assert = require("assert");
PUSH3.init();
const G = "[EqualizerRack1_[Channel1]_Effect1]";
// EQ detent, tick by tick like the hardware. from far away: stops at center
const turn = (ticks, d) => { for (let i = 0; i < ticks; i++) { PUSH3.nudgeDetent(0, G, "parameter3", d, 0.01); } };
v[G + "parameter3"] = 0.3;
turn(25, 1);
assert.equal(v[G + "parameter3"], 0.5);
// after a pause it goes past
PUSH3.held[0] -= 1000;
turn(2, 1);
assert(Math.abs(v[G + "parameter3"] - 0.52) < 1e-9);
// a little off center, turning back: straight past, no stop
turn(30, -1);
assert(Math.abs(v[G + "parameter3"] - 0.22) < 1e-9);
// far again (0.22), back up: stops at center again
turn(40, 1);
assert(Math.abs(v[G + "parameter3"] - 0.5) < 1e-9);
// jog: plain = nudge; Shift held = pull-up (scratch), released on idle
v["[Channel2]jog"] = 0;
PUSH3.jog("[Channel2]", 2);
assert.equal(v["[Channel2]jog"], 2 * PUSH3.NUDGE);
PUSH3.shiftDown = true;
PUSH3.jog("[Channel2]", -3);
PUSH3.jog("[Channel2]", -4);
assert.equal(engine.scratching[2], -7);
assert.equal(v["[Channel2]jog"], 2 * PUSH3.NUDGE); // no nudge while pulling up
Object.values(timers).forEach(fn => fn()); // idle timer fires
assert.equal(engine.scratching[2], undefined);
// Shift up ends it too
PUSH3.jog("[Channel1]", -5);
assert.equal(engine.scratching[1], -5);
send(0xB0, PUSH3.CC.shift, 0);
assert.equal(engine.scratching[1], undefined);
assert.equal(PUSH3.shiftDown, false);
// defaults: keylock + sync on, FX mix full, effects off at meta 0
assert.equal(v["[Channel1]keylock"], 1);
assert.equal(v["[Channel2]sync_enabled"], 1);
assert.equal(v["[EffectRack1_EffectUnit1]mix"], 1);
// step: 7 ticks at STEP_TICKS 3 = 2 presses, 1 tick kept
let n = 0;
const sv = engine.setValue;
engine.setValue = (g, k, x) => { if (k === "beats_translate_later") n++; sv(g, k, x); };
PUSH3.acc[0] = 0;
PUSH3.step(0, 7, "[Channel1]", "beats_translate_earlier", "beats_translate_later");
assert.equal(n, 2);
assert.equal(PUSH3.acc[0], 1);
// master swap + sort page while library open
PUSH3.setMaster(1);
assert.equal(v["[PushHack]master"], 2);
v["[Skin]show_maximized_library"] = 1;
PUSH3.updateTop();
assert.equal(v["[PushHack]top_page"], 3);
// label rows: touch slides them open, still touched = stays, release +
// timeout slides them shut
const anim = () => { for (let i = 0; i < 20 && PUSH3.animTimer; i++) timers[PUSH3.animTimer](); };
const hide = () => { if (timers[PUSH3.uiTimer]) timers[PUSH3.uiTimer](); };
v["[Skin]show_maximized_library"] = 0;
PUSH3.touch(0, true);
anim();
assert.equal(v["[PushHack]ui_h"], PUSH3.UI_H);
hide();
anim();
assert.equal(v["[PushHack]ui_h"], PUSH3.UI_H);
PUSH3.touch(0, false);
hide();
anim();
assert.equal(v["[PushHack]ui_h"], 0);
assert.equal(PUSH3.animTimer, 0);
// volume fader: 4 pads, bottom = 0, top = full, on the side's current deck
PUSH3.flip(1); // right side -> deck 4
assert.equal(PUSH3.decks[1], "[Channel4]");
assert.equal(v["[PushHack]right_page"], 1);
assert.equal(v["[PushHack]active4"], 1);
assert.equal(v["[PushHack]active2"], 0);
// fx unit + master follow the flipped deck
assert.deepEqual(PUSH3.encTarget(1, 0), ["[EqualizerRack1_[Channel4]_Effect1]", "parameter3"]);
PUSH3.setEncMode("fx");
assert.deepEqual(PUSH3.encTarget(1, 0), ["[EffectRack1_EffectUnit4_Effect1]", "meta"]);
PUSH3.setEncMode("eq");
PUSH3.setMaster(1);
assert.equal(v["[PushHack]master"], 4);
PUSH3.flip(1); // back to deck 2
assert.equal(PUSH3.decks[1], "[Channel2]");
assert.equal(v["[PushHack]master"], 2);
assert.equal(v["[PushHack]right_page"], 0);
PUSH3.flip(0);
assert.equal(PUSH3.decks[0], "[Channel3]");
assert.equal(v["[PushHack]left_page"], 1);
// keyboard: off until SEARCH, off again with SEARCH
PUSH3.setKb(true);
assert.equal(PUSH3.kbOn, true);
assert.equal(v["[PushHack]kb"], 1);
PUSH3.setKb(false);
assert.equal(PUSH3.kbOn, false);
assert.equal(v["[PushHack]kb"], 0);
// master meter: half level = 3 of 6 lit, only resent on change
const sent2 = [];
const ms = midi.sendShortMsg;
midi.sendShortMsg = (st, cc, val) => sent2.push([cc, val]);
PUSH3.masterVu(0.5);
assert.deepEqual(sent2.map(x => x[1]), [10, 10, 10, 0, 0, 0]);
PUSH3.masterVu(0.51);
assert.equal(sent2.length, 6);
midi.sendShortMsg = ms;
// library: focus lost (0) or on the search box (1) -> jog takes the track list
for (const f of [0, 1, 6]) {
    v["[Library]focused_widget"] = f;
    PUSH3.libMove(1);
    assert.equal(v["[Library]focused_widget"], 3);
}
v["[Library]focused_widget"] = 2; // tree keeps focus
PUSH3.libMove(1);
assert.equal(v["[Library]focused_widget"], 2);
// GAIN mode: encoder 1 = deck gain, nothing on 3-4, own top page
PUSH3.setEncMode("gain");
assert.equal(v["[PushHack]encoder_mode"], 3);
assert.equal(v["[PushHack]top_page"], 4);
assert.deepEqual(PUSH3.encTarget(1, 0), ["[Channel2]", "pregain"]);
assert.equal(PUSH3.encTarget(1, 2), null);
send(0xB0, PUSH3.ENC_FIRST + 6, 1); // turning an empty slot does nothing
// hold Shift = FX, let go = back to the mode before
send(0xB0, PUSH3.CC.shift, 127);
assert.equal(PUSH3.encMode, "fx");
send(0xB0, PUSH3.CC.shift, 0);
assert.equal(PUSH3.encMode, "gain");
// Accent: reverb at 75% on both controlled decks, again = off
send(0xB0, PUSH3.CC.accent, 127);
assert.equal(v["[EffectRack1_EffectUnit3_Effect1]meta"], 0.75);
assert.equal(v["[EffectRack1_EffectUnit2_Effect1]meta"], 0.75);
send(0xB0, PUSH3.CC.accent, 127);
assert.equal(v["[EffectRack1_EffectUnit2_Effect1]meta"], 0);
// touch strip: let go = crossfader back to center
send(0xE0, 0, 120);
assert(v["[Master]crossfader"] > 0.8);
send(0x93, 12, 0);
assert.equal(v["[Master]crossfader"], 0);
// dialog open (5): jog = Tab through its buttons, focus stays, deck not nudged
v["[Skin]show_maximized_library"] = 0;
v["[Library]focused_widget"] = 5;
v["[Channel2]jog"] = 0;
send(0xB0, PUSH3.CC.jogTurn, 1);
assert.equal(v["[Library]focused_widget"], 5);
assert.equal(v["[Library]MoveVertical"], 1);
assert.equal(v["[Channel2]jog"], 0);
// layout: sync on top of the inner column, key lock below, 6-pad fader;
// Lock / Stop / Mute / Solo = headphone cue of deck 1 / 2 / 3 / 4
assert.deepEqual(PUSH3.INNER, { 6: "keylock", 7: "sync" });
let toggled = [];
const tc = script.toggleControl;
script.toggleControl = (g, k) => toggled.push(g + k);
send(0xB0, PUSH3.CC.lock, 127);
send(0xB0, PUSH3.CC.solo, 127);
script.toggleControl = tc;
assert.deepEqual(toggled, ["[Channel1]pfl", "[Channel4]pfl"]);
// fader: 6 pads, row 5 = full, row 1 = 0.2
send(0x90, PUSH3.padNote(5, 4), 100);
assert.equal(v[PUSH3.decks[1] + "volume"], 1);
send(0x90, PUSH3.padNote(1, 3), 100);
assert(Math.abs(v[PUSH3.decks[0] + "volume"] - 0.2) < 1e-9);
// effect tail: knob to 0 keeps the effect on (reverb rings out), off after TAIL_MS;
// turning it up again in the tail cancels the off
const FX = "[EffectRack1_EffectUnit1_Effect1]";
watchers[FX + "meta"](0.5);
assert.equal(v[FX + "enabled"], 1);
watchers[FX + "meta"](0);
assert.equal(v[FX + "enabled"], 1);
const tail = PUSH3.tailTimers[FX];
watchers[FX + "meta"](0.3);
assert.equal(timers[tail], undefined);
watchers[FX + "meta"](0);
timers[PUSH3.tailTimers[FX]]();
assert.equal(v[FX + "enabled"], 0);
// master volume: Main, then jog -> [Master],gain (mixxx 2.5 has no
// [Main],gain), top page "vol", stops at 0 dB, ends after VOL.ms; then the
// jog nudges deck 2 again
v["[Library]focused_widget"] = 3;
v["[Master]gain"] = 0.5;
send(0xB0, PUSH3.CLIP_CC, 127);
assert.equal(PUSH3.VOL.on, true);
assert.equal(v["[PushHack]top_page"], PUSH3.PAGES.indexOf("vol"));
PUSH3.held[8] -= 1000;
send(0xB0, PUSH3.CC.jogTurn, 1);
assert(Math.abs(v["[Master]gain"] - 0.51) < 1e-9);
assert.equal(v["[Main]gain"], undefined);
timers[PUSH3.VOL.timer]();
assert.equal(PUSH3.VOL.on, false);
v["[Channel2]jog"] = 0;
send(0xB0, PUSH3.CC.jogTurn, 1);
assert(Math.abs(v["[Master]gain"] - 0.51) < 1e-9);
assert.equal(v["[Channel2]jog"], 1);
// transport LEDs: stopped = play blinks with [App],indicator_250ms, playing =
// solid green; cue yellow / orange; pioneer cue mode on every deck
const sent3 = [];
const ms3 = midi.sendShortMsg;
midi.sendShortMsg = (st, cc, val) => sent3.push([cc, val]);
v[PUSH3.decks[0] + "play"] = 0;
v["[App]indicator_250ms"] = 1; watchers["[App]indicator_250ms"](1);
v["[App]indicator_250ms"] = 0; watchers["[App]indicator_250ms"](0);
v[PUSH3.decks[0] + "play"] = 1; watchers["[App]indicator_250ms"](0);
midi.sendShortMsg = ms3;
const playLeds = sent3.filter(x => x[0] === PUSH3.CC.play).map(x => x[1]);
assert.deepEqual(playLeds, [PUSH3.PLAY_COL, 0, PUSH3.PLAY_COL]);
assert.equal(v["[Channel4]cue_mode"], 1);
console.log("mapping checks ok");
