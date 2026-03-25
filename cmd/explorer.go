package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

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

Usage:
  gossm explorer --bucket my-s3-bucket
  gossm explorer --bucket my-s3-bucket -t i-0abc123
  gossm explorer --bucket my-s3-bucket --remote-path /var/log`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			bucket := viper.GetString("explorer-bucket")
			if bucket == "" {
				bucket = os.Getenv("GOSSM_S3_BUCKET")
			}
			if bucket == "" {
				panicRed(fmt.Errorf("S3 bucket required. Use --bucket or GOSSM_S3_BUCKET env var"))
			}

			remotePath := viper.GetString("explorer-remote-path")
			argTarget := strings.TrimSpace(viper.GetString("explorer-target"))

			// Resolve target
			var target *internal.Target
			if argTarget != "" {
				// Direct target specified (used by gossm start --bucket)
				target = &internal.Target{Name: argTarget}
			} else {
				color.Cyan("Fetching SSM-connected instances...")
				var err error
				target, err = internal.AskTarget(ctx, *_credential.awsConfig)
				if err != nil {
					panicRed(err)
				}
			}
			color.Green("Target: %s", target.Name)

			if remotePath == "" {
				remotePath = "/home/ec2-user"
			}

			localDir, err := os.Getwd()
			if err != nil {
				panicRed(err)
			}

			model := tui.NewExplorerModel(*_credential.awsConfig, target.Name, localDir, remotePath, bucket)
			p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

			if _, err := p.Run(); err != nil {
				panicRed(err)
			}
			color.Green("Explorer closed.")
		},
	}
)

func init() {
	explorerCommand.Flags().StringP("bucket", "b", "", "[required] S3 bucket for temporary file staging")
	explorerCommand.Flags().StringP("target", "t", "", "[optional] EC2 instance ID (skip interactive selection)")
	explorerCommand.Flags().String("remote-path", "", "[optional] initial remote directory (default: /home/ec2-user)")

	viper.BindPFlag("explorer-bucket", explorerCommand.Flags().Lookup("bucket"))
	viper.BindPFlag("explorer-target", explorerCommand.Flags().Lookup("target"))
	viper.BindPFlag("explorer-remote-path", explorerCommand.Flags().Lookup("remote-path"))

	rootCmd.AddCommand(explorerCommand)
}
