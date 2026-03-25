# gossm (STCLab Fork)

Interactive CLI tool for AWS EC2 instances via Systems Manager Session Manager.
Connect, transfer files, and browse remote servers — **no SSH keys required** for the explorer.

> Forked from [gjbae1212/gossm](https://github.com/gjbae1212/gossm) (archived)

## What's New (STCLab Fork)

- **`gossm explorer`** — Dual-pane file manager with mouse support (SSM + S3, no SSH keys)
- **`gossm start --bucket`** — Terminal + file explorer split view (tmux)
- **`gossm scp`** — Interactive mode (server selection, direction picker, path input)
- **Editable path bar** — Click or Ctrl+L to type paths directly
- **Back/Forward navigation** — ◀ ▶ buttons + Alt+Arrow keys
- **File filtering** — Press `/` to filter files in current directory
- **Connection history** — Recent servers shown with ★ at top
- **S3 temp file cleanup** — Signal handler + auto-purge stale files
- **Mouse support** — Click, scroll, double-click, right-click to select

## Prerequisite

### EC2 Instance
- [required] [AWS SSM Agent](https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html) installed
- [required] **AmazonSSMManagedInstanceCore** IAM policy attached
- [optional] For `ssh`/`scp` commands: SSM Agent **v2.3.672.0+**
- [optional] For `explorer`: IAM role with **S3 read/write** access

### Local Machine
- [required] AWS credentials (`aws_access_key_id`, `aws_secret_access_key`)
- [required] IAM permissions: `ec2:DescribeInstances`, `ssm:StartSession`, `ssm:TerminateSession`, `ssm:DescribeInstanceInformation`, `ssm:SendCommand`, `ssm:GetCommandInvocation`
- [optional] `ec2:DescribeRegions` for region selection
- [optional] `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject` for explorer file transfer
- [optional] `tmux` for split view (`gossm start --bucket`)

## Install

```bash
# Build from source
git clone https://github.com/STCLab-Inc/gossm.git
cd gossm
go build -o gossm .

# Move to PATH
mv gossm /usr/local/bin/
```

## Commands

### start — SSM Session

```bash
# Normal terminal session
gossm start

# Terminal + File Explorer split view (tmux required)
gossm start --bucket my-s3-bucket

# Direct target (skip selection)
gossm start -t i-0abc123
```

### explorer — Dual-Pane File Manager

No SSH keys required. Uses SSM SendCommand for browsing + S3 for file transfer.

```bash
gossm explorer --bucket my-s3-bucket
gossm explorer --bucket my-s3-bucket --remote-path /var/log
gossm explorer --bucket my-s3-bucket -t i-0abc123
```

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `Tab` | Switch panel (Local ↔ Remote) |
| `↑↓` / `jk` | Navigate files |
| `Enter` | Open directory |
| `Space` | Select/deselect file |
| `a` | Select all / deselect all |
| `c` | Copy selected files to other panel |
| `Ctrl+L` | Edit path bar (type path directly) |
| `/` | Filter files in current directory |
| `~` | Go to home directory |
| `Alt+←` / `-` | Go back |
| `Alt+→` | Go forward |
| `r` / `F5` | Refresh |
| `q` | Quit |

**Mouse:**

| Action | Effect |
|--------|--------|
| Click | Select file / switch panel |
| Double-click | Open directory |
| Right-click | Toggle file selection |
| Scroll wheel | Navigate (3 lines) |
| Click `◀` `▶` | Back / Forward |
| Click path bar | Edit path |
| Click `[Upload]` `[Download]` | Transfer files |

### scp — Interactive File Copy

```bash
# Interactive mode (recommended)
gossm scp
gossm scp -i ~/.ssh/id_rsa

# Legacy mode
gossm scp -e '-i key.pem file user@server:/path'
```

### ssh — SSH Session

```bash
gossm ssh                          # interactive server selection
gossm ssh -i ~/.ssh/id_rsa        # with identity file
gossm ssh -e 'user@server'        # direct connection
```

### cmd — Run Command on Multiple Servers

```bash
gossm cmd -e "uptime"
gossm cmd -e "df -h" -t i-0abc123
```

### fwd — Port Forwarding

```bash
gossm fwd -z 8080 -l 42069
```

### mfa — MFA Authentication

```bash
gossm mfa <your-mfa-code>
```

## Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-p` | AWS profile name | `default` / `$AWS_PROFILE` |
| `-r` | AWS region | interactive selection |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `AWS_PROFILE` | Default AWS profile |
| `GOSSM_S3_BUCKET` | Default S3 bucket for explorer |

## License

MIT — see [LICENSE](LICENSE)
