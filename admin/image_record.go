package admin

import (
	"chat/globals"
	"chat/utils"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ImageRecordData struct {
	Total   int64             `json:"total"`
	Records []ImageRecordItem `json:"records"`
}

type ImageRecordItem struct {
	Username    string  `json:"username"`
	Source      string  `json:"source"`
	Model       string  `json:"model"`
	ChannelName string  `json:"channel_name"`
	ImageCount  int     `json:"image_count"`
	Quota       float32 `json:"quota"`
	Duration    float32 `json:"duration"`
	Status      string  `json:"status"`
	RequestID   string  `json:"request_id"`
	Code        int     `json:"code"`
	Message     string  `json:"message"`
	CreatedAt   string  `json:"created_at"`
}

type ImageModelStat struct {
	Model      string  `json:"model"`
	ImageCount int64   `json:"image_count"`
	Quota      float32 `json:"quota"`
}

type ImageRecordStatsData struct {
	ImagesToday  int64            `json:"images_today"`
	ImagesMonth  int64            `json:"images_month"`
	BillingToday float32          `json:"billing_today"`
	BillingMonth float32          `json:"billing_month"`
	SuccessToday int64            `json:"success_today"`
	FailedToday  int64            `json:"failed_today"`
	TopModels    []ImageModelStat `json:"top_models"`
}

// ImageRecordListAPI 分页返回图片生成观测记录。支持按 source / status 过滤。
func ImageRecordListAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 0 {
		page = 0
	}
	pageSize := 20
	offset := page * pageSize

	source := c.Query("source")
	status := c.Query("status")

	where := "WHERE 1=1"
	args := make([]interface{}, 0)
	if source != "" {
		where += " AND source = ?"
		args = append(args, source)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int64
	globals.QueryRowDb(db, "SELECT COUNT(*) FROM image_generation "+where, args...).Scan(&total)

	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	rows, err := globals.QueryDb(db, `
		SELECT username, source, model, channel_name, image_count, quota, duration,
		       status, request_id, code, message, created_at
		FROM image_generation `+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		c.JSON(http.StatusOK, ImageRecordData{Total: 0, Records: []ImageRecordItem{}})
		return
	}
	defer rows.Close()

	records := make([]ImageRecordItem, 0)
	for rows.Next() {
		var item ImageRecordItem
		var username, source, model, channelName, reqID, message sql.NullString
		var code sql.NullInt64
		rows.Scan(&username, &source, &model, &channelName, &item.ImageCount,
			&item.Quota, &item.Duration, &item.Status, &reqID, &code, &message, &item.CreatedAt)
		item.Username = username.String
		item.Source = source.String
		item.Model = model.String
		item.ChannelName = channelName.String
		item.RequestID = reqID.String
		item.Message = message.String
		item.Code = int(code.Int64)
		records = append(records, item)
	}

	c.JSON(http.StatusOK, ImageRecordData{Total: total, Records: records})
}

// ImageRecordStatsAPI 返回图片生成的张数 / 计费 / 成功失败 / Top 模型统计。
func ImageRecordStatsAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)

	var stats ImageRecordStatsData

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(image_count), 0) FROM image_generation
		WHERE status = 'success' AND DATE(created_at) = CURDATE()
	`).Scan(&stats.ImagesToday)

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(image_count), 0) FROM image_generation
		WHERE status = 'success' AND YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())
	`).Scan(&stats.ImagesMonth)

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(quota), 0) FROM image_generation
		WHERE DATE(created_at) = CURDATE()
	`).Scan(&stats.BillingToday)

	globals.QueryRowDb(db, `
		SELECT COALESCE(SUM(quota), 0) FROM image_generation
		WHERE YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())
	`).Scan(&stats.BillingMonth)

	globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM image_generation
		WHERE status = 'success' AND DATE(created_at) = CURDATE()
	`).Scan(&stats.SuccessToday)

	globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM image_generation
		WHERE status = 'failed' AND DATE(created_at) = CURDATE()
	`).Scan(&stats.FailedToday)

	stats.TopModels = make([]ImageModelStat, 0)
	rows, err := globals.QueryDb(db, `
		SELECT model, COALESCE(SUM(image_count), 0) AS cnt, COALESCE(SUM(quota), 0) AS q
		FROM image_generation
		WHERE status = 'success' AND YEAR(created_at) = YEAR(CURDATE()) AND MONTH(created_at) = MONTH(CURDATE())
		GROUP BY model
		ORDER BY cnt DESC
		LIMIT 8
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m ImageModelStat
			var model sql.NullString
			rows.Scan(&model, &m.ImageCount, &m.Quota)
			m.Model = model.String
			stats.TopModels = append(stats.TopModels, m)
		}
	}

	c.JSON(http.StatusOK, stats)
}
