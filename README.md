![GoodLink Logo](https://gitee.com/konyshe/goodlink/raw/master/logo.png "GoodLink")

# 背景

由于经常异地办公，对于市面上的远程桌面工具，无论带宽、收费、画面模糊等，感觉都不如 windows 自带的远程桌面，但异地如何使用 windows 自带的远程桌面呢？

如果只是为了远程访问公司的内网 WEB，是否可以远程桌面都不用，直接浏览器访问，就跟在公司一模一样？

注: 本项目仅用于开发学习，严禁用于违法行为！！！

# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器，不用担心数据隐私泄露

2. 一条命令搞定，无需安装、无需注册，无需公网 IP，无需配置文件

3. 直连基于 QUIC，高性能，已加密

注：1.1.6 版本开始加强了通信安全，因此和老版本不兼容

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 介绍

1. 本程序即支持命令行方式，也支持 docker 方式，windows 版本也新增了 UI 使用更简单。以下举例仅作参考，可随意切换

2. 两端主机运行同一个程序, 一端主机使用--remote 选项(以下称 remote 端), 另一端主机使用--local 选项(以下称 local 端)

3. local 端和 remote 端之间是直连的，不经过第三方服务器

4. 可以在 local 端访问 remote 端, 但是反过来不可以

5. 可以多个 local 端对应一个 remote 端，但一个 local 端不能对应多个 remote 端。通过相同的--key确认对应关系

6. 如果需要反过来, 或者需要访问多个 remote 端, 就需要运行多个程序或启动多个 Docker

7. 由于直连过程复杂，会出现反复重试，通常 10 分钟内成功。如果长时间无法连接，请[反馈我解决](https://gitee.com/konyshe/goodlink/issues)

8. windows 自带杀毒软件，会将所有 go 语言写的程序都认为是病毒。本程序已开源，可放心食用

9. 以下举例说明中的 key，请不要使用，否则会连上别人的 remote 端，或者被别人的 local 端连上。自己定义一个 16-24 字节长度的--key

10. 1.4.17 版本开始, windows 版本新增了 UI, 目前还在测试阶段，可能不太稳定。如果影响使用，可先使用 1.3.17 之前的 windows 版本

11. 1024以下是操作系统的保留端口, 基本都被占用了, local端请使用1024以上端口。linux系统可以使用命令 `netstat -anp|grep 22` 判断22端口是否已被占用。

# 简单使用

## 工作模式 - 介绍

### 代理模式

    local端需要指定本地端口，以提供Socks5代理服务

    local端需要在系统或者软件中配置Socket5代理，便可访问remote端所处网络中的所有主机端口

### 转发模式

    remote端需要指定所处网络中的某一个主机端口，local端也需要指定本地端口

    local端无需配置Socks5代理，直接访问指定的本地端口，就等于访问remote端指定的主机端口。但也只能访问这一个主机端口

    注：转发模式仅支持TCP协议，一个remote端只能转发一个端口，可运行多个remote端

## 代理模式 - 举例

local 端运行在公司的电脑，remote 端运行在家里的 NAS

在公司电脑上配置代理地址：socks5://127.0.0.1:18080，便可访问家里包括 NAS 在内的所有主机端口

举一反三: 如果出差在外，不必通过远程桌面。就能在笔记本浏览器上直接打开公司的内网 WEB（浏览器商店安装插件 SwitchyOmega 配置代理），和在公司办公一模一样

### 家里的 NAS ( linux，Docker )

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key= nas_202412140928
```

### 公司的电脑 (windows, UI 版本)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/1.png "使用说明")

注：当最下方的按钮变成绿色，表示已连接成功

### 公司的电脑 (windows, cmd 命令行)

[下载程序](https://gitee.com/konyshe/goodlink/releases/tag/v1.3.17)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:18080 --key=nas_202412140928
```

## 转发模式 - 举例

local 端运行在公司的电脑，remote 端运行在家里的 NAS。

在公司访问 http://127.0.0.1:9999 , 等于访问家里的 NAS 管理页面http://192.168.3.2:9999

举一反三: 不是所有的软件都支持配置 Socket5 代理，比如 windows 自带远程桌面，这里就只能使用转发模式，直接将公司电脑的 3389 端口和笔记本的 13389 端口绑定（笔记本自带远程桌面服务已占用 3389 端口），出差在外，随时远程桌面。

### 家里的 NAS (linux，Docker)

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --remote=192.168.3.2:9999 --key=nas_202412140928
```

### 公司的电脑 (windows, UI 版本)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/2.png "使用说明")

注：当最下方的按钮变成绿色，表示已连接成功

### 公司的电脑 (windows, cmd 命令行)

[下载程序](https://gitee.com/konyshe/goodlink/releases/tag/v1.3.17)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:9999 --key=nas_202412140928
```

# 选项说明

```
root@VM-4-9-ubuntu:~/go/src/goodlink# ./bin/goodlink-linux-amd64 -h
Usage of bin/goodlink-linux-amd64:
  --remote string
        remote端所处网络中, 需要被远程访问的主机地址端口。若不加这个选项，就是代理模式
  --local string
        local端监听的地址端口
  --key string
        自己随便定义, 但local端和remote端必须一致。建议16-24个字节长度，防止冲突: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928
  --conn int
        由于remote和local两端默认使用的算法不一样，如果出现超过10分钟无法连接的情况，可能是其中一端和默认的算法不兼容，
        此时可在local端增加 "--conn=1" 选项，以调换两端的算法，就能连接了
```

# [问题解答](https://gitee.com/konyshe/goodlink/issues)
