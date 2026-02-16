package analysis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OllamaAnalyzer викликає локальний Ollama (vision модель) для аналізу зображень.
// Це безкоштовно з точки зору API-запитів, бо все працює локально на вашому ПК.
type OllamaAnalyzer struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaAnalyzer(baseURL, model string) *OllamaAnalyzer {
	return &OllamaAnalyzer{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client: &http.Client{
			// Vision-моделі (llava) часто довго обробляють перший запит — даємо 3 хв
			Timeout: 180 * time.Second,
		},
	}
}

// CheckOllamaReachable перевіряє, чи доступний Ollama за вказаною URL (при старті бота).
func CheckOllamaReachable(baseURL string) error {
	url := strings.TrimRight(baseURL, "/") + "/api/tags"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w (is Ollama running? start the Ollama app or run: ollama serve)", baseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d at %s", resp.StatusCode, baseURL)
	}
	return nil
}

type ollamaGenerateRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images,omitempty"`
	Stream bool     `json:"stream"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

func (a *OllamaAnalyzer) Analyze(ctx context.Context, image []byte) (*BugAnalysis, error) {
	if len(image) == 0 {
		return nil, fmt.Errorf("empty image")
	}

	// Зменшити та стиснути зображення, щоб Ollama не таймаутила на великих фото з Telegram.
	prepared, err := prepareImageForOllama(image)
	if err != nil {
		log.Printf("ollama: prepare image failed, using original: %v", err)
		prepared = image
	}
	log.Printf("[ollama] analyzing image: original=%d bytes, prepared=%d bytes", len(image), len(prepared))
	imgB64 := base64.StdEncoding.EncodeToString(prepared)

	prompt := `You are a senior QA engineer. Analyze this UI screenshot and write CONCRETE, SPECIFIC test cases.

WHAT TO DO:
1) Look at the screenshot and name what you see: app/screen name, buttons, labels, fields, messages, layout.
2) For each clear bug (broken button, wrong text, overlap, missing element, error message, wrong layout): write one test case with SPECIFIC steps and SPECIFIC expected vs actual.

BE SPECIFIC — bad vs good:
- BAD steps: "Open the affected screen", "Perform the steps", "Observe the result".
- GOOD steps: "Open the Login screen", "Click the 'Submit' button", "Check that the 'Save' button in the footer is visible".
- BAD expected: "Expected correct behaviour".
- GOOD expected: "The Save button is visible and clicking it saves the form".
- BAD actual: "Actual behaviour (describe what you see)".
- GOOD actual: "The Save button is cut off on the right and cannot be clicked".

