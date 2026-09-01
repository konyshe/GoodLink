#!/usr/bin/env bash
#
# port_forward.sh - 将本地 IP:端口 映射到公网服务器端口，断线自动重连
# 用法：在本地机器运行，通过 SSH -R 将 公网服务器:公网端口 转发到 本地IP:本地端口
#
# 【外网无法访问公网IP:端口时】
# 默认 -R 在公网服务器上只监听 127.0.0.1，故本机 curl 127.0.0.1:端口 可行，外网访问不行。
# 解决：在公网服务器上修改 /etc/ssh/sshd_config，添加或改为：
#   GatewayPorts yes
# 然后重启 sshd：systemctl restart sshd
# 若使用 GatewayPorts clientspecified，则需本脚本加 -g 并保持 GatewayPorts clientspecified。
#

set -e

# 默认配置（可被环境变量或命令行参数覆盖）
LOCAL_IP="${LOCAL_IP:-127.0.0.1}"
LOCAL_PORT="${LOCAL_PORT:-22}"
PUBLIC_SERVER="${PUBLIC_SERVER:-}"
PUBLIC_PORT="${PUBLIC_PORT:-}"
SSH_USER="${SSH_USER:-root}"
SSH_PORT="${SSH_PORT:-22}"
SSH_IDENTITY_FILE="${SSH_IDENTITY_FILE:-}"
LOG_FILE="${PORT_FORWARD_LOG_FILE:-}"
# 是否允许外网访问公网端口（默认 1；需公网服务器 sshd 配置 GatewayPorts yes 或 clientspecified）
GATEWAY_PORTS="${GATEWAY_PORTS:-1}"
DIAGNOSE="${DIAGNOSE:-0}"

# 子进程 PID，用于 trap 时结束
TUNNEL_PID=""

log() {
  local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
  if [[ -n "$LOG_FILE" ]]; then
    echo "$msg" >> "$LOG_FILE"
  fi
  echo "$msg" >&2
}

usage() {
  cat <<EOF
用法: $0 [选项] -- 公网服务器 公网端口

将本地 IP:端口 映射到公网服务器端口。他人访问 公网IP:公网端口 即连接到 本地IP:本地端口。
连接断开时会自动重连。

选项:
  -l, --local-ip IP       本地服务地址 (默认: 127.0.0.1)
  -p, --local-port PORT   本地服务端口 (默认: 22)
  -H, --host HOST         公网服务器地址 (也可在末尾 positional 指定)
  -P, --public-port PORT  公网端口 (也可在末尾 positional 指定)
  -u, --user USER         SSH 登录用户名(默认: root)
  -s, --ssh-port PORT     公网服务器 SSH 端口 (默认: 22)
  -i, --identity FILE     SSH 私钥路径
  -L, --log FILE          日志文件路径 (不指定则只输出到 stderr)
  -g, --gateway           允许外网访问公网端口（默认开启，需公网机 sshd GatewayPorts）
  -G, --no-gateway        仅本机可访问公网端口（绑定 127.0.0.1）
  --diagnose              打印公网服务器上的排查命令（外网仍不通时在服务器执行）
  -h, --help              显示此帮助

环境变量: LOCAL_IP, LOCAL_PORT, PUBLIC_SERVER, PUBLIC_PORT, SSH_USER,
         SSH_PORT, SSH_IDENTITY_FILE, PORT_FORWARD_LOG_FILE, GATEWAY_PORTS(1=允许外网)

示例:
  $0 -l 192.168.1.100 -p 8080 -u root vps.example.com 80
  PUBLIC_SERVER=1.2.3.4 PUBLIC_PORT=80 SSH_USER=root $0
EOF
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
  case "$1" in
    -l|--local-ip)    LOCAL_IP="$2";   shift 2 ;;
    -p|--local-port)  LOCAL_PORT="$2"; shift 2 ;;
    -H|--host)        PUBLIC_SERVER="$2"; shift 2 ;;
    -P|--public-port) PUBLIC_PORT="$2"; shift 2 ;;
    -u|--user)        SSH_USER="$2";   shift 2 ;;
    -s|--ssh-port)    SSH_PORT="$2";   shift 2 ;;
    -i|--identity)   SSH_IDENTITY_FILE="$2"; shift 2 ;;
    -L|--log)         LOG_FILE="$2";   shift 2 ;;
    -g|--gateway)     GATEWAY_PORTS=1; shift ;;
    -G|--no-gateway)  GATEWAY_PORTS=0; shift ;;
    --diagnose)       DIAGNOSE=1; shift ;;
    -h|--help)        usage; exit 0 ;;
    --)               shift; break ;;
    -*)               echo "未知选项: $1" >&2; usage >&2; exit 1 ;;
    *)                break ;;
  esac
