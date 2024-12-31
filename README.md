![GoodLink Logo](https://gitee.com/konyshe/goodlink/raw/master/logo.png "GoodLink")

# 背景

  本人由于经常异地办公，对于市面上的远程桌面工具，无论带宽、收费、画面模糊等，感觉都不如windows自带的远程桌面，但异地如何使用windows自带的远程桌面呢？就有了该项目

# 特点

1. 两台主机之间直连！直连！直连！不经过第三方服务器，不用担心数据隐私泄露

2. 一条命令搞定，无需安装、无需注册，无需公网 IP，无需配置文件

3. 直连基于 QUIC，高性能，已加密

注：1.1.6 版本开始加强了通信安全，因此和老版本不兼容

![原理图](https://gitee.com/konyshe/goodlink/raw/master/assert/prototype_cn.gif "原理图")

# 介绍

1. 两台主机运行同一个程序, 一台主机加--remote 选项(以下称 remote 端), 另一台主机加--local 选项(以下称 local 端)

2. local 端和 remote 端之间的连接是点对点直连的，不经过第三方服务器

3. 可以在 local 端访问 remote 端, 但是反过来不可以

4. 如果需要反过来, 或者需要访问多个 remote 端, 就运行多个程序或启动多个 Docker

5. 可以多个 local 端对应一个 remote 端，但一个 local 端不能对应多个 remote 端。通过使用相同的--key 确认对应关系

6. 由于直连过程复杂，会出现反复重试，通常 10 分钟内成功。如果长时间无法连接，请[反馈我解决](https://gitee.com/konyshe/goodlink/issues)

7. 本程序即支持命令行方式，也支持 docker 方式，以下举例仅作参考，实际可随意切换

8. windows 自带杀毒软件，会将所有 go 语言写的程序都认为是病毒。本程序已开源，可放心食用

# 简单使用

## 工作模式 - 介绍

### 代理模式

    local端需要指定本地端口，以提供Socks5代理服务

    local端需要在系统或者软件中配置Socket5代理，便可访问remote端所处网络中的所有主机端口

### 转发模式

    remote端需要指定所处网络中的某一个主机端口，local端也需要指定本地端口

    local端无需配置Socks5代理，直接访问指定的本地端口，就等于访问remote端指定的主机端口。但也只能访问这一个端口

    注：转发模式仅支持TCP协议，一个remote端只能转发一个端口，可运行多个remote端

## 代理模式 - 举例

local 端运行在公司的电脑，remote 端运行在家里的 NAS

在公司电脑上配置代理地址：socks5://127.0.0.1:18080，便可访问家里包括 NAS 在内的所有主机端口

### 家里的 NAS ( linux，Docker )

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key= nas_202412140928
```

注：remote 端和 local 端均支持命令行 和 Docker 方式，二选一即可，以上仅作两种方式的举例

### 公司的电脑 (windows)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/1.png "使用说明")

注：当最下方的按钮变成绿色，表示已连接成功

## 转发模式 - 举例

local 端运行在公司的电脑，remote 端运行在家里的 NAS。

在公司访问 http://127.0.0.1:9999 , 等于访问家里的 NAS 管理页面http://192.168.3.2:9999

### 家里的 NAS (linux，Docker)

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --remote=192.168.3.2:9999 --key=nas_202412140928
```

注：remote 端和 local 端均支持命令行 和 Docker 方式，二选一即可，以上仅作两种方式的举例

### 公司的电脑 (windows)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

![使用说明](https://gitee.com/konyshe/goodlink/raw/master/assert/2.png "使用说明")

注：当最下方的按钮变成绿色，表示已连接成功

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