Return STRICT JSON ONLY in ENGLISH (no markdown, no other text):
{
  "bugTitle": "string (short, specific: e.g. 'Save button truncated on Settings screen')",
  "testCases": [
    {
      "id": "TC-001",
      "title": "string (specific: what to verify)",
      "preconditions": ["string (e.g. User is on Settings screen)"],
      "steps": ["string (concrete action 1)", "string (concrete action 2)"],
      "expectedResult": "string (what should happen, specific)",
      "actualResult": "string (what is wrong on the screenshot, specific)",
      "priority": "High | Medium | Low",
      "severity": "Critical | Major | Minor | Trivial"
    }
  ]
}
Rules:
- 2–6 test cases. Each step and expected/actual must describe what is VISIBLE on the screenshot (names of buttons, labels, error text).
- All text in English only. priority/severity: High=must fix, Medium=important, Low=minor; Critical/Major/Minor/Trivial for impact.
- Ignore pure accessibility (contrast, ARIA) unless it breaks normal use.
`

	reqBody := ollamaGenerateRequest{
		Model:  a.model,
		Prompt: prompt,
		Images: []string{imgB64},
		Stream: false,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&reqBody); err != nil {
		return nil, fmt.Errorf("encode ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/generate", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var genResp ollamaGenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w (raw=%s)", err, strings.TrimSpace(string(body)))
	}
	if genResp.Error != "" {
		return nil, fmt.Errorf("ollama error: %s", genResp.Error)
	}

	respPreview := genResp.Response
	if len(respPreview) > 500 {
		respPreview = respPreview[:500] + "..."
	}
	log.Printf("ollama: response len=%d, preview=%q", len(genResp.Response), strings.TrimSpace(respPreview))

	// Модель може повертати JSON у блоці ```json ... ``` — спочатку прибираємо обгортку.
	responseBody := stripMarkdownCodeBlock(genResp.Response)
	jsonText := extractFirstJSONObject(responseBody)

	// Внутрішній DTO; steps/preconditions приймає і рядок, і масив (модель іноді ламає схему).
	var dto struct {
		BugTitle  string `json:"bugTitle"`
		TestCases []struct {
			ID            string          `json:"id"`
			Title         string          `json:"title"`
			Preconditions flexStringSlice `json:"preconditions"`
			Steps         flexStringSlice `json:"steps"`
			Expected      string          `json:"expectedResult"`
			Actual        string          `json:"actualResult"`
			Priority      string          `json:"priority"`
			Severity      string          `json:"severity"`
		} `json:"testCases"`
	}

	// Якщо модель не повернула JSON, використовуємо raw-текст як fallback.
	if jsonText == "" {
		log.Printf("ollama: no JSON object detected in response, using raw fallback")
		return fallbackFromRaw(genResp.Response), nil
	}

	if err := json.Unmarshal([]byte(jsonText), &dto); err != nil {
		log.Printf("ollama: JSON parse error: %v, snippet=%q", err, truncate(jsonText, 300))
		return fallbackFromRaw(genResp.Response), nil
	}

	log.Printf("ollama: parsed bugTitle=%q, testCases=%d", dto.BugTitle, len(dto.TestCases))

	out := &BugAnalysis{
		BugTitle: dto.BugTitle,
	}
	for _, tc := range dto.TestCases {
		steps := []string(tc.Steps)
		if len(steps) == 0 {
			steps = []string{"See actual result"}
		}
		out.TestCases = append(out.TestCases, TestCase{
			ID:            tc.ID,
			Title:         tc.Title,
			Preconditions: []string(tc.Preconditions),
			Steps:         steps,
			Expected:      tc.Expected,
			Actual:        tc.Actual,
			Priority:      tc.Priority,
			Severity:      tc.Severity,
		})
	}
	if out.BugTitle == "" {
		out.BugTitle = "Bug found based on screenshot analysis"
	}
	if len(out.TestCases) == 0 {
		// Фолбек, щоб бот завжди повертав щось корисне.
		out.TestCases = []TestCase{
			{
				ID:       "TC-001",
				Title:    "Verify visual appearance of the UI element on the screenshot",
				Steps:    []string{"Open the screen shown on the screenshot", "Check that key UI elements are fully visible and readable"},
				Expected: "UI elements are fully visible, readable and not overlapping or truncated",
				Actual:   "There is a visual problem on the screen according to the screenshot",
				Priority: "Medium",
				Severity: "Major",
			},
		}
	}

	return out, nil
}

// AnalyzeText аналізує текстовий опис бага (у будь-якій мові) та повертає тест-кейси.
func (a *OllamaAnalyzer) AnalyzeText(ctx context.Context, description string) (*BugAnalysis, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return nil, fmt.Errorf("empty description")
	}

	prompt := `You are a senior QA engineer specializing in functional testing and UI/UX (NOT accessibility).
You will receive a free-text bug description from a tester (it may be in English or another language).
First, understand and mentally translate the description into English.
Then identify ALL clear functional, visual, layout and content issues described.
Ignore accessibility-only concerns (contrast, focus order, screen reader labels, ARIA roles, etc.) unless they clearly break functional behaviour for all users.
Return STRICT JSON ONLY in ENGLISH (no markdown, no explanations, no extra text) with this schema:
{
  "bugTitle": "string",
  "testCases": [
    {
      "id": "TC-001",
      "title": "string",
      "preconditions": ["string"],
      "steps": ["string"],
      "expectedResult": "string",
      "actualResult": "string",
      "priority": "High | Medium | Low",
      "severity": "Critical | Major | Minor | Trivial"
    }
  ]
}
Rules:
- Provide multiple test cases (2-6) covering ALL clearly described functional / UI / layout / content issues.
- All text MUST be in English only.
- Choose priority based on business impact (High = must fix now, Medium = important but not blocking, Low = nice to have).
- Choose severity based on impact on functionality and users (Critical, Major, Minor, Trivial).

