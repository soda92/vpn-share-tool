package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	deployCmd := flag.NewFlagSet("deploy", flag.ExitOnError)
	target := deployCmd.String("target", "server", "SSH target (user@host or host alias)")

	if len(os.Args) < 2 {
		fmt.Println("Usage: dev <subcommand> [args]")
		fmt.Println("Subcommands:")
		fmt.Println("  deploy   Deploy discovery-server")
		fmt.Println("  build    Build main application")
		fmt.Println("  run      Run main application")
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
		if err := runBuild(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	case "run":
		if err := runRun(); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown subcommand: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runBuild() error {
	fmt.Println("Building main application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	frontendDir := filepath.Join(rootDir, "core", "frontend")
	distDir := filepath.Join(frontendDir, "dist")

	// Clean dist
	if err := os.RemoveAll(distDir); err != nil {
		return fmt.Errorf("failed to clean dist dir: %w", err)
	}

	// Build frontend
	if err := runCmd(frontendDir, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Build Go binary
	if err := runCmd(rootDir, "go", "build"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	
	fmt.Println("✅ Build successful.")
	return nil
}

func runRun() error {
	fmt.Println("Running application...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	frontendDir := filepath.Join(rootDir, "core", "frontend")

	// Build frontend
	if err := runCmd(frontendDir, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Run Go app
	if err := runCmd(rootDir, "go", "run", "main.go"); err != nil {
		return fmt.Errorf("go run failed: %w", err)
	}
	return nil
}

func runDeploy(target string) error {
	fmt.Println("Building executable...")

	// Build paths
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	discoveryDir := filepath.Join(rootDir, "discovery-server")
	frontendDir := filepath.Join(discoveryDir, "frontend")
	distDir := filepath.Join(frontendDir, "dist")

	// Clean dist
	if err := os.RemoveAll(distDir); err != nil {
		return fmt.Errorf("failed to clean dist dir: %w", err)
	}

	// Build frontend
	if err := runCmd(frontendDir, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Build Go binary
	// Note: existing script runs 'go build -o discovery-server' inside discovery-server dir
	if err := runCmd(discoveryDir, "go", "build", "-o", "discovery-server"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Printf("Copying executable to %s...\n", target)
	// SCP
	// We assume the binary is at discovery-server/discovery-server
	binaryPath := filepath.Join(discoveryDir, "discovery-server")
	// We use 'scp' command.
	if err := runCmd(rootDir, "scp", binaryPath, fmt.Sprintf("%s:~", target)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	fmt.Printf("Deploying on %s...\n", target)

	// Remote script (Bash compatible)
	// We wrap it in a bash -c command to ensure bash is used regardless of user shell.
	remoteScript := "\nset -e\necho \"--> Stopping discovery-server service...\"\nsudo systemctl stop discovery-server\n\necho \"--> Replacing executable...\"\nsudo mv -f ~/discovery-server /opt/discovery-server\n\necho \"--> Starting discovery-server service...\"\nsudo systemctl start discovery-server\n\necho \"--> Waiting for service to settle...\"\nsleep 3\n\necho \"--> Checking service status...\"\nif systemctl is-failed --quiet discovery-server; then\n    echo \"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\"\n    echo \"!!! Service FAILED to start.       !!!\"\n    echo \"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\"\n    journalctl -u discovery-server -n 20 --no-pager\n    exit 1\nelse\n    echo \"Service started successfully.\"\n    systemctl status discovery-server --no-pager\nfi\n"

	// SSH command
	// usage: ssh <target> 'bash -c "..."'
	if err := runCmd(rootDir, "ssh", target, fmt.Sprintf("bash -c '%s'", remoteScript)); err != nil {
		return fmt.Errorf("ssh deployment failed: %w", err)
	}

	fmt.Println("\n✅ Deployment successful.")
	return nil
}

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
