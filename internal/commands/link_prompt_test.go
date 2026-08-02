package commands

import (
	"bytes"
	"strings"
	"testing"

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
