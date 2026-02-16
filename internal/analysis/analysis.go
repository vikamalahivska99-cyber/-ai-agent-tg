package analysis

import (
	"context"
	"fmt"
	"strings"
)

// TestCase описує один тест-кейс, який повертає сервіс аналізу.
// Поля оформлені так, щоб їх було зручно відображати у відповідях бота.
type TestCase struct {
	ID            string
	Title         string
	Preconditions []string
	Steps         []string
	Expected      string
	Actual        string
	// Priority — бізнес-пріоритет (наприклад, High / Medium / Low).
	Priority string
	// Severity — рівень впливу (наприклад, Critical / Major / Minor).
	Severity string
}

// BugAnalysis містить агреговану інформацію про баг та пов'язані тест-кейси.
type BugAnalysis struct {
	BugTitle  string
	TestCases []TestCase
}

// Analyzer описує інтерфейс сервісу аналізу.
type Analyzer interface {
	Analyze(ctx context.Context, image []byte) (*BugAnalysis, error)
	AnalyzeText(ctx context.Context, description string) (*BugAnalysis, error)
}

// MockAnalyzer — мок-реалізація, яка завжди повертає однаковий результат.
type MockAnalyzer struct{}

// NewMockAnalyzer створює новий MockAnalyzer.
func NewMockAnalyzer() Analyzer {
	return &MockAnalyzer{}
}

// Analyze ігнорує вхідне зображення та повертає статичний набір тест-кейсів.
func (m *MockAnalyzer) Analyze(_ context.Context, _ []byte) (*BugAnalysis, error) {
	return &BugAnalysis{
		BugTitle: "Submit button is visually truncated on the login screen",
		TestCases: []TestCase{
			{
				ID:    "TC-001",
				Title: "Verify that the Submit button is fully visible on the login screen",
				Preconditions: []string{
					"User is on the login screen",
				},
				Steps: []string{
					"Open the login screen",
					"Wait until all fields are fully loaded",
				},
				Expected: "The Submit button is fully visible and clickable",
				Actual:   "The Submit button is partially cut off and not fully visible",
				Priority: "High",
				Severity: "Major",
			},
		},
	}, nil
}

// FallbackTemplate повертає шаблон тест-кейсу, коли основний аналізатор недоступний (для фото).
func FallbackTemplate() *BugAnalysis {
	return &BugAnalysis{
		BugTitle: "Sample bug / test case template",
		TestCases: []TestCase{
			{
				ID:            "TC-001",
				Title:         "Verify the reported issue on the screenshot / description",
				Preconditions: []string{"Application is open", "User has reproduced the bug"},
				Steps:         []string{"Open the affected screen", "Perform the steps that trigger the bug", "Observe the result"},
				Expected:     "Expected correct behaviour according to requirements",
				Actual:       "Actual behaviour (describe what you see)",
				Priority:     "Medium",
				Severity:     "Major",
			},
		},
	}
}

// FallbackFromUserDescription будує тест-кейси на основі тексту користувача, коли AI недоступний.
// Відповідь містить опис користувача, щоб він бачив своє повідомлення в структурі.
func FallbackFromUserDescription(description string) *BugAnalysis {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return FallbackTemplate()
	}
	title := desc
	if len(title) > 120 {
		title = title[:117] + "..."
	}
	if idx := strings.IndexAny(desc, ".\n"); idx > 10 {
		title = strings.TrimSpace(desc[:idx])
		if len(title) > 120 {
			title = title[:117] + "..."
		}
	}
	return &BugAnalysis{
		BugTitle: title,
		TestCases: []TestCase{
			{
				ID:            "TC-001",
				Title:         "Verify the reported issue",
				Preconditions: []string{"Application is open", "User can reproduce the scenario"},
				Steps:         []string{"Reproduce the steps from the description", "Observe the actual behaviour", "Compare with expected behaviour"},
				Expected:      "Behaviour matches requirements and user expectations",
				Actual:        desc,
				Priority:      "Medium",
				Severity:      "Major",
			},
		},
	}
}

// AnalyzeText ігнорує текстовий опис і повертає той самий статичний набір тест-кейсів.
func (m *MockAnalyzer) AnalyzeText(_ context.Context, _ string) (*BugAnalysis, error) {
	return &BugAnalysis{
		BugTitle: "Submit button is visually truncated on the login screen",
		TestCases: []TestCase{
			{
				ID:    "TC-001",
				Title: "Verify that the Submit button is fully visible on the login screen",
				Preconditions: []string{"User is on the login screen"},
				Steps: []string{"Open the login screen", "Wait until all fields are fully loaded"},
				Expected: "The Submit button is fully visible and clickable",
				Actual:   "The Submit button is partially cut off and not fully visible",
				Priority: "High",
				Severity: "Major",
			},
		},
	}, nil
}

// FormatBugAnalysis перетворює структуру аналізу у структуроване англомовне повідомлення.
func FormatBugAnalysis(a *BugAnalysis) string {
	if a == nil {
		return "Failed to generate bug description."
	}

	// Deduplicate test cases that look identical (same title + expected + actual).
	a.TestCases = deduplicateTestCases(a.TestCases)

	var b strings.Builder

	b.WriteString("Automatically generated test cases for the detected bug\n\n")
	b.WriteString("Bug: ")
	b.WriteString(a.BugTitle)
	b.WriteString("\n\n")

	for i, tc := range a.TestCases {
		b.WriteString(formatTestCase(i+1, &tc))
	}

	return b.String()
}

// deduplicateTestCases прибирає дублікати тест-кейсів за ключем (Title, Expected, Actual).
func deduplicateTestCases(in []TestCase) []TestCase {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]TestCase, 0, len(in))
	for _, tc := range in {
		key := strings.TrimSpace(tc.Title) + "||" +
			strings.TrimSpace(tc.Expected) + "||" +
			strings.TrimSpace(tc.Actual)
		if key == "||||" {
			// якщо взагалі порожній, додаємо як є (рідкісний випадок)
			out = append(out, tc)
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tc)
	}
	return out
}

func formatTestCase(idx int, tc *TestCase) string {
	var b strings.Builder

	b.WriteString("────────────────────\n")
	b.WriteString(fmt.Sprintf("Test case %s #%d\n", tc.ID, idx))
	if tc.Title != "" {
		b.WriteString(tc.Title)
		b.WriteString("\n")
	}

	if len(tc.Preconditions) > 0 {
		b.WriteString("\nPreconditions:\n")
		for _, p := range tc.Preconditions {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteString("\n")
		}
	}

	if len(tc.Steps) > 0 {
		b.WriteString("\nSteps:\n")
		for i, s := range tc.Steps {
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, s))
		}
	}

	if tc.Expected != "" {
		b.WriteString("\nExpected result:\n")
		b.WriteString(tc.Expected)
		b.WriteString("\n")
	}

	if tc.Actual != "" {
		b.WriteString("\nActual result:\n")
		b.WriteString(tc.Actual)
		b.WriteString("\n")
	}

	if tc.Priority != "" || tc.Severity != "" {
		b.WriteString("\nPriority / Severity:\n")
		if tc.Priority != "" {
			b.WriteString("- Priority: ")
			b.WriteString(tc.Priority)
			b.WriteString("\n")
		}
		if tc.Severity != "" {
			b.WriteString("- Severity: ")
			b.WriteString(tc.Severity)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	return b.String()
}

