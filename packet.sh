make clean
make ui
make windows
cd bin
rm -rf goodlink.json
wget https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll
cd ..
rm -rf goodlink-windows-amd64
mv bin goodlink-windows-amd64
zip goodlink-windows-amd64.zip goodlink-windows-amd64
rm -rf goodlink-windows-amd64

make clean
make linux
make macos
cd bin

mv ../goodlink-windows-amd64.zip .
zip goodlink-linux-amd64.zip goodlink-linux-amd64; rm -rf goodlink-linux-amd64
zip goodlink-linux-arm64.zip goodlink-linux-arm64; rm -rf goodlink-linux-arm64
zip goodlink-linux-386.zip goodlink-linux-386; rm -rf goodlink-linux-386
zip goodlink-linux-arm.zip goodlink-linux-arm; rm -rf goodlink-linux-arm
zip goodlink-linux-armv6l.zip goodlink-linux-armv6l; rm -rf goodlink-linux-armv6l
zip goodlink-linux-loong64.zip goodlink-linux-loong64; rm -rf goodlink-linux-loong64
zip goodlink-linux-mips.zip goodlink-linux-mips; rm -rf goodlink-linux-mips
zip goodlink-linux-mipsle.zip goodlink-linux-mipsle; rm -rf goodlink-linux-mipsle
zip goodlink-linux-mips64.zip goodlink-linux-mips64; rm -rf goodlink-linux-mips64
zip goodlink-linux-mips64le.zip goodlink-linux-mips64le; rm -rf goodlink-linux-mips64le
zip goodlink-linux-riscv64.zip goodlink-linux-riscv64; rm -rf goodlink-linux-riscv64
zip goodlink-darwin-amd64.zip goodlink-darwin-amd64; rm -rf goodlink-darwin-amd64
zip goodlink-darwin-arm64.zip goodlink-darwin-arm64; rm -rf goodlink-darwin-arm64
