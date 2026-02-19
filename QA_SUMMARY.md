# QA Summary Report — Telegram Bug Report Bot

**Project:** Telegram Bug Report Bot (ai-agent-tg)  
**Tester:** Viktoriia Malakhivska  
**Date:** 2026-02-10  
**Environment:** Windows 10/11, Go 1.21+, PowerShell

---

## 1. Scope of testing

- **In scope:**
  - Telegram commands: `/start`, `/help`, `/describe`
  - Text bug description → generated test cases
  - Photo/screenshot → generated test cases
  - Edit flow: replying to the bot “Edit” message → regenerated test cases
  - Negative / edge cases: unknown command, weird/absurd input, non-UI image (cat photo), off-topic input
  - Safety check: prompt-injection / harmful request (bot should not comply)
- **Out of scope:** Performance/load testing, multi-user concurrency, deployment/hosting, integrations (Jira/GitHub Issues API).

---

## 2. Environment

| Item        | Value |
|------------|--------|
| OS         | Windows (win32 10.0.26200) |
| Go version | TBD (Go 1.21+) |
| Bot mode   | ollama |
| Ollama     | Used (model: llava) |

---

## 3. Test execution summary

| Metric          | Count |
|-----------------|--------|
| Total test cases | 11 |
| Passed          | 11 |
| Failed          | 0 |
| Blocked / N/A   | 0 |

**Pass rate:** 100% *(11 / 11 × 100)*

---

## 4. Issues found

### Issue 1: Bot accepts low-signal input (single character) and generates unrelated test cases

| Field | Description |
|-------|-------------|
| **Title** | Bot accepts low-signal input (e.g. a single dot) and generates unrelated test cases instead of asking for a meaningful description. |
| **Steps to reproduce** | 1. Open chat with the bot. 2. Send a single character (e.g. `.`). |
| **Expected** | Bot asks the user to provide a non-empty or meaningful bug description (or screenshot). |
| **Actual** | Bot treats the input as valid, runs analysis, and returns generated test cases (e.g. about "button text cut off"), which are not based on a real bug description. |
| **Severity** | Minor |
| **Priority** | Low |

*Recommendation:* Add input validation (e.g. minimum length or reject input that is only punctuation/whitespace) so the bot prompts for a proper description in such cases.

---

## 5. Conclusion

- All planned Level 1 test cases passed (11/11). Core flows (/start, /help, text → test cases, photo → test cases, edit → regenerate) worked as expected.
- The bot stayed in role on off-topic inputs and did not crash. For a harmful/prompt-injection request, it did not comply and still generated test cases (acceptable behaviour).
- Notes / risks:
  - Vision output quality depends on the model and screenshot clarity; accuracy may vary.
  - Very low-signal text (e.g., a single dot) is treated as valid input and produces test cases; optional improvement would be stricter input validation.

---

**Attachments / links**

- Test cases: [TESTCASES.md](TESTCASES.md)
- Repository: https://github.com/vikamalahivska99-cyber/-ai-agent-tg
