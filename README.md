<img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">

由于经常异地办公, 对于市面上的远程桌面工具, 无论速度、收费、画面模糊等, 都不如 windows 自带的远程桌面, 但异地如何使用内网的 windows 远程桌面呢？

是否可以无需远程桌面, 直接浏览器访问公司的内网 WEB, 登录内网 GIT, 内网 SSH, 远程 VS CODE 调试等等, 就跟在公司一模一样？

**注: 作者开发该项目的初衷，是方便自己的同时，方便大家，提升热度，希望对未来有所帮助。该项目仅用于学习研究, 目前没有任何商业合作，更没有任何恶意行为。如果未来有广告之类盈利的行为，会郑重告知大家。另外声明：严禁用于违法行为！！！**

[切换回1.6版文档](https://gitee.com/konyshe/goodlink/blob/v1.6/README.md)

# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器, 不用担心数据泄露

2. 一条命令搞定, 无需安装、无需注册, 无需公网 IP, 无需配置文件

3. 直连基于 QUIC, 高性能, 已加密

4. 新增TUN模式, IP直连，不限端口

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 重点

1. 本程序即支持命令行方式, 也支持 docker 方式, windows 版本还新增了UI版本, 适合新手。以下举例仅作参考, 可随意切换

2. 两端主机运行同一个程序 / Docker, 一端使用--remote 选项(以下称 remote 端), 另一端使用--local 选项(以下称 local 端)

3. 可以在 local 端访问 remote 端, 但是反过来不可以

4. 可以无限个 local 端连接同一个 remote 端, 但一个 local 端不能同时连接多个 remote 端。通过相同的密钥(--key)确认连接关系

5. 由于直连过程复杂, 会出现反复重试, 通常 10 分钟内成功。如果长时间无法连接, 点[反馈我解决](https://gitee.com/konyshe/goodlink/issues)

6. windows 自带杀毒软件, 会将所有 go 语言写的程序都默认为病毒。本程序已开源, 可放心食用

7. 以下举例说明中的密钥(--key), 请不要使用, 否则会连上别人的 remote 端, 或者被别人的 local 端连上。自己随机一个 16-24 字节长度的密钥

8. UI版本始于1.4（goodlink-windows-amd64-ui.zip）, cmd是命令行版本

9. Linux平台需要赋予可执行权限, 例如 `chmod +x goodlink-linux-amd64-cmd`

10. Local端要求管理员权限, windows平台需管理员身份运行, Linux端需使用root用户或搭配 `sudo` 命令

11. 该项目刚刚起步, 可能不太稳定, 欢迎到 Issues 上提出问题和建议, 帮忙测试的同学将保证永久免费使用

 <table>
	<th>Remote端</th><th>Local端</th><th>P2P成功</th>
	<tr><td>NAT1-3</td><td>NAT1-4</td><td>YES</td></tr>
      <tr><td>NAT1-4</td><td>NAT1-3</td><td>YES</td></tr>
	<tr><td>NAT4</td><td>NAT4</td><td>由于运营商算法调整，不保证100%</td></tr>
  </table>

# 简单使用

## 工作模式 - 介绍

#### TUN模式

    Local端会创建一个虚拟网卡, 因此需要管理员权限运行。连接成功后，界面会显示对端IP(目前固定: 192.17.19.1)

    不限端口，访问对端IP(目前固定: 192.17.19.1)的任意端口，都相当于访问Remote端本机的任意端口

    注: 仅支持TCP协议, 因此无法 ping 对端IP

#### 代理模式

    代理地址端口: socket5://192.17.19.1:1080

    对端地址目前固定为: 192.17.19.1, 端口目前固定为: 1080

    local端需要在系统或者软件中配置Socket5代理, 访问任意主机端口, 相当于Remote端自己在访问

    注: 仅支持TCP代理

## 代理模式 - 举例 1

目标: 在家里电脑(或出差电脑)浏览器上配置代理: socks5://127.0.0.1:18080, 访问公司所有内网 WEB, 和在公司无异

注: 浏览器可商店安装插件 SwitchyOmega 配置 socks5 代理。其他 GIT, SVN, SSH 等等, 也都支持 socks5 代理, 可以百度搜索

### remote 端运行在公司电脑

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示启动成功

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/3.png "使用说明")

### local 端运行在家里电脑(或出差电脑)

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示连接成功。如果超过 10 分钟无法连接, 按照下图先“点击关闭”, 然后选择“主动连接”, 再“点击启动”

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/4.png "使用说明")

