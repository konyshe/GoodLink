module goodlink

go 1.25.5

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/getlantern/systray v1.2.2
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/quic-go/quic-go v0.59.0
	go2 v0.0.0
	golang.org/x/sys v0.40.0
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2
	golang.zx2c4.com/wireguard v0.0.0-20250521234502-f333402bd9cb
	goodlink3 v0.0.0
	gvisor.dev/gvisor v0.0.0-20260106215814-b2227fa9cfe0
	proxy v0.0.0
)

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/getlantern/context v0.0.0-20190109183933-c447772a6520 // indirect
	github.com/getlantern/errors v0.0.0-20190325191628-abdb3e3e36f7 // indirect
	github.com/getlantern/golog v0.0.0-20190830074920-4ef2e798c2d7 // indirect
	github.com/getlantern/hex v0.0.0-20190417191902-c6586a6fe0b7 // indirect
	github.com/getlantern/hidden v0.0.0-20190325191715-f02dbb02be55 // indirect
	github.com/getlantern/ops v0.0.0-20190325191751-d70cb0d6f85f // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	go.mongodb.org/mongo-driver v1.17.6 // indirect
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
)

replace go2 => ../go2

replace proxy => ../proxy

replace goroutine-pool => ../goroutine-pool

replace goodlink3 => ../goodlink3
