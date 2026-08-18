rm -rf goodlink-windows-amd64 goodlink-windows-amd64.zip bin

make clean
make ui
make windows

cd bin
rm -rf goodlink.json
wget https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll
md5sum goodlink-windows-amd64.exe > md5sum.txt
md5sum wintun.dll >> md5sum.txt
zip ../goodlink-windows-amd64.zip goodlink-windows-amd64.exe wintun.dll md5sum.txt
cd ..
rm -rf bin

make clean
make linux
make macos
cd bin

mv ../goodlink-windows-amd64.zip .
md5sum goodlink-linux-amd64 > md5sum.txt; zip goodlink-linux-amd64.zip goodlink-linux-amd64 md5sum.txt; rm -rf goodlink-linux-amd64 md5sum.txt
md5sum goodlink-linux-arm64 > md5sum.txt; zip goodlink-linux-arm64.zip goodlink-linux-arm64 md5sum.txt; rm -rf goodlink-linux-arm64 md5sum.txt
md5sum goodlink-linux-386 > md5sum.txt; zip goodlink-linux-386.zip goodlink-linux-386 md5sum.txt; rm -rf goodlink-linux-386 md5sum.txt
md5sum goodlink-linux-arm > md5sum.txt; zip goodlink-linux-arm.zip goodlink-linux-arm md5sum.txt; rm -rf goodlink-linux-arm md5sum.txt
md5sum goodlink-linux-armv6l > md5sum.txt; zip goodlink-linux-armv6l.zip goodlink-linux-armv6l md5sum.txt; rm -rf goodlink-linux-armv6l md5sum.txt
md5sum goodlink-linux-loong64 > md5sum.txt; zip goodlink-linux-loong64.zip goodlink-linux-loong64 md5sum.txt; rm -rf goodlink-linux-loong64 md5sum.txt
md5sum goodlink-linux-mips > md5sum.txt; zip goodlink-linux-mips.zip goodlink-linux-mips md5sum.txt; rm -rf goodlink-linux-mips md5sum.txt
md5sum goodlink-linux-mipsle > md5sum.txt; zip goodlink-linux-mipsle.zip goodlink-linux-mipsle md5sum.txt; rm -rf goodlink-linux-mipsle md5sum.txt
md5sum goodlink-linux-mips64 > md5sum.txt; zip goodlink-linux-mips64.zip goodlink-linux-mips64 md5sum.txt; rm -rf goodlink-linux-mips64 md5sum.txt
md5sum goodlink-linux-mips64le > md5sum.txt; zip goodlink-linux-mips64le.zip goodlink-linux-mips64le md5sum.txt; rm -rf goodlink-linux-mips64le md5sum.txt
md5sum goodlink-linux-riscv64 > md5sum.txt; zip goodlink-linux-riscv64.zip goodlink-linux-riscv64 md5sum.txt; rm -rf goodlink-linux-riscv64 md5sum.txt
md5sum goodlink-darwin-amd64 > md5sum.txt; zip goodlink-darwin-amd64.zip goodlink-darwin-amd64 md5sum.txt; rm -rf goodlink-darwin-amd64 md5sum.txt
md5sum goodlink-darwin-arm64 > md5sum.txt; zip goodlink-darwin-arm64.zip goodlink-darwin-arm64 md5sum.txt; rm -rf goodlink-darwin-arm64 md5sum.txt
