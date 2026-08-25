# cuterm

**[English](README.md)** | 简体中文

一个共享终端服务器：启动一个二进制文件，用浏览器打开管理页面，即可创建、进入、关闭终端。正在运行的终端可被多个浏览器页面同时进入，所有客户端之间实时共享输入、输出和最近的历史输出（128 KB 滚动缓冲）。

## 功能

- 浏览器打开 `http://localhost:7681` 进入应用页面（多终端管理 + 终端使用）
- 独立配置页面 `http://localhost:7681/config.html`，可修改监听端口、新建终端使用的 Shell，以及终端字体、字号和配色方案
- 界面与托盘菜单支持中英文（在配置页面切换，即时生效并同步到托盘菜单；默认跟随浏览器 / 系统语言）
- 启动后自动转入后台进程，并在系统托盘（macOS 菜单栏）显示图标
- 托盘菜单：打开应用页面、打开配置页面、退出服务
- 新建终端（默认登录 shell：Unix 用 `$SHELL`，Windows 用 PowerShell / cmd）
- 进入任意正在运行的终端；已退出的终端仍可进入查看历史输出
- 关闭正在运行的终端
- 多客户端同时进入同一终端：输入、输出、窗口尺寸（resize）和历史实时同步
- 单二进制发行，前端资源（xterm.js）全部内嵌，离线可用

## 安装

macOS / Linux 一行安装（自动识别系统和架构，下载最新 Release）：

```bash
curl -fsSL https://raw.githubusercontent.com/cuterxy/cuterm/main/install.sh | sh
```

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/cuterxy/cuterm/main/install.ps1 | iex
```

也可以在 [Releases](https://github.com/cuterxy/cuterm/releases) 页面手动下载：macOS `.pkg` 安装包（把带图标的 `cuterm.app` 装进 /Applications，同时安装 `cuterm` 命令到 /usr/local/bin）、Linux `.deb` / `.rpm` 包（自动带上 libayatana-appindicator3 依赖，并添加带图标的 cuterm 启动器）、Windows `setup.exe` 安装程序（自带应用图标，自动加入 PATH，可在"应用与功能"中卸载），或普通压缩包。Go 用户还可以 `go install github.com/cuterxy/cuterm@latest`（需要本机 C 工具链）。

注意：

- Linux 运行时需要 libayatana-appindicator3（Debian/Ubuntu：`sudo apt install libayatana-appindicator3-1`，使用 `.deb` / `.rpm` 安装会自动处理）
- macOS 若通过浏览器下载的二进制或 `.pkg` 被 Gatekeeper 拦截（下载文件未经 Apple 证书签名），右键选择"打开"，或执行 `xattr -d com.apple.quarantine cuterm`（用上面的 curl 脚本安装无此问题）
- Windows 需要 Windows 10 1809 及以上（依赖 ConPTY）；未签名的安装程序可能触发 SmartScreen 提示，选择"更多信息"→"仍要运行"

## 构建

系统托盘依赖 CGO，无法交叉编译，只能在目标系统上原生构建（Release 由各平台 CI runner 原生构建，见 `.github/workflows/release.yml`，打 `v*` tag 自动发布）：

```bash
go build -o cuterm .        # 本机构建（CGO 必需）
go build -o cuterm-hub ./cmd/cuterm-hub   # 配套的 cuterm-hub 应用
./build.sh 1.0.0            # 构建两个应用的当前平台发行版到 dist/
```

要求：

- macOS / Linux：需要 C 工具链（clang / gcc）
- Linux：构建时需要 libayatana-appindicator3 开发头文件（Debian/Ubuntu：`sudo apt install libayatana-appindicator3-dev`）
- Windows：发行版以 `-H windowsgui` 构建，不弹出控制台窗口

> 注意：老版本 Go 工具链（如 1.22）在较新 macOS 上用 CGO 链接出的二进制可能缺少 `LC_UUID` 导致无法启动，请升级 Go 工具链。

## 使用

```bash
./cuterm                  # 转入后台运行，托盘图标出现，监听 :7681
./cuterm -addr :9000      # 指定端口
./cuterm -foreground      # 前台运行（调试用，日志直接输出到终端）
./cuterm -version         # 打印版本
```

启动后通过系统托盘图标的菜单「打开应用页面」进入 `http://localhost:7681`，或手动在浏览器打开；「打开配置页面」可修改监听端口、新建终端的 Shell、界面语言以及终端字体、字号和配色方案（应用页面侧栏底部也有配置页面和语言切换的入口）。点托盘菜单「退出」即可停止服务；也可以直接 `kill` 后台进程。

配置页面的修改即时生效并持久化到 `~/.cuterm/config.json`，下次启动自动应用；显式使用 `-addr` 参数时优先于配置文件中的端口。后台模式的日志写入 `~/.cuterm/cuterm.log`。

## cuterm-hub

cuterm-hub 是配套的集群管理器：它可以连接任意多个 cuterm 实例（"节点"），把它们的终端集中到一个页面上管理。hub 对 REST 和 WebSocket 流量做透明代理——节点运行原版 cuterm 即可，无需任何插件。

