#!/usr/bin/env python3
"""Capture a live CLI conversation, local summary and daemon restart.

Run make test and make build first. No cloud credentials are passed to children.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import socket
import subprocess
import tempfile
import time
import tomllib
import urllib.request
import http.client


class UnixConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__("localhost", timeout=10)
        self.path = path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", default="results/task1/demo.json")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    output = Path(args.output).resolve()
    if output.exists():
        raise SystemExit(f"Preserving existing result: {output}. Use a different --output path.")
    config_home = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
    cfg = tomllib.loads((config_home / "advent-agent/profiles/local/config.toml").read_text())
    if cfg["provider"]["name"] != "ollama":
        raise SystemExit("The local profile must select Ollama.")
    sock = str(Path(cfg["daemon"]["socket_path"]).expanduser())
    env = dict(os.environ)
    env.pop("DEEPSEEK_API_KEY", None)
    binary = root / "advent-agent"
    daemon = root / "advent-agentd"
    report = {"created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "artifact": str(binary), "artifact_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
              "profile": "local", "settings": cfg,
              "cloud_key_passed": False, "commands": [], "snapshots": []}
    output.parent.mkdir(parents=True, exist_ok=True)

    def save():
        output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")

    def api(path):
        connection = UnixConnection(sock)
        try:
            connection.request("GET", "/v1/" + path, headers={"X-Agent-Profile": "local"})
            response = connection.getresponse()
            if response.status != 200:
                raise RuntimeError(f"Daemon returned HTTP {response.status}")
            return json.loads(response.read())
        finally:
            connection.close()

    with tempfile.TemporaryDirectory(prefix="week5-local-chat-") as cwd:
        # Chat demo has its own project memory and no unrelated repository instructions.
        report["project_directory"] = cwd

        def run(command, input_text=None):
            print("Running:", " ".join(map(str, command)), flush=True)
            start = time.monotonic()
            result = subprocess.run(command, cwd=cwd, env=env, input=input_text or "", text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=600)
            report["commands"].append({"argv": list(map(str, command)), "stdin": input_text,
                                       "stdout": result.stdout, "stderr": result.stderr,
                                       "exit_code": result.returncode, "elapsed_seconds": time.monotonic()-start})
            save()
            print(result.stdout, end="", flush=True)
            if result.returncode:
                raise RuntimeError(result.stderr)
            return result

        result = run([str(binary), "--profile", "local", "-p",
                      "Our project is named Cedar, a Go CLI for offline notes. Answer in one sentence: what is our project and language?", "--stats"])
        match = re.search(r"Session: ([a-f0-9]+)", result.stderr)
        if not match:
            raise RuntimeError("CLI did not identify its session")
        session_id = match.group(1)
        report["session_id"] = session_id

        def snapshot():
            current = api("sessions/" + session_id)
            report["snapshots"].append(current)
            save()
            if current["operation"]["state"] != "completed":
                raise RuntimeError(f"Turn did not complete: {current['operation']}")
            return current

        snapshot()
        run([str(binary), "--profile", "local", "resume", session_id],
            "/remember working language Go\nName one appropriate standard-library package for storing our notes. Answer in one sentence.\n"
            "Suggest a simple file format for the notes. Answer in one sentence.\n/summary\n/exit\n")
        before = snapshot()
        if not before.get("summary") or before.get("summarized_messages", 0) < 2:
            raise RuntimeError("The conversation did not produce a local summary")
        report["capabilities"] = api("capabilities")
        report["mcp_catalog"] = api("mcp")
        run([str(daemon), "--profile", "local", "stop"])
        deadline = time.monotonic() + 10
        while Path(sock).exists() and time.monotonic() < deadline:
            time.sleep(0.05)
        if Path(sock).exists():
            raise RuntimeError("Local daemon did not stop")
        run([str(binary), "--profile", "local", "resume", session_id],
            "What is our project name, language and purpose? Answer in one sentence.\n/summary\n/exit\n")
        after = snapshot()
        memory_file = Path(cfg["daemon"]["session_dir"]).expanduser() / "profiles/local/projects" / after["project_id"] / "working.ini"
        report["working_memory_ini"] = memory_file.read_text()
        if after["settings"]["provider"] != "ollama":
            raise RuntimeError("Resumed session changed provider")
        endpoint = cfg["ollama"]["endpoint"].rstrip("/")
        for name in ("version", "ps"):
            with urllib.request.urlopen(endpoint + "/api/" + name, timeout=10) as response:
                report["ollama_" + name] = json.load(response)
        result = run(["ollama", "ps"])
        report["ollama_ps_text"] = result.stdout
        report["status"] = "completed"
        save()
        print(f"Saved {output}", flush=True)


if __name__ == "__main__":
    main()
