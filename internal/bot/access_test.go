package bot

import (
	"testing"

	"servercommanderovertelegram/internal/config"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildAllowedUsersSet(t *testing.T) {
	allowedUsers := buildAllowedUsersSet([]config.AllowedUser{
		{ID: 111, Label: "admin"},
		{ID: 222, Label: "backup"},
	})

	_, ok := allowedUsers[111]
	assert.True(t, ok)
	_, ok = allowedUsers[222]
	assert.True(t, ok)
	_, ok = allowedUsers[333]
	assert.False(t, ok)
}

func TestUserFromUpdateMessage(t *testing.T) {
	user, ok := userFromUpdate(&models.Update{
		Message: &models.Message{
			From: &models.User{
				ID:        123,
				Username:  "admin",
				FirstName: "Ada",
				LastName:  "Lovelace",
			},
		},
	})

	assert.True(t, ok)
	assert.Equal(t, int64(123), user.ID)
	assert.Equal(t, "admin", user.Username)
	assert.Equal(t, "Ada", user.FirstName)
	assert.Equal(t, "Lovelace", user.LastName)
}

func TestUserFromUpdateCallbackQuery(t *testing.T) {
	user, ok := userFromUpdate(&models.Update{
		CallbackQuery: &models.CallbackQuery{
			From: models.User{
				ID:       456,
				Username: "admin2",
			},
		},
	})

	assert.True(t, ok)
	assert.Equal(t, int64(456), user.ID)
	assert.Equal(t, "admin2", user.Username)
}

func TestUserFromUpdateMissingUser(t *testing.T) {
	_, ok := userFromUpdate(&models.Update{})
	assert.False(t, ok)
}
