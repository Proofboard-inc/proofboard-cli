package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
)

// errPublicProjectNotLinked signals that the ownership prompt identified this
// repository as a public project. Public projects are connected through the
// web dashboard rather than the CLI, so link intentionally stops without
// registering anything. Callers that drive link programmatically (see
// ensureRepoLinked) treat this the same as "nothing to sync", not a failure.
var errPublicProjectNotLinked = errors.New("public project: connect through the Proofboard dashboard")

// errPersonalProjectDeclined signals that the developer answered no to
// Branch 1's "add this project to your career record?" question. Like
// errPublicProjectNotLinked, this is an intentional non-link, not a failure.
var errPersonalProjectDeclined = errors.New("personal project: not added to career record")

const proofboardDashboardURL = "https://proofboard.io/dashboard"

const privateCompanyPlaceholder = "Private Company"

type repoOwnership int

const (
	ownershipPersonal repoOwnership = iota
	ownershipEmployer
	ownershipPublic
)

// resolveOwnership runs the ownership-branch prompts for a not-yet-linked
// repository and returns the company name, role title, and project name to
// send with the link request (project name is only ever set by the
// personal-project branch — employer/public projects have no use for it).
// It returns errPublicProjectNotLinked or errPersonalProjectDeclined when the
// answers mean the repository must not be linked. Every prompt reads through
// the same buffered reader: a second bufio.Reader over the same input would
// miss whatever the first one had already buffered.
func resolveOwnership(ctx context.Context, in io.Reader, out io.Writer, identity model.RemoteIdentity, repo pbgit.Repo, stack *model.StackReport) (companyName string, roleTitle string, projectName string, err error) {
	reader := bufio.NewReader(in)
	switch promptForOwnership(reader, out) {
	case ownershipPublic:
		printPublicProjectNotice(out)
		return "", "", "", errPublicProjectNotLinked
	case ownershipPersonal:
		if !promptPersonalProjectConfirm(reader, out) {
			fmt.Fprintln(out, "Okay, not connecting this project for now. Run `proofboard link` any time to reconsider.")
			return "", "", "", errPersonalProjectDeclined
		}
		projectName = promptProjectNameWithDetectedDefault(reader, out, detectProjectName(identity, repo))
		isSoleAuthor, _ := pbgit.IsSoleAuthor(ctx, repo)
		roleTitle = promptPersonalProjectRole(reader, out, isSoleAuthor, inferRoleTitle(stack))
		return "", roleTitle, projectName, nil
	default: // ownershipEmployer
		if promptEmployerAuthorization(reader, out) {
			companyName = promptCompanyNameWithDetectedDefault(reader, out, identity.Org)
		} else {
			companyName = privateCompanyPlaceholder
		}
		roleTitle = promptRoleTitle(reader, out, inferRoleTitle(stack))
		return companyName, roleTitle, "", nil
	}
}

// promptForOwnership asks who owns the repository being linked. This only
// runs once per repository, on the not-yet-linked path: callers never
// re-prompt for a repo that has already been through this decision.
func promptForOwnership(in io.Reader, out io.Writer) repoOwnership {
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "Proofboard · Project detected")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Who owns this repository?")
	fmt.Fprintln(out, "  1. Personal / Private project")
	fmt.Fprintln(out, "  2. My current or former employer / client")
	fmt.Fprintln(out, "  3. Public project")
	fmt.Fprintln(out)
	for {
		fmt.Fprint(out, "Enter choice [1/2/3]: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return ownershipPersonal
		}
		switch sanitizeTypedInput(line) {
		case "1":
			return ownershipPersonal
		case "2":
			return ownershipEmployer
		case "3":
			return ownershipPublic
		}
	}
}

// promptPersonalProjectConfirm is Branch 1's opening question. A "no" answer
// stops the flow before the project-name/role prompts below are ever shown.
func promptPersonalProjectConfirm(in io.Reader, out io.Writer) bool {
	reader := bufio.NewReader(in)
	fmt.Fprint(out, "Would you like to add this project to your career record? [Y/n]: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return true
	}
	answer := strings.ToLower(sanitizeTypedInput(line))
	return answer != "n" && answer != "no"
}

// promptProjectNameWithDetectedDefault offers a locally-detected project
// name (the repository's origin remote name, or its directory name when no
// remote name is available) as an editable default, same "detect, then let
// the human confirm or override" pattern as
// promptCompanyNameWithDetectedDefault. Personal projects are never asked
// for a company name — this is the one identifying detail they're asked
// for instead.
func promptProjectNameWithDetectedDefault(in io.Reader, out io.Writer, detected string) string {
	reader := bufio.NewReader(in)
	detected = strings.TrimSpace(detected)
	if detected == "" {
		fmt.Fprint(out, "Project name: ")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return ""
			}
			if name := sanitizeTypedInput(line); name != "" {
				return name
			}
			fmt.Fprint(out, "Project name (required): ")
		}
	}
	fmt.Fprintf(out, "Project name [%s] (press enter to accept, or type your own): ", detected)
	line, _ := reader.ReadString('\n')
	typed := sanitizeTypedInput(line)
	if typed == "" {
		return detected
	}
	return typed
}

