"""Separate Pipecat process: stamped final transcripts -> Go -> reply envelope.

Uses the exact Pipecat revision from the attached validation environment.
stdin/stdout JSONL is a replay transport, not a microphone or speaker transport.
Do not silently refresh a stale command: its speech may predate a correction.
"""
import asyncio
import json
import sys

from loguru import logger
from pipecat.frames.frames import EndFrame, TextFrame
from pipecat.pipeline.pipeline import Pipeline
from pipecat.pipeline.worker import PipelineParams, PipelineWorker
from pipecat.processors.frame_processor import FrameProcessor
from pipecat.workers.runner import WorkerRunner

from voice_contract import hub

logger.remove()
logger.add(sys.stderr, level="WARNING")


class GoState(FrameProcessor):
    async def process_frame(self, frame, direction):
        await super().process_frame(frame, direction)
        if isinstance(frame, TextFrame):
            try:
                command = json.loads(frame.text)
                if command.get("action") not in {"utterance", "interrupt", "repeat"}:
                    raise ValueError("voice bridge accepts utterance/interrupt/repeat only")
                result = await asyncio.to_thread(hub, "/api/commands", command)
                state = result["state"]
                result["speech"] = {k: state[k] for k in ("session_id", "task_id", "revision", "media_epoch", "reply")}
                result["speech"]["audio_generated"] = False
                result["speech"]["speak"] = command["action"] != "interrupt"
                if command["action"] == "interrupt":
                    result["speech"]["reply"] = ""
                frame = TextFrame(json.dumps(result, ensure_ascii=False))
            except Exception as error:
                frame = TextFrame(json.dumps({"error": str(error), "audio_generated": False}))
        await self.push_frame(frame, direction)


class Output(FrameProcessor):
    async def process_frame(self, frame, direction):
        await super().process_frame(frame, direction)
        if isinstance(frame, TextFrame):
            print(frame.text, flush=True)
        await self.push_frame(frame, direction)


async def main():
    worker = PipelineWorker(Pipeline([GoState(), Output()]), params=PipelineParams(), enable_rtvi=False)
    runner = WorkerRunner(handle_sigint=False)
    await runner.add_workers(worker)

    async def feed():
        while line := await asyncio.to_thread(sys.stdin.readline):
            await worker.queue_frame(TextFrame(line.strip()))
        await worker.queue_frame(EndFrame())

    await asyncio.gather(runner.run(), feed())


if __name__ == "__main__":
    asyncio.run(main())
