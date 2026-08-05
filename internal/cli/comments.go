package cli

import (
	"fmt"

	"github.com/janpuc/koment/internal/application"
	"github.com/janpuc/koment/internal/store"
)

func runComments(args []string, env Environment) int {
	if len(args) == 0 {
		return misuse(env, "comments needs check, convert, or acknowledge")
	}
	switch args[0] {
	case "check":
		return runCommentsCheck(args[1:], env)
	case "convert":
		return runCommentsConvert(args[1:], env)
	case "acknowledge":
		return runCommentsAcknowledge(args[1:], env)
	default:
		return misuse(env, "unknown comments command %q; want check, convert, or acknowledge", args[0])
	}
}

func runCommentsCheck(args []string, env Environment) int {
	flags := flagSet("comments check", env)
	if err := flags.Parse(args); err != nil {
		return ExitUsage
	}
	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	requested, err := relativePrefixes(annotations, flags.Args())
	if err != nil {
		return fail(env, err)
	}
	violations, err := service.CheckComments(requested)
	if err != nil {
		return fail(env, err)
	}
	for _, violation := range violations {
		fmt.Fprintf(env.Stdout, "%s:%d\n  %s\n  %s\n",
			violation.Comment.File, violation.Comment.Line, violation.Comment.Raw, violation.Reason)
	}
	if len(violations) > 0 {
		fmt.Fprintf(env.Stderr, "koment: %d prohibited inline comments must be converted or explicitly acknowledged\n", len(violations))
		return ExitFailure
	}
	fmt.Fprintln(env.Stdout, "comment policy: ok")
	return ExitOK
}

func runCommentsConvert(args []string, env Environment) int {
	flags := flagSet("comments convert", env)
	excerpt := flags.String("excerpt", "", "complete verbatim comment group to convert")
	kind := flags.String("kind", string(store.TypeWhy), "one of why, gotcha, invariant, anti-pattern")
	author := flags.String("author", "", `override the git identity; "Name" or "Name <email>"`)
	byAgent := flags.Bool("agent", false, "record this as written by an agent, not a person")
	target, ok := onePositional("comments convert", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}
	if *excerpt == "" {
		return misuse(env, "comments convert needs --excerpt")
	}
	parsedKind, err := store.ParseType(*kind)
	if err != nil {
		return misuse(env, "%v", err)
	}
	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	createdBy, err := identity(annotations.Root(), *author, *byAgent)
	if err != nil {
		return misuse(env, "%v", err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}
	mutation, err := service.ConvertComment(application.ConvertCommentInput{
		File: file, Comment: *excerpt, Kind: parsedKind, Author: createdBy,
	})
	if err != nil {
		return fail(env, err)
	}
	writeMutation(env, mutation)
	return ExitOK
}

func runCommentsAcknowledge(args []string, env Environment) int {
	flags := flagSet("comments acknowledge", env)
	excerpt := flags.String("excerpt", "", "complete verbatim comment group to retain")
	body := flags.String("body", "", "why the inline comment is necessary; - reads it from stdin")
	acknowledged := flags.Bool("acknowledge-inline-comment", false, "explicitly waive the normal koment procedure")
	author := flags.String("author", "", `override the git identity; "Name" or "Name <email>"`)
	byAgent := flags.Bool("agent", false, "record this as written by an agent, not a person")
	target, ok := onePositional("comments acknowledge", "a file", flags, args, env)
	if !ok {
		return ExitUsage
	}
	if *excerpt == "" {
		return misuse(env, "comments acknowledge needs --excerpt")
	}
	if !*acknowledged {
		return misuse(env, "comments acknowledge needs --acknowledge-inline-comment")
	}
	text, err := bodyText(*body, env.Stdin)
	if err != nil {
		return misuse(env, "%v", err)
	}
	service, annotations, err := openApplication()
	if err != nil {
		return fail(env, err)
	}
	createdBy, err := identity(annotations.Root(), *author, *byAgent)
	if err != nil {
		return misuse(env, "%v", err)
	}
	file, err := annotations.FromWorkingDirectory(target)
	if err != nil {
		return fail(env, err)
	}
	mutation, err := service.AcknowledgeComment(application.AcknowledgeCommentInput{
		File: file, Comment: *excerpt, Body: text, Author: createdBy, Acknowledged: true,
	})
	if err != nil {
		return fail(env, err)
	}
	writeMutation(env, mutation)
	return ExitOK
}

func writeMutation(env Environment, mutation application.Mutation) {
	writeMutationWarnings(env, mutation)
	fmt.Fprintf(env.Stdout, "%s  %s %s  %s\n", mutation.Record.Metadata.ID, mutation.Record.Spec.Type,
		location(mutation.Record.Spec.Target.File, mutation.Record.Status.LastSeenLine), mutation.Path)
}
