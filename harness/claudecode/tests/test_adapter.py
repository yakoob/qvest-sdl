#!/usr/bin/env python3
import contextlib
import http.server
import json
import os
import pathlib
import subprocess
import tempfile
import threading
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[3]
HOOKS = ROOT / "harness" / "claudecode" / "hooks"


class Recorder(http.server.ThreadingHTTPServer):
    def __init__(self, address):
        super().__init__(address, Handler)
        self.requests = []


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def _record(self):
        size = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(size) if size else b""
        try:
            body = json.loads(raw) if raw else None
        except json.JSONDecodeError:
            body = raw.decode(errors="replace")
        self.server.requests.append((self.command, self.path, dict(self.headers), body))
        if self.command == "POST" and self.path == "/brain/sessions":
            response = {"id": "be9c0084-4490-4744-8a46-6081c90ba1ee", "status": "open"}
        elif self.command == "GET":
            response = {"events": [], "has_state": False}
        else:
            response = {"ok": True}
        encoded = json.dumps(response).encode()
        self.send_response(200 if self.command != "POST" or self.path != "/brain/sessions" else 201)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    do_POST = _record
    do_PATCH = _record
    do_GET = _record


class AdapterTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.server = Recorder(("127.0.0.1", 0))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.env = os.environ.copy()
        self.env.update({
            "AGENT_ID": "fixture-agent",
            "AXON_HARNESS": "claude-code",
            "AXON_SESSION_CACHE": str(pathlib.Path(self.temp.name) / "cache"),
            "BRAIN_BASE": f"http://127.0.0.1:{self.server.server_port}",
            "CLAUDE_PROJECT_DIR": str(ROOT),
            "ANTHROPIC_BASE_URL": "https://example.test/control-plane/proxy",
            "ANTHROPIC_MODEL": "auto",
        })

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
        self.temp.cleanup()

    def hook(self, name, payload, *args, env=None):
        result = subprocess.run(
            ["bash", str(HOOKS / name), *args],
            input=json.dumps(payload), text=True, capture_output=True,
            env=env or self.env, timeout=20, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return result

    def test_prompt_tool_failure_and_close_lifecycle(self):
        self.hook("session_prompt_hook.sh", {
            "session_id": "vendor-session",
            "prompt": "use token=secret-value and keep this request",
        })
        self.hook("session_activity_hook.sh", {
            "session_id": "vendor-session",
            "hook_event_name": "PostToolUse",
            "tool_name": "Write",
            "tool_use_id": "tool-1",
            "tool_input": {"file_path": "/tmp/a.txt", "api_key": "top-secret"},
            "tool_response": {"authorization": "Bearer abc.def", "ok": True},
            "duration_ms": 17,
            "exit_code": 0,
        })
        self.hook("session_activity_hook.sh", {
            "session_id": "vendor-session",
            "hook_event_name": "PostToolUseFailure",
            "tool_name": "Bash",
            "tool_input": {"command": "false"},
            "error": "failed with password=hunter2",
        }, "fail")

        requests = self.server.requests
        opens = [r for r in requests if r[0] == "POST" and r[1] == "/brain/sessions"]
        self.assertEqual(len(opens), 1)
        self.assertEqual(opens[0][3]["agent_id"], "fixture-agent")
        self.assertEqual(opens[0][3]["harness_session_key"], "vendor-session")
        activities = [r for r in requests if r[1].endswith("/activity")]
        self.assertEqual([r[3]["kind"] for r in activities], ["prompt", "tool.post", "tool.fail"])
        self.assertNotIn("secret-value", json.dumps(activities))
        self.assertNotIn("top-secret", json.dumps(activities))
        self.assertNotIn("abc.def", json.dumps(activities))
        self.assertNotIn("hunter2", json.dumps(activities))
        tool = activities[1][3]
        self.assertEqual(tool["files"], ["/tmp/a.txt"])
        self.assertEqual(tool["duration_ms"], 17)
        self.assertEqual(tool["exit_code"], 0)
        self.assertEqual(tool["hash"], "tool-1")
        for _, _, headers, _ in opens + activities:
            self.assertEqual(headers.get("X-Agent-Id"), "fixture-agent")
            self.assertEqual(headers.get("X-Harness"), "claude-code")

        cache = pathlib.Path(self.env["AXON_SESSION_CACHE"])
        self.assertTrue((cache / "fixture-agent-claude-code.session_id").exists())
        health = json.loads((cache / "fixture-agent-claude-code.health.json").read_text())
        self.assertEqual(health["last_success_operation"], "activity")

        command = f'source "{HOOKS / "session_visibility_lib.sh"}"; vis_close_session closed'
        result = subprocess.run(["bash", "-c", command], env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((cache / "fixture-agent-claude-code.session_id").exists())
        close = [r for r in self.server.requests if r[0] == "PATCH"][-1]
        self.assertEqual(close[3], {"status": "closed"})
        self.assertEqual(close[2].get("X-Session-Id"), "be9c0084-4490-4744-8a46-6081c90ba1ee")

    def test_session_start_context_read_is_attributed(self):
        env = self.env.copy()
        env["LIBRARIAN_BASE"] = f"http://127.0.0.1:{self.server.server_port}"
        env["TOOLSHED_BASE"] = f"http://127.0.0.1:{self.server.server_port}"
        self.hook("session_start_librarian.sh", {"session_id": "vendor-session"}, env=env)

        context = [r for r in self.server.requests
                   if r[0] == "GET" and r[1] == "/brain/agent/fixture-agent/context"]
        self.assertEqual(len(context), 1)
        headers = context[0][2]
        self.assertEqual(headers.get("X-Agent-Id"), "fixture-agent")
        self.assertEqual(headers.get("X-Harness"), "claude-code")
        self.assertEqual(headers.get("X-Session-Id"), "be9c0084-4490-4744-8a46-6081c90ba1ee")

    def test_session_end_context_read_is_attributed(self):
        self.hook("session_prompt_hook.sh", {"session_id": "vendor-session", "prompt": "hello"})
        self.hook("session_end_hook.sh", {"session_id": "vendor-session"})

        context = [r for r in self.server.requests
                   if r[0] == "GET" and r[1] == "/brain/agent/fixture-agent/context"]
        self.assertEqual(len(context), 1)
        headers = context[0][2]
        self.assertEqual(headers.get("X-Agent-Id"), "fixture-agent")
        self.assertEqual(headers.get("X-Harness"), "claude-code")
        self.assertEqual(headers.get("X-Session-Id"), "be9c0084-4490-4744-8a46-6081c90ba1ee")
        handoff = [r for r in self.server.requests
                   if r[0] == "POST" and r[1] == "/brain/agent/fixture-agent/handoff"]
        self.assertEqual(len(handoff), 1)
        self.assertEqual(handoff[0][2].get("X-Session-Id"), "be9c0084-4490-4744-8a46-6081c90ba1ee")

    def test_assistant_response_is_redacted_bounded_and_nonrecursive(self):
        self.hook("session_response_hook.sh", {
            "session_id": "vendor-session",
            "hook_event_name": "Stop",
            "last_assistant_message": "done with token=secret-value " + "x" * 20000,
            "stop_hook_active": False,
        })

        activities = [r for r in self.server.requests if r[1].endswith("/activity")]
        self.assertEqual(len(activities), 1)
        response = activities[0][3]
        self.assertEqual(response["kind"], "assistant.response")
        self.assertIsNone(response["input_redacted"])
        self.assertNotIn("secret-value", response["output_redacted"])
        self.assertLessEqual(len(response["output_redacted"]), 16000)
        self.assertEqual(response["metadata"]["role"], "assistant")
        self.assertEqual(response["metadata"]["hook_event_name"], "Stop")
        self.assertEqual(activities[0][2].get("X-Session-Id"), "be9c0084-4490-4744-8a46-6081c90ba1ee")

        self.hook("session_response_hook.sh", {
            "session_id": "vendor-session",
            "last_assistant_message": "must not be duplicated",
            "stop_hook_active": True,
        })
        self.assertEqual(len([r for r in self.server.requests if r[1].endswith("/activity")]), 1)

    def test_malformed_input_and_unreachable_brain_are_nonblocking(self):
        result = subprocess.run(
            ["bash", str(HOOKS / "session_activity_hook.sh")],
            input="not json", text=True, capture_output=True, env=self.env, timeout=20,
        )
        self.assertEqual(result.returncode, 0)
        result = subprocess.run(
            ["bash", str(HOOKS / "session_response_hook.sh")],
            input="not json", text=True, capture_output=True, env=self.env, timeout=20,
        )
        self.assertEqual(result.returncode, 0)
        bad = self.env.copy()
        bad["BRAIN_BASE"] = "http://127.0.0.1:1"
        result = self.hook("session_prompt_hook.sh", {"session_id": "v", "prompt": "hello"}, env=bad)
        self.assertEqual(result.returncode, 0)
        result = self.hook("session_response_hook.sh", {
            "session_id": "v",
            "last_assistant_message": "hello",
            "stop_hook_active": False,
        }, env=bad)
        self.assertEqual(result.returncode, 0)
        health = pathlib.Path(bad["AXON_SESSION_CACHE"]) / "fixture-agent-claude-code.health.json"
        self.assertEqual(json.loads(health.read_text())["last_error_category"], "request_failed")

    def test_identity_uses_separate_cache_files(self):
        self.hook("session_prompt_hook.sh", {"session_id": "one", "prompt": "first"})
        second = self.env.copy()
        second["AGENT_ID"] = "fixture-agent-two"
        self.hook("session_prompt_hook.sh", {"session_id": "two", "prompt": "second"}, env=second)
        cache = pathlib.Path(self.env["AXON_SESSION_CACHE"])
        self.assertTrue((cache / "fixture-agent-claude-code.session_id").exists())
        self.assertTrue((cache / "fixture-agent-two-claude-code.session_id").exists())
        opens = [r for r in self.server.requests if r[1] == "/brain/sessions"]
        self.assertEqual({r[3]["agent_id"] for r in opens}, {"fixture-agent", "fixture-agent-two"})


if __name__ == "__main__":
    unittest.main()
