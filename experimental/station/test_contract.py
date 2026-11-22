import unittest
from unittest.mock import patch
from voice_contract import Ticket, PlaybackGate
from mcp_bridge import tool, handle
from evaluate import summarize


class Safety(unittest.TestCase):
    def test_unmeasured_null_and_leading_zero_mismatch(self):
        rows=[{"model":"test","device":"mock","noise":"quiet","expected":{"lot":"00123","copies":"2"},"observed":{"lot":"123","copies":"2"},"first_pcm_ms":250}]
        report=summarize(rows)[0]
        self.assertEqual(report["field_exact"]["lot"]["rate"],0)
        self.assertEqual(report["field_exact"]["copies"]["rate"],1)
        self.assertIsNone(report["measurements"]["acoustic_start_ms"]["median"])
        self.assertEqual(report["measurements"]["first_pcm_ms"]["median"],250)
    def test_interruption_discards_queued_and_late_audio(self):
        stopped = []
        gate = PlaybackGate(lambda: stopped.append(True))
        old, new = Ticket("s","t",1,1), Ticket("s","t",2,2)
        gate.advance(old)
        self.assertTrue(gate.enqueue(old, b"queued old speech"))
        gate.interrupt_local()
        self.assertFalse(gate.advance(old))
        self.assertIsNone(gate.pop())
        self.assertFalse(gate.enqueue(old, b"late inference"))
        gate.advance(new)
        self.assertFalse(gate.advance(old))
        self.assertFalse(gate.enqueue(old, b"another old result"))
        self.assertTrue(gate.enqueue(new, b"new speech"))
        self.assertEqual(gate.pop(), b"new speech")
        self.assertGreaterEqual(len(stopped), 2)

    def test_model_cannot_confirm_or_fake_yes(self):
        with patch("mcp_bridge.hub") as hub:
            for action in ("confirm", "utterance", "cancel"):
                with self.assertRaises(ValueError):
                    tool("station_propose", {"action":action})
            hub.assert_not_called()

    def test_proposals_use_shared_service(self):
        args = {"action":"patch", "session_id":"a"*32, "revision":2, "media_epoch":3, "fields":{"copies":"2"}}
        with patch("mcp_bridge.hub", return_value={"mock":True}) as hub:
            self.assertEqual(tool("station_propose",args), {"mock":True})
            hub.assert_called_once_with("/api/commands",args)

    def test_protocol_discovery_and_notifications(self):
        self.assertIsNone(handle({"jsonrpc":"2.0","method":"notifications/initialized"}))
        result = handle({"jsonrpc":"2.0","id":1,"method":"tools/list"})
        self.assertEqual(len(result["result"]["tools"]),4)


if __name__ == "__main__":
    unittest.main()
