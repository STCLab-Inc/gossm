package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/gjbae1212/gossm/internal"
	"github.com/gjbae1212/gossm/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	explorerCommand = &cobra.Command{
		Use:   "explorer",
		Short: "Interactive dual-pane file manager via SSM",
		Long: `Interactive dual-pane file manager for transferring files to/from EC2 instances.

No SSH keys required - uses SSM SendCommand for browsing and S3 for file transfer.

Requirements:
  - EC2 instance must have SSM Agent and IAM role with S3 access
  - Local AWS credentials must have SSM and S3 permissions
  - An S3 bucket for temporary file staging

Usage:
  gossm explorer --bucket my-s3-bucket
  gossm explorer --bucket my-s3-bucket --remote-path /home/ec2-user`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			// Validate S3 bucket
			bucket := viper.GetString("explorer-bucket")
			if bucket == "" {
				bucket = os.Getenv("GOSSM_S3_BUCKET")
			}
			if bucket == "" {
				panicRed(fmt.Errorf("S3 bucket is required. Use --bucket flag or GOSSM_S3_BUCKET env var"))
			}

			remotePath := viper.GetString("explorer-remote-path")

			// 1. Select target instance
			color.Cyan("Fetching SSM-connected instances...")
			target, err := internal.AskTarget(ctx, *_credential.awsConfig)
			if err != nil {
				panicRed(err)
			}
			color.Green("Selected: %s", target.Name)

			// 2. Determine remote home directory if not specified
			if remotePath == "" {
				remotePath = "/home/ec2-user"
				color.Yellow("Using default remote path: %s (change with --remote-path)", remotePath)
			}

			// 3. Local directory
			localDir, err := os.Getwd()
			if err != nil {
				panicRed(err)
			}

			// 4. Launch TUI
			model := tui.NewExplorerModel(*_credential.awsConfig, target.Name, localDir, remotePath, bucket)
			p := tea.NewProgram(model, tea.WithAltScreen())

			finalModel, err := p.Run()
			if err != nil {
				panicRed(err)
			}
			_ = finalModel

			color.Green("Explorer closed.")
		},
	}
)

func init() {
	explorerCommand.Flags().StringP("bucket", "b", "", "[required] S3 bucket for temporary file staging")
	explorerCommand.Flags().String("remote-path", "", "[optional] initial remote directory path (default: /home/ec2-user)")

	viper.BindPFlag("explorer-bucket", explorerCommand.Flags().Lookup("bucket"))
	viper.BindPFlag("explorer-remote-path", explorerCommand.Flags().Lookup("remote-path"))

	rootCmd.AddCommand(explorerCommand)
}
