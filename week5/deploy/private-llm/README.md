# Task4 deployment record

This is the configuration installed manually on the user's Ubuntu 24.04.4 VPS.
It runs inference on the VPS CPU, independently of the Mac's Ollama installation.
The Mac is an HTTP client connected through SSH. No domain, public HTTP listener,
cloud model, Docker service, or GPU is required.

## Hardware and build reported during setup

- Ubuntu 24.04.4 LTS, Linux 6.8.0-107-generic, x86_64.
- One QEMU virtual CPU; advertised flags lack AVX, AVX2, FMA, F16C and SSE4.2.
- 961 MiB RAM, 461 MiB swap, 9.6 GB root filesystem.
- Unused Docker/containerd, Xray and Outline workloads were stopped; Docker's
  service/socket, containerd and Xray were disabled. Available RAM increased from
  387 MiB to 726 MiB before the build.
- llama.cpp `0.6.0-dev`, build 1, GNU 13.3.0; full commit
  `a17e94d229fc7fed1567a799baebd11b771fe5af`, also recorded on the VPS at
  `/opt/private-llm/llama-cpp-revision.txt`.
- Source directory `/opt/private-llm/llama.cpp`; build artifact
  `/opt/private-llm/llama.cpp/build/bin/llama-server`; installed executable
  `/usr/local/bin/llama-server`.

The source was shallow-cloned from https://github.com/ggml-org/llama.cpp.
Ubuntu packages used for building: `git cmake build-essential curl
ca-certificates libssl-dev`. The successful build used:

```sh
cmake -S /opt/private-llm/llama.cpp -B /opt/private-llm/llama.cpp/build \
  -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF \
  -DGGML_NATIVE=OFF -DGGML_SSE42=OFF -DGGML_AVX=OFF -DGGML_AVX2=OFF \
  -DGGML_BMI2=OFF -DGGML_FMA=OFF -DGGML_F16C=OFF -DGGML_LLAMAFILE=OFF \
  -DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_EXAMPLES=OFF \
  -DLLAMA_BUILD_APP=OFF -DLLAMA_BUILD_SERVER=ON
cmake --build /opt/private-llm/llama.cpp/build --target llama-server -j 1
```

Upstream's unit tests were not built on the constrained VPS. The executable's
version check and real inference were verified by the user. Repository tests
and the separate live verification are described in the week README.

