package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var flutterCmd = &cobra.Command{
	Use:                "flutter [args...]",
	Short:              "Run flutter commands",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no flutter subcommand provided")
		}
		return runFlutter(args)
	},
}

func init() {
	rootCmd.AddCommand(flutterCmd)
}

func runFlutter(args []string) error {
	fmt.Println("Running Flutter command...")
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}
	flutterDir := filepath.Join(rootDir, "flutter_gui")

	env := append(os.Environ(),
		"ANDROID_HOME="+androidHome,
		"ANDROID_NDK_HOME="+androidNdkHome,
	)

	if err := execCmd(flutterDir, env, "flutter", args...); err != nil {
		return fmt.Errorf("flutter command failed: %w", err)
	}
	return nil
}
