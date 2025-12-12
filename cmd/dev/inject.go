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

	destPath := filepath.Join(rootDir, "dist", "libproxy.py")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Replace placeholder
	placeholder := `CA_CERT_PEM = """__CA_CERT_PLACEHOLDER__"""`
	newDef := fmt.Sprintf("CA_CERT_PEM = \"\"\"%s\"\"\"", caContent)
	
	var newContent string
	if strings.Contains(libContent, placeholder) {
		newContent = strings.Replace(libContent, placeholder, newDef, 1)
	} else if strings.Contains(libContent, "CA_CERT_PEM = \"\"\"") {
		// Already injected? Re-inject with current cert
		fmt.Println("ℹ️ Source libproxy.py seems already injected. Updating in dist...")
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
		
		newContent = libContent[:startIndex] + newDef + libContent[endIndex+3:]
	} else {
		return fmt.Errorf("placeholder not found in libproxy.py")
	}
	
	if err := os.WriteFile(destPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write dist/libproxy.py: %w", err)
	}

	fmt.Printf("✅ CA Certificate injected into %s\n", destPath)
	return nil
}
