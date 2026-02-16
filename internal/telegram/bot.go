package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"bugreportbot/internal/analysis"
)

// editPromptText is sent after each result; when the user replies to it, we regenerate test cases from the reply.
const editPromptText = "✏️ Edit: reply to this message with your corrections or extra details, and I'll regenerate test cases."

// Bot інкапсулює логіку обробки апдейтів Telegram.
type Bot struct {
	api      *tgbotapi.BotAPI
	analyzer analysis.Analyzer
}

// NewBot створює новий екземпляр Bot.
func NewBot(api *tgbotapi.BotAPI, analyzer analysis.Analyzer) *Bot {
	return &Bot{
		api:      api,
		analyzer: analyzer,
	}
}

// Run запускає цикл обробки апдейтів до завершення контексту.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case upd, ok := <-updates:
			if !ok {
				return fmt.Errorf("updates channel closed")
			}
			if err := b.handleUpdate(ctx, &upd); err != nil {
				log.Printf("[DEBUG] handleUpdate error: %v", err)
				_ = b.sendText(upd.FromChat().ID, "Внутрішня помилка. Спробуйте ще раз. (Деталі — у консолі, де запущено бота.)")
			}
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, upd *tgbotapi.Update) error {
	if upd.Message == nil {
		return nil
	}

	chatID := upd.Message.Chat.ID

	if upd.Message.IsCommand() {
		switch upd.Message.Command() {
		case "start":
			return b.handleStart(chatID)
		case "describe", "text":
			return b.handleDescribeHint(chatID)
		case "help":
			return b.handleHelp(chatID)
		default:
			return b.sendText(chatID, "Unknown command. Use /start, /describe or /help. You can also send a photo or a text bug description.")
		}
	}

	// Reply to "Edit" prompt → regenerate test cases from the reply text.
	if upd.Message.ReplyToMessage != nil && upd.Message.ReplyToMessage.From != nil && upd.Message.ReplyToMessage.From.IsBot {
		if strings.TrimSpace(upd.Message.ReplyToMessage.Text) == editPromptText {
			return b.handleEdit(ctx, upd)
		}
	}

	// Спочатку обробляємо фото/документ (навіть якщо є підпис — аналізуємо зображення).
	if len(upd.Message.Photo) > 0 {
		return b.handlePhoto(ctx, upd)
	}
	if upd.Message.Document != nil && isImageDocument(upd.Message.Document) {
		return b.handleDocument(ctx, upd)
	}

	if txt := strings.TrimSpace(upd.Message.Text); txt != "" {
		return b.handleText(ctx, upd)
	}

	return b.sendText(chatID, "Надішліть, будь ласка, одне фото/скріншот багу або опишіть баг текстом.")
}

func (b *Bot) handleStart(chatID int64) error {
	text := "Hi! 👋\n\n" +
		"I analyze both screenshots and text descriptions of bugs, and generate functional test cases in English.\n\n" +
		"• Photo — send a screenshot of the bug; I analyze the image and generate test cases.\n\n" +
		"• Text — describe the bug in your own words (any language). I turn your description into test cases with priority and severity.\n\n" +
		"Just send a photo or write a message with the bug description."
	return b.sendText(chatID, text)
}

func (b *Bot) handleDescribeHint(chatID int64) error {
	text := "Describe the bug in text (you can use any language).\n\n" +
		"For example: what screen, what you did, what you expected, what actually happened. I will analyze it and generate test cases."
	return b.sendText(chatID, text)
}

func (b *Bot) handleHelp(chatID int64) error {
	text := "Commands\n\n" +
		"• /start — welcome and how to use the bot\n" +
		"• /describe — hint for describing a bug in text\n" +
		"• /help — this message\n\n" +
		"Usage\n\n" +
		"• Send a photo (screenshot) — I analyze the image and generate test cases.\n" +
		"• Send text — describe the bug in your own words (any language); I generate test cases with priority and severity.\n\n" +
		"Edit\n\n" +
		"After you get test cases, I send an \"Edit\" message. Reply to it with your corrections or extra details, and I'll regenerate test cases from your text."
	return b.sendText(chatID, text)
}

func (b *Bot) handlePhoto(ctx context.Context, upd *tgbotapi.Update) error {
	photoSizes := upd.Message.Photo
	if len(photoSizes) == 0 {
		return b.sendText(upd.Message.Chat.ID, "Не знайшов фото в повідомленні. Спробуйте ще раз.")
	}

	// Беремо найбільше за розміром фото.
	fileID := photoSizes[len(photoSizes)-1].FileID
	return b.processImageByFileID(ctx, upd.Message.Chat.ID, fileID)
}

func (b *Bot) handleDocument(ctx context.Context, upd *tgbotapi.Update) error {
	fileID := upd.Message.Document.FileID
	return b.processImageByFileID(ctx, upd.Message.Chat.ID, fileID)
}

