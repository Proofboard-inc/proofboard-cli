package detection

import (
	"testing"

	"github.com/proofboard/proofboard/internal/model"
)

// Regression test for the "tech stack showing only Jest, NestJS" bug: a
// NestJS backend using mongoose/stripe/ioredis was silently dropping every
// dependency outside the old ~15-entry hardcoded map, with no fallback
// bucket. A dictionary-supplied StackSignals table should recognize all of
// them.
func TestDetectStackUsesDictionaryStackSignalsWhenPresent(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "package.json", `{
		"dependencies": {
			"@nestjs/core": "^10.0.0",
			"mongoose": "8.13.2",
			"stripe": "^18.5.0",
			"ioredis": "^5.6.1"
		},
		"devDependencies": { "jest": "^29.0.0" }
	}`)
	commitAll(t, dir)

	dict := model.Dictionary{
		Version: "1.5.0",
		Categories: map[string]model.Signals{
			"Feature Development": {Impact: "feature"},
		},
		StackSignals: map[string]string{
			"@nestjs/core": "NestJS",
			"mongoose":     "Mongoose",
			"stripe":       "Stripe",
			"ioredis":      "Redis",
			"jest":         "Jest",
		},
	}

	report, err := DetectStack(dir, dict)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}

	for _, want := range []string{"NestJS", "Mongoose", "Stripe", "Redis", "Jest"} {
		if !contains(report.TechStack, want) {
			t.Errorf("TechStack = %v, missing %q (previously dropped by the hardcoded fallback)", report.TechStack, want)
		}
	}
}

func TestDetectStackFallsBackToDefaultTableWhenDictionaryEmpty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "package.json", `{"dependencies": {"react": "^18.0.0"}}`)
	commitAll(t, dir)

	report, err := DetectStack(dir, model.Dictionary{})
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if !contains(report.TechStack, "React") {
		t.Errorf("TechStack = %v, want React via the built-in fallback table", report.TechStack)
	}
}

func TestDetectStackPicksTheMostFrequentIndustrySignal(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "package.json", `{
		"dependencies": { "stripe": "^18.5.0", "braintree": "^3.0.0", "twilio": "^5.8.0" }
	}`)
	commitAll(t, dir)

	dict := model.Dictionary{
		Version: "1.5.0",
		Categories: map[string]model.Signals{
			"Feature Development": {Impact: "feature"},
		},
		IndustrySignals: map[string]string{
			"stripe":    "Fintech",
			"braintree": "Fintech",
			"twilio":    "Telecom",
		},
	}

	report, err := DetectStack(dir, dict)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if len(report.IndustryHints) == 0 || report.IndustryHints[0] != "Fintech" {
		t.Errorf("IndustryHints = %v, want [Fintech, ...] (2 Fintech matches outrank 1 Telecom match)", report.IndustryHints)
	}
}

