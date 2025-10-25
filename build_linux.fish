#!/usr/bin/env fish

go build -buildmode=c-shared -o flutter_gui/linux/libcore.so ./linux_bridge
