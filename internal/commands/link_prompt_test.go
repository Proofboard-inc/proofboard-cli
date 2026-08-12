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

// FIX: declining the detected organisation must not abandon the autofill
// flow — the user still gets a chance to type their own company name, and
// role title still defaults to the stack-inferred suggestion.
func TestPromptForCompanyAndRoleStillAsksAfterDecliningOrg(t *testing.T) {
	in := strings.NewReader("n\nAcme Corp\n\n")
	var out bytes.Buffer
	stack := &model.StackReport{TechStack: []string{"React", "Next.js"}}

	company, role := promptForCompanyAndRole(in, &out, "Proboardly", stack)
	if company != "Acme Corp" {
		t.Errorf("company = %q, want %q", company, "Acme Corp")
	}
	if role != "Frontend Engineer" {
		t.Errorf("role = %q, want inferred %q", role, "Frontend Engineer")
	}
	if !strings.Contains(out.String(), "Detected organisation: Proboardly") {
		t.Errorf("expected detected-organisation line, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Frontend Engineer") {
		t.Errorf("expected inferred role suggested in prompt text, got %q", out.String())
	}
}

func TestPromptForCompanyAndRoleAcceptsOrgAndOverridesInferredRole(t *testing.T) {
	in := strings.NewReader("y\nStaff Engineer\n")
	var out bytes.Buffer
	stack := &model.StackReport{TechStack: []string{"NestJS"}}

	company, role := promptForCompanyAndRole(in, &out, "Proboardly", stack)
	if company != "Proboardly" {
		t.Errorf("company = %q, want detected org %q", company, "Proboardly")
	}
	if role != "Staff Engineer" {
		t.Errorf("role = %q, want typed override %q", role, "Staff Engineer")
	}
}

// FIX: a raw bufio.Reader does no line editing, so pressing an arrow key
// while typing (e.g. reaching for shell-style history navigation) inserts
// the raw ANSI escape sequence for that key literally into the input
// buffer instead of moving a cursor. Reproduces a real corrupted value seen
// in production: roleTitle stored as "\x1b[ASweeftly" (ESC '[' 'A' = Up,
// immediately followed by the typed text).
func TestPromptForCompanyAndRoleStripsArrowKeyEscapeSequence(t *testing.T) {
	in := strings.NewReader("n\nAcme Corp\n\x1b[ASweeftly\n")
	var out bytes.Buffer

	company, role := promptForCompanyAndRole(in, &out, "Proboardly", nil)
	if company != "Acme Corp" {
		t.Errorf("company = %q, want %q", company, "Acme Corp")
	}
	if role != "Sweeftly" {
		t.Errorf("role = %q, want escape sequence stripped to %q", role, "Sweeftly")
	}
}

func TestPromptForCompanyAndRoleSkipsOrgConfirmWhenNoOrgDetected(t *testing.T) {
	in := strings.NewReader("\n\n")
	var out bytes.Buffer

	company, role := promptForCompanyAndRole(in, &out, "", nil)
	if company != "" {
		t.Errorf("company = %q, want empty", company)
	}
	if role != "" {
		t.Errorf("role = %q, want empty", role)
	}
	if strings.Contains(out.String(), "Is this your employer") {
		t.Errorf("should not ask org-confirmation question when no org was detected: %q", out.String())
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

// FIX: two Volume-Proof projects can share the same name/role — the picker
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