// Regression test for "the dictionary only covers JS/npm projects" — a
// Python, Java, or .NET repo should get the same dictionary-driven breadth
// an npm project already gets, not just the small built-in fallback.
func TestDetectStackUsesManifestStackSignalsForNonNpmEcosystems(t *testing.T) {
	dict := model.Dictionary{
		Version: "2.0.0",
		Categories: map[string]model.Signals{
			"Feature Development": {Impact: "feature"},
		},
		ManifestStackSignals: map[string]map[string]string{
			"requirements.txt": {"fastapi": "FastAPI", "sqlalchemy": "SQLAlchemy"},
			"pom.xml":          {"spring-boot-starter-web": "Spring Boot"},
			"*.csproj":         {"Microsoft.AspNetCore": "ASP.NET Core"},
		},
	}

	t.Run("python requirements.txt", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		writeFile(t, dir, "requirements.txt", "fastapi==0.110.0\nsqlalchemy==2.0.0\n")
		commitAll(t, dir)

		report, err := DetectStack(dir, dict)
		if err != nil {
			t.Fatalf("DetectStack() error: %v", err)
		}
		for _, want := range []string{"FastAPI", "SQLAlchemy"} {
			if !contains(report.TechStack, want) {
				t.Errorf("TechStack = %v, missing %q", report.TechStack, want)
			}
		}
	})

	t.Run("java pom.xml (new manifest type, not previously scanned)", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		writeFile(t, dir, "pom.xml", "<project><dependencies><dependency><artifactId>spring-boot-starter-web</artifactId></dependency></dependencies></project>")
		commitAll(t, dir)

		report, err := DetectStack(dir, dict)
		if err != nil {
			t.Fatalf("DetectStack() error: %v", err)
		}
		if !contains(report.TechStack, "Spring Boot") {
			t.Errorf("TechStack = %v, want Spring Boot detected from pom.xml", report.TechStack)
		}
	})

	t.Run(".csproj suffix-matched manifest (basename varies per project)", func(t *testing.T) {
		dir := t.TempDir()
		initGitRepo(t, dir)
		writeFile(t, dir, "MyApp.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web"><ItemGroup><PackageReference Include="Microsoft.AspNetCore" /></ItemGroup></Project>`)
		commitAll(t, dir)

		report, err := DetectStack(dir, dict)
		if err != nil {
			t.Fatalf("DetectStack() error: %v", err)
		}
		if !contains(report.TechStack, "ASP.NET Core") {
			t.Errorf("TechStack = %v, want ASP.NET Core detected from MyApp.csproj", report.TechStack)
		}
	})
}

// Regression test for a real mislabeling: a food-delivery marketplace repo
// (src/modules/{vendors,products,orders,carts,delivery}) that merely uses
// Stripe for checkout was labeled "Fintech" with zero E-commerce/Logistics
// hints, because industry detection only ever looked at manifest
// dependencies — and Stripe/payment-processor presence is far too generic
// to imply the business itself is a financial product. Folder/module names
// are the actual strongest signal here.
func TestDetectStackPathKeywordsOutrankGenericPaymentProcessorDependency(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	writeFile(t, dir, "package.json", `{"dependencies": {"stripe": "^18.5.0"}}`)
	writeFile(t, dir, "src/modules/vendors/vendors.service.ts", "export class VendorsService {}")
	writeFile(t, dir, "src/modules/products/products.service.ts", "export class ProductsService {}")
	writeFile(t, dir, "src/modules/orders/orders.service.ts", "export class OrdersService {}")
	writeFile(t, dir, "src/modules/carts/carts.service.ts", "export class CartsService {}")
	writeFile(t, dir, "src/modules/delivery/delivery.service.ts", "export class DeliveryService {}")
	// A second, distinct Logistics-matching directory: the confidence floor
	// (minIndustryMatches) requires 2+ distinct directory matches before an
	// industry is reported at all, so a single "delivery" dir alone would
	// no longer clear it.
	writeFile(t, dir, "src/modules/shipment/shipment.service.ts", "export class ShipmentService {}")
	commitAll(t, dir)

	dict := model.Dictionary{
		Version: "2.1.0",
		Categories: map[string]model.Signals{
			"Feature Development": {Impact: "feature"},
		},
		// Deliberately mirrors the fixed backend dictionary: stripe is NOT
		// mapped to any industry (too generic), only folder names are.
		IndustryPathKeywords: map[string][]string{
			"E-commerce": {"vendors", "products", "orders", "carts", "checkout"},
			"Logistics":  {"delivery", "shipment", "fleet"},
		},
	}

	report, err := DetectStack(dir, dict)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if len(report.IndustryHints) == 0 {
		t.Fatalf("IndustryHints = %v, want E-commerce/Logistics detected from folder names", report.IndustryHints)
	}
	if report.IndustryHints[0] != "E-commerce" {
		t.Errorf("IndustryHints = %v, want E-commerce ranked first (4 distinct module matches vs 2 Logistics matches)", report.IndustryHints)
	}
	if !contains(report.IndustryHints, "Logistics") {
		t.Errorf("IndustryHints = %v, want Logistics also present", report.IndustryHints)
	}
	if contains(report.IndustryHints, "Fintech") {
		t.Errorf("IndustryHints = %v, must NOT contain Fintech — stripe alone doesn't make this a financial product", report.IndustryHints)
	}
}

// Regression test for "a pure-Go CLI tool has no path to any industry
// signal at all" — go.mod (a non-npm manifest) resolving two of its own
// dependencies to the same industry label must clear the 2-match confidence
// floor on its own, the same way package.json already could.
func TestDetectStackNonNpmManifestIndustrySignalsClearConfidenceFloor(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "go.mod", "module example.com/tool\n\nrequire (\n\tgithub.com/spf13/cobra v1.8.0\n\tgithub.com/spf13/viper v1.18.0\n)\n")
	writeFile(t, dir, "main.go", "package main\nfunc main() {}\n")
	commitAll(t, dir)

	dict := model.Dictionary{
		Version: "2.4.0",
		Categories: map[string]model.Signals{
			"Feature Development": {Impact: "feature"},
		},
		IndustrySignals: map[string]string{
			"github.com/spf13/cobra": "Developer Tools & Infrastructure",
			"github.com/spf13/viper": "Developer Tools & Infrastructure",
		},
	}

	report, err := DetectStack(dir, dict)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if !contains(report.IndustryHints, "Developer Tools & Infrastructure") {
		t.Errorf("IndustryHints = %v, want Developer Tools & Infrastructure (2 matching deps in go.mod)", report.IndustryHints)
	}
}

// Pins the confidence-floor behavior directly: a single incidental path
// match (one file, one directory) must never produce an industry hint on
// its own — this is the exact false-positive shape from the original bug
// report (one PaymentTracker.tsx file wrongly tagging an entire repo
// E-commerce).
func TestDetectStackSingleIncidentalPathMatchProducesNoIndustryHint(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "src/proposals/PaymentTracker.tsx", "export {}")
	commitAll(t, dir)

	dict := model.Dictionary{
		IndustryPathKeywords: map[string][]string{
			"E-commerce": {"payment", "checkout"},
		},
	}

	report, err := DetectStack(dir, dict)
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if len(report.IndustryHints) != 0 {
		t.Errorf("IndustryHints = %v, want empty (single match must not clear the floor)", report.IndustryHints)
	}
}

func TestDetectStackIndustryHintEmptyWhenNoIndustrySignalsMatch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, dir, "package.json", `{"dependencies": {"react": "^18.0.0"}}`)
	commitAll(t, dir)

	report, err := DetectStack(dir, model.Dictionary{
		IndustrySignals: map[string]string{"stripe": "Fintech"},
	})
	if err != nil {
		t.Fatalf("DetectStack() error: %v", err)
	}
	if len(report.IndustryHints) != 0 {
		t.Errorf("IndustryHints = %v, want empty", report.IndustryHints)
	}
}
