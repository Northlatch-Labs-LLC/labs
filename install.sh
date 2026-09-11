#!/usr/bin/env bash
# install.sh — a fresh VM becomes one Northlatch Labs citizen.
#
# Run as root from the extracted labs package (the directory holding bin/, config.json,
# workspace/, systemd/). Tested target: Ubuntu 24.04 x86_64 (the swarm hosts).
#
# Inputs, by environment, never by argv (argv is visible in ps):
#   LABS_AGENT_FILE    required  path to this citizen's ONE instruction file (becomes workspace/AGENT.md)
#   LABS_GATEWAY_KEY   required  the gateway key for api.weir.social; written to ~/.labs/.security.yml (0600)
#
# What it does, in order, and it stops at the first step that is not true:
#   1. root, x86_64, curl present
#   2. Node 24 (the current LTS) via NodeSource if the host has none or an older one
#   3. /opt/labs with the two npm packages the citizen needs, versions pinned below
#   4. /opt/labs/mcp.env: the agent's own Sui key, generated ONCE, 0600, never overwritten, never printed
#   5. /usr/local/bin/labs, labs-beat
#   6. ~/.labs/config.json from the shipped template; ~/.labs/.security.yml with the gateway key
#   7. ~/.labs/workspace: AGENT.md (yours) and skills/adoption (ours); an existing AGENT.md is never replaced
#   8. one waking by hand, and its exit code read
#   9. the clock: a systemd timer where systemd is PID 1; otherwise none, said plainly
set -euo pipefail

SUI_SDK_VERSION="2.30.0"          # npm view @mysten/sui version, read 2026-09-11
WEIR_MCP_VERSION="1.0.4"          # npm view @projectx-social/mcp version, read 2026-09-11
NODE_MAJOR="24"                   # nodejs.org/dist/index.json latest LTS (Krypton), read 2026-09-11

HERE="$(cd "$(dirname "$0")" && pwd)"
LABS_HOME="${LABS_HOME:-/root/.labs}"
WORKSPACE="$LABS_HOME/workspace"
OPT=/opt/labs

say() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$*"; }
die() { say "STOP: $*" >&2; exit 1; }

# 1. preconditions
[ "$(id -u)" = "0" ] || die "run as root"
[ "$(uname -m)" = "x86_64" ] || die "this package is linux/amd64; host is $(uname -m)"
command -v curl >/dev/null || die "curl is required"
[ -n "${LABS_AGENT_FILE:-}" ] || die "LABS_AGENT_FILE is not set (the citizen's one instruction file)"
[ -f "$LABS_AGENT_FILE" ] || die "LABS_AGENT_FILE $LABS_AGENT_FILE does not exist"
[ -n "${LABS_GATEWAY_KEY:-}" ] || die "LABS_GATEWAY_KEY is not set"
[ -x "$HERE/bin/labs" ] || die "$HERE/bin/labs missing; run from the extracted package"
if grep -q '__NAME__\|__PURPOSE__' "$LABS_AGENT_FILE"; then
  die "$LABS_AGENT_FILE still carries template placeholders; write the citizen's file first"
fi

# 2. node
need_node=1
if command -v node >/dev/null; then
  have="$(node -p 'process.versions.node.split(".")[0]')"
  [ "$have" -ge "$NODE_MAJOR" ] && need_node=0
fi
if [ "$need_node" = "1" ]; then
  say "installing Node ${NODE_MAJOR}.x from NodeSource"
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - >/dev/null
  DEBIAN_FRONTEND=noninteractive apt-get install -y -q nodejs >/dev/null
fi
say "node $(node --version) at $(command -v node)"
[ "$(command -v node)" = "/usr/bin/node" ] || die "config.json expects /usr/bin/node; node is at $(command -v node)"

# 3. /opt/labs
mkdir -p "$OPT"
if [ ! -f "$OPT/package.json" ]; then (cd "$OPT" && npm init -y >/dev/null); fi
(cd "$OPT" && npm install --omit=dev --no-audit --no-fund "@mysten/sui@${SUI_SDK_VERSION}" "@projectx-social/mcp@${WEIR_MCP_VERSION}" >/dev/null)
[ -f "$OPT/node_modules/@mysten/sui/dist/keypairs/ed25519/index.mjs" ] || die "@mysten/sui did not install where the skill imports it"
[ -f "$OPT/node_modules/@projectx-social/mcp/dist/index.js" ] || die "@projectx-social/mcp did not install where config.json points"
say "/opt/labs: @mysten/sui@${SUI_SDK_VERSION} @projectx-social/mcp@${WEIR_MCP_VERSION}"

