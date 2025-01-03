<img src="https://gitee.com/konyshe/goodlink/raw/master/assert/letter-g-2.png" width="400" height="100">

由于经常异地办公, 对于市面上的远程桌面工具, 无论速度、收费、画面模糊等, 都不如 windows 自带的远程桌面, 但异地如何使用 windows 远程桌面呢？

是否可以远程桌面都不用, 直接浏览器访问公司的内网 WEB, 登录内网 GIT, 内网 SSH, 远程 VS CODE 调试等等, 就跟在公司一模一样？

注: 作者开发该项目的初衷，是方便自己的同时，方便大家，还能提升热度，希望对未来的工作有所帮助，毕竟现在的大环境不好。本项目仅用于学习研究, 目前没有任何商业合作，更没有任何恶意行为。如果将来有广告之类盈利的行为，会在这里郑重告知大家。另外声明：严禁用于违法行为！！！

# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器, 不用担心数据隐私泄露

2. 一条命令搞定, 无需安装、无需注册, 无需公网 IP, 无需配置文件

3. 直连基于 QUIC, 高性能, 已加密

注: 1.1.6 版本开始加强了通信安全, 因此和老版本不兼容

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

 <table>
	<th>服务端NAT</th><th>客户端NAT</th><th>P2P成功</th>
	<tr><td>NAT1-3</td><td>NAT1-3</td><td>YES</td></tr>
	<tr><td>NAT1-2</td><td>NAT4</td><td>YES</td></tr>
	<tr><td>NAT4</td><td>NAT1-2</td><td>YES</td></tr>
	<tr><td>NAT4</td><td>NAT3-4</td><td>YES</td></tr>
	<tr><td>NAT3-4</td><td>NAT4</td><td>YES</td></tr>
  </table>

# 介绍

1. 本程序即支持命令行方式, 也支持 docker 方式, windows 版本也新增了 UI 使用更简单。以下举例仅作参考, 可随意切换

2. 两端主机运行同一个程序, 一端主机使用--remote 选项(以下称 remote 端), 另一端主机使用--local 选项(以下称 local 端)

3. local 端和 remote 端之间是直连的, 不经过第三方服务器

4. 可以在 local 端访问 remote 端, 但是反过来不可以

5. 可以多个 local 端对应一个 remote 端, 但一个 local 端不能对应多个 remote 端。通过相同的密钥(--key)确认对应关系

6. 如果需要反过来, 或者需要访问多个 remote 端, 就需要运行多个程序或启动多个 Docker

7. 由于直连过程复杂, 会出现反复重试, 通常 10 分钟内成功。如果长时间无法连接, 点[反馈我解决](https://gitee.com/konyshe/goodlink/issues)

8. windows 自带杀毒软件, 会将所有 go 语言写的程序都认为是病毒。本程序已开源, 可放心食用

9. 以下举例说明中的密钥(--key), 请不要使用, 否则会连上别人的 remote 端, 或者被别人的 local 端连上。自己定义一个 16-24 字节长度的密钥

10. 1.4.17 版本开始, windows 版本新增了 UI, 目前还在测试阶段, 可能不太稳定。如果影响使用, 可先使用 1.3.17 之前的 windows 版本

11. 1024 以下是操作系统的保留端口, 基本都被占用了, local 端请使用 1024 以上端口。linux 系统可以使用命令 `netstat -anp|grep 22` 判断 22 端口是否已被占用。

# 简单使用

## 工作模式 - 介绍

#### 代理模式

    local端需要指定本地端口, 以提供Socks5代理服务

    local端需要在系统或者软件中配置Socket5代理, 便可访问remote端所处网络中的所有主机端口

#### 转发模式

    remote端需要指定所处网络中的某一个主机端口, local端也需要指定本地端口

    local端无需配置Socks5代理, 直接访问指定的本地端口, 就等于访问remote端指定的主机端口。但也只能访问这一个主机端口

    注: 转发模式仅支持TCP协议, 一个remote端只能转发一个端口, 可运行多个remote端

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
        用于加密通信的密钥, 自己随便定义, local端和remote端必须一致。建议16-24个字节长度, 防止冲突: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928
  --conn int
        由于remote和local两端默认使用的算法不一样, 如果出现超过10分钟无法连接的情况, 可能是其中一端和默认的算法不兼容,
        此时可在local端增加 "--conn=1" 选项, 以调换两端的算法, 就能连接了
```

# [问题解答](https://gitee.com/konyshe/goodlink/issues)
