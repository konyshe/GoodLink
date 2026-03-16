<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">


  <p><strong>全网最简单、零成本的内网穿透</strong></p>

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
</div>

外出办公, 对比市面上的远程工具, 无论画质、软件适配, 都不如 windows 自带的远程桌面, 但外出如何使用 windows远程桌面？

是否可以无需远程桌面, 直接访问公司的内网 WEB, GIT, SSH 等？

windows 自带杀毒软件, 会将所有 go 语言写的程序都默认为病毒。本程序已开源, 放心食用

**注: 仅用于学习研究, 无商业合作，更无恶意行为。如有广告之类盈利行为，会告知大家。**

**郑重声明：严禁用于违法行为！！！**

# 特点

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 一定要看

1. **建议直连光猫拨号，成功率最高。否则请将路由器和光猫之间使用桥接方式，关闭防火墙，开启UPNP**

2. **如超过3分钟无法直连，找客服（电信10000,移动10086,联通10010）改NAT类型，优先NAT1>NAT2>NAT3**

3. 两端主机运行同一个程序 / Docker, 一端使用--remote 选项(以下称 remote 端), 另一端使用--local 选项(以下称 local 端)

4. 可以在 local 端访问 remote 端, 但是反过来不可以。通过相同的密钥(--key)确认连接关系

5. 遇到无法连接windows远程桌面的情况，在IP后面加上 :13389，再尝试连接

### 📡 NAT兼容清单

| Remote端NAT | Local端NAT | P2P连接 | 说明 |
|-------------|------------|---------|------|
| NAT1-3 | NAT1-4 | ✅ 支持 | 推荐配置 |
| NAT1-4 | NAT1-3 | ✅ 支持 | 推荐配置 |
| NAT4 | NAT4 | ⚠️ 不保证 | 运营商限制 |
| 移动网络 | 移动网络 | ❌ 不支持 | 运营商限制 |

# 快速使用

###  **启动 remote端(以下方式任选)**

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/5.png "使用说明")

#### windows, 命令行

```
.\goodlink-windows-amd64-cmd.exe --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### linux, Docker

```
docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### linux, 命令行

