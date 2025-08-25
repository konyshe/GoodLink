<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">
  
  <h1>GoodLink</h1>
  
  <p><strong>基于P2P的安全内网穿透工具</strong></p>
  
  <p>
    <a href="https://gitee.com/konyshe/goodlink/releases">
      <img src="https://img.shields.io/badge/release-最新版本-blue" alt="Release">
    </a>
    <a href="https://github.com/konyshe/goodlink/blob/master/LICENSE">
      <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
    </a>
    <a href="https://gitee.com/konyshe/goodlink/stargazers">
      <img src="https://gitee.com/konyshe/goodlink/badge/star.svg" alt="Stars">
    </a>
  </p>
  
  <p>📥 <a href="https://gitee.com/konyshe/goodlink/releases"><strong>下载最新版本</strong></a></p>
</div>

## 🎯 项目简介

**GoodLink** 是一个基于P2P技术的内网穿透工具，专为解决远程办公中的网络访问问题而设计。

### 💡 解决的问题
- 外出办公时如何使用Windows自带的远程桌面？
- 如何在外网直接访问公司内网的WEB、GIT、SSH等服务？
- 如何避免第三方服务器带来的数据安全风险？

### ⚖️ 免责声明
> **重要提醒**：本项目仅供学习研究使用，严禁用于任何违法行为！
> 
> 项目目前为开源免费，如未来有商业化计划将提前告知社区。

