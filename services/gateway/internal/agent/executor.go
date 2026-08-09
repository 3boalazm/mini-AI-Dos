package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	// commandTimeout bounds a single run_command invocation.
	commandTimeout = 60 * time.Second
	// maxCommandOutput caps captured output fed back to the model.
	maxCommandOutput = 8 * 1024
)

// blockedCommands maps a first-token (or package-manager subcommand)
// to why it is withheld. These are exactly the "sensitive" categories
// AGENT_ROADMAP.md defers to the approval system (A8): until that
// exists, the agent's terminal runs read/build/test-style commands but
// not install, delete, git, network fetch, or privilege escalation.
var blockedCommands = map[string]string{
	"rm": "delete", "rmdir": "delete", "del": "delete", "rd": "delete",
	"git":  "git (roadmap A10)",
	"sudo": "privilege escalation",
	"apt":  "package install", "apt-get": "package install", "pip": "package install",
	"pip3": "package install", "gem": "package install", "brew": "package install",
	"choco": "package install", "winget": "package install",
	"curl": "network fetch", "wget": "network fetch", "nc": "network",
	"ssh": "remote access", "scp": "remote access",
	"sudo-": "privilege escalation",
	"kill":  "process control", "pkill": "process control", "killall": "process control",
	"shutdown": "power", "reboot": "power", "halt": "power",
	"mkfs": "disk", "dd": "disk", "fdisk": "disk", "format": "disk",
	"chmod": "permissions", "chown": "permissions",
}

// commandIsBlocked reports whether a command touches a deferred
// sensitive category, and why. It tokenizes on shell separators so a
// blocked verb anywhere in a pipeline is caught, and specifically
// blocks "<pkg-manager> install|add".
func commandIsBlocked(command string) (string, bool) {
	lower := strings.ToLower(command)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ';' || r == '|' || r == '&' || r == '(' || r == ')' || r == '`'
	})
	for _, f := range fields {
		if reason, ok := blockedCommands[f]; ok {
			return reason, true
		}
	}
	// "<pm> install/add/i" — the package managers we otherwise allow to
	// run scripts (npm run build is fine; npm install is not).
	for i := 0; i+1 < len(fields); i++ {
		switch fields[i] {
		case "npm", "yarn", "pnpm", "cargo", "bundle", "poetry":
			switch fields[i+1] {
			case "install", "add", "i", "ci":
				return "package install", true
			}
		}
	}
	return "", false
}

// RunCommand executes a shell command in the workspace root with a
// timeout and bounded, combined output. Blocked commands and failures
// come back as text results (never Go errors) so the agent loop reads
// them and adapts, exactly like the file tools.
func (w *Workspace) RunCommand(ctx context.Context, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "ERROR: command is required"
	}
	if reason, blocked := commandIsBlocked(command); blocked {
		return "ERROR: '" + reason + "' commands require approval, which isn't available yet (roadmap A8). Not run: " + command
	}

	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(cctx, "sh", "-c", command)
	}
	cmd.Dir = w.root

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()

	out := buf.String()
	if len(out) > maxCommandOutput {
		out = out[:maxCommandOutput] + "\n...[truncated]"
	}
	if cctx.Err() == context.DeadlineExceeded {
		return fmt.Sprintf("ERROR: command timed out after %s\n%s", commandTimeout, out)
	}
	if runErr != nil {
		if out == "" {
			out = "(no output)"
		}
		return fmt.Sprintf("EXIT non-zero (%v)\n%s", runErr, out)
	}
	if out == "" {
		return "OK (exit 0, no output)"
	}
	return "OK (exit 0)\n" + out
}
