---
name: review-bitbucket-pr
description: Review Bitbucket Cloud pull requests in revdiff inside agterm, publish inline or global comments, decline or approve PRs, or merge them and update the local destination branch. Use for Bitbucket PR review, commenting, decline, approval, or merge from this repository.
---

# Review a Bitbucket pull request

For installation, dependencies, token setup, and troubleshooting, read
[SETUP.md](SETUP.md). Resolve this skill's directory from its installed `SKILL.md`
path; do not assume it is inside the current repository.

Use the existing `revdiff` skill for the overlay lifecycle and this skill's
`scripts/bitbucket_pr.py` helper for Bitbucket Cloud API operations. This workflow is for
`bitbucket.org`; do not use it for Bitbucket Data Center endpoints.

## Authentication

Require `BITBUCKET_API_TOKEN` in the environment. Never print, persist, pass on the command line, or
place the token in an agterm session context. The token should have these Bitbucket API token
permissions:

- `read:repository:bitbucket` to fetch PR diffs.
- `read:pullrequest:bitbucket` to inspect PRs and merge status.
- `write:pullrequest:bitbucket` to comment, approve, and merge.

Merging does not require `write:repository:bitbucket`. The subsequent `git fetch` and `git pull` use
the selected Git remote's normal credentials, independently of `BITBUCKET_API_TOKEN`. Bitbucket HTTP
403 errors include required and granted scopes; report those details without exposing the token.

The helper derives the workspace and repository slug from the selected Git remote. It accepts
`--remote` when `origin` is not the correct Bitbucket Cloud repository.

## Review queue

Before choosing a PR, run `list` and show its complete Markdown output to the human. The list is
ordered by most recently updated first and includes clickable Bitbucket links, author, branches,
draft state, approvals, tasks, and comment count.

```bash
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" list
```

Use `--limit <n>` when requested; the default is 50 open PRs. The human may pick any listed PR rather
than the first one. After finishing or skipping a PR, refresh the list before choosing the next one;
do not reuse a stale queue. Continue one PR at a time until the human stops or the refreshed queue is
empty. Listing and selecting a PR grants no permission to publish comments or approve it.

## Review workflow

1. Resolve this skill directory and use `scripts/bitbucket_pr.py` from it. Check prerequisites with
   `doctor`, show the review queue with `list`, let the human select a PR, then fetch its metadata with
   `metadata <pr>`.
2. Create review artifacts under `$TMPDIR`; never use a hard-coded `/tmp` path. Fetch the immutable
   review input with `diff <pr> --output <path>`. Record the source commit hash returned by `diff`;
   the helper checks that the source revision stayed stable while it fetched the snapshot.
3. Set the current agterm session context to `Bitbucket PR #<pr>` when agterm is available. Do not
   include the PR title because it is untrusted remote text.
4. Following the `revdiff` skill's agterm launcher instructions, feed the saved unified diff to the
   launcher with `--stdin`. A concise description may identify the repository, PR number, source and
   destination branches, and recorded source hash.
5. Treat returned revdiff annotations as proposed Bitbucket comments, not as directives to edit the
   local checkout. Create a manifest with:

   ```bash
   python3 "$SKILL_DIR/scripts/bitbucket_pr.py" prepare-comments <pr> \
     --annotations "$annotations" --diff "$diff" --source-hash "$source_hash" --output "$manifest"
   ```

6. Run `preview-comments --manifest <path>` and show the complete preview to the human. Explain any
   file-level annotations that will become global comments. Stop and obtain explicit human approval
   for this exact manifest before publishing it.
7. Only after that approval, run `publish-comments --manifest <path> --confirm-source <hash>`. The
   helper rechecks the PR source hash and refuses stale manifests. Do not retry a partial failure:
   report the created comment IDs and the failed comment so the human can choose what to do.
8. Approval is a separate remote mutation. Offer it after comment publication, but run
   `approve <pr> --confirm-source <hash>` only after a separate explicit human approval naming the PR
   and source hash. A request to review or publish comments is not approval to approve the PR.

