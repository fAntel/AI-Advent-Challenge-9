#!/usr/bin/env python3

import importlib.util
import io
import json
import sys
import tempfile
import unittest
from argparse import Namespace
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("bitbucket_pr.py")
SPEC = importlib.util.spec_from_file_location("bitbucket_pr", MODULE_PATH)
assert SPEC and SPEC.loader
bb = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = bb
SPEC.loader.exec_module(bb)


DIFF = """diff --git a/src/Old.kt b/src/New.kt
similarity index 90%
rename from src/Old.kt
rename to src/New.kt
--- a/src/Old.kt
+++ b/src/New.kt
@@ -10,4 +10,5 @@
 context
-removed
+added
+second added
 context
"""


class BitbucketPrTest(unittest.TestCase):
    def test_parses_supported_remotes(self):
        expected = ("example-workspace", "example-sdk")
        self.assertEqual(expected, bb.parse_bitbucket_remote("git@bitbucket.org:example-workspace/example-sdk.git"))
        self.assertEqual(expected, bb.parse_bitbucket_remote("https://bitbucket.org/example-workspace/example-sdk.git"))
        self.assertEqual(expected, bb.parse_bitbucket_remote("ssh://git@bitbucket.org/example-workspace/example-sdk.git"))

    def test_maps_added_removed_ranges_and_file_level_comments(self):
        files = bb.parse_diff(DIFF)
        annotations = bb.parse_annotations(
            """## src/New.kt:11-12 (+)
new-side note

## src/New.kt:11 (-)
old-side note

## src/New.kt (file-level)
whole file note
"""
        )
        added, placement = bb.comment_payload(annotations[0], files)
        self.assertEqual("inline", placement)
        self.assertEqual({"path": "src/New.kt", "to": 12, "start_to": 11}, added["inline"])
        removed, _ = bb.comment_payload(annotations[1], files)
        self.assertEqual({"path": "src/New.kt", "from": 11}, removed["inline"])
        file_note, placement = bb.comment_payload(annotations[2], files)
        self.assertEqual("global", placement)
        self.assertIn("**File:** src/New.kt", file_note["content"]["raw"])

    def test_rejects_line_outside_saved_diff(self):
        files = bb.parse_diff(DIFF)
        annotation = bb.parse_annotations("## src/New.kt:99 (+)\nnot in diff\n")[0]
        with self.assertRaisesRegex(bb.BridgeError, "not in the saved diff"):
            bb.comment_payload(annotation, files)

    def test_diff_content_that_looks_like_a_file_header_stays_in_hunk(self):
        diff = """diff --git a/a.txt b/a.txt
--- a/a.txt
+++ b/a.txt
@@ -1 +1 @@
---- removed header-like text
++++ added header-like text
"""
        files = bb.parse_diff(diff)
        self.assertEqual({1}, files["a.txt"].old_lines)
        self.assertEqual({1}, files["a.txt"].new_lines)

    def test_annotation_body_header_escape_round_trips(self):
        annotation = bb.parse_annotations("## src/New.kt:11 (+)\n ## heading in body\n")[0]
        self.assertEqual("## heading in body", annotation["content"])

    def test_json_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "manifest.json")
            bb.write_json(path, {"value": "Привет"})
            self.assertEqual({"value": "Привет"}, bb.load_json(path))

    def test_prepares_and_previews_standalone_global_comment(self):
        metadata = {
            "id": 5,
            "state": "OPEN",
            "source_hash": "abc123",
            "url": "https://bitbucket.org/example-workspace/sdk/pull-requests/5",
        }
        with tempfile.TemporaryDirectory() as directory:
            body = Path(directory) / "comment.md"
            manifest_path = Path(directory) / "manifest.json"
            body.write_text("Declined as superseded.\n\nReplacement: `def456`.", encoding="utf-8")
            with (
                mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
                mock.patch.object(bb, "pr_metadata", return_value=metadata),
                redirect_stdout(io.StringIO()),
            ):
                bb.command_prepare_global(
                    Namespace(
                        remote="origin",
                        pr=5,
                        body=str(body),
                        source_hash="abc123",
                        output=str(manifest_path),
                    )
                )

            manifest = bb.load_json(str(manifest_path))
            self.assertEqual("global", manifest["comments"][0]["source"]["kind"])
            self.assertEqual(
                "Declined as superseded.\n\nReplacement: `def456`.",
                manifest["comments"][0]["payload"]["content"]["raw"],
            )
            preview = io.StringIO()
            with redirect_stdout(preview):
                bb.command_preview(Namespace(manifest=str(manifest_path)))
            self.assertIn("[1] global comment", preview.getvalue())

    def test_rejects_empty_global_comment(self):
        metadata = {
            "id": 5,
            "state": "OPEN",
            "source_hash": "abc123",
            "url": "https://bitbucket.org/example-workspace/sdk/pull-requests/5",
        }
        with tempfile.TemporaryDirectory() as directory:
            body = Path(directory) / "comment.md"
            body.write_text(" \n", encoding="utf-8")
            with (
                mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
                mock.patch.object(bb, "pr_metadata", return_value=metadata),
            ):
                with self.assertRaisesRegex(bb.BridgeError, "body is empty"):
                    bb.command_prepare_global(
                        Namespace(
                            remote="origin",
                            pr=5,
                            body=str(body),
                            source_hash="abc123",
                            output=str(Path(directory) / "manifest.json"),
                        )
                    )

    def test_renders_clickable_review_queue(self):
        raw = {
            "id": 42,
            "title": "Fix [sync] | retry",
            "author": {"display_name": "Ada"},
            "source": {"branch": {"name": "feature/sync"}},
            "destination": {"branch": {"name": "develop"}},
            "updated_on": "2026-09-08T10:20:30+00:00",
            "draft": True,
            "reviewers": [{}, {}],
            "participants": [{"approved": True, "state": "approved"}],
            "comment_count": 3,
            "task_count": 1,
        }
        summary = bb.summarize_pull_request("example-workspace", "sdk", raw)
        rendered = bb.render_pull_request_list("example-workspace", "sdk", [summary])
        self.assertIn("[#42 — Fix \\[sync\\] \\| retry](https://bitbucket.org/example-workspace/sdk/pull-requests/42)", rendered)
        self.assertIn("1/2 approvals", rendered)
        self.assertIn("**DRAFT**", rendered)

    def test_pull_request_queue_is_paginated_in_updated_order(self):
        calls = []
        responses = [
            {"values": [{"id": 3}, {"id": 2}], "next": "another-page"},
            {"values": [{"id": 1}]},
        ]
        original_request = bb.request

        def fake_request(method, url, body=None):
            calls.append((method, url, body))
            return 200, json.dumps(responses[len(calls) - 1]).encode()

        bb.request = fake_request
        try:
            values = bb.list_pull_requests("example-workspace", "sdk", "OPEN", 3)
        finally:
            bb.request = original_request

        self.assertEqual([3, 2, 1], [value["id"] for value in values])
        self.assertIn("sort=-updated_on", calls[0][1])
        self.assertIn("page=2", calls[1][1])

    def test_local_pull_preflight_requires_destination_branch(self):
        result = mock.Mock(returncode=0, stdout="feature\n", stderr="")
        with mock.patch.object(bb, "run_git", return_value=result):
            with self.assertRaisesRegex(bb.BridgeError, "check out destination branch 'develop'"):
                bb.ensure_local_pull_ready("origin", "develop")

    def test_local_pull_preflight_fetches_and_requires_fast_forward(self):
        responses = [
            mock.Mock(returncode=0, stdout="develop\n", stderr=""),
            mock.Mock(returncode=0, stdout="", stderr=""),
            mock.Mock(returncode=0, stdout="", stderr=""),
            mock.Mock(returncode=1, stdout="", stderr=""),
        ]
        with mock.patch.object(bb, "run_git", side_effect=responses) as run_git:
            with self.assertRaisesRegex(bb.BridgeError, "cannot fast-forward"):
                bb.ensure_local_pull_ready("origin", "develop")
        self.assertEqual(["fetch", "--quiet", "origin", "develop"], run_git.call_args_list[2].args[0])
        self.assertEqual(
            ["merge-base", "--is-ancestor", "HEAD", "FETCH_HEAD"],
            run_git.call_args_list[3].args[0],
        )

    def test_async_merge_reports_bitbucket_failure_reason(self):
        responses = [
            (202, {"location": "/merge/task/123"}, b""),
            (200, {}, json.dumps({"task_status": "FAILED", "error": {"message": "Merge conflict"}}).encode()),
        ]
        with mock.patch.object(bb, "request_with_headers", side_effect=responses):
            with self.assertRaisesRegex(bb.BridgeError, "Merge conflict"):
                bb.merge_pull_request("example-workspace", "sdk", 5)

    def test_synchronous_merge_requires_merged_state(self):
        response = json.dumps({"id": 5, "state": "MERGED"}).encode()
        with mock.patch.object(bb, "request_with_headers", return_value=(200, {}, response)):
            self.assertEqual("MERGED", bb.merge_pull_request("example-workspace", "sdk", 5)["state"])

    def test_async_merge_waits_for_success_and_verifies_pr_state(self):
        responses = [
            (202, {"location": "/repositories/example-workspace/sdk/pullrequests/5/merge/task-status/123"}, b""),
            (200, {}, json.dumps({"task_status": "PENDING"}).encode()),
            (200, {}, json.dumps({"task_status": "SUCCESS"}).encode()),
        ]
        metadata = {"id": 5, "state": "MERGED"}
        with (
            mock.patch.object(bb, "request_with_headers", side_effect=responses),
            mock.patch.object(bb, "pr_metadata", return_value=metadata),
            mock.patch.object(bb.time, "sleep") as sleep,
        ):
            self.assertEqual(metadata, bb.merge_pull_request("example-workspace", "sdk", 5))
        sleep.assert_called_once_with(bb.MERGE_POLL_SECONDS)

    def test_merge_command_merges_then_pulls_destination(self):
        metadata = {
            "source_hash": "abc123",
            "destination_branch": "develop",
        }
        output = io.StringIO()
        with (
            mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
            mock.patch.object(bb, "verify_manifest", return_value=metadata),
            mock.patch.object(bb, "ensure_local_pull_ready") as preflight,
            mock.patch.object(bb, "merge_pull_request") as merge,
            mock.patch.object(bb, "pull_destination", return_value="def456") as pull,
            redirect_stdout(output),
        ):
            bb.command_merge(Namespace(remote="origin", pr=5, confirm_source="abc123"))

        preflight.assert_called_once_with("origin", "develop")
        merge.assert_called_once_with("example-workspace", "sdk", 5)
        pull.assert_called_once_with("origin", "develop")
        self.assertEqual(
            {
                "pull_request": 5,
                "source_hash": "abc123",
                "destination_branch": "develop",
                "merged": True,
                "pulled": True,
                "local_head": "def456",
            },
            json.loads(output.getvalue()),
        )

    def test_merge_command_reports_remote_success_when_pull_fails(self):
        metadata = {
            "source_hash": "abc123",
            "destination_branch": "develop",
        }
        output = io.StringIO()
        with (
            mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
            mock.patch.object(bb, "verify_manifest", return_value=metadata),
            mock.patch.object(bb, "ensure_local_pull_ready"),
            mock.patch.object(bb, "merge_pull_request"),
            mock.patch.object(bb, "pull_destination", side_effect=bb.BridgeError("network unavailable")),
            redirect_stdout(output),
        ):
            with self.assertRaisesRegex(bb.BridgeError, "merged PR #5.*not updated"):
                bb.command_merge(Namespace(remote="origin", pr=5, confirm_source="abc123"))

        self.assertEqual(
            {
                "pull_request": 5,
                "source_hash": "abc123",
                "destination_branch": "develop",
                "merged": True,
                "pulled": False,
            },
            json.loads(output.getvalue()),
        )

    def test_decline_command_confirms_declined_state_and_source(self):
        metadata = {
            "source_hash": "abc123",
            "destination_branch": "develop",
        }
        response = json.dumps(
            {
                "state": "DECLINED",
                "source": {"commit": {"hash": "abc123"}},
            }
        ).encode()
        output = io.StringIO()
        with (
            mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
            mock.patch.object(bb, "verify_manifest", return_value=metadata),
            mock.patch.object(bb, "request", return_value=(200, response)) as request,
            redirect_stdout(output),
        ):
            bb.command_decline(Namespace(remote="origin", pr=5, confirm_source="abc123"))

        request.assert_called_once_with(
            "POST",
            "https://api.bitbucket.org/2.0/repositories/example-workspace/sdk/pullrequests/5/decline",
        )
        self.assertEqual(
            {
                "pull_request": 5,
                "source_hash": "abc123",
                "destination_branch": "develop",
                "declined": True,
            },
            json.loads(output.getvalue()),
        )

    def test_decline_reports_irreversible_source_race(self):
        metadata = {
            "source_hash": "abc123",
            "destination_branch": "develop",
        }
        response = json.dumps(
            {
                "state": "DECLINED",
                "source": {"commit": {"hash": "changed456"}},
            }
        ).encode()
        with (
            mock.patch.object(bb, "repository", return_value=("example-workspace", "sdk")),
            mock.patch.object(bb, "verify_manifest", return_value=metadata),
            mock.patch.object(bb, "request", return_value=(200, response)),
            redirect_stdout(io.StringIO()),
        ):
            with self.assertRaisesRegex(bb.BridgeError, "decline is irreversible"):
                bb.command_decline(Namespace(remote="origin", pr=5, confirm_source="abc123"))


if __name__ == "__main__":
    unittest.main()
