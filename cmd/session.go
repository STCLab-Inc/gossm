package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fatih/color"
	"github.com/gjbae1212/gossm/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	startSessionCommand = &cobra.Command{
		Use:   "start",
		Short: "Exec `start-session` under AWS SSM with interactive CLI",
		Long: `Exec start-session under AWS SSM with interactive CLI.

With --bucket flag, opens a split view with terminal + file explorer:
  gossm start --bucket my-s3-bucket

Without --bucket, opens a normal SSM terminal session:
  gossm start
  gossm start -t i-0abc123`,
		Run: func(cmd *cobra.Command, args []string) {
			var (
				target *internal.Target
				err    error
			)
			ctx := context.Background()

			// get target
			argTarget := strings.TrimSpace(viper.GetString("start-session-target"))
			if argTarget != "" {
				table, err := internal.FindInstances(ctx, *_credential.awsConfig)
				if err != nil {
					panicRed(err)
				}
				for _, t := range table {
					if t.Name == argTarget {
						target = t
						break
					}
				}
			}
			if target == nil {
				target, err = internal.AskTarget(ctx, *_credential.awsConfig)
				if err != nil {
					panicRed(err)
				}
			}

			// Check if --bucket is specified -> split mode with file explorer
			bucket := strings.TrimSpace(viper.GetString("start-bucket"))
			if bucket == "" {
				bucket = os.Getenv("GOSSM_S3_BUCKET")
			}

			if bucket != "" {
				runSplitMode(target, bucket)
				return
			}

			// Normal SSM session
			runNormalSession(ctx, target)
		},
	}
)

// runSplitMode launches tmux with terminal + file explorer side by side.
func runSplitMode(target *internal.Target, bucket string) {
	// Check tmux availability
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		color.Yellow("tmux not found. Install tmux for split view: brew install tmux")
		color.Yellow("Falling back to normal session...")
		runNormalSession(context.Background(), target)
		return
	}

	internal.PrintReady("start+explorer", _credential.awsConfig.Region, target.Name)

	// Build gossm commands for each pane
	gossmPath, _ := os.Executable()
	if gossmPath == "" {
		gossmPath = "gossm"
	}

	// Common flags
	var commonFlags []string
	if _credential.awsProfile != "default" {
		commonFlags = append(commonFlags, "-p", _credential.awsProfile)
	}
	if _credential.awsConfig.Region != "" {
		commonFlags = append(commonFlags, "-r", _credential.awsConfig.Region)
	}

	// Left pane: gossm start -t <instance-id>
	startCmd := fmt.Sprintf("%s start -t %s %s",
		gossmPath, target.Name, strings.Join(commonFlags, " "))

	// Right pane: gossm explorer --bucket <bucket> -t <instance-id>
	explorerCmd := fmt.Sprintf("%s explorer --bucket %s -t %s %s",
		gossmPath, bucket, target.Name, strings.Join(commonFlags, " "))

	sessionName := fmt.Sprintf("gossm-%s", target.Name)

	// Kill existing session if any
	exec.Command(tmuxPath, "kill-session", "-t", sessionName).Run()

	// Create tmux session with the terminal in the main pane
	// Then split horizontally and run explorer in the right pane
	tmuxArgs := []string{
		"new-session", "-d", "-s", sessionName,
		"-x", "200", "-y", "50",
		startCmd,
	}

	if err := exec.Command(tmuxPath, tmuxArgs...).Run(); err != nil {
		color.Red("Failed to create tmux session: %v", err)
		color.Yellow("Falling back to normal session...")
		runNormalSession(context.Background(), target)
		return
	}

	// Split the window: right pane (35%) for explorer
	splitArgs := []string{
		"split-window", "-h", "-t", sessionName,
		"-p", "35",
		explorerCmd,
	}
	if err := exec.Command(tmuxPath, splitArgs...).Run(); err != nil {
		color.Red("Failed to split tmux pane: %v", err)
		// Attach to the session anyway (terminal only)
	}

	// Focus on the left pane (terminal)
	exec.Command(tmuxPath, "select-pane", "-t", sessionName+":0.0").Run()

	// Enable mouse support in tmux
	exec.Command(tmuxPath, "set-option", "-t", sessionName, "mouse", "on").Run()

	// Attach to the tmux session
	attachCmd := exec.Command(tmuxPath, "attach-session", "-t", sessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr

	color.Green("Launching split view: Terminal (left) + Explorer (right)")
	color.Green("tmux session: %s", sessionName)
	color.Yellow("Tip: Click on panes to switch. Use 'exit' in terminal to close.")

	if err := attachCmd.Run(); err != nil {
		color.Red("tmux attach failed: %v", err)
	}

	// Cleanup: kill the tmux session when done
	exec.Command(tmuxPath, "kill-session", "-t", sessionName).Run()
}

// runNormalSession runs a standard SSM session (original behavior).
func runNormalSession(ctx context.Context, target *internal.Target) {
	internal.PrintReady("start-session", _credential.awsConfig.Region, target.Name)

	input := &ssm.StartSessionInput{Target: aws.String(target.Name)}
	session, err := internal.CreateStartSession(ctx, *_credential.awsConfig, input)
	if err != nil {
		panicRed(err)
	}

	sessJson, err := json.Marshal(session)
	if err != nil {
		panicRed(err)
	}

	paramsJson, err := json.Marshal(input)
	if err != nil {
		panicRed(err)
	}

	if err := internal.CallProcess(_credential.ssmPluginPath, string(sessJson),
		_credential.awsConfig.Region, "StartSession",
		_credential.awsProfile, string(paramsJson)); err != nil {
		color.Red("%v", err)
	}

	if err := internal.DeleteStartSession(ctx, *_credential.awsConfig, &ssm.TerminateSessionInput{
		SessionId: session.SessionId,
	}); err != nil {
		panicRed(err)
	}
}

func init() {
	startSessionCommand.Flags().StringP("target", "t", "", "[optional] EC2 instance ID")
	startSessionCommand.Flags().StringP("bucket", "b", "", "[optional] S3 bucket - enables split view with file explorer")

	viper.BindPFlag("start-session-target", startSessionCommand.Flags().Lookup("target"))
	viper.BindPFlag("start-bucket", startSessionCommand.Flags().Lookup("bucket"))

	rootCmd.AddCommand(startSessionCommand)
}