## Standalone global comments

Use a standalone global comment for an overall review decision, supersession note, or other conclusion
that does not belong to one changed file. Write the proposed Markdown body under `$TMPDIR`, then create a
normal comment manifest:

```bash
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" prepare-global-comment <pr> \
  --body <path> --source-hash <hash> --output <manifest>
```

Run `preview-comments --manifest <path>` and show the complete preview to the human. Stop and obtain
explicit approval for that exact manifest before running `publish-comments`. Do not publish directly from
the body file or combine publication with another remote mutation.

## Decline

Declining is an irreversible Bitbucket state change. It preserves the pull request discussion but does
not delete or otherwise change the source branch. Before declining:

1. Recommend publishing a clear global explanation first when the reason is not already durable in the
   pull request discussion. Comment publication and decline require separate approvals.
2. Refresh `metadata <pr>` and require explicit human confirmation naming the PR and current source hash.
3. Run:

   ```bash
   python3 "$SKILL_DIR/scripts/bitbucket_pr.py" decline <pr> --confirm-source <hash>
   ```

The helper verifies that the PR is open and its source still matches immediately before the request. If
Bitbucket returns an unexpected or uncertain result, report the error verbatim and tell the human to check
the PR state. Never retry a decline automatically. Only report success when the helper returns
`"declined": true` with the confirmed source hash.

After a decline, refresh the review queue. Do not approve or merge the declined PR. If the human wants to
continue the work later, Bitbucket requires a new pull request.

## Source branch cleanup

Declining never authorizes deleting the source branch, and the helper intentionally has no combined
decline-and-delete operation. Offer remote branch deletion as a separate cleanup only when the branch is
obsolete and its rationale is preserved in the pull request. Before deletion, resolve the exact remote and
branch with read-only checks, then obtain separate explicit human approval naming both. Use the repository's
normal Git credentials or Bitbucket UI. Report that remote deletion does not remove contributors' local
branches.

## Merge and local update

Merging is a separate, consequential mutation. A review, comment publication, or approval does not
authorize it. Before running the command, obtain explicit human confirmation naming the PR, reviewed
source hash, destination branch, and that the destination should be pulled locally, for example:

> Merge PR #5 at source hash `abc123` into `develop` and pull it locally.

Then run:

```bash
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" merge <pr> --confirm-source <hash>
```

The helper rechecks that the PR is open and still has the reviewed source hash. Before the remote
mutation it requires the current local branch to be the PR destination, refuses tracked local changes,
fetches the destination, and verifies that the local branch can fast-forward. It then asks Bitbucket to
merge using the repository's configured merge behavior, follows asynchronous merge status when needed,
and runs `git pull --ff-only` for the destination branch.

Report the helper's outcome precisely:

- If preflight or Bitbucket merge fails, state that the PR was not confirmed merged and the pull was not
  attempted. Include Bitbucket's reason, such as merge conflicts, unmet merge checks, permissions, changed
  refs, or timeout. When the status is uncertain, tell the human to check the PR before retrying.
- If Bitbucket merges but `git pull --ff-only` fails, state clearly that the remote merge succeeded but
  the local branch was not updated, and include the Git error. Never retry the merge.
- Only report full success when the helper returns both `"merged": true` and `"pulled": true`.

After any merge attempt, refresh the PR queue. A successful merge removes the PR from the open queue.

If the human asks to continue reviewing after publishing, fetch the diff and metadata again so a
new source revision cannot masquerade as the reviewed one.

## Comment mapping

The helper maps `(+)` annotations to Bitbucket's `inline.to`/`start_to` fields and `(-)` annotations
to `inline.from`/`start_from`. It validates paths and line ranges against the saved diff. Since
Bitbucket Cloud has no reliable file-only anchor in this endpoint, `(file-level)` annotations become
global comments prefixed with the file path; the preview makes this explicit.

Never silently downgrade an invalid inline annotation to a global comment. Fix or remove it in
revdiff, regenerate the manifest, and obtain approval for the new manifest.
