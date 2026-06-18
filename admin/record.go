package admin

import (
	"chat/globals"
	"chat/utils"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type RecordData struct {
	Total   int64        `json:"total"`
	Records []RecordItem `json:"records"`
}

type RecordItem struct {
	Username        string  `json:"username"`
	Type            string  `json:"type"`
	TokenName       string  `json:"token_name"`
	Model           string  `json:"model"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	Quota           float32 `json:"quota"`
	Duration        float32 `json:"duration"`
	Detail          string  `json:"detail"`
	Prompts         string  `json:"prompts"`
	ResponsePrompts string  `json:"response_prompts"`
	Channel         int     `json:"channel"`
	ChannelName     string  `json:"channel_name"`
	CreatedAt       string  `json:"created_at"`
}

type RecordStatsData struct {
	BillingToday float32 `json:"billing_today"`
	BillingMonth float32 `json:"billing_month"`
	RequestToday int64   `json:"request_today"`
	RequestMonth int64   `json:"request_month"`
	RPM          float32 `json:"rpm"`
	TPM          float32 `json:"tpm"`
}

func RecordListAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 0 {
		page = 0
	}
	pageSize := 20
	offset := page * pageSize

	var total int64
	globals.QueryRowDb(db, "SELECT COUNT(*) FROM record").Scan(&total)

	rows, err := globals.QueryDb(db, `
		SELECT username, type, token_name, model, input_tokens, output_tokens,
		       quota, duration, detail, prompts, response_prompts, channel, channel_name, created_at
		FROM record
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusOK, RecordData{Total: 0, Records: []RecordItem{}})
		return
	}
	defer rows.Close()

	records := make([]RecordItem, 0)
	for rows.Next() {
		var item RecordItem
		var channel sql.NullInt64
		var channelName sql.NullString
		rows.Scan(&item.Username, &item.Type, &item.TokenName, &item.Model,
			&item.InputTokens, &item.OutputTokens, &item.Quota, &item.Duration,
			&item.Detail, &item.Prompts, &item.ResponsePrompts,
			&channel, &channelName, &item.CreatedAt)
		if channel.Valid {
			item.Channel = int(channel.Int64)
		}
		if channelName.Valid {
			item.ChannelName = channelName.String
		}
		records = append(records, item)
	}

	c.JSON(http.StatusOK, RecordData{Total: total, Records: records})
}

func RecordStatsAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)

	var billingToday, billingMonth float32
	var requestToday, requestMonth int64
	var rpm, tpm int64

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(quota), 0) FROM record
		WHERE type = 'consume' AND DATE(created_at) = CURDATE()
	`).Scan(&billingToday)

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(quota), 0) FROM record
		WHERE type = 'consume' AND YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())
	`).Scan(&billingMonth)

	globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM record WHERE DATE(created_at) = CURDATE()
	`).Scan(&requestToday)

	globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM record
		WHERE YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())
	`).Scan(&requestMonth)

	globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM record
		WHERE created_at >= DATE_SUB(NOW(), INTERVAL 1 MINUTE)
	`).Scan(&rpm)

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(input_tokens + output_tokens), 0) FROM record
		WHERE created_at >= DATE_SUB(NOW(), INTERVAL 1 MINUTE)
	`).Scan(&tpm)

	c.JSON(http.StatusOK, RecordStatsData{
		BillingToday: billingToday,
		BillingMonth: billingMonth,
		RequestToday: requestToday,
		RequestMonth: requestMonth,
		RPM:          float32(rpm),
		TPM:          float32(tpm),
	})
}