func (b *Bot) processImageByFileID(ctx context.Context, chatID int64, fileID string) error {
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return b.sendText(chatID, "Не вдалося отримати файл з Telegram. Спробуйте, будь ласка, ще раз.")
	}

	url := file.Link(b.api.Token)
	resp, err := http.Get(url)
	if err != nil {
		return b.sendText(chatID, "Помилка при завантаженні зображення. Спробуйте, будь ласка, ще раз.")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return b.sendText(chatID, "Не вдалося завантажити зображення. Спробуйте, будь ласка, ще раз.")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return b.sendText(chatID, "Помилка при читанні зображення. Спробуйте, будь ласка, ще раз.")
	}

	progressMsgID, _ := b.sendTextWithID(chatID, "Analyzing your screenshot... (this may take 1–2 min)")
	analysisResult, err := b.analyzer.Analyze(ctx, data)
	if progressMsgID != 0 {
		_ = b.editMessage(chatID, progressMsgID, "Analysis complete.")
	}
	if err != nil {
		log.Printf("[DEBUG] Analyze(image) error: %v", err)
		fallback := analysis.FallbackTemplate()
		errHint := err.Error()
		if len(errHint) > 200 {
			errHint = errHint[:200] + "..."
		}
		msg := "Аналіз фото не вдався: " + errHint + "\n\n" +
			"Для скріншотів потрібна vision-модель (не звичайна текстова). Перевір:\n" +
			"• У .env: OLLAMA_MODEL=llava\n" +
			"• Виконай один раз: ollama pull llava\n" +
			"• Ollama має бути запущений (додаток або ollama serve)\n\n" +
			"Шаблон, можна відредагувати:\n\n" + analysis.FormatBugAnalysis(fallback)
		_ = b.sendLongText(chatID, msg)
		_ = b.sendText(chatID, editPromptText)
		return nil
	}

	text := analysis.FormatBugAnalysis(analysisResult)
	_ = b.sendLongText(chatID, text)
	_ = b.sendText(chatID, editPromptText)
	return nil
}

func (b *Bot) handleEdit(ctx context.Context, upd *tgbotapi.Update) error {
	chatID := upd.Message.Chat.ID
	replyText := strings.TrimSpace(upd.Message.Text)
	if replyText == "" {
		return b.sendText(chatID, "Please reply with your corrections or extra details (non-empty text).")
	}
	progressMsgID, _ := b.sendTextWithID(chatID, "Regenerating test cases from your edit...")
	result, err := b.analyzer.AnalyzeText(ctx, replyText)
	if progressMsgID != 0 {
		_ = b.editMessage(chatID, progressMsgID, "Analysis complete.")
	}
	if err != nil {
		log.Printf("[DEBUG] AnalyzeText(edit) error: %v", err)
		fallback := analysis.FallbackFromUserDescription(replyText)
		msg := "Test cases based on your edit (AI was unavailable):\n\n" + analysis.FormatBugAnalysis(fallback)
		_ = b.sendLongText(chatID, msg)
		_ = b.sendText(chatID, editPromptText)
		return nil
	}
	_ = b.sendLongText(chatID, analysis.FormatBugAnalysis(result))
	_ = b.sendText(chatID, editPromptText)
	return nil
}

func (b *Bot) handleText(ctx context.Context, upd *tgbotapi.Update) error {
	chatID := upd.Message.Chat.ID
	desc := strings.TrimSpace(upd.Message.Text)
	if desc == "" {
		return b.sendText(chatID, "Please provide a non-empty bug description or send a screenshot.")
	}

	progressMsgID, _ := b.sendTextWithID(chatID, "Analyzing your description...")
	analysisResult, err := b.analyzer.AnalyzeText(ctx, desc)
	if progressMsgID != 0 {
		_ = b.editMessage(chatID, progressMsgID, "Analysis complete.")
	}
	if err != nil {
		log.Printf("[DEBUG] AnalyzeText error: %v", err)
		fallback := analysis.FallbackFromUserDescription(desc)
		msg := "Test cases based on your description (AI was unavailable; start Ollama for full analysis):\n\n" + analysis.FormatBugAnalysis(fallback)
		_ = b.sendLongText(chatID, msg)
		_ = b.sendText(chatID, editPromptText)
		return nil
	}

	text := analysis.FormatBugAnalysis(analysisResult)
	_ = b.sendLongText(chatID, text)
	_ = b.sendText(chatID, editPromptText)
	return nil
}

func (b *Bot) sendText(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	return err
}

// sendTextWithID sends a message and returns its ID (or 0 on failure), so it can be edited for progress.
func (b *Bot) sendTextWithID(chatID int64, text string) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

// editMessage updates an existing message (e.g. progress "Analyzing..." -> "Analysis complete.").
func (b *Bot) editMessage(chatID int64, messageID int, text string) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	_, err := b.api.Send(edit)
	return err
}

// sendLongText надсилає текст частинами, щоб не перевищити ліміт Telegram 4096 символів.
func (b *Bot) sendLongText(chatID int64, text string) error {
	const maxLen = 4096
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			chunk = text[:maxLen]
			// Розрізати по останньому переносу рядка, щоб не обрізати посередині слова.
			if i := strings.LastIndex(chunk, "\n"); i > maxLen/2 {
				chunk = text[:i+1]
			}
		}
		if err := b.sendText(chatID, chunk); err != nil {
			return err
		}
		text = text[len(chunk):]
	}
	return nil
}

func isImageDocument(doc *tgbotapi.Document) bool {
	if doc == nil {
		return false
	}
	// Дуже проста перевірка за mime-типом/розширенням.
	if doc.MimeType == "image/png" || doc.MimeType == "image/jpeg" {
		return true
	}
	return false
}

