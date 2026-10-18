# Setup for review-bitbucket-pr

This archive contains the skill instructions, Codex UI metadata, the Bitbucket
Cloud helper, and its offline tests. It works with the Bitbucket Cloud repository
selected by your Git remote. Dependencies and credentials
are not bundled. Bitbucket Data Center is not supported.

## Requirements

- Python 3.10 or newer and Git on `PATH`. The helper uses only the Python standard
  library; no pip installation is needed.
- Codex with local skill support.
- For interactive diff review: the `revdiff` binary, its Codex skill/plugin, and
  agterm with `agtermctl` available. API-only commands such as listing PRs do not
  require a terminal overlay.
- A local clone with a supported `bitbucket.org` remote, network access to
  `api.bitbucket.org`, and permission to access the repository.
- `BITBUCKET_API_TOKEN` and a writable `TMPDIR` in the environment inherited by
  Codex and its helper processes.

## Install this archive

From the target repository root, extract the ZIP into `.agents/skills`:

```sh
mkdir -p .agents/skills
unzip /path/to/review-bitbucket-pr.zip -d .agents/skills
```

The resulting entrypoint must be
`.agents/skills/review-bitbucket-pr/SKILL.md`, without an extra nested folder.
Check an existing installation before extracting over it. For personal use across
repositories, extract into `$HOME/.agents/skills` instead. Choose one location to
avoid duplicate skill names. These are documented
[Codex skill locations](https://learn.chatgpt.com/docs/build-skills).
Start a new Codex session if the skill does not appear.

## Install the review tools on macOS

With Homebrew installed:

```sh
brew install umputun/apps/revdiff
brew install --cask umputun/apps/agterm
codex plugin marketplace add umputun/revdiff
codex plugin add revdiff@revdiff
```

The binary and plugin are separate requirements. Automatic plan review is not
required. See the [revdiff installation instructions](https://github.com/umputun/revdiff#codex-plugin).

Open agterm and launch Codex from a shell inside it. Homebrew installs `agtermctl`;
with the DMG installation, use **Help → Install Command Line Tool…**. The agterm
agent skill is optional for this workflow; its Help menu can install it. See
[agterm installation](https://github.com/umputun/agterm#install) for supported Macs
and download options.

Check the tools from that shell:

```sh
python3 --version
git --version
command -v revdiff
command -v agtermctl
agtermctl version
```

## Configure Bitbucket authentication

Each recipient creates their own scoped API token using Atlassian's
[token creation guide](https://support.atlassian.com/bitbucket-cloud/docs/create-an-api-token/).
Select these scopes for the full workflow:

| Scope | Used for |
|---|---|
| `read:repository:bitbucket` | Fetching PR diffs |
| `read:pullrequest:bitbucket` | Listing and inspecting PRs and merge status |
| `write:pullrequest:bitbucket` | Publishing comments, approving, declining, and merging |

For read-only review, the two read scopes suffice; write actions also require
the write scope and the corresponding repository permissions. Scopes do not grant
repository access by themselves.

The helper sends the token as a Bearer credential; no email or username variable
is needed. Atlassian documents this in
[Using API tokens](https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/#Using-Bearer-authentication).

Set the token in the shell that will launch Codex. For zsh on macOS, this hidden
prompt keeps its value out of shell history:

```zsh
read -rs 'BITBUCKET_API_TOKEN?Bitbucket API token: '
export BITBUCKET_API_TOKEN
printf '\n'
```

Do not run this while shell tracing (`set -x`) is enabled. Do not paste the token
into chat, source files, remote URLs, screenshots, or this archive. For persistent
setup, use your own credential manager to inject it into Codex's launch environment.
The helper does not read `.env` files. Restart an already-running Codex process
after changing its parent shell environment.

Leave `BITBUCKET_API_BASE_URL` unset for normal use; the default is
`https://api.bitbucket.org/2.0`. That override changes where credentials are sent.

Merging also fetches and pulls through Git using the remote's ordinary SSH or
HTTPS credentials. Configure those separately; the helper's environment token
does not configure Git authentication. `write:repository:bitbucket` is not needed
for this workflow's merge-and-pull operation.

## Verify from your repository

Set `SKILL_DIR` to the actual installed directory:

```sh
SKILL_DIR="$PWD/.agents/skills/review-bitbucket-pr"
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" doctor
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" list
```

`doctor` checks the remote format and presence of the token and `TMPDIR`. It does
not contact Bitbucket or validate credentials, scopes, or overlay dependencies.
`list` is the read-only network/authentication check.

The helper defaults to `origin`. For another remote, put the option before the
subcommand:

```sh
python3 "$SKILL_DIR/scripts/bitbucket_pr.py" --remote upstream list
```

Supported remote forms include `git@bitbucket.org:workspace/repository.git`,
`ssh://git@bitbucket.org/workspace/repository.git`, and
`https://bitbucket.org/workspace/repository.git`.

On macOS, `TMPDIR` is normally already set. If it is missing, set it to a writable
temporary directory supplied by your operating system before starting Codex.
Review diffs and comment manifests are created there.

Run the bundled tests without network access or real credentials:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s "$SKILL_DIR/scripts" -p 'test_bitbucket_pr.py'
```

Then ask Codex:

```text
$review-bitbucket-pr list active PRs
$review-bitbucket-pr show me diff for PR #17
```

Replace `17` with a PR from your own queue. Listing or opening a diff does not
authorize publishing comments, approving, declining, or merging. The skill requires
separate explicit confirmations for those operations. Follow `SKILL.md` for the
review and publication workflow.

## Troubleshooting

- **Token missing / HTTP 401:** check that Codex inherited the variable and that
  the token has not expired or been revoked. Never print it to diagnose this.
- **HTTP 403:** check the scopes reported by Bitbucket and the account's repository
  permissions. Recreate the token with the needed scopes if necessary.
- **Remote rejected:** run inside the intended clone and select a supported
  Bitbucket Cloud remote with `--remote`.
- **No overlay terminal available:** launch Codex inside agterm and confirm
  `agtermctl` and `revdiff` are on its `PATH`.
- **Socket access denied:** the launcher may need Codex sandbox approval to access
  agterm's local socket. The revdiff skill explains how to preserve the captured
  `AGTERM_SESSION_ID` and `AGTERM_SOCKET` when escalation strips them. Never target
  an unrelated active session as a workaround.
- **Stale source hash:** fetch a new diff and repeat the review/preview before
  confirming a mutation. Do not bypass the source-hash check.
- **Merge preflight failed:** check out the destination branch and resolve local
  tracked changes or divergence yourself before retrying. A remote merge followed
  by a failed local pull must not be retried as another merge.

## Sharing

Share an archive made from this sanitized skill directory. It contains this guide, `SKILL.md`,
`agents/openai.yaml`, and the two Python files under `scripts/`. It contains no
tokens, checkout, PR diffs, comment manifests, or bundled third-party tools.
The active skill remains installed on the sender's machine.
