package tui

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// RunRemoteCommand executes a shell command on a remote instance via SSM SendCommand
// and returns the stdout output.
func RunRemoteCommand(cfg aws.Config, instanceId, command string) (string, error) {
	ctx := context.Background()
	client := ssm.NewFromConfig(cfg)

	output, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName:   aws.String("AWS-RunShellScript"),
		InstanceIds:    []string{instanceId},
		Parameters:     map[string][]string{"commands": {command}},
		TimeoutSeconds: 30,
	})
	if err != nil {
		return "", fmt.Errorf("SendCommand: %w", err)
	}

	commandId := output.Command.CommandId

	// Poll for result with initial delay
	time.Sleep(800 * time.Millisecond)
	for i := 0; i < 30; i++ {
		inv, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  commandId,
			InstanceId: aws.String(instanceId),
		})
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		status := strings.ToLower(string(inv.Status))
		switch status {
		case "pending", "inprogress", "delayed":
			time.Sleep(500 * time.Millisecond)
			continue
		case "success":
			return aws.ToString(inv.StandardOutputContent), nil
		default:
			stderr := aws.ToString(inv.StandardErrorContent)
			if stderr == "" {
				stderr = "unknown error"
			}
			return "", fmt.Errorf("command failed: %s", stderr)
		}
	}

	return "", fmt.Errorf("command timed out after 15s")
}

// ListRemoteDir lists files in a remote directory via SSM.
func ListRemoteDir(cfg aws.Config, instanceId, dirPath string) ([]FileEntry, error) {
	// Check if dir exists and list it in one command
	cmd := fmt.Sprintf("test -d %s && ls -1pa %s || echo 'GOSSM_DIR_NOT_FOUND'", shellQuote(dirPath), shellQuote(dirPath))
	output, err := RunRemoteCommand(cfg, instanceId, cmd)
	if err != nil {
		return nil, fmt.Errorf("cannot list %s: %w", dirPath, err)
	}
	output = strings.TrimSpace(output)
	if output == "GOSSM_DIR_NOT_FOUND" {
		return nil, fmt.Errorf("directory not found: %s", dirPath)
	}
	return parseRemoteListing(output), nil
}

// TransferUpload transfers a file from local to remote via S3.
// Flow: local -> S3 -> remote (SSM SendCommand "aws s3 cp")
func TransferUpload(cfg aws.Config, instanceId, localPath, remotePath, bucket, s3Prefix string) error {
	filename := path.Base(localPath)
	s3Key := fmt.Sprintf("%s/%d-%s", s3Prefix, time.Now().UnixNano(), filename)
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, s3Key)

	// 1. Local -> S3
	if out, err := exec.Command("aws", "s3", "cp", localPath, s3URI, "--quiet").CombinedOutput(); err != nil {
		return fmt.Errorf("S3 upload failed: %s", string(out))
	}
	defer s3Delete(bucket, s3Key)

	// 2. S3 -> Remote
	remoteCmd := fmt.Sprintf("aws s3 cp %s %s", s3URI, shellQuote(remotePath))
	if _, err := RunRemoteCommand(cfg, instanceId, remoteCmd); err != nil {
		return fmt.Errorf("remote download from S3 failed: %w", err)
	}

	return nil
}

// TransferDownload transfers a file from remote to local via S3.
// Flow: remote (SSM SendCommand "aws s3 cp") -> S3 -> local
func TransferDownload(cfg aws.Config, instanceId, remotePath, localPath, bucket, s3Prefix string) error {
	filename := path.Base(remotePath)
	s3Key := fmt.Sprintf("%s/%d-%s", s3Prefix, time.Now().UnixNano(), filename)
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, s3Key)

	// 1. Remote -> S3
	remoteCmd := fmt.Sprintf("aws s3 cp %s %s", shellQuote(remotePath), s3URI)
	if _, err := RunRemoteCommand(cfg, instanceId, remoteCmd); err != nil {
		return fmt.Errorf("remote upload to S3 failed: %w", err)
	}
	defer s3Delete(bucket, s3Key)

	// 2. S3 -> Local
	if out, err := exec.Command("aws", "s3", "cp", s3URI, localPath, "--quiet").CombinedOutput(); err != nil {
		return fmt.Errorf("S3 download failed: %s", string(out))
	}

	return nil
}

func s3Delete(bucket, s3Key string) {
	s3URI := fmt.Sprintf("s3://%s/%s", bucket, s3Key)
	exec.Command("aws", "s3", "rm", s3URI, "--quiet").Run()
}

func parseRemoteListing(output string) []FileEntry {
	entries := []FileEntry{{Name: "..", IsDir: true}}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "./" || line == "../" {
			continue
		}

		isDir := strings.HasSuffix(line, "/")
		name := strings.TrimRight(line, "/*@|")

		if name == "" {
			continue
		}

		entries = append(entries, FileEntry{Name: name, IsDir: isDir})
	}

	return entries
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