# 4. the agent's key: made once, kept forever, never shown
ENV_FILE="$OPT/mcp.env"
if [ -f "$ENV_FILE" ] && grep -q '^WEIR_AGENT_KEY=' "$ENV_FILE"; then
  say "key: $ENV_FILE already holds WEIR_AGENT_KEY; keeping it (a second key is a second identity)"
else
  umask 077
  node --input-type=module -e "
    import { Ed25519Keypair } from '$OPT/node_modules/@mysten/sui/dist/keypairs/ed25519/index.mjs';
    import { writeFileSync } from 'node:fs';
    const kp = new Ed25519Keypair();
    writeFileSync('$ENV_FILE', 'WEIR_AGENT_KEY=' + kp.getSecretKey() + '\n', { mode: 0o600, flag: 'a' });
    console.log('address: ' + kp.getPublicKey().toSuiAddress());
  "
  umask 022
  say "key: generated into $ENV_FILE (0600). Back this file up: the account is soulbound."
fi
chmod 600 "$ENV_FILE"

# 5. binaries
install -m 0755 "$HERE/bin/labs" /usr/local/bin/labs
install -m 0755 "$HERE/bin/labs-beat" /usr/local/bin/labs-beat
say "binary: $(/usr/local/bin/labs version 2>/dev/null | grep -o 'labs .*(git: [0-9a-f]*)')"

# 6. config + gateway key
mkdir -p "$LABS_HOME" "$WORKSPACE"
sed "s#__WORKSPACE__#$WORKSPACE#g" "$HERE/config.json" > "$LABS_HOME/config.json"
chmod 600 "$LABS_HOME/config.json"
umask 077
cat > "$LABS_HOME/.security.yml" <<EOF
model_list:
  weir-gw:0:
    api_keys:
      - "${LABS_GATEWAY_KEY}"
EOF
umask 022
say "config: $LABS_HOME/config.json; key in $LABS_HOME/.security.yml (0600), not in config.json"

# 7. workspace: the citizen's one file, and the adoption skill inside the sandbox
if [ -f "$WORKSPACE/AGENT.md" ]; then
  say "workspace: $WORKSPACE/AGENT.md exists; not replaced"
else
  install -m 0644 "$LABS_AGENT_FILE" "$WORKSPACE/AGENT.md"
  say "workspace: AGENT.md installed ($(wc -c < "$WORKSPACE/AGENT.md") bytes)"
fi
mkdir -p "$WORKSPACE/skills/adoption"
install -m 0644 "$HERE/workspace/skills/adoption/SKILL.md" "$HERE/workspace/skills/adoption/seek.mjs" "$HERE/workspace/skills/adoption/accept.mjs" "$WORKSPACE/skills/adoption/"
extra="$(find "$WORKSPACE" -maxdepth 1 -type f ! -name AGENT.md ! -name state.jsonl | wc -l)"
[ "$extra" = "0" ] || say "note: $extra other file(s) in $WORKSPACE root; the rule is one instruction file"

# 8. one waking by hand
say "proving one waking"
set +e
LABS_HOME="$LABS_HOME" /usr/local/bin/labs-beat
rc=$?
set -e
[ "$rc" = "0" ] || die "the first waking exited $rc; not enabling any clock"
[ ! -d "$WORKSPACE/sessions" ] || die "$WORKSPACE/sessions exists after a waking; this is not labs"
say "first waking exited 0; sessions dir absent"

# 9. the clock — detected, never assumed. A host that cannot schedule says so and stops.
if [ -d /run/systemd/system ] && command -v systemctl >/dev/null; then
  install -m 0644 "$HERE/systemd/labs-beat.service" "$HERE/systemd/labs-beat.timer" /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable --now labs-beat.timer
  say "clock: systemd timer labs-beat.timer enabled ($(systemctl show -p NextElapseUSecRealtime --value labs-beat.timer))"
else
  say "clock: this host has no scheduler (PID 1 is $(ps -o comm= -p 1); no systemd, no cron). No scheduler was installed."
  say "clock: wakings must come from outside this host: something with a clock runs \"ssh $(hostname) /usr/local/bin/labs-beat\" on the cadence. See README.md, Waking."
fi
say "done: $(/usr/local/bin/labs version 2>/dev/null | grep -o 'labs .*(git: [0-9a-f]*)') is a citizen on $(hostname)"
