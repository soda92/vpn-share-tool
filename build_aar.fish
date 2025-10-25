#!/usr/bin/env fish

set -x ANDROID_NDK_HOME /home/soda/Android/Sdk/ndk/27.0.12077973
gomobile bind -target=android -androidapi 21 -o flutter_gui/android/libs/core.aar github.com/soda92/vpn-share-tool/mobile
