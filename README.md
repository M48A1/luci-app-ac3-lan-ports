# SEED AC3 LAN 网口协商速度

为 BeeconMini SEED AC3 的 iStoreOS 管理页面增加 8 个 RJ45 LAN 口的连接状态和实际协商速度，每 5 秒刷新。

## 已验证环境

| 项目 | 版本 |
| --- | --- |
| 设备 | BeeconMini SEED AC3 |
| 系统 | iStoreOS 25.12.5 |
| 架构 | aarch64_cortex-a53 |
| 内核 | 6.12.94 |
| 交换机驱动 | kmod-switch-rtl8373 6.12.94-r1 |
| 软件包 | luci-app-lan-ports 1.0.1-r0 |

只适用于上述硬件及已验证的驱动版本。安装脚本检查机型和驱动，软件包依赖固定的交换机驱动版本。

## 功能与端口对应

- 显示连接状态和协商速度，不把内部 CPU 链路的 2.5 Gbps 当作外部 LAN 口速度。
- 机壳 LAN1–LAN8 分别对应芯片端口 7–0。
- 无连接或读取失败时不显示历史速率。
- 不修改 VLAN、网络配置、网口速率或双工设置。

## 安装与卸载

将构建出的 APK 上传至路由器 `/tmp`，在路由器终端执行：

```sh
apk add --allow-untrusted /tmp/luci-app-lan-ports-1.0.1-r0.apk
```

这是 OpenWrt/iStoreOS 软件包，不是 Android 应用。自定义包未签名，`--allow-untrusted` 只用于本次安装。

重新登录管理页面后打开 **状态 → LAN 网口协商速度**。安装、升级和卸载会重启 RPC 服务，当前网页会话需要重新登录。

可在软件包管理器搜索 `luci-app-lan-ports` 卸载，或执行：

```sh
apk del luci-app-lan-ports
```

## 构建

在 Linux 环境准备 Go 1.22 或更新版本，以及包含 `mkpkg` 命令的完整 apk-tools v3。部分路由器固件精简了 `mkpkg`，可以用官方 Alpine 的完整静态构建工具临时打包。

```sh
sudo ./build.sh
# 指定其他打包工具路径：
sudo APK_BUILDER=/path/to/apk.static ./build.sh
```

构建脚本交叉编译静态 ARM64 程序，然后生成 `luci-app-lan-ports-1.0.1-r0.apk`。脚本需要 root 权限，为打包文件设置 root 所有权；它不安装应用。

## 目录

| 文件 | 用途 |
| --- | --- |
| `main.go` | 通过 MDIO netlink 读取交换机状态并输出 JSON |
| `files/www/luci-static/resources/view/status/lan_ports.js` | LuCI 页面及自动刷新 |
| `files/usr/share/luci/menu.d/` | 页面菜单 |
| `files/usr/share/rpcd/acl.d/` | 只允许执行读取程序的权限 |
| `scripts/` | 机型检查和安装、卸载后刷新菜单 |
| `build.sh` | 编译和生成 APK |

## 实现说明

程序使用 [mdio-tools 的 MDIO netlink 协议](https://github.com/wkz/mdio-tools)，读取 RTL8373 的链接状态寄存器 `0x63e8` 和速率寄存器 `0x63f0`。

读取过程会向 MDIO 管理寄存器写入目标地址及读取命令 `0x1b`，不发送 ASIC 配置写入命令。一个事务内锁定 MDIO 总线，并在结束前恢复地址和数据暂存寄存器，配合现有驱动的后台检测。硬件访问出错或忙碌时返回错误。

本项目以实际运行的交换机驱动和设备读取结果核对寄存器及速度编码，尚未验证其他机型、其他驱动版本或所有协商速率。

## 验证

已在上述设备完成安装、卸载、重新安装和 1.0.0 → 1.0.1 升级；已验证页面显示 8 个端口并自动刷新。设备的物理 LAN6、LAN7、LAN8 显示 2.5 Gbps，其余端口未连接。

## 许可证

[MIT](LICENSE)
