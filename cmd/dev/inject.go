package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var injectCertCmd = &cobra.Command{
	Use:   "inject-cert",
	Short: "Inject CA certificate into libproxy.py",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInjectCert()
	},
}

func init() {
	rootCmd.AddCommand(injectCertCmd)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}

func runInjectCert() error {
	rootDir, err := findProjectRoot()
	if err != nil {
		return err
	}

	// Try certs/ca.crt first (generated), then core/ca.crt (embedded/committed)
	caPath := filepath.Join(rootDir, "certs", "ca.crt")
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		caPath = filepath.Join(rootDir, "core", "ca.crt")
		if _, err := os.Stat(caPath); os.IsNotExist(err) {
			return fmt.Errorf("CA certificate not found in certs/ or core/")
		}
	}

	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert from %s: %w", caPath, err)
	}
	caContent := string(caBytes)

	libPath := filepath.Join(rootDir, "libproxy.py")
	libBytes, err := os.ReadFile(libPath)
	if err != nil {
		return fmt.Errorf("failed to read libproxy.py: %w", err)
	}
	libContent := string(libBytes)

	destPath := filepath.Join(rootDir, "dist", "libproxy.py")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Regex to match CA_CERT_PEM block (placeholder or existing)
	// Matches: CA_CERT_PEM = """...""" allowing for whitespace
	reCACertBlock := regexp.MustCompile(`CA_CERT_PEM\s*=\s*"""[\s\S]*?"""`)
	
	newDef := fmt.Sprintf("CA_CERT_PEM = \"\"\"%s\"\"\"", caContent)
	
	if reCACertBlock.MatchString(libContent) {
		newContent := reCACertBlock.ReplaceAllString(libContent, newDef)
		
		if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write dist/libproxy.py: %w", err)
		}
		fmt.Printf("✅ CA Certificate injected into %s\n", destPath)
		return nil
	} 
	
	return fmt.Errorf("CA_CERT_PEM definition not found in libproxy.py")
}
