![GoodLink Logo](https://gitee.com/konyshe/goodlink/raw/master/logo.png "GoodLink")

# 介绍

1. 两台主机之间直连！直连！直连！不经过第三方服务器，不用担心数据隐私泄露

2. 一条命令搞定，无需安装、无需注册，无需公网IP，无需配置文件

3. 连接基于QUIC，高性能，已加密

4. 由于直连过程复杂，会出现反复重试，通常10分钟内成功。如果长时间无法连接，请[反馈我](https://gitee.com/konyshe/goodlink/issues)解决！

注：1.1.6版本开始加强了通信安全，因此和老版本不兼容

# 使用说明

## 术语说明

### local端

    使用了--local选项的主机

### remote端

    使用了--remote选项的主机

## 工作模式 - 介绍

### P2P代理模式

    local端需要指定本地端口，以提供Socks5代理服务

    local端需要在系统或者软件中配置Socket5代理，便可访问remote端所处网络中的所有主机端口

### P2P转发模式

    remote端需要指定所处网络中的某一个主机端口，local端也需要指定本地端口。

    local端无需配置Socks5代理，直接访问指定的本地端口，就等于访问remote端指定的主机端口。但也只能访问这一个端口

    注：P2P转发模式仅支持TCP协议，如果remote端需要转发多个TCP端口，需执行多个命令或启动多个 Docker（--key不能重复）

## P2P代理模式 - 举例

local端运行在公司的电脑，remote端运行在家里的NAS。

在公司电脑上配置代理地址：socks5://127.0.0.1:18080，便可访问家里包括NAS在内的所有主机端口。

### 家里的NAS ( linux，Docker )

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --key= nas_202412140928
```

### 公司的电脑  ( windows, 命令行 )

[下载程序](https://gitee.com/konyshe/goodlink/releases)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:18080 --key=nas_202412140928
```

注：remote端和local端均支持命令行 和 Docker 方式，二选一即可，以上仅作两种方式的举例。

## P2P转发模式 - 举例

local端运行在公司的电脑，remote端运行在家里的NAS。

在公司访问 http://127.0.0.1:9999 , 等于访问家里的NAS管理页面http://192.168.3.2:9999

### 家里的NAS (linux，Docker)

下载镜像：registry.cn-shanghai.aliyuncs.com/kony/goodlink

```
docker rm goodlink -f; docker run -d --name=goodlink --net=host --restart=always registry.cn-shanghai.aliyuncs.com/kony/goodlink --remote=192.168.3.2:9999 --key=nas_202412140928
```

### 公司的电脑 (windows, 命令行)

[下载程序](https://gitee.com/konyshe/goodlink/releases)

```
.\goodlink-windows-amd64.exe --local=127.0.0.1:9999 --key=nas_202412140928
```

# 选项说明

```
root@VM-4-9-ubuntu:~/go/src/goodlink# ./bin/goodlink-linux-amd64 -h
Usage of bin/goodlink-linux-amd64:
  --remote string
        remote端所处网络中, 需要被远程访问的主机地址端口。若不加这个选项就，就是代理模式
  --local string
        local端监听的地址端口
  --key string
        自定义, local端和remote端必须一致。16-24个字节长度: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928
  --conn int
        由于remote和local两端默认使用的算法不一样，如果出现超过10分钟无法连接的情况，可能是其中一端和默认的算法不兼容，
        此时可在local端增加 "--conn=1" 选项，以调换两端的算法，就能连接了
```

# 自己如何编译

```
cd /root/go/src
git clone -b main https://gitee.com/konyshe/gogo.git
git clone https://gitee.com/konyshe/goodlink.git
cd goodlink
make clean; make
```

# [详细使用说明](https://gitee.com/konyshe/goodlink/wikis)
