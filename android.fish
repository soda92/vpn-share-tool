#! /usr/bin/env fish
set -x ANDROID_HOME /home/soda/Android/Sdk
set -x ANDROID_NDK_HOME /home/soda/Android/Sdk/ndk/27.0.12077973/
fyne package -os android -app-id com.example.vpnsharetool -icon Icon.png