done

# 末尾两个参数：公网服务器 公网端口
if [[ $# -ge 2 ]]; then
  PUBLIC_SERVER="$1"
  PUBLIC_PORT="$2"
  shift 2
elif [[ $# -eq 1 ]]; then
  if [[ -z "$PUBLIC_SERVER" ]]; then
    PUBLIC_SERVER="$1"
    shift
  elif [[ -z "$PUBLIC_PORT" ]]; then
    PUBLIC_PORT="$1"
    shift
  fi
fi

# 必填校验（diagnose 时可不填公网端口）
if [[ "$DIAGNOSE" != "1" ]] && [[ -z "$PUBLIC_SERVER" || -z "$PUBLIC_PORT" || -z "$SSH_USER" ]]; then
  echo "错误: 必须指定 公网服务器、公网端口 和 SSH 用户。" >&2
  echo "可通过 -H/-P/-u 或环境变量 PUBLIC_SERVER, PUBLIC_PORT, SSH_USER 指定。" >&2
  usage >&2
  exit 1
fi

# 仅打印诊断说明后退出
if [[ "$DIAGNOSE" == "1" ]]; then
  PP="${PUBLIC_PORT:-公网端口}"
  cat <<DIAG
外网无法访问 公网IP:${PP} 时，请 SSH 登录到公网服务器后依次执行下面命令排查：

1) 确认 sshd 已开启 GatewayPorts（需为 yes 或 clientspecified）：
   grep -i GatewayPorts /etc/ssh/sshd_config
   若没有或为 no，请添加/改为：GatewayPorts yes
   然后：sudo systemctl restart sshd  或  sudo service sshd restart

2) 确认端口在公网机上监听在 0.0.0.0（不是 127.0.0.1）：
   ss -tlnp | grep :${PP}
   或：netstat -tlnp | grep :${PP}
   应看到 0.0.0.0:${PP} 或 *:${PP}；若只有 127.0.0.1:${PP} 说明未开 GatewayPorts 或未加 -g。

3) 确认防火墙放行该端口（任选其一执行查看）：
   sudo ufw status
   sudo iptables -L -n | head -30
   若使用 ufw：sudo ufw allow ${PP}/tcp && sudo ufw reload
   若使用 firewalld：sudo firewall-cmd --add-port=${PP}/tcp --permanent && sudo firewall-cmd --reload

4) 云服务器还需在控制台放行安全组/网络 ACL 的入站端口 ${PP}。

5) 本机先测公网机本机是否通：在公网机上执行
   curl -v 127.0.0.1:${PP}
   若本机通、外网不通，多半是 3) 或 4) 未放行。

DIAG
  exit 0
fi

# 构建 -R 与可选 -i；ControlMaster 用于检测 SSH 登录成功
# -g 时绑定 0.0.0.0，需公网机 sshd 的 GatewayPorts yes 或 clientspecified
if [[ "$GATEWAY_PORTS" == "1" ]]; then
  R_SPEC="0.0.0.0:${PUBLIC_PORT}:${LOCAL_IP}:${LOCAL_PORT}"
else
  R_SPEC="${PUBLIC_PORT}:${LOCAL_IP}:${LOCAL_PORT}"