```
./goodlink-linux-amd64-cmd --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

###  **启动 local端(以下方式任选)**

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/6.png "使用说明")

#### windows, 命令行

```
.\goodlink-windows-amd64-cmd.exe --fork --key=AIabJpEIYHMDIA6NBgOBboYJ --local
```

#### linux, Docker

```
Docker暂不支持虚拟网卡（TUN模式）
```

#### linux, 命令行

```
./goodlink-linux-amd64-cmd --key=AIabJpEIYHMDIA6NBgOBboYJ --local
```

### 🛠️ 常用参数说明

| 参数 | 说明 | 示例 |
|------|------|------|
| `--key` | 连接密钥（必须） | `--key=AIabJpEIYHMDIA6NBgOBboYJ` |
| `--remote` | 运行为Remote端（必须） | `--remote` |
| `--local` | 运行为Local端（必须） | `--local` |
| `--proxy` | Local端本地代理转发地址（可选） | `--proxy=0.0.0.0:1080` |
| `--forward` | Local端本地端口转发地址，多个用逗号间隔（可选） | `--forward=0.0.0.0:22@127.0.0.1:22,0.0.0.0:80@127.0.0.1:80` |
| `-v` | 查看版本信息（命令行版本） | `-v` |

#  Local端工作模式

注：以上操作步骤，会同时启动TUN直连模式和TUN代理模式

### TUN直连模式

    Local端会创建一个虚拟网卡, 因此需要管理员权限运行。连接成功后，界面会显示: Remote端IP (192.17.19.1)
    支持TCP和UDP连接

    访问192.17.19.1，就等于内网直接访问Remote端

    举例: 在Local端打开 windows 远程桌面, 填写: 192.17.19.1:13389, 即可访问Remote端的远程桌面
    
### TUN代理模式

    socket5代理地址端口: socket5://192.17.19.1:1080
    http代理地址端口: http://192.17.19.1:1080
    仅支持TCP代理

    举例: 在Local端配置socket5代理: socks5://192.17.19.1:1080, 即可利用Remote端做跳板, 访问所有的网络资源

### 本地代理模式（该模式下，TUN直连模式、TUN代理模式不会启动）

    适用于无法创建虚拟网卡的环境（如MacOS、Docker、无管理员权限等），或同一主机有多个Local端的场景（虚拟网卡不能创建多个）
    该模式目前只支持命令行版本，使用 --proxy 选项，即可启动该模式
    仅支持TCP代理

#### linux, 命令行（其他环境以此类推）

```
./goodlink-linux-amd64-cmd --key=AIabJpEIYHMDIA6NBgOBboYJ --local --proxy=0.0.0.0:1080
```

    启动后，在本机配置代理即可使用:
    socks5://127.0.0.1:1080 或 http://127.0.0.1:1080

### 本地端口模式（该模式下，TUN直连模式、TUN代理模式不会启动）

    在本地代理模式的基础上，适用于不支持代理方式访问的场景
    如果Local端是NAT4，使用本地转发模式，可利用NAT1-3环境的主机做中转，穿透NAT4环境的Remote端
    该模式目前只支持命令行版本，使用 --forward 选项，即可启动该模式。访问Local端本地端口等同于在Remote端访问指定地址和端口
    格式: --forward=本地监听地址:本地端口@Remote端目标地址:Remote端目标端口，多个转发规则用逗号间隔

    注：--proxy 和 --forward 可以同时使用

#### linux, 命令行，单个端口转发（其他环境以此类推）

```
./goodlink-windows-amd64-cmd.exe --key=AIabJpEIYHMDIA6NBgOBboYJ --local --forward=0.0.0.0:22@127.0.0.1:22
```

#### linux, 命令行，多个端口转发（其他环境以此类推）

```
./goodlink-windows-amd64-cmd.exe --key=AIabJpEIYHMDIA6NBgOBboYJ --local --forward=0.0.0.0:22@127.0.0.1:22,0.0.0.0:80@127.0.0.1:80
```

#### linux, 命令行，同时使用代理和端口转发（其他环境以此类推）

```
./goodlink-windows-amd64-cmd.exe --key=AIabJpEIYHMDIA6NBgOBboYJ --local --proxy=0.0.0.0:1080 --forward=0.0.0.0:22@127.0.0.1:22,0.0.0.0:80@127.0.0.1:80
```

    以上示例启动后:
    - 本地1080端口提供socks5/http代理服务
    - 访问本地22端口等同于在Remote端访问127.0.0.1:22（SSH）
    - 访问本地80端口等同于在Remote端访问127.0.0.1:80（WEB）

## Linux平台如何使用代理
```bash
# 配置全局系统代理
export all_proxy="http://192.17.19.1:1080"
export http_proxy="http://192.17.19.1:1080"
export https_proxy="http://192.17.19.1:1080"

# Git通过代理访问
git config --global http.proxy http://192.17.19.1:1080
git config --global https.proxy http://192.17.19.1:1080

# SSH通过代理访问（通过ProxyCommand）
ssh -o ProxyCommand='nc -X 5 -x 192.17.19.1:1080 %h %p' user@target_host
```

## 浏览器如何使用代理
- Chrome/Edge：推荐使用 [SwitchyOmega](https://microsoftedge.microsoft.com/addons/detail/proxy-switchyomega-3-zer/dmaldhchmoafliphkijbfhaomcgglmgd) 插件
- Firefox：内置代理设置支持

## 🙏 致谢

- 所有点了⭐ Star的同学
- 所有帮助测试和推广的同学
- 所有提交Issue和建议的同学

## 💬 交流方式

- **GitHub Issues**：[提交问题和建议](https://github.com/konyshe/GoodLink/issues)
- **Gitee Issues**：[国内用户交流](https://gitee.com/konyshe/goodlink/issues)

## 🎯 贡献指南

- 🐛 发现Bug？请提交Issue
- 💡 有新想法？欢迎在Issues中讨论
- 🔧 想要贡献代码？请先fork项目并提交PR
- 📖 完善文档？欢迎提交文档改进建议

## 📄 许可证

本项目采用 MIT 许可证开源，详情请查看 [LICENSE](./LICENSE) 文件。

---

<div align="center">
  <p><strong>让内网访问变得简单安全！</strong></p>
  <p>Made with ❤️ by GoodLink Team</p>
</div>