// promptPersonalProjectRole asks the role/contributor question for a
// personal project. When the local git history shows this user as the
// repository's sole commit author (isSoleAuthor), "Owner" is offered as the
// primary default; a stack-based suggestion from inferRoleTitle, if any, is
// still surfaced rather than silently dropped by folding it into the prompt
// text as an example. Falls back to the plain promptRoleTitle behaviour
// (stack suggestion as the default, or a bare optional prompt) when
// isSoleAuthor is false.
func promptPersonalProjectRole(in io.Reader, out io.Writer, isSoleAuthor bool, stackSuggestion string) string {
	if !isSoleAuthor {
		return promptRoleTitle(in, out, stackSuggestion)
	}
	reader := bufio.NewReader(in)
	if stackSuggestion != "" {
		fmt.Fprintf(out, "Role [Owner] (press enter to accept, or type your own, e.g. %s): ", stackSuggestion)
	} else {
		fmt.Fprint(out, "Role [Owner] (press enter to accept, or type your own): ")
	}
	line, _ := reader.ReadString('\n')
	typed := sanitizeTypedInput(line)
	if typed == "" {
		return "Owner"
	}
	return typed
}

// printPublicProjectNotice is Branch 2: public projects are never processed
// through the CLI.
func printPublicProjectNotice(out io.Writer) {
	fmt.Fprintln(out, "Public projects are connected through the Proofboard web dashboard, not the CLI.")
	fmt.Fprintf(out, "Visit %s to connect this project.\n", proofboardDashboardURL)
}

// promptEmployerAuthorization is Branch 3's authorization question. A "no"
// answer never blocks syncing, it only means the employer/client identity is
// kept private ("Private Company") in the career record instead of disclosed.
func promptEmployerAuthorization(in io.Reader, out io.Writer) bool {
	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "PRIVATE WORK")
	fmt.Fprintln(out, "This repository appears to belong to an employer or client.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Before continuing, confirm that you are authorized to use information")
	fmt.Fprintln(out, "from this repository to create a personal career record.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Proofboard processes the repository locally. Source code, file")
	fmt.Fprintln(out, "contents, and commit messages are not uploaded to Proofboard.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Local processing does not override your employment agreement,")
	fmt.Fprintln(out, "confidentiality obligations, NDA, or company policies.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "If you continue without authorization, Proofboard will keep the")
	fmt.Fprintln(out, "employer or client identity private and use \"Private Company\" in")
	fmt.Fprintln(out, "your career record.")
	fmt.Fprintln(out)
	fmt.Fprint(out, "Are you authorized to use this work for personal career\ndocumentation? [Y/n]: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(sanitizeTypedInput(line))
	return answer != "n" && answer != "no"
}

// promptCompanyNameWithDetectedDefault offers the locally-detected
// organisation name (from pbgit.ParseRemote, via identity.Org) as an
// editable default. Only reached after Branch 3's authorization question was
// answered yes, so there is no further consent step here, just what name to
// store. Falls back to a plain free-text prompt when no org was detected,
// same as promptForCompanyAndRole did for that case.
func promptCompanyNameWithDetectedDefault(in io.Reader, out io.Writer, org string) string {
	reader := bufio.NewReader(in)
	if strings.TrimSpace(org) == "" {
		fmt.Fprint(out, "Company name (optional, press enter to skip): ")
		line, _ := reader.ReadString('\n')
		return sanitizeTypedInput(line)
	}
	fmt.Fprintf(out, "Detected organisation: %s\n", org)
	fmt.Fprintf(out, "Company name [%s] (press enter to accept): ", org)
	line, _ := reader.ReadString('\n')
	typed := sanitizeTypedInput(line)
	if typed == "" {
		return org
	}
	return typed
}

// promptRoleTitle asks for a role title, offering the stack-inferred
// suggestion as an editable default. Shared by every branch that reaches a
// role question.
func promptRoleTitle(in io.Reader, out io.Writer, suggested string) string {
	reader := bufio.NewReader(in)
	if suggested != "" {
		fmt.Fprintf(out, "Role title [%s] (press enter to accept, or type your own): ", suggested)
	} else {
		fmt.Fprint(out, "Role title (optional, press enter to skip): ")
	}
	line, _ := reader.ReadString('\n')
	roleTitle := sanitizeTypedInput(line)
	if roleTitle == "" {
		roleTitle = suggested
	}
	return roleTitle
}
