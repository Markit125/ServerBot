package bot

import (
	"context"
	"log"
	"serverbot/internal/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type telegramUser struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
}

func (sb *ServerBot) accessMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	allowedUsers := buildAllowedUsersSet(sb.config.AllowedUsers)

	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if !sb.config.AccessEnabled {
			next(ctx, b, update)
			return
		}

		user, ok := userFromUpdate(update)
		if !ok {
			log.Printf("[ACCESS] denied update without user")
			return
		}
		if _, allowed := allowedUsers[user.ID]; allowed {
			next(ctx, b, update)
			return
		}

		log.Printf("[ACCESS] denied user_id=%d username=%q first_name=%q last_name=%q", user.ID, user.Username, user.FirstName, user.LastName)
		if chatID, ok := chatIDFromUpdate(update); ok && sb.config.AccessDenyMessage != "" {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   sb.config.AccessDenyMessage,
			})
		}
	}
}

func buildAllowedUsersSet(users []config.AllowedUser) map[int64]struct{} {
	allowedUsers := make(map[int64]struct{}, len(users))
	for _, user := range users {
		allowedUsers[user.ID] = struct{}{}
	}
	return allowedUsers
}

func userFromUpdate(update *models.Update) (telegramUser, bool) {
	if update == nil {
		return telegramUser{}, false
	}
	if update.Message != nil && update.Message.From != nil {
		return telegramUserFromModel(update.Message.From), true
	}
	if update.CallbackQuery != nil && update.CallbackQuery.From.ID != 0 {
		return telegramUserFromModel(&update.CallbackQuery.From), true
	}

	return telegramUser{}, false
}

func telegramUserFromModel(user *models.User) telegramUser {
	return telegramUser{
		ID:        user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
}
