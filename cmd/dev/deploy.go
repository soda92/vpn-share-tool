package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var deployTarget string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy discovery-server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeploy(deployTarget)
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVar(&deployTarget, "target", "server", "SSH target (user@host or host alias)")
}

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
	if err := execCmd(discoveryDir, nil, "go", "build", "-o", "discovery-server"); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	fmt.Printf("Copying executable to %s...\n", target)
	binaryPath := filepath.Join(discoveryDir, "discovery-server")
	if err := execCmd(rootDir, nil, "scp", binaryPath, fmt.Sprintf("%s:~", target)); err != nil {
		return fmt.Errorf("scp binary failed: %w", err)
	}

	fmt.Printf("Copying service file to %s...\n", target)
	servicePath := filepath.Join(discoveryDir, "discovery-server.service")
	if err := execCmd(rootDir, nil, "scp", servicePath, fmt.Sprintf("%s:~", target)); err != nil {
		return fmt.Errorf("scp service file failed: %w", err)
	}

	fmt.Printf("Deploying on %s...\n", target)

	remoteScript := `
set -e
echo "--> Stopping discovery-server service..."
# Ignore error if service doesn't exist yet
sudo systemctl stop discovery-server || true
sudo systemctl disable discovery-server || true

echo "--> Creating/Ensuring working directory..."
sudo mkdir -p /var/lib/discovery-server
sudo chown nobody:nogroup /var/lib/discovery-server

echo "--> Installing executable..."
sudo mv -f ~/discovery-server /opt/discovery-server
sudo chmod +x /opt/discovery-server

echo "--> Installing service file..."
sudo rm -f /etc/systemd/system/discovery-server.service
sudo mv -f ~/discovery-server.service /etc/systemd/system/discovery-server.service

echo "--> Reloading systemd..."
sudo systemctl daemon-reload

echo "--> Starting discovery-server service..."
sudo systemctl enable discovery-server
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
	// Write remote script to a temporary file
	tmpScript, err := os.CreateTemp("", "deploy_script_*.sh")
	if err != nil {
		return fmt.Errorf("failed to create temp script: %w", err)
	}
	defer os.Remove(tmpScript.Name())

	if _, err := tmpScript.WriteString(remoteScript); err != nil {
		return fmt.Errorf("failed to write to temp script: %w", err)
	}
	tmpScript.Close()

	// SCP the script to the server
	remoteTmpScript := "/tmp/discovery_deploy.sh"
	if err := execCmd(rootDir, nil, "scp", tmpScript.Name(), fmt.Sprintf("%s:%s", target, remoteTmpScript)); err != nil {
		return fmt.Errorf("scp script failed: %w", err)
	}

	// Execute the script on the server using bash
	if err := execCmd(rootDir, nil, "ssh", target, fmt.Sprintf("bash %s && rm %s", remoteTmpScript, remoteTmpScript)); err != nil {
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
	return execCmd(".", nil, "ssh", "-o", "ConnectTimeout=5", target, "echo '✅ Connection established'")
}
