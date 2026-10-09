#!/usr/bin/env python3
"""Install the isolated local profile without overwriting existing config."""
import json
import os
from pathlib import Path


def xdg(name, fallback):
    value = os.environ.get(name, "")
    return Path(value) if Path(value).is_absolute() else Path.home() / fallback


def main():
    source = Path(__file__).resolve().parent.parent / "config.local.example.toml"
    state = xdg("XDG_STATE_HOME", ".local/state") / "advent-agent/local"
    config = xdg("XDG_CONFIG_HOME", ".config") / "advent-agent/profiles/local"
    text = source.read_text()
    for suffix in ("/agent.sock", "/agentd.log", ""):
        text = text.replace(json.dumps("~/.local/state/advent-agent/local" + suffix), json.dumps(str(state) + suffix))
    path = config / "config.toml"
    if path.exists():
        if path.read_text() != text:
            raise SystemExit(f"Existing profile preserved: {path}. Review it before installing a different configuration.")
        print(f"Local profile already installed: {path}")
        return
    config.mkdir(parents=True, exist_ok=True, mode=0o700)
    # Exclusive creation also protects against a concurrent install.
    with path.open("x") as output:
        output.write(text)
    path.chmod(0o600)
    print(f"Installed local profile: {path}")
    print(f"Isolated daemon socket: {state / 'agent.sock'}")


if __name__ == "__main__":
    main()
