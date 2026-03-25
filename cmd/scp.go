package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fatih/color"
	"github.com/gjbae1212/gossm/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scpCommand = &cobra.Command{
		Use:   "scp",
		Short: "Exec `scp` under AWS SSM with interactive CLI",
		Long: `Exec scp under AWS SSM with interactive CLI.

Interactive mode (recommended):
  gossm scp
  gossm scp -i ~/.ssh/id_rsa
  gossm scp -r                     # recursive copy

Legacy mode (pass raw scp args):
  gossm scp -e '-i key.pem file user@server:/path'`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			exec := strings.TrimSpace(viper.GetString("scp-exec"))
			identity := strings.TrimSpace(viper.GetString("scp-identity"))
			recursive := viper.GetBool("scp-recursive")

			if exec != "" {
				// Legacy mode: pass-through raw scp command
				runScpLegacy(ctx, exec)
				return
			}

			// Interactive mode
			runScpInteractive(ctx, identity, recursive)
		},
	}
)

func runScpInteractive(ctx context.Context, identity string, recursive bool) {
	// 1. Select target server
	target, err := internal.AskTarget(ctx, *_credential.awsConfig)
	if err != nil {
		panicRed(err)
	}
	targetName := target.Name

	// 2. SSH user
	sshUser, err := internal.AskUser()
	if err != nil {
		panicRed(err)
	}

	// 3. Transfer direction
	direction, err := internal.AskDirection()
	if err != nil {
		panicRed(err)
	}

	// 4. Paths
	var localPath, remotePath string
	switch direction {
	case "upload":
		localPath, err = internal.AskPath("Local file/dir path:", "")
		if err != nil {
			panicRed(err)
		}
		// Expand ~ in path
		localPath = expandHome(localPath)
		// Validate local path exists
		if _, statErr := os.Stat(localPath); os.IsNotExist(statErr) {
			panicRed(fmt.Errorf("local path not found: %s", localPath))
		}

		// Suggest remote path based on local filename
		defaultRemote := fmt.Sprintf("/home/%s/%s", sshUser.Name, filepath.Base(localPath))
		if sshUser.Name == "root" {
			defaultRemote = fmt.Sprintf("/root/%s", filepath.Base(localPath))
		}
		remotePath, err = internal.AskPath("Remote destination path:", defaultRemote)
		if err != nil {
			panicRed(err)
		}

	case "download":
		remotePath, err = internal.AskPath("Remote file/dir path:", "")
		if err != nil {
			panicRed(err)
		}

		// Suggest local path based on remote filename
		defaultLocal := fmt.Sprintf("./%s", filepath.Base(remotePath))
		localPath, err = internal.AskPath("Local destination path:", defaultLocal)
		if err != nil {
			panicRed(err)
		}
		localPath = expandHome(localPath)
	}

	// Auto-detect directory for recursive flag
	if !recursive {
		if info, statErr := os.Stat(localPath); statErr == nil && info.IsDir() {
			recursive = true
			color.Yellow("[auto] -r enabled (directory detected)")
		}
	}

	// Resolve domain for SCP connection
	domain := target.PublicDomain
	if domain == "" {
		domain = target.PrivateDomain
	}
	if domain == "" {
		panicRed(fmt.Errorf("no domain found for instance %s", targetName))
	}

	// Build SCP args
	var scpArgs []string
	if direction == "upload" {
		fmt.Printf("\n%s %s %s %s:%s\n",
			color.CyanString("[Upload]"),
			color.GreenString(localPath),
			color.YellowString("→"),
			color.GreenString(targetName),
			color.GreenString(remotePath))
		scpArgs = buildScpArgs(identity, recursive, localPath,
			fmt.Sprintf("%s@%s:%s", sshUser.Name, domain, remotePath))
	} else {
		fmt.Printf("\n%s %s:%s %s %s\n",
			color.CyanString("[Download]"),
			color.GreenString(targetName),
			color.GreenString(remotePath),
			color.YellowString("→"),
			color.GreenString(localPath))
		scpArgs = buildScpArgs(identity, recursive,
			fmt.Sprintf("%s@%s:%s", sshUser.Name, domain, remotePath), localPath)
	}

	// Start SSM session and execute SCP
	execScp(ctx, targetName, scpArgs)
}

