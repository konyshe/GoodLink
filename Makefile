NAME=goodlink
BINDIR=bin
# 获取源码最近一次 git commit log，包含 commit sha 值，以及 commit message
GitCommitLog=$(shell git log --pretty=oneline -n 1)
BuildTime=$(shell date +'%Y-%m-%d %H:%M:%S')
GOBUILD=GO111MODULE=on \
		GOPROXY="https://goproxy.cn,direct" \
		go build -trimpath -ldflags \
		'-X "gogo.GitCommitLog=$(GitCommitLog)" \
    	-X "gogo.GitStatus=$(GitStatus)" \
    	-X "gogo.BuildTime=$(BuildTime)" \
		-w -s -buildid='

PLATFORM_LIST = \
	linux-386-cmd \
	linux-amd64-cmd \
	linux-arm-cmd \
	linux-arm64-cmd \
	darwin-amd64-cmd \
	darwin-arm64-cmd \
	windows-amd64-cmd \
	windows-arm64-cmd \
	windows-amd64-ui

all: clean $(PLATFORM_LIST) strip

linux-386-cmd:
	GOARCH=386 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-amd64-cmd:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

linux-arm-cmd:
	GOARCH=arm GOOS=linux $(GOBUILD) -tags "cmd" -o $(BINDIR)/$(NAME)-$@

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
	fyne package; mv *.exe bin/

strip:
	upx $(BINDIR)/*

clean:
	rm -rf $(BINDIR) *.exe
