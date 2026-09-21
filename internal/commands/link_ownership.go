package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
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

// promptPersonalProjectConfirm is Branch 1's only question. No company name
// applies to a personal project, so there is nothing else to ask here.
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
