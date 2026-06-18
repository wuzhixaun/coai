package manager

import (
	"chat/auth"
	"chat/globals"
	"chat/utils"
	"database/sql"
	"fmt"
)

func SaveUsageRecord(db *sql.DB, user *auth.User, buffer *utils.Buffer, chargedQuota float32, detail string) {
	if db == nil || user == nil || buffer == nil || buffer.IsEmpty() {
		return
	}

	if detail == "" {
		detail = "chat completion"
	}

	if _, err := globals.ExecDb(db, `
		INSERT INTO record (
			user_id, username, type, token_name, model, input_tokens, output_tokens,
			quota, duration, detail, prompts, response_prompts, channel, channel_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.GetID(db), user.Username, "consume", buffer.GetTokenName(), buffer.GetModel(),
		buffer.CountInputToken(), buffer.CountOutputToken(false), chargedQuota, buffer.GetDuration(),
		detail, buffer.GetRecordPrompts(), buffer.GetRecordResponsePrompts(),
		buffer.GetChannelID(), buffer.GetChannelName()); err != nil {
		globals.Warn(fmt.Sprintf("[record] failed to save usage record: %s", err.Error()))
	}
}
