<div align="center">
  <img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="200" height="54">

  <p><strong>全网最简单、最快、 免费的内网穿透</strong></p>

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

外出办公，对比市面上的远程工具，无论画质、软件适配，都不如 windows 自带的远程桌面，但外出如何使用 windows远程桌面？

是否可以无需远程桌面，直接访问公司的内网 WEB，GIT，SSH 等？

windows 自带杀毒软件，会将所有 go 语言写的程序都默认为病毒。本程序已开源，放心食用

**注: 仅用于学习研究，无商业合作，更无恶意行为。如有广告之类盈利行为，会告知大家。**

**郑重声明：严禁用于违法行为！！！**

# 特点

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 一定要看

1. **建议路由器和光猫使用桥接方式，关闭路由器防火墙，开启路由器UPNP**

2. **如超过5分钟无法直连，找客服（电信10000,移动10086,联通10010）改NAT类型，优先NAT1>NAT2>NAT3**

3. 只支持直连，不支持中转，UI底部会有NAT类型显示, 如果两端都是NAT4, 则无法建立直连

3. 两端主机运行同一个程序 / Docker，一端使用--remote 选项(以下称 remote 端)，另一端使用--local 选项(以下称 local 端)

4. 可以在 local 端访问 remote 端，但是反过来不可以。通过相同的密钥(--key)确认连接关系

5. TUN模式如果无法连接windows远程桌面，可在IP后面加上 :13389，再尝试连接

## 📡 NAT兼容清单

| Remote端NAT | Local端NAT | P2P连接 | 说明 |
|-------------|------------|---------|------|
| NAT1-3 | NAT1-4 | ✅ 支持 | 推荐配置 |
| NAT1-4 | NAT1-3 | ✅ 支持 | 推荐配置 |
| NAT4 | NAT4 | ⚠️ 不保证 | 运营商限制 |
| 移动网络 | 移动网络 | ❌ 不支持 | 运营商限制 |

# 快速使用 (仅针对 v3.0.0 以上版本)

直接双击 goodlink-windows-amd64.exe 启动即可

浏览器访问: http://127.0.0.1:16780/

程序会启动启动浏览器，如果16780端口已被占用，会变成其他端口，参考程序输出日志:

``` UI server started on [::]:16780```

## **在需要被连接的电脑上，启动 remote端**

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/doc/5.png "使用说明")

## **在需要发起连接的电脑上，启动 local端**

注: local端默认使用TUN模式，简单易用，适合小白，但需要管理员权限启动 goodlink-windows-amd64.exe

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/doc/6.png "使用说明")



# Local端工作模式介绍

## TUN模式 (简单易用，需管理权限)

### [TUN模式]会创建一个虚拟网卡，因此需要管理员权限运行

    连接成功后，界面会显示: Remote端IP (192.17.19.1)

    连接成功后，访问192.17.19.1，就等于访问Remote端

    举例: 在Local端打开 windows 远程桌面，填写: 192.17.19.1:13389，即可访问Remote端的远程桌面

### [TUN模式]可利用Remote端做代理跳板，访问Remote端所有的网络资源

    连接成功后，在本机配置代理即可使用(仅支持TCP代理):
    socks5://192.17.19.1:1080 或 http://192.17.19.1:1080

## 转发模式 (自由灵活，无需管理权限)

注: 如果电脑无法创建虚拟网卡，可使用转发模式

### [转发模式]不会创建虚拟网卡，因此不需要管理员权限运行

    通过网页 UI 选择「转发模式」配置端口映射

    连接成功后，在Local端访问本地指定端口等于在Remote端访问指定地址和端口

    例如: TCP 0.0.0.0:22 -> 127.0.0.1:22，访问本地22端口等同于在Remote端访问SSH

    代理与端口转发可同时配置多条规则，修改后点击确认即可，无需重新连接

### [转发模式]同样可利用Remote端做代理跳板，访问Remote端所有的网络资源

    连接成功后，在本机配置代理即可使用(仅支持TCP代理):
    socks5://127.0.0.1:1080 或 http://127.0.0.1:1080

    默认监听 127.0.0.1:1080；若端口已被占用，会自动改用随机端口，以界面显示的代理地址为准。

# 高阶使用，跨平台，嵌入第三方软件

### 该程序还支持 Linux，MacOS，Docker

```
# MacOS
./goodlink-linux-amd64
```

```
# linux
./goodlink-linux-amd64
```

```
# linux，Docker
docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink
```

### 如果想直接启动Remote端，无需再网页操作:

```
# windows
.\goodlink-windows-amd64.exe --key=a0i7oeRSQvTKYJR0iL50dxXQbExDmHU1kcCr6gotwwsSrLf --remote
```

```
# MacOS
./goodlink-linux-amd64 --key=a0i7oeRSQvTKYJR0iL50dxXQbExDmHU1kcCr6gotwwsSrLf --remote
```

```
# linux
./goodlink-linux-amd64 --key=a0i7oeRSQvTKYJR0iL50dxXQbExDmHU1kcCr6gotwwsSrLf --remote
```

```
# linux，Docker
docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key=a0i7oeRSQvTKYJR0iL50dxXQbExDmHU1kcCr6gotwwsSrLf --remote
```


## 💬 交流方式

- **GitHub Issues**：[提交问题和建议](https://github.com/konyshe/Goodlink/issues)
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
  <p>Made with ❤️ by Goodlink Team</p>
</div>