## 代理模式 - 举例 2

目标: 在公司电脑上配置代理: socks5://127.0.0.1:18080, 访问家里包括 NAS 在内的所有主机端口

### remote 端运行在家里的 NAS

#### ( linux, Docker )

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key=nas_202412140928
```

#### ( linux, 命令行 )

```
./goodlink-linux-amd64 --key=nas_202412140928
```

#### ( windows, 命令行 )

```
.\goodlink-windows-amd64.exe --key=nas_202412140928
```

### local 端运行在公司电脑

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示已连接成功

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/1.png "使用说明")

#### ( linux, Docker )

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --local=127.0.0.1:18080 --key=nas_202412140928
```

#### ( linux, 命令行 )

```
./goodlink-linux-amd64 --local=127.0.0.1:18080 --key=nas_202412140928
```

#### (windows, 命令行)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:18080 --key=nas_202412140928
```

## 转发模式 - 举例 1

目标: 在家里电脑(或出差电脑), 打开 windows 远程桌面, 连接 127.0.0.1:13389, 访问公司电脑的远程桌面

注: 不是所有软件都支持 Socket5 代理, 比如 windows 自带远程桌面, 这时可用转发模式, 将公司电脑的 3389 端口和家里电脑(或出差电脑)的 13389 端口绑定（本机远程桌面服务已占用 3389 端口）。还有一个场景，出于安全考虑, 只希望 Remote 端指定的主机端口能被访问

### remote 端运行在公司电脑

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示启动成功

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/5.png "使用说明")

### local 端运行在家里电脑(或者出差笔记本)

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示连接成功。如果超过 10 分钟无法连接, 按照下图先“点击关闭”, 然后选择“主动连接”, 再“点击启动”

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/6.png "使用说明")

## 转发模式 - 举例 2

目标: 在公司访问 http://127.0.0.1:9999 , 等于访问家里的 NAS 管理页面http://192.168.3.2:9999

### remote 端运行在家里的 NAS

#### (linux, Docker)

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --remote=192.168.3.2:9999 --key=nas_202412140928
```

#### ( linux, 命令行 )

```
./goodlink-linux-amd64 --remote=192.168.3.2:9999 --key=nas_202412140928
```

#### (windows, 命令行)

```
.\goodlink-windows-amd64.exe --remote=192.168.3.2:9999 --key=nas_202412140928
```

### local 端运行在公司电脑

#### (windows, UI)

注: 当最下方的按钮变成绿色, 表示已连接成功

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/2.png "使用说明")

#### (linux, Docker)

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --local=127.0.0.1:9999 --key=nas_202412140928
```

#### ( linux, 命令行 )

```
./goodlink-linux-amd64 --local=127.0.0.1:9999 --key=nas_202412140928
```

#### (windows, 命令行)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:9999 --key=nas_202412140928
```

# 选项说明

```
root@VM-4-9-ubuntu:~/go/src/goodlink# ./bin/goodlink-linux-amd64 -h
Usage of bin/goodlink-linux-amd64:
  --remote string
        remote端所处网络中, 需要被远程访问的主机地址端口。若不加这个选项, 就是代理模式
  --local string
        local端监听的地址端口
  --key string
        用于加密通信的密钥, 自己随便定义, local端和remote端必须一致。且必须16-24个字节长度, 防止冲突: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928
```

# 感谢支持

  danshiyuan

# [问题解答](https://gitee.com/konyshe/goodlink/issues)
