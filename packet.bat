make clean
rm -rf goodlink-windows-amd64
make windows
cd bin
rm -rf goodlink.json
wget https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll
cd ..
mv bin goodlink-windows-amd64
zip goodlink-windows-amd64.zip goodlink-windows-amd64
rm -rf goodlink-windows-amd64

make linux
mv goodlink-windows-amd64.zip bin/
cd bin
zip goodlink-linux-amd64-cmd.zip goodlink-linux-amd64-cmd
zip goodlink-linux-arm64-cmd.zip goodlink-linux-arm64-cmd
zip goodlink-linux-386-cmd.zip goodlink-linux-386-cmd
zip goodlink-linux-arm-cmd.zip goodlink-linux-arm-cmd
zip goodlink-linux-armv6l-cmd.zip goodlink-linux-armv6l-cmd
zip goodlink-windows-arm64-cmd.zip goodlink-windows-arm64-cmd.exe
zip goodlink-linux-loong64-cmd.zip goodlink-linux-loong64-cmd
zip goodlink-linux-mips-cmd.zip goodlink-linux-mips-cmd
zip goodlink-linux-mipsle-cmd.zip goodlink-linux-mipsle-cmd
zip goodlink-linux-mips64-cmd.zip goodlink-linux-mips64-cmd
zip goodlink-linux-mips64le-cmd.zip goodlink-linux-mips64le-cmd
zip goodlink-linux-riscv64-cmd.zip goodlink-linux-riscv64-cmd
