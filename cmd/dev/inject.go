package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func runInjectCert() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	caPath := filepath.Join(rootDir, "certs", "ca.crt")
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert: %w", err)
	}
	caContent := string(caBytes)

	libPath := filepath.Join(rootDir, "libproxy.py")
	libBytes, err := os.ReadFile(libPath)
	if err != nil {
		return fmt.Errorf("failed to read libproxy.py: %w", err)
	}
	libContent := string(libBytes)

	// Replace placeholder
	placeholder := `CA_CERT_PEM = """__CA_CERT_PLACEHOLDER__"""`
	
	// Escape backslashes in CA content if any (PEM usually fine, but windows line endings?)
	// Python multiline string handles it well.
	
	newDef := fmt.Sprintf("CA_CERT_PEM = \"\"\"%s\"\"\"", caContent)
	
	if !strings.Contains(libContent, placeholder) {
		// Check if it's already injected or modified
		if strings.Contains(libContent, "CA_CERT_PEM = \"\"\"----BEGIN CERTIFICATE") {
			fmt.Println("ℹ️ CA Cert seems already injected. Updating...")
			// Regex replace could be safer, but let's assume standard format
			// Simple start/end match
			startMarker := "CA_CERT_PEM = \"\"\""
			startIndex := strings.Index(libContent, startMarker)
			if startIndex == -1 {
				return fmt.Errorf("could not find CA_CERT_PEM definition")
			}
			endIndex := strings.Index(libContent[startIndex+len(startMarker):], "\"\"\"")
			if endIndex == -1 {
				return fmt.Errorf("could not find end of CA_CERT_PEM definition")
			}
			endIndex += startIndex + len(startMarker) // Adjust to absolute
			
			newContent := libContent[:startIndex] + newDef + libContent[endIndex+3:]
			if err := os.WriteFile(libPath, []byte(newContent), 0644); err != nil {
				return err
			}
			fmt.Println("✅ CA Certificate updated in libproxy.py")
			return nil
		}
		return fmt.Errorf("placeholder not found in libproxy.py")
	}

	newContent := strings.Replace(libContent, placeholder, newDef, 1)
	
	if err := os.WriteFile(libPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write libproxy.py: %w", err)
	}

	fmt.Println("✅ CA Certificate injected into libproxy.py")
	return nil
}
