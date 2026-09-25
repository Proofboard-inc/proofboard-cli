package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/proofboard/proofboard/internal/api"
	"github.com/proofboard/proofboard/internal/model"
)

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
	cases := []struct {
		name        string
		input       string
		wantErr     error
		wantCompany string
		wantRole    string
	}{
		{name: "personal declined", input: "1\nn\n", wantErr: errPersonalProjectDeclined},
		{name: "personal accepted", input: "1\ny\n"},
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
			company, role, err := resolveOwnership(in, &bytes.Buffer{}, "Proboardly", stack)
			if err != tc.wantErr {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if company != tc.wantCompany || role != tc.wantRole {
				t.Fatalf("company, role = %q, %q; want %q, %q", company, role, tc.wantCompany, tc.wantRole)
			}
		})
	}
}
