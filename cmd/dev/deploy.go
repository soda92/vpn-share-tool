package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func runDeploy(target string) error {
	if err := checkConnection(target); err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}

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
	if err := runCmd(frontendDir, nil, "npm", "run", "build"); err != nil {
		return fmt.Errorf("frontend build failed: %w", err)
	}

	// Build Go binary
	if err := runCmd(discoveryDir, nil, "go", "build", "-o", "discovery-server"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Printf("Copying executable to %s...\n", target)
	binaryPath := filepath.Join(discoveryDir, "discovery-server")
	if err := runCmd(rootDir, nil, "scp", binaryPath, fmt.Sprintf("%s:~", target)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	fmt.Printf("Deploying on %s...\n", target)

	remoteScript := "\nset -e\necho \"--> Stopping discovery-server service...\"\nsudo systemctl stop discovery-server\n\necho \"--> Replacing executable...\"\nsudo mv -f ~/discovery-server /opt/discovery-server\n\necho \"--> Starting discovery-server service...\"\nsudo systemctl start discovery-server\n\necho \"--> Waiting for service to settle...\"\nsleep 3\n\necho \"--> Checking service status...\"\nif systemctl is-failed --quiet discovery-server; then\n    echo \"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\"\n    echo \"!!! Service FAILED to start.       !!!\"\n    echo \"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\"\n    journalctl -u discovery-server -n 20 --no-pager\n    exit 1\nelse\n    echo \"Service started successfully.\"\n    systemctl status discovery-server --no-pager\nfi\n"

	if err := runCmd(rootDir, nil, "ssh", target, fmt.Sprintf("bash -c '%s'", remoteScript)); err != nil {
		return fmt.Errorf("ssh deployment failed: %w", err)
	}

	fmt.Println("\n✅ Deployment successful.")
	return nil
}

func checkConnection(target string) error {
	fmt.Printf("Checking SSH connection to %s...\n", target)
	// We use -o ConnectTimeout=5 to fail fast if IP is unreachable.
	// We run 'exit' to just check connectivity/auth.
	// Stdin is connected via runCmd, so password prompts will work.
	return runCmd(".", nil, "ssh", "-o", "ConnectTimeout=5", target, "echo '✅ Connection established'")
}