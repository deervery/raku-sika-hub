"""Shared lab transport and playback generation guard; no ASR/TTS engine here."""
from dataclasses import dataclass
import json
import os
import urllib.error
import urllib.request


def hub(path, body=None):
    base = os.environ.get("STATION_HUB", "http://127.0.0.1:3110").rstrip("/")
    req = urllib.request.Request(base + path,
        data=None if body is None else json.dumps(body, ensure_ascii=False).encode(),
        headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise ValueError(json.load(error).get("error", str(error))) from error


@dataclass(frozen=True)
class Ticket:
    session_id: str
    task_id: str
    revision: int
    media_epoch: int

    @classmethod
    def from_state(cls, state):
        return cls(*(state[k] for k in ("session_id", "task_id", "revision", "media_epoch")))


class PlaybackGate:
    """Adapter-side guard. Caller supplies the actual audio-device stop callback.

    Discarding an old result is NOT cancellation of its inference worker.
    A future worker's cancellation acknowledgment must be measured separately.
    This class is driven on one event loop (not thread safe).
    """
    def __init__(self, stop_playback):
        self.current = None
        self.last = None
        self.minimum_epoch = 0
        self.queue = []
        self.stop_playback = stop_playback

    def advance(self, ticket):
        if self.last is not None and (ticket.session_id != self.last.session_id or ticket.revision < self.last.revision or ticket.media_epoch < self.minimum_epoch):
            return False
        if self.current == ticket:
            return True
        self.stop_playback()
        self.queue.clear()
        self.current = ticket
        self.last = ticket
        self.minimum_epoch = ticket.media_epoch
        return True

    def enqueue(self, ticket, audio):
        if ticket != self.current:
            return False
        self.queue.append((ticket, audio))
        return True

    def pop(self):
        while self.queue:
            ticket, audio = self.queue.pop(0)
            if ticket == self.current:
                return audio
        return None

    def interrupt_local(self):
        # Stop locally at speech start, before awaiting a network round trip.
        # The caller then POSTs action=interrupt and adopts the returned ticket.
        self.stop_playback()
        self.queue.clear()
        self.current = None
        if self.last is not None:
            self.minimum_epoch = max(self.minimum_epoch, self.last.media_epoch + 1)
