#! /usr/bin/env fish
set -x ANDROID_HOME /opt/android-sdk
set -x ANDROID_NDK_HOME /opt/android-ndk
fyne package -os android -app-id com.example.vpnsharetool -icon Icon.png
