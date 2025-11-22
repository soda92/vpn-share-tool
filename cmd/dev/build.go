package main

import (
	"fmt"
	"os"
)

func runBuildDesktop() error {
	fmt.Println("Building main application (Desktop)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontend(rootDir); err != nil {
		return err
	}

	// Build Go binary
	if err := runCmd(rootDir, nil, "go", "build"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Println("✅ Build successful.")
	return nil
}

func runBuildAndroidFyne() error {
	fmt.Println("Building Fyne Android application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	env := append(os.Environ(),
		"ANDROID_HOME="+androidHome,
		"ANDROID_NDK_HOME="+androidNdkHome,
	)

	if err := runCmd(rootDir, env, "fyne", "package", "-os", "android", "-app-id", "com.example.vpnsharetool", "-icon", "Icon.png"); err != nil {
		return fmt.Errorf("fyne package failed: %w", err)
	}

	fmt.Println("✅ Android Fyne build successful.")
	return nil
}

func runBuildAAR() error {
	fmt.Println("Building Android AAR for Flutter...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	env := append(os.Environ(),
		"ANDROID_NDK_HOME="+androidNdkHome,
	)

	// gomobile bind -target=android -androidapi 21 -o flutter_gui/android/libs/core.aar github.com/soda92/vpn-share-tool/mobile
	if err := runCmd(rootDir, env, "gomobile", "bind", "-target=android", "-androidapi", "21", "-o", "flutter_gui/android/libs/core.aar", "github.com/soda92/vpn-share-tool/mobile"); err != nil {
		return fmt.Errorf("gomobile bind failed: %w", err)
	}

	fmt.Println("✅ AAR build successful.")
	return nil
}

func runBuildLinux() error {
	fmt.Println("Building Linux C-shared library for Flutter...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	// go build -buildmode=c-shared -o flutter_gui/linux/libcore.so ./linux_bridge
	if err := runCmd(rootDir, nil, "go", "build", "-buildmode=c-shared", "-o", "flutter_gui/linux/libcore.so", "./linux_bridge"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	fmt.Println("✅ Linux build successful.")
	return nil
}

func runBuildWindows() error {
	fmt.Println("Building Windows application (cross-compile)...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// Build frontend
	if err := buildFrontend(rootDir); err != nil {
		return err
	}

	// fyne-cross windows -arch amd64 --app-id vpn.share.tool
	if err := runCmd(rootDir, nil, "fyne-cross", "windows", "-arch", "amd64", "--app-id", "vpn.share.tool"); err != nil {
		return fmt.Errorf("fyne-cross failed: %w", err)
	}
	fmt.Println("✅ Windows build successful.")
	return nil
}
