package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	target := deployCmd.String("target", "server", "SSH target (user@host or host alias)")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "deploy":
		deployCmd.Parse(os.Args[2:])
		if err := runDeploy(*target); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	case "build":
		args := os.Args[2:]
		if len(args) > 0 {
			switch args[0] {
			case "android":
				if err := runBuildAndroidFyne(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			case "aar":
				if err := runBuildAAR(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			case "desktop":
				if err := runBuildDesktop(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			case "linux":
				if err := runBuildLinux(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			case "windows":
				if err := runBuildWindows(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			case "test", "test-project":
				if err := runBuildTestProject(); err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					os.Exit(1)
				}
			default:
				fmt.Printf("Unknown build target: %s\n", args[0])
				printUsage()
				os.Exit(1)
			}
		} else {
			// Default to desktop
			if err := runBuildDesktop(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		}
	case "run":
		args := os.Args[2:]
		if len(args) > 0 && (args[0] == "test" || args[0] == "test-project") {
			if err := runRunTestProject(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			if err := runRunDesktop(); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				os.Exit(1)
			}
		}
	case "flutter":
		if len(os.Args) < 3 {
			fmt.Println("Usage: dev flutter <subcommand> [args]")
			os.Exit(1)
		}
		if err := runFlutter(os.Args[2:]); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown subcommand: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: dev <subcommand> [args]")
	fmt.Println("Subcommands:")
	fmt.Println("  deploy         Deploy discovery-server")
	fmt.Println("  build          Build main application (desktop)")
	fmt.Println("  build android  Build Fyne Android application")
	fmt.Println("  build aar      Build Android AAR for Flutter")
	fmt.Println("  build linux    Build Linux C-shared library")
	fmt.Println("  build windows  Build Windows application (fyne-cross)")
	fmt.Println("  build test     Build test project")
	fmt.Println("  run            Run main application (desktop)")
	fmt.Println("  run test       Run test project")
	fmt.Println("  flutter        Run flutter commands (e.g., dev flutter run)")
}