Model: `bartowski/SmolLM2-360M-Instruct-GGUF`, file
`SmolLM2-360M-Instruct-Q4_K_M.gguf` (approximately 271 MB), downloaded from
[Hugging Face](https://huggingface.co/bartowski/SmolLM2-360M-Instruct-GGUF/blob/main/SmolLM2-360M-Instruct-Q4_K_M.gguf).
The installer checked SHA-256
`2fa3f013dcdd7b99f9b237717fa0b12d75bbb89984cc1274be1471a465bac9c2`.
Installed path: `/opt/private-llm/models/smollm2-360m-q4_k_m.gguf` (mode 0644;
parent directory 0755).

## Services and access

```text
Mac 127.0.0.1:18080 --SSH--> VPS 127.0.0.1:8080 (Nginx)
                            --> VPS 127.0.0.1:8081 (llama-server)
```

The adjacent configuration files reproduce the settings supplied during setup:

| Repository file | VPS destination |
| --- | --- |
| `private-llm.service` | `/etc/systemd/system/private-llm.service` |
| `private-llm-gateway.service` | `/etc/systemd/system/private-llm-gateway.service` |
| `private-llm-gateway.conf` | `/etc/private-llm-gateway.conf` |

`nginx.service` was masked before installing `nginx-light`, preventing the stock
public listener from starting. The separate `private-llm-gateway.service` runs
Nginx with its own configuration. Both private services are enabled on boot.
Validate changes with `nginx -t -c /etc/private-llm-gateway.conf` before reloading
the gateway. Run `systemctl daemon-reload` after editing service units.

Open this tunnel on the Mac, using the same SSH destination used for administration:

```sh
ssh -N -L 127.0.0.1:18080:127.0.0.1:8080 \
  -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 USER@VPS_IP
```

SSH authenticates and encrypts remote access. The loopback HTTP endpoints have
no separate API key; this assumes trusted accounts on both machines. VPS users
and unrestricted SSH users can reach the backend directly, bypassing gateway
limits. This is a private single-owner deployment, not tenant isolation.

Only `/health`, `/v1/models`, and `/v1/chat/completions` are exposed by the gateway.
The browser UI and other inference routes return 404. Chat is through the HTTP
API; each request must include conversation history in `messages`. There is no
server-side conversation database in this setup. Week5's separate `llamacpp`
provider and `scripts/vps-chat.sh` launcher connect `advent-agent` to this endpoint;
the agent stores conversation sessions on the Mac.

## Restrictions

- Model context: 1,024 tokens, one generation slot and one CPU thread; context
  shifting disabled. Model discovery reports a trained context of 8,192, which
  does not change the deployed 1,024-token limit.
- Generation setting: 128 tokens. The live tests request at most 32 output
  tokens (8 for rejection probes). They do not prove enforcement of a hard
  output cap against client overrides.
- Additional prompt cache disabled; prompt batches limited to 64 tokens.
- Shared gateway rate: six chat requests/minute, no bursts (roughly one start
  every ten seconds). Health and model discovery are exempt.
- One active gateway chat request; excess rate or concurrency produces HTTP 429.
  `Retry-After: 10` is advisory; an ongoing generation can remain busy longer.
- Request body limit: 16 KiB (HTTP 413). This differs from token context size.
- Upstream read timeout: 300 seconds between reads, not a total request deadline.
- systemd restarts failures, limited to three starts per five minutes for the
  inference service. Reboot recovery and forced-crash recovery are not tested.

## Resource checks on the VPS

```sh
systemctl show --no-pager private-llm.service \
  -p ActiveState -p MemoryCurrent -p MemoryPeak -p NRestarts
ps -C llama-server -o pid,rss,vsz,pcpu,etime
free -h
df -h /
ss -ltnp '( sport = :8080 or sport = :8081 )'
journalctl -u private-llm.service -n 30 --no-pager
```

Process RSS is in KiB; systemd memory values are bytes. File-backed model pages
may already be charged to another cgroup because the model was downloaded before
service startup. RSS, cgroup memory and system available RAM are different
measurements; none should be substituted for another.

The user's initial chat produced 52 tokens at 1.895 tokens/second, with `stop`
and zero service restarts. The subsequent snapshot reported 664 MiB available
RAM, `MemoryCurrent=70041600`, `MemoryPeak=70316032`, and 17 MiB system swap used.
These are user-reported snapshots, not peak process-RSS measurements or a
long-running stability benchmark. Initial model discovery through SSH and the
malformed-JSON/rate-limit probe returned 200 and 400/429 respectively.

During the verification run, the user reported process RSS of 341,876 KiB
(333.9 MiB), an active service and zero restarts. Cgroup memory was 120,827,904
bytes with a peak of 121,098,240 bytes. The [resource record](../../results/task4/vps-reported.json)
retains these values and their provenance. This single sample is not peak RSS;
the 936,776 KiB VSZ value is virtual address space, not resident memory.

## Stop and start on demand

On the VPS, stop inference and its gateway and prevent automatic startup at boot:

```sh
sudo systemctl disable --now private-llm-gateway.service private-llm.service
systemctl is-active private-llm-gateway.service private-llm.service
```

Both should report `inactive` (the command's nonzero exit status is expected).
Explicitly stopping the service does not trigger its `Restart=on-failure` policy.
Start both when needed, without re-enabling boot startup:

```sh
sudo systemctl start private-llm.service private-llm-gateway.service
```

Use `stop` instead of `disable --now` if boot startup should remain enabled.
Closing the SSH tunnel alone does not stop inference on the VPS. After shutting
down, press Ctrl-C in the Mac terminal running `ssh -N`. Model files and saved
conversations remain on disk. Stopping services frees runtime resources but
does not cancel VPS hosting charges.

The Mac's Ollama instance was confirmed to run as a Homebrew service. Stop it
and unregister login startup with:

```sh
brew services stop ollama
```

For an on-demand start without login registration, use `brew services run ollama`.
Use `brew services start ollama` if automatic login startup is desired.
See [Homebrew service commands](https://docs.brew.sh/Manpage#services-subcommand).
To unload only a model while keeping Ollama running, use `ollama ps` followed by
`ollama stop MODEL_NAME` for each loaded model
([Ollama FAQ](https://docs.ollama.com/faq)).

Optionally stop the lightweight Mac VPS-profile agent daemon too, from `week5/`:

```sh
XDG_CONFIG_HOME="$PWD/data/task4-config" \
XDG_STATE_HOME="$PWD/data/task4-state" \
./advent-agentd --profile vps stop
```

`scripts/vps-chat.sh` starts this daemon again automatically. It does not start
the VPS services or open the SSH tunnel, which must already be available.