fi
SSH_TARGET="${SSH_USER}@${PUBLIC_SERVER}"
CONTROL_SOCK="/tmp/port_forward_$$.sock"
SSH_OPTS=(
  -o "ServerAliveInterval=30"
  -o "ServerAliveCountMax=3"
  -o "ExitOnForwardFailure=yes"
  -o "ControlPersist=no"
  -M
  -S "$CONTROL_SOCK"
  -N
  -R "$R_SPEC"
)
[[ -n "$SSH_PORT" ]] && SSH_OPTS+=(-p "$SSH_PORT")
[[ -n "$SSH_IDENTITY_FILE" ]] && SSH_OPTS+=(-i "$SSH_IDENTITY_FILE")

# 启动前提示绑定方式（外网不通时便于排查）
if [[ "$R_SPEC" == 0.0.0.0:* ]]; then
  log "转发绑定 0.0.0.0（允许外网访问），公网机需已设 GatewayPorts 且防火墙放行 $PUBLIC_PORT"
else
  log "转发仅绑定 127.0.0.1（仅公网机本机可访问），外网访问请加 -g 并配置 GatewayPorts"
fi

run_ssh() {
  ssh "${SSH_OPTS[@]}" "$SSH_TARGET"
}

# 通过 ControlMaster socket 检测 SSH 是否已登录成功
check_ssh_connected() {
  local check_opts=(-S "$CONTROL_SOCK" -o BatchMode=yes -O check "$SSH_TARGET")
  [[ -n "$SSH_PORT" ]] && check_opts=(-p "$SSH_PORT" "${check_opts[@]}")
  ssh "${check_opts[@]}" 2>/dev/null
}

wait_ssh_ready() {
  local i
  for (( i = 0; i < 20; i++ )); do
    check_ssh_connected && return 0
    sleep 0.5
  done
  return 1
}

cleanup() {
  if [[ -n "$TUNNEL_PID" ]] && kill -0 "$TUNNEL_PID" 2>/dev/null; then
    log "收到退出信号，结束隧道进程 $TUNNEL_PID"
    kill "$TUNNEL_PID" 2>/dev/null || true
    wait "$TUNNEL_PID" 2>/dev/null || true
  fi
  [[ -n "$CONTROL_SOCK" ]] && rm -f "$CONTROL_SOCK"
  exit 0
}

trap cleanup SIGTERM SIGINT EXIT

# 优先使用 autossh，否则 while + ssh
if command -v autossh &>/dev/null; then
  log "使用 autossh: $PUBLIC_SERVER:$PUBLIC_PORT -> $LOCAL_IP:$LOCAL_PORT"
  AUTOSSH_OPTS=(
    -M 0
    -o "ServerAliveInterval=30"
    -o "ServerAliveCountMax=3"
    -o "ExitOnForwardFailure=yes"
    -o "ControlPersist=no"
    -M
    -S "$CONTROL_SOCK"
    -N
    -R "$R_SPEC"
  )
  [[ -n "$SSH_PORT" ]] && AUTOSSH_OPTS+=(-p "$SSH_PORT")
  [[ -n "$SSH_IDENTITY_FILE" ]] && AUTOSSH_OPTS+=(-i "$SSH_IDENTITY_FILE")
  autossh "${AUTOSSH_OPTS[@]}" "$SSH_TARGET" &
  TUNNEL_PID=$!
  if wait_ssh_ready && check_ssh_connected; then
    log "连接成功: $PUBLIC_SERVER:$PUBLIC_PORT -> $LOCAL_IP:$LOCAL_PORT"
  fi
  wait $TUNNEL_PID
else
  log "未检测到 autossh，使用 ssh + 循环自动重连。安装 autossh 可获更好体验。"
  while true; do
    run_ssh &
    TUNNEL_PID=$!
    if wait_ssh_ready && check_ssh_connected; then
      log "连接成功: $PUBLIC_SERVER:$PUBLIC_PORT -> $LOCAL_IP:$LOCAL_PORT"
    fi
    if wait $TUNNEL_PID; then
      :
    else
      log "隧道退出，5 秒后重连..."
    fi
    sleep 5
  done
fi

