# gossm

Interactive CLI tool for AWS EC2 instances via Systems Manager Session Manager.

> Forked from [gjbae1212/gossm](https://github.com/gjbae1212/gossm) (archived)

## Highlights

### `gossm start --bucket my-s3-bucket`

**SSH 키 없이** S3를 경유하여 EC2 파일 업로드/다운로드가 가능합니다.
터미널과 파일 탐색기가 한 화면에 동시에 표시됩니다.

```
┌──────────────────────────────┬────────────────────────┐
│                              │ ◀ ▶ /home/ec2-user     │
│   SSM Terminal (65%)         │ 📁 app/                 │
│   $ ls /app                  │ 📁 logs/                │
│   main.go  config.yaml      │ 📄 config.yaml     824B │
│   $ _                        │ 📄 data.csv        12M  │
│                              │                        │
│                              │ [Upload] [Download]    │
└──────────────────────────────┴────────────────────────┘
```

- **SSM SendCommand**로 리모트 파일 탐색 (SSH 키 불필요)
- **S3 버킷**을 중간 스테이징으로 사용하여 파일 전송
- **마우스 클릭/스크롤/더블클릭** 지원
- **경로 바 직접 편집**, ◀ ▶ 뒤로/앞으로 네비게이션

## Features

- **Terminal + File Explorer split view** — `gossm start --bucket`
- **Interactive SCP** — Server selection, direction picker, path input
- **Editable path bar** — Click or Ctrl+L to type paths directly
- **Back/Forward navigation** — ◀ ▶ buttons + Alt+Arrow keys
- **File filtering** — Press `/` to filter files in current directory
- **Connection history** — Recent servers shown with ★ at top
- **Mouse support** — Click, scroll, double-click, right-click to select

## Prerequisite

### EC2 Instance
- [required] [AWS SSM Agent](https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html) installed
- [required] **AmazonSSMManagedInstanceCore** IAM policy attached
- [optional] For `ssh`/`scp` commands: SSM Agent **v2.3.672.0+**
- [optional] For file transfer (`--bucket`): IAM role with **S3 read/write** access

### Local Machine
- [required] AWS credentials (`aws_access_key_id`, `aws_secret_access_key`)
- [required] IAM permissions: `ec2:DescribeInstances`, `ssm:StartSession`, `ssm:TerminateSession`, `ssm:DescribeInstanceInformation`, `ssm:SendCommand`, `ssm:GetCommandInvocation`
- [optional] `ec2:DescribeRegions` for region selection
- [optional] `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject` for file transfer
- [optional] `tmux` for split view (`gossm start --bucket`)

## Install

### Homebrew (macOS / Linux)

```bash
brew tap STCLab-Inc/gossm
brew install gossm
```

### Build from source

```bash
git clone https://github.com/STCLab-Inc/gossm.git
cd gossm
go build -o gossm .
mv gossm /usr/local/bin/
```

## Commands

### start — SSM Session + File Explorer

```bash
# Normal terminal session
gossm start

# Terminal + File Explorer split view (recommended)
gossm start --bucket my-s3-bucket

# Direct target (skip selection)
gossm start -t i-0abc123 --bucket my-s3-bucket
```

**File Explorer keyboard shortcuts (right pane):**

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
| `q` | Quit explorer |

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
| `GOSSM_S3_BUCKET` | Default S3 bucket for file transfer |

## License

MIT — see [LICENSE](LICENSE)