Bug description from tester:
` + desc + `
`

	reqBody := ollamaGenerateRequest{
		Model:  a.model,
		Prompt: prompt,
		Stream: false,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&reqBody); err != nil {
		return nil, fmt.Errorf("encode ollama text request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/generate", &buf)
	if err != nil {
		return nil, fmt.Errorf("create text request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama (text): %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama text http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var genResp ollamaGenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, fmt.Errorf("decode ollama text response: %w (raw=%s)", err, strings.TrimSpace(string(body)))
	}
	if genResp.Error != "" {
		return nil, fmt.Errorf("ollama text error: %s", genResp.Error)
	}

	responseBody := stripMarkdownCodeBlock(genResp.Response)
	jsonText := extractFirstJSONObject(responseBody)

	// Внутрішній DTO; steps/preconditions приймає і рядок, і масив.
	var dto struct {
		BugTitle  string `json:"bugTitle"`
		TestCases []struct {
			ID            string          `json:"id"`
			Title         string          `json:"title"`
			Preconditions flexStringSlice `json:"preconditions"`
			Steps         flexStringSlice `json:"steps"`
			Expected      string          `json:"expectedResult"`
			Actual        string          `json:"actualResult"`
			Priority      string          `json:"priority"`
			Severity      string          `json:"severity"`
		} `json:"testCases"`
	}

	// Якщо модель не повернула JSON, використовуємо raw-текст як fallback.
	if jsonText == "" {
		log.Printf("ollama text: no JSON object detected, using raw response fallback")
		return fallbackFromRaw(genResp.Response), nil
	}

	if err := json.Unmarshal([]byte(jsonText), &dto); err != nil {
		log.Printf("ollama text: failed to parse JSON, using raw response fallback: %v, json=%s", err, jsonText)
		return fallbackFromRaw(genResp.Response), nil
	}

	out := &BugAnalysis{
		BugTitle: dto.BugTitle,
	}
	for _, tc := range dto.TestCases {
		steps := []string(tc.Steps)
		if len(steps) == 0 {
			steps = []string{"See actual result"}
		}
		out.TestCases = append(out.TestCases, TestCase{
			ID:            tc.ID,
			Title:         tc.Title,
			Preconditions: []string(tc.Preconditions),
			Steps:         steps,
			Expected:      tc.Expected,
			Actual:        tc.Actual,
			Priority:      tc.Priority,
			Severity:      tc.Severity,
		})
	}
	if out.BugTitle == "" {
		out.BugTitle = "Bug found based on textual description"
	}
	if len(out.TestCases) == 0 {
		out.TestCases = []TestCase{
			{
				ID:       "TC-001",
				Title:    "Verify behaviour described in the bug report",
				Steps:    []string{"Follow the steps from the tester description", "Observe the behaviour that should be fixed"},
				Expected: "The application behaves according to the functional requirements",
				Actual:   desc,
				Priority: "Medium",
				Severity: "Major",
			},
		}
	}

	return out, nil
}

// flexStringSlice приймає з JSON як один рядок, так і масив рядків (модель іноді повертає "steps": "one step" замість масиву).
type flexStringSlice []string

func (f *flexStringSlice) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*f = nil
		return nil
	}
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = nil
		} else {
			*f = []string{s}
		}
		return nil
	}
	return json.Unmarshal(data, (*[]string)(f))
}

// stripMarkdownCodeBlock прибирає обгортку ```json ... ``` або ``` ... ``` з відповіді моделі.
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	const fence = "```"
	idx := strings.Index(s, fence)
	if idx < 0 {
		return s
	}
	// Пропустити відкриваючий fence та опційне слово "json"
	afterOpen := s[idx+len(fence):]
	afterOpen = strings.TrimSpace(afterOpen)
	if strings.HasPrefix(afterOpen, "json") {
		afterOpen = strings.TrimSpace(afterOpen[4:])
	}
	closeIdx := strings.Index(afterOpen, fence)
	if closeIdx < 0 {
		return afterOpen
	}
	return strings.TrimSpace(afterOpen[:closeIdx])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractFirstJSONObject намагається витягнути перший JSON object {...} з тексту.
func extractFirstJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			continue
		}
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// fallbackFromRaw створює базовий BugAnalysis, якщо модель не дотрималась JSON-контракту.
func fallbackFromRaw(raw string) *BugAnalysis {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "Model returned an empty response."
	}
	return &BugAnalysis{
		BugTitle: "Bug description from model (unstructured)",
		TestCases: []TestCase{
			{
				ID:       "TC-RAW-001",
				Title:    "Review bug description generated by the model",
				Steps:    []string{"Review the following description produced by the AI model", "Convert it into formal test cases if needed"},
				Expected: "The description below accurately reflects the visible issues on the screenshot",
				Actual:   raw,
				Priority: "Medium",
				Severity: "Major",
			},
		},
	}
}

