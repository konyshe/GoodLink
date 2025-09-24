<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">


  <p><strong>是全网最简单、零成本的内网穿透</strong></p>

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

由于经常外出办公, 对于市面上的远程桌面工具, 无论画面、适配等, 都不如 windows 自带的远程桌面, 但外出如何使用 windows远程桌面呢？

是否可以无需远程桌面, 直接访问公司的内网 WEB, GIT, SSH 等？

注: 该项目仅用于学习研究, 目前无商业合作，更无恶意行为。如果未来有广告之类盈利的行为，会郑重告知大家。另外声明：严禁用于违法行为！！！

 **特别通知：该项目被邀请【GitCode百大项目】提名，帮忙助力有机会获得奖品，[点击这里提个咨询问题or需求即可助力](https://gitcode.com/konyshe/goodlink/issues) 。同时我会尽力满足助力同学一次深度问题or需求, 再次感谢支持！**

<img src="https://gitee.com/konyshe/goodlink/raw/master/assert/gitcode.png">


# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器, 不用担心数据泄露

2. 一条命令搞定, 无需安装、无需注册, 无需公网 IP, 无需配置文件

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 重点

1. **请关闭路由器防火墙，最好同时设置路由器DMZ为本机**

2. **如超过3分钟无法直连，找客服（电信10000,移动10086,联通10010）改NAT类型，优先NAT1>NAT2>NAT3**

3. 本程序即支持命令行方式, 也支持 docker 方式, windows 还有UI版本（但cmd版本更稳定、性能更高、内存占用非常小）, 可随意搭配

4. 两端主机运行同一个程序 / Docker, 一端使用--remote 选项(以下称 remote 端), 另一端使用--local 选项(以下称 local 端)

5. 可以在 local 端访问 remote 端, 但是反过来不可以

6. 可以无限个 local 端连接同一个 remote 端, 但一个 local 端不能同时连接多个 remote 端。通过相同的密钥(--key)确认连接关系

7. 由于Local端需要创建虚拟网卡，因此一个PC端只能运行一个 local 端，确定右下角任务栏只能一个GoodLink图标

8. windows 自带杀毒软件, 会将所有 go 语言写的程序都默认为病毒。本程序已开源, 可放心食用

9. 以下举例说明中的密钥(--key), 请不要使用, 否则会连上别人的 remote 端, 或者被别人的 local 端连上。自己随机一个 16-24 字节长度的密钥

10. 对于有安全疑问，或者想进阶使用的同学，可以看: [使用GoodLink 是否足够安全？](https://gitee.com/konyshe/goodlink/issues/IBFKC2)

11. 该项目刚刚起步, 可能不太稳定, 欢迎提出ISSUES, 帮忙测试的同学将保证永久免费使用

12. 连接remote端的windows远程桌面，可在ip后面加上:13389，尝试连接。3389端口貌似有路由处理，和goodlink虚拟网卡冲突

#### 💻 部署选项
| 平台 | 支持方式 | 推荐程度 | 说明 |
|------|----------|----------|------|
| Windows | 命令行 / UI界面 | ⭐⭐⭐⭐⭐ | 命令行版本更稳定 |
| Linux | 命令行 / Docker | ⭐⭐⭐⭐⭐ | 支持 |
| macOS | 命令行 | ⭐⭐⭐⭐ | v2暂不支持 |

### 📡 NAT兼容性

| Remote端NAT | Local端NAT | P2P连接 | 说明 |
|-------------|------------|---------|------|
| NAT1-3 | NAT1-4 | ✅ 支持 | 推荐配置 |
| NAT1-4 | NAT1-3 | ✅ 支持 | 推荐配置 |
| NAT4 | NAT4 | ⚠️ 不保证 | 运营商限制 |
| 移动网络 | 移动网络 | ❌ 不支持 | 运营商限制 |

# 工作模式

注：以下两个模式同时存在, 无需选择

### TUN模式

    Local端会创建一个虚拟网卡, 因此需要管理员权限运行。连接成功后，界面会显示: Remote端IP

    举例: 在Local端打开 windows 远程桌面, 填写Remote端IP, 即可访问Remote端的远程桌面

### 代理模式

    socket5代理地址端口: socket5://Remote端IP:1080
    http代理地址端口: http://Remote端IP:1080

    举例: 在Local端配置socket5代理: socks5://Remote端IP:1080, 即可利用Remote端做跳板, 访问所有的网络资源

**Linux平台代理配置示例**
```bash
# 代理地址配置
export all_proxy="http://127.0.0.1:1080"
export http_proxy="http://127.0.0.1:1080"
export https_proxy="http://127.0.0.1:1080"

# Git代理配置
git config --global http.proxy http://127.0.0.1:1080
git config --global https.proxy http://127.0.0.1:1080

# SSH代理配置（通过ProxyCommand）
ssh -o ProxyCommand='nc -X 5 -x 127.0.0.1:1080 %h %p' user@target_host
```

**浏览器代理配置**
- Chrome/Edge：推荐使用 [SwitchyOmega](https://chrome.google.com/webstore/detail/proxy-switchyomega/padekgcemlokbadohgkifijomclgjgif) 插件
- Firefox：内置代理设置支持

# 简单使用

###  **启动 remote端**

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/5.png "使用说明")

#### linux, Docker

```
docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### linux, 命令行

```
./goodlink-linux-amd64 --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

#### windows, 命令行

```
.\goodlink-windows-amd64.exe --fork --key=AIabJpEIYHMDIA6NBgOBboYJ --remote
```

###  **启动 local端**

#### windows, UI

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/v2/6.png "使用说明")

#### linux, Docker

```
由于Local端需要创建虚拟网卡，Docker中并不支持
```

#### linux, 命令行

```
./goodlink-linux-amd64 --key=AIabJpEIYHMDIA6NBgOBboYJ --local
```

#### windows, 命令行

```
.\goodlink-windows-amd64.exe --fork --key=AIabJpEIYHMDIA6NBgOBboYJ --local
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
- **danshiyuan**
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