- 应用页面 `http://localhost:7682`：终端按节点分组展示，可在任意节点上新建、进入、重命名、关闭终端
- 配置页面 `http://localhost:7682/config.html`：添加/删除节点（名称 + `主机:端口`）、配置每个节点的 Shell，以及监听端口、界面语言、登录时自动启动、终端字体、字号、配色方案和滚动缓冲行数
- 与 cuterm 一致的体验：启动后转入后台、系统托盘图标（蓝色）、中英文界面，配置持久化到 `~/.cuterm-hub/config.json`

用同一套脚本安装，指定应用名即可：

```bash
curl -fsSL https://raw.githubusercontent.com/cuterxy/cuterm/main/install.sh | sh -s cuterm-hub
```

```powershell
.\install.ps1 -App cuterm-hub
```

Release 同样发布 cuterm-hub 的各类包（同样的命名、加 `cuterm-hub-` 前缀：压缩包、`.pkg`、`.deb` / `.rpm`、`setup.exe`）。源码构建：

```bash
go build -o cuterm-hub ./cmd/cuterm-hub
```

### Hub HTTP API

hub 提供与 cuterm 相同的设置接口（`/api/port`、`/api/appearance`、`/api/language`、`/api/autostart`、`/api/version`），并新增：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/nodes` | 列出节点及实时状态：`[{"id":"...","name":"...","addr":"主机:7681","online":true,"version":"..."}]` |
| POST | `/api/nodes` | 添加节点，body：`{"name":"...","addr":"主机:7681"}`（name 可选；端口缺省为 7681） |
| PATCH | `/api/nodes/{id}` | 修改节点名称 / 地址 |
| DELETE | `/api/nodes/{id}` | 删除节点 |
| GET/POST/PATCH/DELETE | `/api/nodes/{id}/terminals...` | 原样代理到节点自身的 `/api/terminals...` |
| GET/POST | `/api/nodes/{id}/shells`、`/api/nodes/{id}/shell` | 代理节点的 Shell 设置 |
| GET | `/ws/nodes/{id}/terminals/{tid}` | WebSocket 接入终端（桥接到节点） |

## 架构

- `internal/terminal` — 终端会话管理：每个终端一个 PTY，输出扇出给所有订阅者，保留 128 KB 历史供新客户端回放；`pty_unix.go`（creack/pty）与 `pty_windows.go`（ConPTY）两个平台实现
- `internal/server` — HTTP API + WebSocket；WebSocket 使用二进制帧，首字节为类型：`0` 输出、`1` 输入、`2` resize、`3` 终端已退出
- `web/` — 应用页面（`index.html`：终端管理 + xterm.js 终端使用）与配置页面（`config.html`：修改端口、Shell、字体、字号和配色方案），字体与配色预设见 `web/themes.js`，界面文案中英文见 `web/i18n.js`，通过 `go:embed` 嵌入二进制
- `internal/hub` — cuterm-hub 的代理服务：节点注册表与在线状态、到节点 API 的透明 REST 代理、到节点终端 WebSocket 的桥接
- `cmd/cuterm-hub` — cuterm-hub 二进制：与 cuterm 主包结构一致（配置、后台化、托盘），内嵌 Web 界面在 `cmd/cuterm-hub/web/`，蓝色图标资源在 `cmd/cuterm-hub/assets/`
- 托盘菜单语言跟随配置页面的语言设置并即时切换，未设置时按系统语言（`lang_unix.go` / `lang_windows.go`）选择

### HTTP API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/terminals` | 列出所有终端 |
| POST | `/api/terminals` | 新建终端，body：`{"name":"...", "cols":80, "rows":24}`（均可选，name 缺省自动生成） |
| PATCH | `/api/terminals/{id}` | 重命名终端，body：`{"name":"新名称"}` |
| DELETE | `/api/terminals/{id}` | 关闭终端 |
| GET | `/ws/terminals/{id}` | WebSocket 接入终端 |
| POST | `/api/port` | 更换监听端口，body：`{"port":9000}`（配置页面可改） |
| GET | `/api/shells` | 当前 Shell 与可选列表：`{"current":"/bin/zsh","available":[...]}` |
| POST | `/api/shell` | 设置新建终端的 Shell，body：`{"shell":"/bin/zsh"}`（空串恢复自动检测；仅对新建终端生效） |
| GET | `/api/appearance` | 当前外观配置：`{"fontFamily":"...","fontSize":14,"theme":"default"}`（字段缺省表示用内置默认值） |
| POST | `/api/appearance` | 设置终端字体、字号、配色方案，body：`{"fontFamily":"...","fontSize":14,"theme":"dracula"}` |
| GET | `/api/language` | 当前界面语言：`{"language":"zh-CN"}`（空串表示跟随浏览器 / 系统语言） |
| POST | `/api/language` | 设置界面语言，body：`{"language":"en"}`（`"en"` / `"zh-CN"` / `""`；即时切换托盘菜单语言） |

## 安全提示

本程序未做认证鉴权，任何能访问该端口的人都能获得 shell。请只在可信网络 / 本机使用，或自行在前面加反向代理做认证。

## 开源协议

[MIT License](LICENSE)
