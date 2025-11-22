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

	// Build frontend
	if err := buildFrontendIn(frontendDir); err != nil {
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

	remoteScript := `
set -e
echo "--> Stopping discovery-server service..."
sudo systemctl stop discovery-server

echo "--> Replacing executable..."
sudo mv -f ~/discovery-server /opt/discovery-server

echo "--> Starting discovery-server service..."
sudo systemctl start discovery-server

echo "--> Waiting for service to settle..."
sleep 3

echo "--> Checking service status..."
if systemctl is-failed --quiet discovery-server; then
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo "!!! Service FAILED to start.       !!!"
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    journalctl -u discovery-server -n 20 --no-pager
    exit 1
else
    echo "Service started successfully."
    systemctl status discovery-server --no-pager
fi
`

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