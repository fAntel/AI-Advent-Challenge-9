#!/bin/sh
# Use the newly built week5 artifacts and isolated VPS chat state.
set -eu
vps_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
export XDG_CONFIG_HOME="$vps_root/data/task4-config"
export XDG_STATE_HOME="$vps_root/data/task4-state"
python3 "$vps_root/scripts/install-local-profile.py" --profile vps >/dev/null
# Repository instructions alone can exceed the VPS's 1,024-token context.
# ProjectIdentity resolves nested directories to the Git root, so this must
# be outside the repository. Session data still lives under task4-state.
vps_project="${TMPDIR:-/tmp}/advent-agent-vps-chat-$(id -u)"
mkdir -p "$vps_project"
chmod 0700 "$vps_project"
cd "$vps_project"
if [ "$#" -eq 0 ]; then
    set -- --chat --stats
fi
exec "$vps_root/advent-agent" --profile vps "$@"
