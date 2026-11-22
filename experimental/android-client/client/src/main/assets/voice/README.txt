Local voice assets / 0.3.2
Silero VAD v6.2.1 (MIT)
Source: https://github.com/snakers4/silero-vad/tree/v6.2.1
Model: src/silero_vad/data/silero_vad.onnx
SHA-256: 1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3
16 kHz, 512-sample input, 64-sample context, state [2,1,128].

ONNX Runtime Android 1.23.2 (MIT), pinned Maven Central AAR.
Copyright and third party notices are included alongside this file.
Notices verified against the v1.23.2 source via jsDelivr's versioned GitHub mirror;
the direct GitHub source endpoint was unavailable during this build.
No inference model or executable code was obtained from that mirror.

The androidTest-only silero-speech.pcm fixture is the first 8 seconds of
https://raw.githubusercontent.com/snakers4/silero-vad/v6.2.1/tests/data/test.wav
(16 kHz, mono, signed PCM16). It is NOT included in the user APK.
This fixture tests VAD wiring, not Japanese ASR accuracy or factory acoustics.
