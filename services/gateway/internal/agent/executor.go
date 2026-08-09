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

// sensitiveCommands maps a first-token (or package-manager subcommand)
// to its category. These are the actions AGENT_ROADMAP.md routes
// through the approval system (A8): the agent runs read/build/test
// commands freely, but install, delete, git, network fetch, and
// privilege escalation each pause for the user's Allow/Deny decision.
var sensitiveCommands = map[string]string{
	"rm": "delete", "rmdir": "delete", "del": "delete", "rd": "delete",
	"git":  "git",
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

// commandCategory reports whether a command falls in a sensitive
// category (and which). It tokenizes on shell separators so a sensitive
// verb anywhere in a pipeline is caught, and specifically flags
// "<pkg-manager> install|add". The engine routes sensitive commands
// through the approval gate (A8); everything else runs freely.
func commandCategory(command string) (string, bool) {
	lower := strings.ToLower(command)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ';' || r == '|' || r == '&' || r == '(' || r == ')' || r == '`'
	})
	for _, f := range fields {
		if reason, ok := sensitiveCommands[f]; ok {
			return reason, true
		}
	}
	// "<pm> install/add/i" — the package managers we otherwise allow to
	// run scripts (npm run build is fine; npm install needs approval).
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
// timeout and bounded, combined output. It does NOT gate sensitive
// commands — that decision belongs to the engine, which alone can pause
// the run for the user's approval (A8). Failures come back as text
// results (never Go errors) so the agent loop reads them and adapts.
func (w *Workspace) RunCommand(ctx context.Context, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "ERROR: command is required"
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
