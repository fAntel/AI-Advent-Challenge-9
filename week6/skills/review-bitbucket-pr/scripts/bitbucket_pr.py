#!/usr/bin/env python3
"""Bitbucket Cloud bridge for pull-request review actions."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


API_BASE = os.environ.get("BITBUCKET_API_BASE_URL", "https://api.bitbucket.org/2.0").rstrip("/")
ANNOTATION_HEADER = re.compile(
    r"^## (?P<path>.+?)(?::(?P<start>\d+)(?:-(?P<end>\d+))?)? "
    r"\((?P<kind>\+|-|file-level)\)$"
)
HUNK_HEADER = re.compile(r"^@@ -(?P<old>\d+)(?:,\d+)? \+(?P<new>\d+)(?:,\d+)? @@")
MERGE_POLL_ATTEMPTS = 60
MERGE_POLL_SECONDS = 1.0


class BridgeError(RuntimeError):
    pass


@dataclass
class DiffFile:
    api_path: str
    aliases: set[str] = field(default_factory=set)
    old_lines: set[int] = field(default_factory=set)
    new_lines: set[int] = field(default_factory=set)


def git_remote_url(remote: str) -> str:
    result = subprocess.run(
        ["git", "remote", "get-url", remote],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise BridgeError(result.stderr.strip() or f"Git remote {remote!r} was not found")
    return result.stdout.strip()


def parse_bitbucket_remote(url: str) -> tuple[str, str]:
    patterns = (
        r"^git@bitbucket\.org:(?P<workspace>[^/]+)/(?P<repo>.+?)(?:\.git)?$",
        r"^ssh://git@bitbucket\.org/(?P<workspace>[^/]+)/(?P<repo>.+?)(?:\.git)?$",
        r"^https?://(?:[^/@]+@)?bitbucket\.org/(?P<workspace>[^/]+)/(?P<repo>.+?)(?:\.git)?/?$",
    )
    for pattern in patterns:
        match = re.match(pattern, url)
        if match:
            return match.group("workspace"), match.group("repo")
    raise BridgeError(f"Remote is not a supported Bitbucket Cloud URL: {url}")


def repository(remote: str) -> tuple[str, str]:
    return parse_bitbucket_remote(git_remote_url(remote))


def token() -> str:
    value = os.environ.get("BITBUCKET_API_TOKEN", "")
    if not value:
        raise BridgeError("BITBUCKET_API_TOKEN is not set")
    return value


def api_url(workspace: str, repo: str, suffix: str) -> str:
    parts = [urllib.parse.quote(part, safe="") for part in (workspace, repo)]
    return f"{API_BASE}/repositories/{parts[0]}/{parts[1]}/{suffix.lstrip('/')}"


def request_with_headers(
    method: str,
    url: str,
    body: dict[str, Any] | None = None,
) -> tuple[int, dict[str, str], bytes]:
    data = None if body is None else json.dumps(body).encode("utf-8")
    headers = {"Authorization": f"Bearer {token()}", "Accept": "application/json"}
    if data is not None:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            response_headers = {name.lower(): value for name, value in response.headers.items()}
            return response.status, response_headers, response.read()
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace").strip()
        raise BridgeError(f"Bitbucket API {method} failed with HTTP {exc.code}: {detail}") from exc
    except urllib.error.URLError as exc:
        raise BridgeError(f"Bitbucket API request failed: {exc.reason}") from exc


def request(method: str, url: str, body: dict[str, Any] | None = None) -> tuple[int, bytes]:
    status, _, raw = request_with_headers(method, url, body)
    return status, raw


def pr_metadata(workspace: str, repo: str, pr: int) -> dict[str, Any]:
    _, raw = request("GET", api_url(workspace, repo, f"pullrequests/{pr}"))
    try:
        value = json.loads(raw)
        return {
            "workspace": workspace,
            "repository": repo,
            "id": value["id"],
            "title": value["title"],
            "state": value["state"],
            "source_branch": value["source"]["branch"]["name"],
            "source_hash": value["source"]["commit"]["hash"],
            "destination_branch": value["destination"]["branch"]["name"],
            "url": value["links"]["html"]["href"],
        }
    except (KeyError, TypeError, json.JSONDecodeError) as exc:
        raise BridgeError("Bitbucket returned an unexpected pull-request response") from exc


def list_pull_requests(workspace: str, repo: str, state: str, limit: int) -> list[dict[str, Any]]:
    values: list[dict[str, Any]] = []
    page = 1
    while len(values) < limit:
        query = urllib.parse.urlencode(
            {
                "state": state,
                "sort": "-updated_on",
                "pagelen": min(50, limit - len(values)),
                "page": page,
            }
        )
        _, raw = request("GET", api_url(workspace, repo, f"pullrequests?{query}"))
        try:
            response = json.loads(raw)
            page_values = response["values"]
        except (KeyError, TypeError, json.JSONDecodeError) as exc:
            raise BridgeError("Bitbucket returned an unexpected pull-request list response") from exc
        if not isinstance(page_values, list):
            raise BridgeError("Bitbucket returned an unexpected pull-request list response")
        values.extend(page_values[: limit - len(values)])
        if not response.get("next") or not page_values:
            break
        page += 1
    return values


def safe_text(value: Any) -> str:
    return " ".join(str(value or "").split())


def markdown_text(value: Any) -> str:
    return (
        safe_text(value)
        .replace("\\", "\\\\")
        .replace("|", "\\|")
        .replace("[", "\\[")
        .replace("]", "\\]")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace("`", "'")
    )


def summarize_pull_request(workspace: str, repo: str, value: dict[str, Any]) -> dict[str, Any]:
    try:
        reviewers = value.get("reviewers") or []
        participants = value.get("participants") or []
        approved = [participant for participant in participants if participant.get("approved") is True]
        change_requests = [
            safe_text(participant.get("user", {}).get("display_name"))
            for participant in participants
            if str(participant.get("state", "")).lower() in {"changes_requested", "changes requested"}
        ]
        pr = int(value["id"])
        return {
            "id": pr,
            "title": safe_text(value["title"]),
            "author": safe_text(value.get("author", {}).get("display_name")) or "unknown",
            "source_branch": safe_text(value["source"]["branch"]["name"]),
            "destination_branch": safe_text(value["destination"]["branch"]["name"]),
            "updated_on": safe_text(value.get("updated_on")),
            "draft": bool(value.get("draft")),
            "approvals": len(approved),
            "reviewers": len(reviewers),
            "changes_requested_by": [name for name in change_requests if name],
            "comments": int(value.get("comment_count") or 0),
            "tasks": int(value.get("task_count") or 0),
            "url": f"https://bitbucket.org/{urllib.parse.quote(workspace, safe='')}/"
            f"{urllib.parse.quote(repo, safe='')}/pull-requests/{pr}",
        }
    except (KeyError, TypeError, ValueError) as exc:
        raise BridgeError("Bitbucket returned an unexpected pull request in the list") from exc


def render_pull_request_list(workspace: str, repo: str, pull_requests: list[dict[str, Any]]) -> str:
    lines = [f"Open pull requests for `{workspace}/{repo}` ({len(pull_requests)})", ""]
    if not pull_requests:
        lines.append("No open pull requests.")
        return "\n".join(lines)
    lines.extend(
        [
            "| PR | Author | Branches | Review | Activity |",
            "|---:|---|---|---|---|",
        ]
    )
    for pr in pull_requests:
        title = markdown_text(pr["title"])
        draft = " **DRAFT**" if pr["draft"] else ""
        changes = ""
        if pr["changes_requested_by"]:
            changes = "; changes requested by " + ", ".join(markdown_text(v) for v in pr["changes_requested_by"])
        review = f"{pr['approvals']}/{pr['reviewers']} approvals{changes}"
        activity = f"{pr['comments']} comments; {pr['tasks']} tasks; {markdown_text(pr['updated_on'])}"
        branches = f"`{markdown_text(pr['source_branch'])}` → `{markdown_text(pr['destination_branch'])}`"
        lines.append(
            f"| [#{pr['id']} — {title}]({pr['url']}){draft} | {markdown_text(pr['author'])} | "
            f"{branches} | {review} | {activity} |"
        )
    return "\n".join(lines)


def normalize_patch_path(value: str) -> str | None:
    value = value.strip()
    if value == "/dev/null":
        return None
    if value.startswith('"'):
        try:
            value = json.loads(value)
        except json.JSONDecodeError as exc:
            raise BridgeError(f"Unsupported quoted diff path: {value}") from exc
    if value.startswith(("a/", "b/")):
        value = value[2:]
    return value


def parse_diff(diff_text: str) -> dict[str, DiffFile]:
    files: dict[str, DiffFile] = {}
    current: DiffFile | None = None
    old_path: str | None = None
    old_line = new_line = 0
    in_hunk = False

    for raw_line in diff_text.splitlines():
        if raw_line.startswith("diff --git "):
            current = None
            old_path = None
            in_hunk = False
            continue
        if not in_hunk and raw_line.startswith("--- "):
            old_path = normalize_patch_path(raw_line[4:])
            in_hunk = False
            continue
        if not in_hunk and raw_line.startswith("+++ "):
            new_path = normalize_patch_path(raw_line[4:])
            api_path = new_path or old_path
            if api_path is None:
                raise BridgeError("Diff file has neither an old nor a new path")
            current = DiffFile(api_path=api_path)
            current.aliases.add(api_path)
            if old_path:
                current.aliases.add(old_path)
            if new_path:
                current.aliases.add(new_path)
            for alias in current.aliases:
                if alias in files and files[alias] is not current:
                    raise BridgeError(f"Ambiguous path appears more than once in diff: {alias}")
                files[alias] = current
            in_hunk = False
            continue
        match = HUNK_HEADER.match(raw_line)
        if match and current is not None:
            old_line = int(match.group("old"))
            new_line = int(match.group("new"))
            in_hunk = True
            continue
        if not in_hunk or current is None or not raw_line:
            continue
        prefix = raw_line[0]
        if prefix == " ":
            current.old_lines.add(old_line)
            current.new_lines.add(new_line)
            old_line += 1
            new_line += 1
        elif prefix == "-":
            current.old_lines.add(old_line)
            old_line += 1
        elif prefix == "+":
            current.new_lines.add(new_line)
            new_line += 1
        elif prefix == "\\":
            continue
        else:
            in_hunk = False
    return files


def parse_annotations(text: str) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    current: dict[str, Any] | None = None
    body: list[str] = []

    def finish() -> None:
        nonlocal current, body
        if current is None:
            return
        while body and body[-1] == "":
            body.pop()
        content = "\n".join(line[1:] if line.startswith(" ## ") else line for line in body).strip()
        if not content:
            raise BridgeError(f"Annotation for {current['path']} has an empty body")
        current["content"] = content
        records.append(current)
        current = None
        body = []

    for line in text.splitlines():
        match = ANNOTATION_HEADER.match(line)
        if match:
            finish()
            start = int(match.group("start")) if match.group("start") else None
            end = int(match.group("end")) if match.group("end") else start
            current = {
                "path": match.group("path"),
                "kind": match.group("kind"),
                "start": start,
                "end": end,
            }
        elif current is not None:
            body.append(line)
        elif line.strip():
            raise BridgeError(f"Unexpected text before first annotation: {line}")
    finish()
    if not records:
        raise BridgeError("No revdiff annotations were found")
    return records


def comment_payload(annotation: dict[str, Any], files: dict[str, DiffFile]) -> tuple[dict[str, Any], str]:
    path = annotation["path"]
    kind = annotation["kind"]
    content = annotation["content"]
    if path not in files:
        raise BridgeError(f"Annotation path is not present in the saved diff: {path}")
    diff_file = files[path]
    if kind == "file-level":
        return {"content": {"raw": f"**File:** {markdown_text(diff_file.api_path)}\n\n{content}"}}, "global"

    start = annotation["start"]
    end = annotation["end"]
    if start is None or end is None or end < start:
        raise BridgeError(f"Invalid line range for {path}: {start}-{end}")
    valid_lines = diff_file.new_lines if kind == "+" else diff_file.old_lines
    missing = [line for line in range(start, end + 1) if line not in valid_lines]
    if missing:
        sample = ", ".join(str(line) for line in missing[:5])
        raise BridgeError(f"Annotation range {path}:{start}-{end} is not in the saved diff (missing {sample})")

    inline: dict[str, Any] = {"path": diff_file.api_path}
    if kind == "+":
        inline["to"] = end
        if start != end:
            inline["start_to"] = start
    else:
        inline["from"] = end
        if start != end:
            inline["start_from"] = start
    return {"content": {"raw": content}, "inline": inline}, "inline"


def load_json(path: str) -> dict[str, Any]:
    try:
        return json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise BridgeError(f"Could not read JSON from {path}: {exc}") from exc


def write_json(path: str, value: dict[str, Any]) -> None:
    Path(path).write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def ensure_open(metadata: dict[str, Any]) -> None:
    if metadata["state"] != "OPEN":
        raise BridgeError(f"Pull request #{metadata['id']} is {metadata['state']}, not OPEN")


def run_git(arguments: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *arguments],
        check=False,
        capture_output=True,
        text=True,
    )


def git_failure(result: subprocess.CompletedProcess[str]) -> str:
    return safe_text(result.stderr) or safe_text(result.stdout) or f"git exited with status {result.returncode}"


def ensure_local_pull_ready(remote: str, destination_branch: str) -> None:
    branch = run_git(["branch", "--show-current"])
    if branch.returncode != 0:
        raise BridgeError(f"Could not determine the current Git branch: {git_failure(branch)}")
    current_branch = branch.stdout.strip()
    if not current_branch:
        raise BridgeError("Cannot merge and pull while Git is in detached HEAD state")
    if current_branch != destination_branch:
        raise BridgeError(
            f"Current Git branch is {current_branch!r}; check out destination branch "
            f"{destination_branch!r} before merging"
        )

    status = run_git(["status", "--porcelain", "--untracked-files=no"])
    if status.returncode != 0:
        raise BridgeError(f"Could not inspect the Git worktree: {git_failure(status)}")
    if status.stdout.strip():
        raise BridgeError("Tracked Git changes are present; commit or stash them before merging and pulling")

    fetch = run_git(["fetch", "--quiet", remote, destination_branch])
    if fetch.returncode != 0:
        raise BridgeError(f"Could not fetch {remote}/{destination_branch}: {git_failure(fetch)}")
    ancestor = run_git(["merge-base", "--is-ancestor", "HEAD", "FETCH_HEAD"])
    if ancestor.returncode == 1:
        raise BridgeError(
            f"Current branch cannot fast-forward to {remote}/{destination_branch}; "
            "reconcile the local branch before merging"
        )
    if ancestor.returncode != 0:
        raise BridgeError(f"Could not compare the local and remote destination branches: {git_failure(ancestor)}")


def merge_task_reason(value: dict[str, Any]) -> str:
    error = value.get("error")
    if isinstance(error, dict):
        parts = [safe_text(error.get("message")), safe_text(error.get("detail"))]
        message = ": ".join(part for part in parts if part)
        if message:
            return message
    elif error:
        return safe_text(error)
    for key in ("message", "detail"):
        message = safe_text(value.get(key))
        if message:
            return message
    return safe_text(json.dumps(value, ensure_ascii=False)) or "Bitbucket returned no failure detail"


def merge_pull_request(workspace: str, repo: str, pr: int) -> dict[str, Any]:
    endpoint = api_url(workspace, repo, f"pullrequests/{pr}/merge")
    status, headers, raw = request_with_headers("POST", endpoint)
    if status == 200:
        try:
            response = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise BridgeError("Bitbucket returned invalid JSON after merging the pull request") from exc
        if not isinstance(response, dict):
            raise BridgeError("Bitbucket returned an unexpected response after merging the pull request")
        if response.get("state") != "MERGED":
            raise BridgeError(f"Bitbucket returned HTTP 200 but PR #{pr} state is {response.get('state')!r}")
        return response
    if status != 202:
        raise BridgeError(f"Bitbucket returned unexpected HTTP {status} for the merge request")

    location = headers.get("location")
    if not location:
        raise BridgeError(
            "Bitbucket accepted the merge asynchronously but omitted its status URL; "
            "check the PR state before retrying"
        )
    status_url = urllib.parse.urljoin(f"{API_BASE}/", location)
    for attempt in range(MERGE_POLL_ATTEMPTS):
        _, _, task_raw = request_with_headers("GET", status_url)
        try:
            task = json.loads(task_raw)
        except json.JSONDecodeError as exc:
            raise BridgeError("Bitbucket returned invalid JSON for the merge task") from exc
        if not isinstance(task, dict):
            raise BridgeError("Bitbucket returned an unexpected merge task response")
        task_status = safe_text(task.get("task_status")).upper()
        if task_status in {"SUCCESS", "COMPLETED"}:
            metadata = pr_metadata(workspace, repo, pr)
            if metadata["state"] != "MERGED":
                raise BridgeError(
                    f"Bitbucket merge task succeeded but PR #{pr} state is {metadata['state']}; "
                    "check the PR before retrying"
                )
            return metadata
        if task_status not in {"PENDING", "IN_PROGRESS"}:
            raise BridgeError(f"Bitbucket merge task {task_status or 'failed'}: {merge_task_reason(task)}")
        if attempt + 1 < MERGE_POLL_ATTEMPTS:
            time.sleep(MERGE_POLL_SECONDS)
    raise BridgeError("Timed out waiting for Bitbucket's merge task; check the PR state before retrying")


def pull_destination(remote: str, destination_branch: str) -> str:
    pull = run_git(["pull", "--ff-only", remote, destination_branch])
    if pull.returncode != 0:
        raise BridgeError(f"git pull --ff-only failed: {git_failure(pull)}")
    head = run_git(["rev-parse", "HEAD"])
    if head.returncode != 0:
        raise BridgeError(f"Pull completed but the local HEAD could not be read: {git_failure(head)}")
    return head.stdout.strip()


def command_doctor(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    token()
    if not os.environ.get("TMPDIR"):
        raise BridgeError("TMPDIR is not set")
    print(
        json.dumps(
            {
                "workspace": workspace,
                "repository": repo,
                "token_configured": True,
                "required_token_scopes": [
                    "read:repository:bitbucket",
                    "read:pullrequest:bitbucket",
                    "write:pullrequest:bitbucket",
                ],
            }
        )
    )


def command_metadata(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    print(json.dumps(pr_metadata(workspace, repo, args.pr), indent=2, ensure_ascii=False))


def command_list(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    values = list_pull_requests(workspace, repo, args.state, args.limit)
    pull_requests = [summarize_pull_request(workspace, repo, value) for value in values]
    if args.format == "json":
        print(json.dumps(pull_requests, indent=2, ensure_ascii=False))
    else:
        print(render_pull_request_list(workspace, repo, pull_requests))


def command_diff(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    before = pr_metadata(workspace, repo, args.pr)
    ensure_open(before)
    _, raw = request("GET", api_url(workspace, repo, f"pullrequests/{args.pr}/diff"))
    after = pr_metadata(workspace, repo, args.pr)
    ensure_open(after)
    if after["source_hash"] != before["source_hash"]:
        raise BridgeError(
            f"Pull request source changed while fetching the diff: {before['source_hash']} -> "
            f"{after['source_hash']}. Fetch again."
        )
    if args.output:
        Path(args.output).write_bytes(raw)
        print(json.dumps({"output": args.output, "source_hash": after["source_hash"]}))
    else:
        sys.stdout.buffer.write(raw)


def command_prepare(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    metadata = pr_metadata(workspace, repo, args.pr)
    ensure_open(metadata)
    if metadata["source_hash"] != args.source_hash:
        raise BridgeError(
            f"Pull request source changed: diff is {args.source_hash}, current is {metadata['source_hash']}. Review again."
        )
    annotations = parse_annotations(Path(args.annotations).read_text(encoding="utf-8"))
    files = parse_diff(Path(args.diff).read_text(encoding="utf-8", errors="replace"))
    comments = []
    for index, annotation in enumerate(annotations, start=1):
        payload, placement = comment_payload(annotation, files)
        comments.append(
            {
                "index": index,
                "source": annotation,
                "placement": placement,
                "payload": payload,
            }
        )
    manifest = {
        "version": 1,
        "workspace": workspace,
        "repository": repo,
        "pull_request": args.pr,
        "source_hash": args.source_hash,
        "pull_request_url": metadata["url"],
        "comments": comments,
    }
    write_json(args.output, manifest)
    print(json.dumps({"output": args.output, "comments": len(comments), "source_hash": metadata["source_hash"]}))


def command_prepare_global(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    metadata = pr_metadata(workspace, repo, args.pr)
    ensure_open(metadata)
    if metadata["source_hash"] != args.source_hash:
        raise BridgeError(
            f"Pull request source changed: expected {args.source_hash}, current {metadata['source_hash']}. Review again."
        )
    content = Path(args.body).read_text(encoding="utf-8").strip()
    if not content:
        raise BridgeError("Global comment body is empty")
    manifest = {
        "version": 1,
        "workspace": workspace,
        "repository": repo,
        "pull_request": args.pr,
        "source_hash": args.source_hash,
        "pull_request_url": metadata["url"],
        "comments": [
            {
                "index": 1,
                "source": {
                    "path": None,
                    "kind": "global",
                    "start": None,
                    "end": None,
                    "content": content,
                },
                "placement": "global",
                "payload": {"content": {"raw": content}},
            }
        ],
    }
    write_json(args.output, manifest)
    print(json.dumps({"output": args.output, "comments": 1, "source_hash": metadata["source_hash"]}))


def command_preview(args: argparse.Namespace) -> None:
    manifest = load_json(args.manifest)
    print(
        f"Bitbucket PR #{manifest['pull_request']} at {manifest['source_hash']} "
        f"({manifest['workspace']}/{manifest['repository']})"
    )
    for comment in manifest["comments"]:
        source = comment["source"]
        if source["kind"] == "global":
            location = "global comment"
        elif source["kind"] == "file-level":
            location = f"{source['path']} (global comment; file-level annotation)"
        else:
            end = source["end"]
            line_range = str(source["start"]) if source["start"] == end else f"{source['start']}-{end}"
            location = f"{source['path']}:{line_range} ({source['kind']})"
        print(f"\n[{comment['index']}] {location}\n{source['content']}")


def verify_manifest(manifest: dict[str, Any], confirm_source: str) -> dict[str, Any]:
    expected = manifest.get("source_hash")
    if confirm_source != expected:
        raise BridgeError("--confirm-source must exactly match the manifest source hash")
    metadata = pr_metadata(manifest["workspace"], manifest["repository"], manifest["pull_request"])
    ensure_open(metadata)
    if metadata["source_hash"] != expected:
        raise BridgeError(
            f"Pull request source changed: reviewed {expected}, current {metadata['source_hash']}. Review again."
        )
    return metadata


def command_publish(args: argparse.Namespace) -> None:
    manifest = load_json(args.manifest)
    current_repository = repository(args.remote)
    if current_repository != (manifest.get("workspace"), manifest.get("repository")):
        raise BridgeError("Manifest repository does not match the selected Git remote")
    verify_manifest(manifest, args.confirm_source)
    endpoint = api_url(
        manifest["workspace"],
        manifest["repository"],
        f"pullrequests/{manifest['pull_request']}/comments",
    )
    created: list[dict[str, Any]] = []
    for comment in manifest["comments"]:
        try:
            if created:
                verify_manifest(manifest, args.confirm_source)
            _, raw = request("POST", endpoint, comment["payload"])
            response = json.loads(raw)
            created.append({"index": comment["index"], "id": response.get("id")})
            print(json.dumps({"created": created[-1]}), flush=True)
        except (BridgeError, json.JSONDecodeError) as exc:
            raise BridgeError(
                f"Stopped at comment {comment['index']}; already created: {json.dumps(created)}; error: {exc}"
            ) from exc
    print(json.dumps({"published": len(created), "comments": created}))


def command_approve(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    manifest_stub = {
        "workspace": workspace,
        "repository": repo,
        "pull_request": args.pr,
        "source_hash": args.confirm_source,
    }
    metadata = verify_manifest(manifest_stub, args.confirm_source)
    _, raw = request("POST", api_url(workspace, repo, f"pullrequests/{args.pr}/approve"))
    response = json.loads(raw)
    print(json.dumps({"pull_request": args.pr, "source_hash": metadata["source_hash"], "approved": response.get("approved")}))


def command_decline(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    manifest_stub = {
        "workspace": workspace,
        "repository": repo,
        "pull_request": args.pr,
        "source_hash": args.confirm_source,
    }
    metadata = verify_manifest(manifest_stub, args.confirm_source)
    _, raw = request("POST", api_url(workspace, repo, f"pullrequests/{args.pr}/decline"))
    try:
        response = json.loads(raw)
        state = response["state"]
        declined_source = response["source"]["commit"]["hash"]
    except (KeyError, TypeError, json.JSONDecodeError) as exc:
        raise BridgeError(
            f"Bitbucket returned an unexpected response after declining PR #{args.pr}; "
            "check the PR state before taking any further action"
        ) from exc
    if state != "DECLINED":
        raise BridgeError(
            f"Bitbucket returned after the decline request with PR #{args.pr} state {state!r}; "
            "check the PR state before retrying"
        )
    result = {
        "pull_request": args.pr,
        "source_hash": declined_source,
        "destination_branch": metadata["destination_branch"],
        "declined": True,
    }
    print(json.dumps(result), flush=True)
    if declined_source != metadata["source_hash"]:
        raise BridgeError(
            f"Bitbucket declined PR #{args.pr}, but its source changed from the confirmed hash "
            f"{metadata['source_hash']} to {declined_source} during the operation. The decline is irreversible."
        )


def command_merge(args: argparse.Namespace) -> None:
    workspace, repo = repository(args.remote)
    manifest_stub = {
        "workspace": workspace,
        "repository": repo,
        "pull_request": args.pr,
        "source_hash": args.confirm_source,
    }
    metadata = verify_manifest(manifest_stub, args.confirm_source)
    try:
        ensure_local_pull_ready(args.remote, metadata["destination_branch"])
    except BridgeError as exc:
        raise BridgeError(f"Merge was not attempted because the local pull preflight failed: {exc}") from exc
    try:
        merge_pull_request(workspace, repo, args.pr)
    except BridgeError as exc:
        raise BridgeError(
            f"Bitbucket did not confirm merge of PR #{args.pr}; local pull was not attempted: {exc}"
        ) from exc
    try:
        local_head = pull_destination(args.remote, metadata["destination_branch"])
    except BridgeError as exc:
        print(
            json.dumps(
                {
                    "pull_request": args.pr,
                    "source_hash": metadata["source_hash"],
                    "destination_branch": metadata["destination_branch"],
                    "merged": True,
                    "pulled": False,
                }
            ),
            flush=True,
        )
        raise BridgeError(
            f"Bitbucket merged PR #{args.pr}, but the local destination branch was not updated: {exc}"
        ) from exc
    print(
        json.dumps(
            {
                "pull_request": args.pr,
                "source_hash": metadata["source_hash"],
                "destination_branch": metadata["destination_branch"],
                "merged": True,
                "pulled": True,
                "local_head": local_head,
            }
        )
    )


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--remote", default="origin", help="Git remote used to derive workspace/repository")
    sub = result.add_subparsers(dest="command", required=True)

    doctor = sub.add_parser("doctor")
    doctor.set_defaults(func=command_doctor)

    metadata = sub.add_parser("metadata")
    metadata.add_argument("pr", type=int)
    metadata.set_defaults(func=command_metadata)

    list_command = sub.add_parser("list")
    list_command.add_argument("--state", default="OPEN", choices=("OPEN", "MERGED", "DECLINED", "SUPERSEDED"))
    list_command.add_argument("--limit", type=int, default=50, choices=range(1, 101), metavar="1..100")
    list_command.add_argument("--format", choices=("markdown", "json"), default="markdown")
    list_command.set_defaults(func=command_list)

    diff = sub.add_parser("diff")
    diff.add_argument("pr", type=int)
    diff.add_argument("--output")
    diff.set_defaults(func=command_diff)

    prepare = sub.add_parser("prepare-comments")
    prepare.add_argument("pr", type=int)
    prepare.add_argument("--annotations", required=True)
    prepare.add_argument("--diff", required=True)
    prepare.add_argument("--source-hash", required=True)
    prepare.add_argument("--output", required=True)
    prepare.set_defaults(func=command_prepare)

    prepare_global = sub.add_parser("prepare-global-comment")
    prepare_global.add_argument("pr", type=int)
    prepare_global.add_argument("--body", required=True)
    prepare_global.add_argument("--source-hash", required=True)
    prepare_global.add_argument("--output", required=True)
    prepare_global.set_defaults(func=command_prepare_global)

    preview = sub.add_parser("preview-comments")
    preview.add_argument("--manifest", required=True)
    preview.set_defaults(func=command_preview)

    publish = sub.add_parser("publish-comments")
    publish.add_argument("--manifest", required=True)
    publish.add_argument("--confirm-source", required=True)
    publish.set_defaults(func=command_publish)

    approve = sub.add_parser("approve")
    approve.add_argument("pr", type=int)
    approve.add_argument("--confirm-source", required=True)
    approve.set_defaults(func=command_approve)

    decline = sub.add_parser("decline")
    decline.add_argument("pr", type=int)
    decline.add_argument("--confirm-source", required=True)
    decline.set_defaults(func=command_decline)

    merge = sub.add_parser("merge")
    merge.add_argument("pr", type=int)
    merge.add_argument("--confirm-source", required=True)
    merge.set_defaults(func=command_merge)
    return result


def main() -> int:
    args = parser().parse_args()
    try:
        args.func(args)
        return 0
    except (BridgeError, OSError, UnicodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
