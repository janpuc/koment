package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/janpuc/koment/internal/agentpolicy"
	"github.com/janpuc/koment/internal/anchor"
	"github.com/janpuc/koment/internal/policy"
)

func runAgents(args []string, env Environment) int {
	if len(args) == 0 {
		return misuse(env, "agents needs install, check, or hook")
	}
	switch args[0] {
	case "install":
		return runAgentsInstall(args[1:], env)
	case "check":
		return runAgentsCheck(args[1:], env)
	case "hook":
		return runAgentsHook(args[1:], env)
	default:
		return misuse(env, "unknown agents command %q; want install, check, or hook", args[0])
	}
}

func runAgentsInstall(args []string, env Environment) int {
	flags := flagSet("agents install", env)
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		return misuse(env, "agents install takes no arguments")
	}
	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	configured, policyCreated, err := policy.Install(annotations.Root())
	if err != nil {
		return fail(env, err)
	}
	if policyCreated {
		fmt.Fprintf(env.Stdout, "installed %s\n", policy.FileName)
	}
	changes, err := agentpolicy.Install(annotations.Root(), configured)
	if err != nil {
		return fail(env, err)
	}
	for _, change := range changes {
		fmt.Fprintf(env.Stdout, "%s %s\n", change.Action, change.Path)
	}
	if !policyCreated && len(changes) == 0 {
		fmt.Fprintln(env.Stdout, "agent policy: already current")
	}
	return ExitOK
}

func runAgentsCheck(args []string, env Environment) int {
	flags := flagSet("agents check", env)
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	if flags.NArg() != 0 {
		return misuse(env, "agents check takes no arguments")
	}
	annotations, err := openStore()
	if err != nil {
		return fail(env, err)
	}
	configured, err := policy.Load(annotations.Root())
	if err != nil {
		return fail(env, err)
	}
	drift, err := agentpolicy.Check(annotations.Root(), configured)
	if err != nil {
		return fail(env, err)
	}
	if len(drift) == 0 {
		fmt.Fprintln(env.Stdout, "agent policy: ok")
		return ExitOK
	}
	for _, entry := range drift {
		fmt.Fprintf(env.Stdout, "%s: %s\n", entry.Path, entry.Reason)
	}
	fmt.Fprintf(env.Stderr, "koment: %d agent policy adapters need `koment agents install`\n", len(drift))
	return ExitFailure
}

func runAgentsHook(args []string, env Environment) int {
	if len(args) != 1 {
		return misuse(env, "agents hook needs pre-tool or stop")
	}
	input, err := io.ReadAll(env.Stdin)
	if err != nil {
		return fail(env, fmt.Errorf("reading hook input: %w", err))
	}
	switch args[0] {
	case "pre-tool":
		output, err := agentpolicy.PreToolOutput(input)
		if err != nil {
			return fail(env, err)
		}
		if _, err := env.Stdout.Write(output); err != nil {
			return fail(env, fmt.Errorf("writing hook output: %w", err))
		}
		return ExitOK
	case "stop":
		return runAgentsStopHook(input, env)
	default:
		return misuse(env, "unknown agents hook %q; want pre-tool or stop", args[0])
	}
}

func runAgentsStopHook(input []byte, env Environment) int {
	continued, err := agentpolicy.StopWasContinued(input)
	if err != nil {
		return fail(env, err)
	}
	reason := completionFailure()
	response := map[string]any{}
	if reason != "" {
		if continued {
			response["continue"] = false
			response["stopReason"] = reason
			response["systemMessage"] = reason
		} else {
			response["decision"] = "block"
			response["reason"] = reason
		}
	}
	if err := json.NewEncoder(env.Stdout).Encode(response); err != nil {
		return fail(env, fmt.Errorf("writing Stop output: %w", err))
	}
	return ExitOK
}

func completionFailure() string {
	service, annotations, err := openApplication()
	if err != nil {
		return "koment could not open the repository: " + err.Error()
	}
	snapshot, err := service.Snapshot()
	if err != nil {
		return "koment could not resolve annotations: " + err.Error()
	}
	counts := snapshot.Counts()
	annotationFailures := counts[anchor.StatusAmbiguous] + counts[anchor.StatusDrifted] + counts[anchor.StatusOrphaned]
	violations, err := service.CheckComments(nil)
	if err != nil {
		return "koment could not check comments: " + err.Error()
	}
	configured, err := policy.Load(annotations.Root())
	if err != nil {
		return "koment could not load agent policy: " + err.Error()
	}
	drift, err := agentpolicy.Check(annotations.Root(), configured)
	if err != nil {
		return "koment could not check agent adapters: " + err.Error()
	}
	var failures []string
	if annotationFailures > 0 {
		failures = append(failures, fmt.Sprintf("%d annotations are unresolved; run `koment check`", annotationFailures))
	}
	if len(violations) > 0 {
		failures = append(failures, fmt.Sprintf("%d inline comments violate policy; run `koment comments check` and convert or acknowledge them", len(violations)))
	}
	if len(drift) > 0 {
		failures = append(failures, fmt.Sprintf("%d agent adapters drifted; run `koment agents install`", len(drift)))
	}
	return strings.Join(failures, "; ")
}
