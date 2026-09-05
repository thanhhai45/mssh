# mssh

A small desktop app for managing and connecting to remote servers, built with
[Wails](https://wails.io) (Go + React). It keeps your connections organised in
workspaces, opens real SSH sessions, and keeps them alive while you move around
the app.

Passwords are stored in your operating system's credential store, never in the
application database.

---

## Install

### Download a build

Grab the latest `mssh-<version>-macos-universal.zip` from the
[Releases page](https://github.com/thanhhai45/mssh/releases), unzip it, and drag
`mssh.app` into `/Applications`.

**macOS will refuse to open it the first time.** The app is not signed with an
Apple Developer certificate, so Gatekeeper blocks it:

> "mssh" cannot be opened because Apple cannot check it for malicious software.

Clear the quarantine flag once:

```bash
xattr -dr com.apple.quarantine /Applications/mssh.app
```
---
```
## Features

- **Workspaces** — group connections by account, environment or customer, each
  with its own colour and AWS defaults.
- **Three ways to reach a machine** — direct SSH, AWS Session Manager, or SSH
  tunnelled through Session Manager.
- **Sessions that survive navigation** — switch to another connection and back;
  your scrollback, working directory and running `top` are all still there.
- **Real terminal** — a full pty, so `vim`, `htop` and colours work, and the
  remote shell follows the window when you resize it.
- **Local SQLite database** — everything lives on your machine. No account, no
  sync, no telemetry.

---

## Connection kinds

| | **Direct SSH** | **AWS SSM** | **SSH over SSM** |
|---|---|---|---|
| Talks to | port 22 on the host | the AWS Session Manager API | the API, then SSH inside it |
| Needs on your machine | nothing extra | AWS CLI + Session Manager plugin | AWS CLI + plugin + an SSH key |
| Needs on the server | an open SSH port | SSM Agent and an IAM role | SSM Agent, IAM role, and `sshd` |
| You land as | the user you configure | `ssm-user` | the user you configure |
| Authenticates with | your key, agent or password | IAM, via the AWS CLI | IAM **and** SSH credentials |
| Works with `scp` later | yes | no | yes |

Pick **Direct SSH** for anything reachable over the network. Pick **AWS SSM**
for EC2 instances with no open port and no public IP. Pick **SSH over SSM** when
you want the tunnel *and* your own account rather than `ssm-user`.

---

## Requirements

### Always

- **macOS** (developed and tested here) or **Linux**.
  Windows is untested: the SSM kinds need a pseudo-terminal, which behaves
  differently there.

Nothing else. Direct SSH connections work out of the box — the SSH client is
compiled into the binary.

### Only for the AWS SSM kinds

```bash
brew install awscli
brew install --cask session-manager-plugin
```

The app checks for both before it tries to connect, and tells you which one is
missing rather than failing silently. It also reports an expired SSO session and
prints the `aws sso login` command you need.

Your AWS credentials stay entirely with the AWS CLI. mssh only passes it a
profile and region name.

### Only to build from source

- Go 1.26 or newer
- Node.js 20 or newer
- The Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

---

## Install

### Build from source

```bash
git clone git@github.com:thanhhai45/mssh.git && cd mssh
wails build
```

The application is written to `build/bin/mssh.app`. Move it into
`/Applications` if you want it in Launchpad.

macOS will refuse to open it the first time because it is not signed by a
registered developer. Right-click the app, choose **Open**, then confirm — you
only have to do this once.

---

## First run

The database is created automatically the first time the app starts:

| Platform | Location |
|---|---|
| macOS | `~/Library/Application Support/mssh/mssh.db` |
| Linux | `~/.config/mssh/mssh.db` |

To back up your connections, copy that file. To start over, delete it — a fresh
one is created on the next launch.

The database schema is versioned and upgraded in place, so your data survives
app updates.

---

## ⚠️ Connecting to a server for the first time

**This is the one thing that will surprise you.**

If you see this:

```
203.0.113.10 has never been connected to from this machine.
Run `ssh 203.0.113.10` once, check the fingerprint it shows, and accept it
— that records the key in /Users/you/.ssh/known_hosts, which mssh reads too
```

it means that machine has never been connected to from this computer.

mssh verifies every server's host key against `~/.ssh/known_hosts` — the same
file the `ssh` command uses. If a machine is not in that file, there is nothing
to check the key against, and the connection is refused rather than trusted
blindly.

### The fix

Connect once with the `ssh` command, so you can see and accept the fingerprint:

```bash
ssh -p 22 youruser@203.0.113.10
```

You will be asked:

```
The authenticity of host '203.0.113.10' can't be established.
ED25519 key fingerprint is SHA256:AbCd...
Are you sure you want to continue connecting (yes/no/[fingerprint])?
```

Type `yes`, log in, then `exit`. The key is now recorded, and mssh will connect.

For **SSH over SSM**, the host name is the instance id, so add this to
`~/.ssh/config` first and then connect once the same way:

```
Host i-* mi-*
    ProxyCommand aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p
    User ec2-user
```

```bash
ssh ec2-user@i-0abc123456789
```

### 🚨 If the key has *changed*

A different message means something else entirely:

```
the host key for 203.0.113.10 has CHANGED since it was recorded
```

Do **not** clear the entry to make this go away. It means one of two things:

1. The server was rebuilt or reinstalled — normal, but confirm it with whoever
   runs the machine before continuing.
2. Something is intercepting your connection. Everything you type, including
   passwords, would go to them first.

Only once you are certain it is case 1:

```bash
ssh-keygen -R 203.0.113.10
```

then connect again and accept the new fingerprint.

---

## Security

- **Passwords are never written to the database.** They go into the macOS
  Keychain (Credential Manager on Windows, Secret Service on Linux), and the
  database only records that a connection uses password authentication. You can
  verify this yourself:

  ```bash
  strings ~/Library/Application\ Support/mssh/mssh.db | grep '<your password>'
  ```

  It finds nothing.

- **Deleting a connection deletes its password**, and so does deleting the
  workspace that contains it.
- **Host keys are always verified.** There is no "connect anyway" option, by
  design.
- **AWS credentials are never handled by mssh.** SSO refresh, MFA and
  assume-role are all done by the AWS CLI in a separate process.
- **Passphrase-protected SSH keys are not read directly.** Load those into
  `ssh-agent` with `ssh-add` and choose the agent authentication method.

---

## Development

```bash
wails dev
```

This starts the app with hot reload for the frontend. It also serves the app at
<http://localhost:34115>, where you can open browser devtools and call the Go
methods directly as `window.go.main.App.*`.

Note that <http://localhost:5173> is the raw Vite server — the Go bridge is not
injected there, so `window.go` will be undefined.

### Tests and checks

```bash
go test ./...            # store, secrets and transport
go vet ./...
gofmt -l .
```

```bash
cd frontend && npm run build   # runs tsc, then vite build
```

### Regenerating the frontend bindings

After changing any method on `App`:

```bash
wails generate module
```

---

## Project layout

```
main.go                  wiring: open the database, start Wails
app.go                   the bridge to the frontend; thin by design

internal/store/          SQLite: workspaces, connections, settings
    migrations/          numbered .sql files, applied in order
internal/secrets/        passwords, in the OS credential store
internal/transport/      one Dialer per connection kind
internal/session/        live sessions, kept alive independently of the UI

frontend/src/
    lib/api.ts                 the only module that imports the Wails bindings
    lib/terminal-session.ts    terminals, deliberately outside React
    components/                dialogs, sidebar, terminal view
    routes/                    pages
```

---

## Troubleshooting

| Message | What to do |
|---|---|
| `... has never been connected to from this machine` | Connect once with `ssh` — see the section above |
| `the host key ... has CHANGED` | Stop and verify with the server's owner |
| `ssh-agent is not running` | Start it, or switch the connection to a key file or password |
| `key ... is protected by a passphrase` | `ssh-add <key>`, then use the SSH agent method |
| `the Session Manager plugin is not installed` | `brew install --cask session-manager-plugin` |
| `your AWS session has expired` | Run the `aws sso login` command in the message |
| `... is not reachable through Session Manager` | Check the instance is running, has the SSM Agent, and has an IAM role with `AmazonSSMManagedInstanceCore` |
| `this AWS profile is not allowed to run ssm:StartSession` | Add that permission to the role or user |
| `connection ... is already open` | The session is still running; disconnect it first |
