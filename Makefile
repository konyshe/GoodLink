NAME=goodlink
BINDIR=bin
# 获取源码最近一次 git commit log，包含 commit sha 值，以及 commit message
GitCommitLog=$(shell git log --pretty=oneline -n 1)
BuildTime=$(shell date +'%Y-%m-%d %H:%M:%S')
GOBUILD=GO111MODULE=on \
		GOPROXY="https://goproxy.cn,direct" \
		GOEXPERIMENT=jsonv2 \
		GOEXPERIMENT=loopvar \
		go build -trimpath -ldflags \
		'-X "go2.GitCommitLog=$(GitCommitLog)" \
    	-X "go2.GitStatus=$(GitStatus)" \
    	-X "go2.BuildTime=$(BuildTime)" \
		-w -s -buildid='

LINUX_PLATFORM_LIST = \
	linux-386-cmd \
	linux-amd64-cmd \
	linux-arm-cmd \
	linux-armv6l-cmd \
	linux-arm64-cmd \
	linux-loong64-cmd \
	linux-mips-cmd \
	linux-mipsle-cmd \
	linux-mips64-cmd \
	linux-riscv64-cmd \
	linux-mips64le-cmd \

WINDOWS_PLATFORM_LIST = \
	windows-amd64-ui \
	windows-amd64-cmd \

debug: create_nac $(WINDOWS_PLATFORM_LIST) rm_nac

windows: create_nac $(WINDOWS_PLATFORM_LIST) rm_nac strip

linux: $(LINUX_PLATFORM_LIST) strip

linux-386-cmd:
	GOARCH=386 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-amd64-cmd:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-loong64-cmd:
	GOARCH=loong64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-mips-cmd:
	GOARCH=mips GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-mipsle-cmd:
	GOARCH=mipsle GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-mips64-cmd:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-mips64le-cmd:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-riscv64-cmd:
	GOARCH=riscv64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-arm-cmd:
	GOARCH=arm GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-armv6l-cmd:
	GOARCH=arm GOARM=6 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-arm64-cmd:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

darwin-amd64-cmd:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

darwin-arm64-cmd:
	GOARCH=arm64 GOOS=darwin $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

windows-amd64-cmd:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@.exe

windows-arm64-cmd:
	GOARCH=arm64 GOOS=windows $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64-ui:
#	CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 fyne package -os windows -icon theme/favicon.ico
#	go build -ldflags -H=windowsgui
	mkdir bin; fyne package; mv *.exe bin/

create_nac:
	rsrc -manifest nac.manifest -o nac.syso

rm_nac:
	rm -rf nac.syso

strip:
	upx $(BINDIR)/*

clean:
	rm -rf $(BINDIR) *.exe