func runScpLegacy(ctx context.Context, scpCmd string) {
	seps := strings.Split(scpCmd, " ")
	if len(seps) < 2 {
		panicRed(fmt.Errorf("[err] invalid exec argument"))
	}

	dst := seps[len(seps)-1]
	dstSeps := strings.Split(strings.Split(dst, ":")[0], "@")
	seps = strings.Split(strings.TrimSpace(strings.Join(seps[0:(len(seps)-1)], " ")), " ")

	src := seps[len(seps)-1]
	srcSeps := strings.Split(strings.Split(src, ":")[0], "@")

	var ips []net.IP
	var err error
	switch {
	case len(srcSeps) == 2:
		ips, err = net.LookupIP(srcSeps[1])
	case len(dstSeps) == 2:
		ips, err = net.LookupIP(dstSeps[1])
	default:
		panicRed(fmt.Errorf("[err] invalid scp args"))
	}
	if err != nil {
		panicRed(fmt.Errorf("[err] invalid server domain name"))
	}

	ip := ips[0].String()
	instId, err := internal.FindInstanceIdByIp(ctx, *_credential.awsConfig, ip)
	if err != nil {
		panicRed(err)
	}
	if instId == "" {
		panicRed(fmt.Errorf("[err] not found matched server"))
	}

	internal.PrintReady("scp", _credential.awsConfig.Region, instId)
	color.Cyan("scp " + scpCmd)

	sshArgs := []string{}
	for _, sep := range strings.Split(scpCmd, " ") {
		if sep != "" {
			sshArgs = append(sshArgs, sep)
		}
	}

	execScp(ctx, instId, sshArgs)
}

func execScp(ctx context.Context, targetName string, extraArgs []string) {
	internal.PrintReady("scp", _credential.awsConfig.Region, targetName)

	// Start SSM session
	docName := "AWS-StartSSHSession"
	port := "22"
	input := &ssm.StartSessionInput{
		DocumentName: aws.String(docName),
		Parameters:   map[string][]string{"portNumber": {port}},
		Target:       aws.String(targetName),
	}

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

	// Build ProxyCommand and call scp
	proxy := fmt.Sprintf("ProxyCommand=%s '%s' %s %s %s '%s'",
		_credential.ssmPluginPath, string(sessJson), _credential.awsConfig.Region,
		"StartSession", _credential.awsProfile, string(paramsJson))

	sshArgs := []string{"-o", proxy}
	sshArgs = append(sshArgs, extraArgs...)

	if err := internal.CallProcess("scp", sshArgs...); err != nil {
		color.Red("%v", err)
	}

	// Cleanup session
	if err := internal.DeleteStartSession(ctx, *_credential.awsConfig, &ssm.TerminateSessionInput{
		SessionId: session.SessionId,
	}); err != nil {
		panicRed(err)
	}
}

func buildScpArgs(identity string, recursive bool, src, dst string) []string {
	var args []string
	if identity != "" {
		args = append(args, "-i", identity)
	}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, src, dst)
	return args
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func init() {
	scpCommand.Flags().StringP("exec", "e", "", "[legacy] raw scp args, ex) \"-i key.pem file user@server:/path\"")
	scpCommand.Flags().StringP("identity", "i", "", "[optional] SSH identity file path, ex) ~/.ssh/id_rsa")
	scpCommand.Flags().Bool("recursive", false, "[optional] recursive copy for directories (-r)")

	viper.BindPFlag("scp-exec", scpCommand.Flags().Lookup("exec"))
	viper.BindPFlag("scp-identity", scpCommand.Flags().Lookup("identity"))
	viper.BindPFlag("scp-recursive", scpCommand.Flags().Lookup("recursive"))

	rootCmd.AddCommand(scpCommand)
}