### 🏆 项目荣誉
> **特别通知**：本项目已被邀请参与【GitCode百大项目】评选！
> 
> 欢迎 [点击这里](https://gitcode.com/konyshe/goodlink/issues) 提出问题或需求，帮助项目发展的同时有机会获得奖品！

<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/gitcode.png" width="200">
</div>


## ⚠️ 重要说明

1. **关闭路由器防火墙**，建议设置DMZ为本机IP

2. **NAT类型优化**：如3分钟内无法建立连接，请联系运营商客服修改NAT类型
   - 电信：10000 | 移动：10086 | 联通：10010
   - 优先级：NAT1 > NAT2 > NAT3 > NAT4

3. **如超过3分钟无法直连，找客服（电信10000,移动10086,联通10010）改NAT类型，优先NAT1>NAT2>NAT3** 

4. 本程序即支持命令行方式, 也支持 docker 方式, windows 还有UI版本（但cmd版本更稳定、性能更高、内存占用非常小）, 可随意搭配

5. 两端主机运行同一个程序 / Docker, 一端使用--remote 选项(以下称 remote 端), 另一端使用--local 选项(以下称 local 端)

6. 可以在 local 端访问 remote 端, 但是反过来不可以

7. 可以无限个 local 端连接同一个 remote 端, 但一个 local 端不能同时连接多个 remote 端。通过相同的密钥(--key)确认连接关系

8. 由于Local端需要创建虚拟网卡，因此一个PC端只能运行一个 local 端，确定右下角任务栏只能一个GoodLink图标

9. windows 自带杀毒软件, 会将所有 go 语言写的程序都默认为病毒。本程序已开源, 可放心食用

10. 以下举例说明中的密钥(--key), 请不要使用, 否则会连上别人的 remote 端, 或者被别人的 local 端连上。自己随机一个 16-24 字节长度的密钥

11. 该项目刚刚起步, 可能不太稳定, 欢迎提出ISSUES, 帮忙测试的同学将保证永久免费使用

## ✨ 核心特点

### 🔒 **点对点直连**
- **真正的P2P连接**：两台主机直接连接，数据不经过任何第三方服务器
- **数据安全保障**：所有通信数据端到端加密，无数据泄露风险
- **低延迟传输**：直连模式确保最佳的网络性能

### 🚀 **开箱即用**
- **零配置部署**：一条命令即可启动，无需复杂配置
- **免安装使用**：绿色软件，下载即用
- **无需公网IP**：利用NAT穿透技术，普通家庭网络即可使用
- **跨平台支持**：支持Windows、Linux、Docker多种部署方式

<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif" alt="工作原理图" width="600">
  <p><em>GoodLink工作原理示意图</em></p>
</div>

## ⚠️ 重要说明

### 🔧 网络配置要求

> **关键配置**
> 1. **关闭路由器防火墙**，建议设置DMZ为本机IP
> 2. **NAT类型优化**：如3分钟内无法建立连接，请联系运营商客服修改NAT类型
>    - 电信：10000 | 移动：10086 | 联通：10010
>    - 优先级：NAT1 > NAT2 > NAT3 > NAT4

### 🏗️ 架构说明

#### 🔗 连接模式
- **Remote端**：作为服务提供方，接受连接
- **Local端**：作为客户端，发起连接
- **连接方向**：Local端 → Remote端（单向访问）
- **连接数量**：多个Local端可连接同一Remote端，但Local端不能同时连接多个Remote端

#### 🔑 安全认证
- 通过共享密钥（`--key`）建立连接
- **安全提醒**：请使用16-24字节的随机密钥，切勿使用示例密钥

#### 💻 部署选项
| 平台 | 支持方式 | 推荐程度 | 说明 |
|------|----------|----------|------|
| Windows | 命令行 / UI界面 | ⭐⭐⭐⭐⭐ | 命令行版本更稳定 |
| Linux | 命令行 / Docker | ⭐⭐⭐⭐⭐ | 完全支持 |
| macOS | 命令行 | ⭐⭐⭐⭐ | 基本支持 |

### 📡 NAT兼容性

| Remote端NAT | Local端NAT | P2P连接 | 说明 |
|-------------|------------|---------|------|
| NAT1-3 | NAT1-4 | ✅ 支持 | 推荐配置 |
| NAT1-4 | NAT1-3 | ✅ 支持 | 推荐配置 |
| NAT4 | NAT4 | ⚠️ 不保证 | 可能需要中继 |
| 移动网络 | 移动网络 | ❌ 不支持 | 运营商限制 |

### 🛡️ 安全提醒

- **Windows Defender**：可能误报病毒，请添加信任
- **开源透明**：所有代码公开，可自行审查
- **数据安全**：点对点加密，无中间人攻击风险
- **详细安全说明**：[查看安全分析](https://gitee.com/konyshe/goodlink/issues/IBFKC2)

### 📝 使用限制

- **Local端限制**：每台设备只能运行一个Local端实例
- **虚拟网卡**：Local端需要管理员权限创建虚拟网卡
- **项目状态**：早期版本，欢迎反馈问题和建议

## 🔧 工作模式

> **双模式并行**：以下两种模式同时工作，无需选择，提供最大的灵活性

### 🌐 TUN模式（虚拟网卡）

**工作原理**
- Local端创建虚拟网卡，建立透明的网络隧道
- 需要管理员权限运行
- 连接成功后显示Remote端IP地址

**使用场景**
```bash
# 示例：使用Windows远程桌面
# 1. 获取Remote端IP（如：192.168.100.1）
# 2. 在Local端打开"远程桌面连接"
# 3. 输入Remote端IP，即可直接连接
mstsc /v:192.168.100.1
```

**适用于**
- Windows远程桌面（RDP）
- 直接IP访问的应用
- 网络设备管理
- 内网服务访问

### 🔀 SOCKS5代理模式

**工作原理**
- 提供标准SOCKS5代理服务
- 代理地址：`socks5://Remote端IP:1080`
- 支持TCP协议（UDP暂不支持）

**配置示例**
```bash
# 代理地址配置
SOCKS5_PROXY=socks5://192.168.100.1:1080

# Git代理配置
git config --global http.proxy socks5://192.168.100.1:1080
git config --global https.proxy socks5://192.168.100.1:1080

# SSH代理配置（通过ProxyCommand）
ssh -o ProxyCommand='nc -X 5 -x 192.168.100.1:1080 %h %p' user@target_host
```

**浏览器配置**
- Chrome/Edge：推荐使用 [SwitchyOmega](https://chrome.google.com/webstore/detail/proxy-switchyomega/padekgcemlokbadohgkifijomclgjgif) 插件
- Firefox：内置代理设置支持

**适用于**
- Web浏览器代理
- Git/SVN版本控制
- SSH远程连接
- 各类支持SOCKS5的应用

### 🔄 模式对比

| 特性 | TUN模式 | SOCKS5模式 |
|------|---------|------------|
| 权限要求 | 管理员权限 | 普通权限 |
| 配置复杂度 | 零配置 | 需应用配置 |
| 支持协议 | 全协议 | TCP |
| 透明度 | 完全透明 | 需手动配置 |
| 适用场景 | 系统级访问 | 应用级代理 |

## 🚀 快速开始

### 📋 准备工作

1. **下载程序**：从 [发布页面](https://gitee.com/konyshe/goodlink/releases) 下载对应平台版本
2. **生成密钥**：创建16-24字节的随机密钥（重要：不要使用示例密钥！）
3. **网络配置**：确保路由器NAT类型为NAT1-NAT3

### 🔧 部署Remote端（服务提供方）

> **说明**：Remote端通常部署在公司内网或目标网络环境

#### 🪟 Windows

**UI界面方式**（推荐新手）

<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/v2/5.png" alt="Windows UI Remote端配置" width="500">
</div>

**命令行方式**（推荐服务器）
```powershell
# 以管理员身份运行PowerShell
.\goodlink-windows-amd64.exe --fork --key=YOUR_SECRET_KEY_HERE --remote
```

#### 🐧 Linux

**命令行方式**
```bash
# 下载并运行
wget https://gitee.com/konyshe/goodlink/releases/download/latest/goodlink-linux-amd64
chmod +x goodlink-linux-amd64
./goodlink-linux-amd64 --key=YOUR_SECRET_KEY_HERE --remote
```

**Docker方式**（推荐生产环境）
```bash
# 拉取并运行容器
docker run -d \
  --name=goodlink-remote \
  --net=host \
  --restart=always \
  registry.cn-shanghai.aliyuncs.com/kony/goodlink \
  --key=YOUR_SECRET_KEY_HERE \
  --remote
```

**系统服务方式**
```bash
# 创建systemd服务文件
sudo tee /etc/systemd/system/goodlink.service > /dev/null <<EOF
[Unit]
Description=GoodLink Remote Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/goodlink-linux-amd64 --key=YOUR_SECRET_KEY_HERE --remote
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable goodlink
sudo systemctl start goodlink
```

### 📱 部署Local端（客户端）

> **说明**：Local端部署在需要访问内网的设备上（如个人电脑、笔记本）

#### 🪟 Windows

**UI界面方式**（推荐）

<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/v2/6.png" alt="Windows UI Local端配置" width="500">
</div>

**命令行方式**
```powershell
# 以管理员身份运行（必须！）
.\goodlink-windows-amd64.exe --fork --key=YOUR_SECRET_KEY_HERE --local
```

#### 🐧 Linux

**命令行方式**
```bash
# 需要root权限
sudo ./goodlink-linux-amd64 --key=YOUR_SECRET_KEY_HERE --local
```

> **⚠️ Docker限制**：由于Local端需要创建虚拟网卡，Docker环境下暂不支持

### 🔍 连接状态检查

**成功连接的标志**
- Local端显示Remote端IP地址
- 可以ping通Remote端IP
- 日志显示"P2P connection established"

**连接测试**
```bash
# 在Local端测试连接
ping <Remote端IP>
telnet <Remote端IP> 1080  # 测试SOCKS5代理
```

### 🛠️ 常用参数说明

| 参数 | 说明 | 示例 |
|------|------|------|
| `--key` | 连接密钥（必须） | `--key=MySecretKey123456` |
| `--remote` | 运行为Remote端 | `--remote` |
| `--local` | 运行为Local端 | `--local` |
| `--fork` | 后台运行（Windows） | `--fork` |
| `--log-level` | 日志级别 | `--log-level=debug` |
| `--port` | 自定义端口 | `--port=8080` |


## 📚 更多资源

### 📖 文档链接
- 🔧 [详细使用教程](https://gitee.com/konyshe/goodlink/wikis)
- 🛡️ [安全性分析](https://gitee.com/konyshe/goodlink/issues/IBFKC2)
- 🐛 [问题反馈](https://gitee.com/konyshe/goodlink/issues)
- 💡 [功能建议](https://gitee.com/konyshe/goodlink/issues/new)

### 🤝 社区支持

#### 💬 交流方式
- **GitHub Issues**：[提交问题和建议](https://gitee.com/konyshe/goodlink/issues)
- **Gitee Issues**：[国内用户交流](https://gitee.com/konyshe/goodlink/issues)
- **项目Wiki**：[查看详细文档](https://gitee.com/konyshe/goodlink/wikis)

#### 🎯 贡献指南
- 🐛 发现Bug？请提交Issue并附上详细日志
- 💡 有新想法？欢迎在Issues中讨论
- 🔧 想要贡献代码？请先fork项目并提交PR
- 📖 完善文档？欢迎提交文档改进建议

### 🙏 致谢

**特别感谢以下贡献者：**
- **danshiyuan** - 早期测试和反馈
- 所有提交Issue和建议的用户
- 帮助测试和推广的社区成员

### 📄 许可证

本项目采用 MIT 许可证开源，详情请查看 [LICENSE](./LICENSE) 文件。

### ⭐ 支持项目

如果这个项目对您有帮助，请：
- 给项目点个 ⭐ Star
- 分享给更多需要的朋友
- 提交使用反馈和建议
- 参与社区讨论

---

<div align="center">
  <p><strong>让内网访问变得简单安全！</strong></p>
  <p>Made with ❤️ by GoodLink Team</p>
</div>
