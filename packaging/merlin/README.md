# cuterm-hub 梅林软件中心插件

把 cuterm-hub 打包成 koolshare 软件中心（1.5 代 API）的离线安装包，可在路由器的
「软件中心 → 离线安装」中直接上传安装。

## 支持平台

| 包 | 适用固件 | 代表机型 |
|---|---|---|
| `dist/merlin/arm/cutermhub.tar.gz` | koolshare 梅林改 384/386（armv7l，内核 2.6） | RT-AC68U、RT-AC88U、R7000 等 |
| `dist/merlin/hnd/cutermhub.tar.gz` | koolshare 梅林改/官改 hnd/axhnd/axhnd.675x（内核 4.1+） | RT-AC86U、RT-AX88U、GT-AX11000、TUF-AX3000 等 |

两个包携带同一个 32 位 ARM 静态编译二进制（rogsoft 建议 32 位以兼容全部机型，
armv8 机型同样运行 32 位用户态），仅 `.valid` 校验标记不同（`arm384` / `hnd`）。
不支持 380 及更早的 1.0 代软件中心（无 skipd，`install.sh` 会拒绝安装）。

## 构建

在 macOS / Linux 开发机上执行（只需 Go 工具链，交叉编译无 CGO）：

```bash
./packaging/merlin/build.sh 1.1.0   # 参数为版本号，默认 dev
```

产物为 `dist/merlin/{arm,hnd}/cutermhub.tar.gz`。

正式发版无需手动构建：推送 `v*` 标签会触发 `.github/workflows/release.yml` 的
`merlin` job，自动构建并作为 Release 资产上传，文件名分别为
`cuterm-hub-<版本>-merlin-arm.tar.gz` 和 `cuterm-hub-<版本>-merlin-hnd.tar.gz`
（软件中心离线安装只识别压缩包内的 install.sh，与文件名无关）。

## 安装与使用

1. 路由器后台进入「软件中心 → 离线安装」，上传对应平台的 `cutermhub.tar.gz`。
2. 安装完成后在软件中心打开 cuterm-hub 插件页，打开开关并提交。
3. 管理页面监听 `7682` 端口，插件页会内嵌显示管理界面，也可直接访问
   `http://<路由器IP>:7682`。
4. 在管理页面的设置页添加 cuterm 节点（名称 + `host:port`）。

说明：

- cuterm-hub 以无托盘（headless）模式运行，配置保存在
  `/koolshare/configs/cutermhub/.cuterm-hub/`（jffs 分区，重启不丢失，
  卸载插件不会删除该目录）。
- 开机自启通过 `/koolshare/init.d/S98cutermhub.sh` 软链实现，开关状态存于
  dbus `cutermhub_enable`。
- **安全**：cuterm-hub 没有鉴权，任何能访问 7682 端口的设备都能操作节点上的
  shell。请勿对公网开放该端口，仅在可信内网使用。

## 目录结构

```
packaging/merlin/
├── build.sh                        # 交叉编译 + 打包脚本
└── cutermhub/                      # 插件模板（模块名 cutermhub）
    ├── install.sh                  # 安装脚本（平台/皮肤检测、dbus 注册）
    ├── uninstall.sh                # 卸载脚本
    ├── scripts/cutermhub_config.sh # 服务控制（启动/停止/页面提交/开机启动）
    ├── webs/Module_cutermhub.asp   # 软件中心插件页（开关、状态、内嵌管理页）
    └── res/icon-cutermhub.png      # 插件图标
```

构建时 `build.sh` 会额外生成 `version`、`.valid`，并把编译好的二进制放入
`bin/cuterm-hub`，再打成 `cutermhub.tar.gz`（包内顶层为 `cutermhub/` 目录）。
`version` 文件存的是去掉 `v` 前缀的纯版本号（如 `1.2.5`），因为软件中心插件
列表显示时会自行加上 `v`。
软件中心离线安装（armsoft / rogsoft 的 `ks_tar_install.sh`）会把压缩包解压到
`/tmp`，用 `find -maxdepth 2` 定位唯一的 `install.sh`，并要求同级存在
`webs/Module_<模块名>.asp` 与 `scripts/` 目录，因此包内必须保留模块目录这一层。

## 无头构建

路由器上没有桌面环境，托盘及其 CGO 依赖通过 Go 构建标签剔除：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -tags headless -trimpath -ldflags "-s -w -X main.version=$VERSION" \
  -o cuterm-hub ./cmd/cuterm-hub
```

`-tags headless` 时 `tray.go`（`!headless`）被 `tray_headless.go` 替代：
`runTray` 退化为阻塞等待 SIGINT/SIGTERM 后退出，`systray` 依赖不参与编译，
因此可以纯静态交叉编译。不带该标签的常规桌面构建不受影响。
