# Telegram Bug Report Bot (Go)

Телеграм-бот на Go: приймає **фото/скріншот** або **текстовий опис** багу і повертає згенеровані тест-кейси англійською (з пріоритетом і severity). Підтримує локальний AI через Ollama.

---

## Documentation & deliverables

| File | Description |
|------|-------------|
| [**TESTCASES.md**](TESTCASES.md) | Test cases for the bot (manual testing results, Pass/Fail). |
| [**QA_SUMMARY.md**](QA_SUMMARY.md) | QA summary report: scope, environment, execution summary, issues found, conclusion. |
| **`.env`** | **Not in the repository** (secrets). Use [**.env.example**](.env.example) as a template: copy to `.env` and set your `TELEGRAM_BOT_TOKEN` and `ANALYSIS_MODE`. |

```bash
cp .env.example .env
# Edit .env and set TELEGRAM_BOT_TOKEN=your_token_from_BotFather
```

---

## Requirements

- **Go** 1.21+
- Telegram bot token from [@BotFather](https://t.me/BotFather)
- (Optional) [Ollama](https://ollama.com/download) for AI analysis of screenshots

---

## Quick start (copy-paste)

**1. Clone and enter the project:**

```bash
git clone https://github.com/vikamalahivska99-cyber/-ai-agent-tg.git
cd -ai-agent-tg
```

**2. Create `.env` from the example:**

```bash
cp .env.example .env
```

Edit `.env` and set at least:

- `TELEGRAM_BOT_TOKEN=your_token_from_BotFather`
- `ANALYSIS_MODE=mock` (or `ollama` if you have Ollama running)

**3. Install dependencies and run:**

```bash
go mod tidy
go run ./cmd/bot
```

You should see: `authorized on account: @YourBot`. Then open Telegram and message your bot.

---

## Command examples (for beginners)

Run everything from the **project root** (where `go.mod` and `.env` are).

| What you want | Command |
|---------------|--------|
| Run the bot | `go run ./cmd/bot` |
| Build binary | `go build -o bin/bot ./cmd/bot` |
| Run binary | `./bin/bot` (Linux/macOS) or `bin\bot.exe` (Windows) |
| Run tests | `go test ./...` |
| With Makefile | `make run` — run bot; `make build` — build; `make test` — tests |

**PowerShell (Windows) — run without .env file:**

```powershell
$env:TELEGRAM_BOT_TOKEN = "your_token_here"
$env:ANALYSIS_MODE = "mock"
go run ./cmd/bot
```

**Linux/macOS:**

```bash
export TELEGRAM_BOT_TOKEN="your_token_here"
export ANALYSIS_MODE="mock"
go run ./cmd/bot
```

---

## Docker (quick start)

If you have Docker installed, you can run the bot without installing Go locally.

**1. Create `.env`** (copy from `.env.example`, set `TELEGRAM_BOT_TOKEN`).

**2. Build and run:**

```bash
docker compose up -d
```

**3. View logs:**

```bash
docker compose logs -f
```

**4. Stop:**

```bash
docker compose down
```

**Manual Docker commands:**

```bash
docker build -t bugreport-bot:latest .
docker run --rm --env-file .env bugreport-bot:latest
```

> **Note:** For `ANALYSIS_MODE=ollama`, Ollama must run on the host or in another container; the default Docker setup runs the bot only (mock mode works out of the box).

---

## Налаштування

1. Створіть бота через `@BotFather` і отримайте токен.
2. Встановіть змінну середовища з токеном:

   **PowerShell (Windows):**
   ```powershell
   $env:TELEGRAM_BOT_TOKEN="ВАШ_ТОКЕН_ТУТ"
   ```

   **bash/zsh (Linux/macOS):**
   ```bash
   export TELEGRAM_BOT_TOKEN="ВАШ_ТОКЕН_ТУТ"
   ```

3. Or use **.env**: copy [.env.example](.env.example) to `.env`, set `TELEGRAM_BOT_TOKEN` and `ANALYSIS_MODE`; the bot loads `.env` at startup.

4. (One-time) Install dependencies if needed:

   ```bash
   go mod tidy
   ```

## Безкоштовний AI-аналіз зображень (локально через Ollama)

Щоб бот **реально аналізував скріншот** і генерував тест-кейси з нього (без платних API), потрібен **запущений Ollama** з vision-моделлю.

1. Встановіть Ollama: **https://ollama.com/download**
2. Завантажте vision-модель (один раз):
   ```powershell
   ollama pull llava
   ```
3. **Запустіть Ollama** — відкрийте додаток Ollama або в терміналі:
   ```powershell
   ollama serve
   ```
   (Сервер має слухати на `http://127.0.0.1:11434`.)
4. Увімкніть режим аналізу й запустіть бота:
   ```powershell
   $env:TELEGRAM_BOT_TOKEN="ваш_токен"
   $env:ANALYSIS_MODE="ollama"
   $env:OLLAMA_URL="http://127.0.0.1:11434"
   $env:OLLAMA_MODEL="llava"
   go run ./cmd/bot
   ```
   У консолі має з’явитися: `Ollama is reachable; AI analysis enabled.` Якщо замість цього попередження про недоступність Ollama — спочатку запустіть Ollama (крок 3).

## Запуск

Проєкт містить `Makefile` з базовими командами.

- **Зібрати бота:**

  ```bash
  make build
  ```

- **Запустити бота:**

  ```bash
  make run
  ```

- **Запустити тести:**

  ```bash
  make test
  ```

## Як це працює

1. Користувач надсилає боту фото або зображення-документ.
2. Бот завантажує файл з Telegram через `getFile`.
3. Зображення передається в аналізатор (`internal/analysis`):
   - `mock` (за замовчуванням) — завжди повертає один і той самий приклад тест-кейсів
   - `ollama` — локально аналізує зображення та генерує тест-кейси без платних API
4. Результат надсилається користувачу у вигляді структурованих тест-кейсів.

### Чому Ollama не працює? (чекліст)

1. **Увімкнений режим Ollama**  
   У файлі `.env` (в корені проєкту) має бути:
   ```env
   ANALYSIS_MODE=ollama
   ```
   Якщо стоїть `mock` або змінна не задана — бот **ніколи** не викликає Ollama, тільки мок.  
   Тепер бот при старті сам підвантажує `.env`; перезапустіть бота після зміни.

2. **Ollama запущений**  
   Відкрийте додаток Ollama або в терміналі: `ollama serve`.  
   Сервер має слухати на `http://127.0.0.1:11434`.

3. **Модель завантажена**  
   Для фото потрібна vision-модель: `ollama pull llava`.  
   Для лише тексту підійде й звичайна модель, наприклад `ollama pull llama3.2`; у `.env` тоді можна вказати `OLLAMA_MODEL=llama3.2`.

4. **Що бачити в консолі при старті**  
   - Якщо все ок: `analysis mode: ollama` і далі `Ollama is reachable; AI analysis enabled.`  
   - Якщо Ollama вимкнений: `WARNING: ollama not reachable at ...` — спочатку запустіть Ollama (крок 2).

5. **Запуск з .env**  
   Запускайте бота з тієї папки, де лежить `.env` (корінь проєкту), наприклад: `go run ./cmd/bot` або `make run`.

### Якщо замість аналізу скріна приходять лише шаблони

- Переконайтеся, що **Ollama запущений** (відкритий додаток або `ollama serve`).
- Перевірте в консолі при старті бота: має бути рядок `Ollama is reachable; AI analysis enabled.`
- Перевірте, що в `.env` стоїть **`ANALYSIS_MODE=ollama`** (не `mock`).
- Використовуйте `OLLAMA_URL="http://127.0.0.1:11434"` (на Windows краще ніж localhost).
- Один раз виконайте `ollama pull llava`.
- Після запуску Ollama можна знову надіслати фото — перезапуск бота не обов’язковий.

---

## FAQ (common questions)

**Q: Where do I get the bot token?**  
A: Open [@BotFather](https://t.me/BotFather) in Telegram, send `/newbot`, follow the steps. Copy the token into `.env` as `TELEGRAM_BOT_TOKEN=...`.

**Q: Why does the bot always return the same template?**  
A: You are in `mock` mode. Set `ANALYSIS_MODE=ollama` in `.env`, install and run Ollama, and run `ollama pull llava` for screenshot analysis.

**Q: Why does photo analysis fail but text works?**  
A: Photo analysis needs a **vision** model. Use `OLLAMA_MODEL=llava` in `.env` and run `ollama pull llava`. Text-only models (e.g. llama3.2) do not accept images.

**Q: Can I run the bot 24/7?**  
A: When running on your PC, the bot stops when you close the terminal or turn off the computer. For 24/7, run it on a server (e.g. VPS) or use Docker on a machine that stays on.

**Q: Is my `.env` file safe?**  
A: `.env` is in `.gitignore` and is **not** committed to GitHub. Never share your token. Use `.env.example` as a template; it contains no secrets.

**Q: How do I run tests?**  
A: From the project root: `go test ./...` or `make test`.

---

## Recent improvements (documentation & DX)

- **.env.example** — Added; copy to `.env` and set your token. No secrets in the repo.
- **FAQ** — Added for common questions (token, Ollama, photo vs text, 24/7, safety).
- **Docker** — Added Dockerfile and docker-compose for quick start without installing Go.
- **README** — More command examples, clearer formatting, table of commands, and explicit links to [TESTCASES.md](TESTCASES.md) and [QA_SUMMARY.md](QA_SUMMARY.md).

