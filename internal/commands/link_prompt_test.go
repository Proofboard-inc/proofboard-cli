package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/proofboard/proofboard/internal/api"
	pbgit "github.com/proofboard/proofboard/internal/git"
	"github.com/proofboard/proofboard/internal/model"
)

func TestDetectProjectNamePrefersParsedRemoteRepoName(t *testing.T) {
	identity := model.RemoteIdentity{Repo: "proofboard-cli"}
	got := detectProjectName(identity, pbgit.Repo{Path: "/home/dev/some-other-dir-name"})
	if got != "proofboard-cli" {
		t.Errorf("detectProjectName() = %q, want detected remote repo name %q", got, "proofboard-cli")
	}
}

func TestDetectProjectNameFallsBackToDirectoryNameWithoutRemote(t *testing.T) {
	dir := filepath.Join("home", "dev", "my-local-project")
	got := detectProjectName(model.RemoteIdentity{}, pbgit.Repo{Path: dir})
	if got != "my-local-project" {
		t.Errorf("detectProjectName() = %q, want directory name %q", got, "my-local-project")
	}
}

func TestInferRoleTitle(t *testing.T) {
	cases := []struct {
		name  string
		stack *model.StackReport
		want  string
	}{
		{"nil stack", nil, ""},
		{"no recognizable signals", &model.StackReport{TechStack: []string{"Jest"}}, ""},
		{"frontend only", &model.StackReport{TechStack: []string{"React", "Next.js", "Tailwind CSS"}}, "Frontend Engineer"},
		{"backend only", &model.StackReport{TechStack: []string{"NestJS"}}, "Backend Engineer"},
		{"frontend and backend", &model.StackReport{TechStack: []string{"React", "NestJS"}}, "Full-Stack Engineer"},
		{"mobile language, no framework signal", &model.StackReport{Languages: map[string]int{"Swift": 10}}, "Mobile Engineer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferRoleTitle(c.stack); got != c.want {
				t.Errorf("inferRoleTitle() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPromptForOwnershipReturnsSelectedBranch(t *testing.T) {
	cases := []struct {
		input string
		want  repoOwnership
	}{
		{"1\n", ownershipPersonal},
		{"2\n", ownershipEmployer},
		{"3\n", ownershipPublic},
	}
	for _, c := range cases {
		got := promptForOwnership(strings.NewReader(c.input), &bytes.Buffer{})
		if got != c.want {
			t.Errorf("promptForOwnership(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestPromptForOwnershipReprompsOnInvalidChoice(t *testing.T) {
	in := strings.NewReader("bogus\n2\n")
	var out bytes.Buffer
	if got := promptForOwnership(in, &out); got != ownershipEmployer {
		t.Errorf("promptForOwnership() = %v, want ownershipEmployer after invalid input", got)
	}
}

func TestPromptCompanyNameWithDetectedDefaultAcceptsOrg(t *testing.T) {
	company := promptCompanyNameWithDetectedDefault(strings.NewReader("\n"), &bytes.Buffer{}, "Proboardly")
	if company != "Proboardly" {
		t.Errorf("company = %q, want detected org %q", company, "Proboardly")
	}
}

func TestPromptCompanyNameWithDetectedDefaultOverridesOrg(t *testing.T) {
	company := promptCompanyNameWithDetectedDefault(strings.NewReader("Acme Corp\n"), &bytes.Buffer{}, "Proboardly")
	if company != "Acme Corp" {
		t.Errorf("company = %q, want typed override %q", company, "Acme Corp")
	}
}

func TestPromptCompanyNameWithDetectedDefaultFallsBackToFreeTextWhenNoOrgDetected(t *testing.T) {
	var out bytes.Buffer
	company := promptCompanyNameWithDetectedDefault(strings.NewReader("\n"), &out, "")
	if company != "" {
		t.Errorf("company = %q, want empty", company)
	}
	if strings.Contains(out.String(), "Detected organisation") {
		t.Errorf("should not print a detected-organisation line when no org was detected: %q", out.String())
	}
}

// FIX: a raw bufio.Reader does no line editing, so pressing an arrow key
// while typing (e.g. reaching for shell-style history navigation) inserts
// the raw ANSI escape sequence for that key literally into the input
// buffer instead of moving a cursor. Reproduces a real corrupted value seen
// in production: roleTitle stored as "\x1b[ASweeftly" (ESC '[' 'A' = Up,
// immediately followed by the typed text).
func TestPromptRoleTitleStripsArrowKeyEscapeSequence(t *testing.T) {
	role := promptRoleTitle(strings.NewReader("\x1b[ASweeftly\n"), &bytes.Buffer{}, "")
	if role != "Sweeftly" {
		t.Errorf("role = %q, want escape sequence stripped to %q", role, "Sweeftly")
	}
}

func TestPromptRoleTitleFallsBackToSuggestion(t *testing.T) {
	role := promptRoleTitle(strings.NewReader("\n"), &bytes.Buffer{}, "Frontend Engineer")
	if role != "Frontend Engineer" {
		t.Errorf("role = %q, want inferred %q", role, "Frontend Engineer")
	}
}

func TestPromptProjectNameWithDetectedDefaultAcceptsDetected(t *testing.T) {
	name := promptProjectNameWithDetectedDefault(strings.NewReader("\n"), &bytes.Buffer{}, "proofboard-cli")
	if name != "proofboard-cli" {
		t.Errorf("name = %q, want detected default %q", name, "proofboard-cli")
	}
}

func TestPromptProjectNameWithDetectedDefaultOverridesDetected(t *testing.T) {
	name := promptProjectNameWithDetectedDefault(strings.NewReader("My Side Project\n"), &bytes.Buffer{}, "proofboard-cli")
	if name != "My Side Project" {
		t.Errorf("name = %q, want typed override %q", name, "My Side Project")
	}
}

func TestPromptProjectNameWithDetectedDefaultRepromptsUntilNonEmptyWhenNothingDetected(t *testing.T) {
	var out bytes.Buffer
	name := promptProjectNameWithDetectedDefault(strings.NewReader("\n\nMy Project\n"), &out, "")
	if name != "My Project" {
		t.Errorf("name = %q, want %q after reprompting past blank input", name, "My Project")
	}
	if strings.Contains(out.String(), "press enter to accept") {
		t.Errorf("should not offer an editable default when nothing was detected: %q", out.String())
	}
}

func TestPromptProjectNameWithDetectedDefaultReturnsEmptyOnReadError(t *testing.T) {
	name := promptProjectNameWithDetectedDefault(strings.NewReader(""), &bytes.Buffer{}, "")
	if name != "" {
		t.Errorf("name = %q, want empty on read error with nothing detected", name)
	}
}

func TestPromptPersonalProjectRoleDefaultsToOwnerWhenSoleAuthor(t *testing.T) {
	role := promptPersonalProjectRole(strings.NewReader("\n"), &bytes.Buffer{}, true, "Frontend Engineer")
	if role != "Owner" {
		t.Errorf("role = %q, want %q", role, "Owner")
	}
}

func TestPromptPersonalProjectRoleShowsStackSuggestionAsExampleWhenSoleAuthor(t *testing.T) {
	var out bytes.Buffer
	promptPersonalProjectRole(strings.NewReader("\n"), &out, true, "Frontend Engineer")
	if !strings.Contains(out.String(), "Frontend Engineer") {
		t.Errorf("expected stack suggestion to still be surfaced, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Owner") {
		t.Errorf("expected Owner to be offered as the primary default, got %q", out.String())
	}
}

func TestPromptPersonalProjectRoleAllowsCustomOverrideWhenSoleAuthor(t *testing.T) {
	role := promptPersonalProjectRole(strings.NewReader("Maintainer\n"), &bytes.Buffer{}, true, "Frontend Engineer")
	if role != "Maintainer" {
		t.Errorf("role = %q, want typed override %q", role, "Maintainer")
	}
}

func TestPromptPersonalProjectRoleFallsBackToStackSuggestionWhenNotSoleAuthor(t *testing.T) {
	role := promptPersonalProjectRole(strings.NewReader("\n"), &bytes.Buffer{}, false, "Backend Engineer")
	if role != "Backend Engineer" {
		t.Errorf("role = %q, want stack-inferred %q when not the sole author", role, "Backend Engineer")
	}
}

func TestPromptPersonalProjectRoleFallsBackToOptionalPromptWhenNotSoleAuthorNoStack(t *testing.T) {
	var out bytes.Buffer
	role := promptPersonalProjectRole(strings.NewReader("\n"), &out, false, "")
	if role != "" {
		t.Errorf("role = %q, want empty", role)
	}
	if strings.Contains(out.String(), "Owner") {
		t.Errorf("should not suggest Owner when not the sole author: %q", out.String())
	}
}

func TestPromptEmployerAuthorizationDefaultsToNoOnReadError(t *testing.T) {
	if promptEmployerAuthorization(strings.NewReader(""), &bytes.Buffer{}) {
		t.Error("promptEmployerAuthorization() = true on read error, want false (safe/anonymizing default)")
	}
}

func TestPromptPersonalProjectConfirmDefaultsToYesOnReadError(t *testing.T) {
	if !promptPersonalProjectConfirm(strings.NewReader(""), &bytes.Buffer{}) {
		t.Error("promptPersonalProjectConfirm() = false on read error, want true")
	}
}

// FIX: when none of the candidate projects carry a repo identity, the picker
// must keep printing the original 2-column format (name + role only).
func TestPromptForProjectPrintsTwoColumnsWithoutRepoFullName(t *testing.T) {
	in := strings.NewReader("1\n")
	var out bytes.Buffer
	options := []api.ExistingProjectOption{
		{ID: "proj-1", Name: "Acme Corp", Role: "Backend Engineer"},
	}

	id, createNew := promptForProject(in, &out, options)
	if id != "proj-1" || createNew {
		t.Fatalf("promptForProject() = (%q, %v), want (%q, false)", id, createNew, "proj-1")
	}
	if got := out.String(); !strings.Contains(got, "  1  Acme Corp       Backend Engineer\n") {
		t.Errorf("expected 2-column line, got %q", got)
	}
}

// FIX: two Volume-Proof projects can share the same name/role, the picker
// must show the repo identity as a third column so the user can tell them
// apart before attaching a CLI proof to the wrong project.
func TestPromptForProjectPrintsThreeColumnsWithRepoFullName(t *testing.T) {
	in := strings.NewReader("2\n")
	var out bytes.Buffer
	options := []api.ExistingProjectOption{
		{ID: "proj-1", Name: "Acme Corp", Role: "Backend Engineer", RepoFullName: "acme/api"},
		{ID: "proj-2", Name: "Acme Corp", Role: "Backend Engineer", RepoFullName: "acme/worker"},
	}

	id, createNew := promptForProject(in, &out, options)
	if id != "proj-2" || createNew {
		t.Fatalf("promptForProject() = (%q, %v), want (%q, false)", id, createNew, "proj-2")
	}
	got := out.String()
	if !strings.Contains(got, "  1  Acme Corp       Backend Engineer     acme/api\n") {
		t.Errorf("expected 3-column line for option 1, got %q", got)
	}
	if !strings.Contains(got, "  2  Acme Corp       Backend Engineer     acme/worker\n") {
		t.Errorf("expected 3-column line for option 2, got %q", got)
	}
}

// Answers piped in one go (a script, or a paste of several lines) must reach
// the prompt each was meant for. Each prompt used to wrap stdin in its own
// bufio.Reader, so the first prompt buffered every line and the later ones
// saw end of input and fell back to their defaults: "1\nn\n" linked a
// personal project the developer had just declined.
func TestResolveOwnershipHonoursPipedAnswers(t *testing.T) {
	stack := &model.StackReport{TechStack: []string{"React", "Next.js"}}
	// identity.Repo gives promptProjectNameWithDetectedDefault a fixed,
	// non-empty detected default ("acme-repo") so an accepted personal
	// branch can be driven with a blank line instead of depending on the
	// test's tmp directory name. repo.Path is not a real git checkout, so
	// pbgit.IsSoleAuthor deterministically fails closed (not the sole
	// author) and the personal branch's role prompt falls back to the
	// plain stack-inferred suggestion, same as the employer branch's.
	identity := model.RemoteIdentity{Org: "Proboardly", Repo: "acme-repo"}
	repo := pbgit.Repo{Path: t.TempDir()}
	cases := []struct {
		name            string
		input           string
		wantErr         error
		wantCompany     string
		wantRole        string
		wantProjectName string
	}{
		{name: "personal declined", input: "1\nn\n", wantErr: errPersonalProjectDeclined},
		{name: "personal accepted, defaults accepted", input: "1\ny\n\n\n", wantRole: "Frontend Engineer", wantProjectName: "acme-repo"},
		{name: "personal accepted, custom project name and role", input: "1\ny\nMy Cool App\nMaintainer\n", wantRole: "Maintainer", wantProjectName: "My Cool App"},
		{name: "employer authorized", input: "2\ny\nAcme Corp\nStaff Engineer\n", wantCompany: "Acme Corp", wantRole: "Staff Engineer"},
		{name: "employer authorized, defaults accepted", input: "2\ny\n\n\n", wantCompany: "Proboardly", wantRole: "Frontend Engineer"},
		{name: "employer not authorized", input: "2\nn\nStaff Engineer\n", wantCompany: privateCompanyPlaceholder, wantRole: "Staff Engineer"},
		{name: "public", input: "3\n", wantErr: errPublicProjectNotLinked},
		{name: "invalid then personal declined", input: "9\n1\nno\n", wantErr: errPersonalProjectDeclined},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A plain io.Reader, not a bufio.Reader: the same shape os.Stdin has.
			in := strings.NewReader(tc.input)
			company, role, projectName, err := resolveOwnership(context.Background(), in, &bytes.Buffer{}, identity, repo, stack)
			if err != tc.wantErr {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if company != tc.wantCompany || role != tc.wantRole || projectName != tc.wantProjectName {
				t.Fatalf("company, role, projectName = %q, %q, %q; want %q, %q, %q",
					company, role, projectName, tc.wantCompany, tc.wantRole, tc.wantProjectName)
			}
		})
	}
}
