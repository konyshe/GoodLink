NAME=goodlink
BINDIR=bin
# 获取源码最近一次 git commit log，包含 commit sha 值，以及 commit message
GitCommitLog=$(shell git log --pretty=oneline -n 1)
BuildTime=$(shell date +'%Y-%m-%d %H:%M:%S')
GOBUILD=GO111MODULE=on \
		GOPROXY="https://goproxy.cn,direct" \
		GOEXPERIMENT=jsonv2,loopvar \
		go build -trimpath -ldflags \
		'-X "go2.GitCommitLog=$(GitCommitLog)" \
    	-X "go2.GitStatus=$(GitStatus)" \
    	-X "go2.BuildTime=$(BuildTime)" \
		-w -s -buildid='
LINUX_PLATFORM_LIST = \
	linux-386 \
	linux-amd64 \
	linux-arm \
	linux-armv6l \
	linux-arm64 \
	linux-loong64 \
	linux-mips \
	linux-mipsle \
	linux-mips64 \
	linux-riscv64 \
	linux-mips64le \

DARWIN_PLATFORM_LIST = \
	darwin-amd64 \
	darwin-arm64 \

WINDOWS_PLATFORM_LIST = \
	windows-amd64 \

WINDOWS_PLATFORM2_LIST = \
	windows-386 \
	windows-arm \

.PHONY: ui
ui:
	cd ui2/frontend && npm ci && npm run build

debug: create_nac $(WINDOWS_PLATFORM_LIST) rm_nac linux-amd64

macos: $(DARWIN_PLATFORM_LIST) strip

windows: create_nac $(WINDOWS_PLATFORM_LIST) rm_nac strip

# linux-build 只产出二进制，不调用 upx；packet.sh 在 glibc 2.17 容器中使用该目标
linux-build: $(LINUX_PLATFORM_LIST)

linux: linux-build strip

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-loong64:
	GOARCH=loong64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips:
	GOARCH=mips GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle:
	GOARCH=mipsle GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-riscv64:
	GOARCH=riscv64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-arm:
	GOARCH=arm GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv6l:
	GOARCH=arm GOARM=6 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-arm64:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-arm64:
	GOARCH=arm64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-arm64:
	GOARCH=arm64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-386:
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-arm:
	GOARCH=arm GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe
create_nac:
	rsrc -manifest nac.manifest -ico assert/favicon.ico -o nac.syso

rm_nac:
	rm -rf nac.syso

strip:
	upx $(BINDIR)/*

clean:
	rm -rf $(BINDIR) *.exe
