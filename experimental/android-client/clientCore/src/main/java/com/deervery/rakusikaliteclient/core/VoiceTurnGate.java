package com.deervery.rakusikaliteclient.core;

/** 16 kHz / 512-sample frames. No audio, transcript or business state is persisted. */
public final class VoiceTurnGate {
    public enum Event { NONE, START, END, TOO_LONG }
    private int onset, silence, frames;
    private boolean talking, draining;
    public Event accept(float probability) {
        if (!Float.isFinite(probability)) probability = 0;
        if (draining) {
            silence = probability < .35f ? silence + 1 : 0;
            if (silence >= 24) {draining = false; silence = 0;}
            return Event.NONE;
        }
        if (!talking) {
            onset = probability >= .6f ? onset + 1 : 0;
            if (onset < 4) return Event.NONE; // 128 ms of speech, not one noise spike.
            talking = true; onset = 0; frames = 4; silence = 0;
            return Event.START;
        }
        frames++;
        silence = probability < .35f ? silence + 1 : 0;
        if (silence >= 24) {talking = false; silence = 0; return Event.END;} // 768 ms.
        if (frames >= 469) {talking = false; draining = true; silence = 0; return Event.TOO_LONG;}
        return Event.NONE;
    }
}
