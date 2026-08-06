package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/koment-dev/koment/internal/agentpolicy"
	"github.com/koment-dev/koment/internal/policy"
)

func runBootstrap(args []string, env Environment) int {
	flags := flagSet("bootstrap", env)
	agentsList := flags.String("agents", "", "comma-separated adapter names to install (claude, copilot, cursor, codex, opencode, agents)")
	all := flags.Bool("all", false, "install every supported adapter")
	policyOnly := flags.Bool("policy-only", false, "install the policy only; do not refresh any adapter")
	nonInteractive := flags.Bool("non-interactive", false, "do not prompt; require explicit --agents, --all or --policy-only")
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		return misuse(env, "bootstrap takes no arguments")
	}

	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	existing, err := policy.Load(annotations.Root())
	if err != nil && !isMissingPolicy(err) {
		return fail(env, err)
	}
	havePolicy := err == nil

	selected, err := resolveBootstrapAdapters(*agentsList, *all, *policyOnly, *nonInteractive, existing, env)
	if err != nil {
		return misuse(env, "%v", err)
	}

	configured := existing
	if !havePolicy {
		configured = policy.Default()
	}
	if len(selected) > 0 {
		configured.Spec.Agents.Adapters = selected
	}

	if !havePolicy {
		if err := policy.Save(annotations.Root(), configured); err != nil {
			return fail(env, err)
		}
		fmt.Fprintf(env.Stdout, "installed %s\n", policy.FileName)
	}

	var changes []agentpolicy.Change
	if *policyOnly {
		changes, err = agentpolicy.InstallContributing(annotations.Root(), configured)
	} else {
		changes, err = agentpolicy.Install(annotations.Root(), configured)
	}
	if err != nil {
		return fail(env, err)
	}
	for _, change := range changes {
		fmt.Fprintf(env.Stdout, "%s %s\n", change.Action, change.Path)
	}
	if len(changes) == 0 {
		fmt.Fprintln(env.Stdout, "agent policy: already current")
	}
	return ExitOK
}

func resolveBootstrapAdapters(agentsList string, all, policyOnly, nonInteractive bool, existing policy.Policy, env Environment) ([]policy.Adapter, error) {
	switch {
	case agentsList != "":
		return parseAdapterList(agentsList)
	case all:
		return append([]policy.Adapter(nil), policy.Default().Spec.Agents.Adapters...), nil
	case policyOnly:
		return nil, nil
	case existing.Spec.Agents.Adapters != nil:
		return append([]policy.Adapter(nil), existing.Spec.Agents.Adapters...), nil
	}
	if nonInteractive || !stdinIsTerminal() {
		return nil, fmt.Errorf("no adapter selection resolved; pass --agents <list>, --all, or --policy-only (or run interactively in a terminal)")
	}
	return promptAdapters(existing.Spec.Agents.Adapters, env)
}

func parseAdapterList(raw string) ([]policy.Adapter, error) {
	parts := strings.Split(raw, ",")
	selected := make([]policy.Adapter, 0, len(parts))
	seen := map[policy.Adapter]bool{}
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		adapter := policy.Adapter(name)
		if !validAdapter(adapter) {
			return nil, fmt.Errorf("unknown adapter %q", name)
		}
		if seen[adapter] {
			continue
		}
		seen[adapter] = true
		selected = append(selected, adapter)
	}
	return selected, nil
}

func validAdapter(adapter policy.Adapter) bool {
	for _, candidate := range policy.Default().Spec.Agents.Adapters {
		if candidate == adapter {
			return true
		}
	}
	return false
}

func promptAdapters(current []policy.Adapter, env Environment) ([]policy.Adapter, error) {
	all := policy.Default().Spec.Agents.Adapters
	selected := make([]policy.Adapter, 0, len(all))
	for _, adapter := range all {
		defaultYes := containsAdapter(current, adapter)
		answer, err := askYesNo(adapterName(adapter), defaultYes, env)
		if err != nil {
			return nil, err
		}
		if answer {
			selected = append(selected, adapter)
		}
	}
	return selected, nil
}

func askYesNo(prompt string, defaultYes bool, env Environment) (bool, error) {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Fprintf(env.Stdout, "%s %s ", prompt, hint)
	reader := bufio.NewReader(env.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("reading answer: %w", err)
	}
	answer := strings.TrimSpace(line)
	switch strings.ToLower(answer) {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	}
	return defaultYes, nil
}

func containsAdapter(list []policy.Adapter, adapter policy.Adapter) bool {
	for _, candidate := range list {
		if candidate == adapter {
			return true
		}
	}
	return false
}

func adapterName(adapter policy.Adapter) string {
	switch adapter {
	case policy.AdapterAgents:
		return "AGENTS.md (managed contract for every client)"
	case policy.AdapterClaude:
		return "Claude Code (.mcp.json + CLAUDE.md import)"
	case policy.AdapterCopilot:
		return "GitHub Copilot (.github/copilot-instructions.md + .vscode/mcp.json)"
	case policy.AdapterCursor:
		return "Cursor (.cursor/rules/koment.mdc + .cursor/mcp.json)"
	case policy.AdapterCodex:
		return "Codex CLI (.codex/hooks.json + .codex/config.toml)"
	case policy.AdapterOpencode:
		return "opencode (.opencode/plugins/koment.js + opencode.json)"
	}
	return string(adapter)
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func isMissingPolicy(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist)
}
