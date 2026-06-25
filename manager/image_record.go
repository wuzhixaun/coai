package manager

import (
	"chat/auth"
	"chat/globals"
	"chat/utils"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
)

// 图片生成观测记录：区别于通用 record 表，专门保存图片链路的可排查元数据
// （张数、火山 task_id / request_id / code、失败原因），供管理后台用量视图与售后排查使用。

const (
	imageStatusSuccess = "success"
	imageStatusFailed  = "failed"

	// 图片来源（三条入口）
	ImageSourceChat  = "chat"
	ImageSourceAPI   = "api"
	ImageSourcePhoto = "photo"

	imageMessageMaxLen = 512
)

var (
	jimengCodeRegex      = regexp.MustCompile(`code=(\d+)`)
	jimengRequestIDRegex = regexp.MustCompile(`request_id=([^\s]+)`)
)

// parseJimengError 从错误文本中提取火山官方返回的 code 与 request_id。
// 即梦适配器 (adapter/jimengapi) 的错误统一格式为
// "...: code=<n> message=<msg> request_id=<id>"，这里做尽力解析，解析失败返回零值。
func parseJimengError(message string) (code int, requestID string) {
	if message == "" {
		return 0, ""
	}
	if m := jimengCodeRegex.FindStringSubmatch(message); len(m) == 2 {
		code, _ = strconv.Atoi(m[1])
	}
	if m := jimengRequestIDRegex.FindStringSubmatch(message); len(m) == 2 {
		requestID = m[1]
	}
	return code, requestID
}

// imageOutcome 根据生成结果的 error 派生落库所需的状态与诊断字段。
func imageOutcome(err error) (status, message string, code int, requestID string) {
	if err == nil {
		return imageStatusSuccess, "", 0, ""
	}
	message = err.Error()
	code, requestID = parseJimengError(message)
	if len(message) > imageMessageMaxLen {
		message = message[:imageMessageMaxLen]
	}
	return imageStatusFailed, message, code, requestID
}

// ImageGenerationRecord 是一条图片生成观测记录。
type ImageGenerationRecord struct {
	UserID      int64
	Username    string
	Source      string // chat | api | photo
	Model       string
	Channel     int
	ChannelName string
	ImageCount  int
	Quota       float32
	Duration    float32
	Status      string
	TaskID      string
	RequestID   string
	Code        int
	Message     string
}

// SaveImageGenerationRecord 将一条图片生成记录写入 image_generation 表。
// 失败仅告警，不阻断主流程（观测能力不应影响出图）。
func SaveImageGenerationRecord(db *sql.DB, rec ImageGenerationRecord) {
	if db == nil {
		return
	}
	if rec.Status == "" {
		rec.Status = imageStatusSuccess
	}
	if _, err := globals.ExecDb(db, `
		INSERT INTO image_generation (
			user_id, username, source, model, channel, channel_name,
			image_count, quota, duration, status, task_id, request_id, code, message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.UserID, rec.Username, rec.Source, rec.Model, rec.Channel, rec.ChannelName,
		rec.ImageCount, rec.Quota, rec.Duration, rec.Status, rec.TaskID, rec.RequestID,
		rec.Code, rec.Message); err != nil {
		globals.Warn(fmt.Sprintf("[image-record] failed to save image generation record: %s", err.Error()))
	}
}

// RecordImageOutcome 以基础字段落库一条图片生成观测记录，供没有 buffer/auth.User 的
// 入口（如 Photo 处理流水线）调用。失败时强制 quota=0（未成功不计费）。
func RecordImageOutcome(db *sql.DB, userID int64, username, source, model string, channel int, channelName string, imageCount int, quota, duration float32, err error) {
	if db == nil {
		return
	}
	status, message, code, requestID := imageOutcome(err)
	if err != nil {
		quota = 0
	}
	SaveImageGenerationRecord(db, ImageGenerationRecord{
		UserID:      userID,
		Username:    username,
		Source:      source,
		Model:       model,
		Channel:     channel,
		ChannelName: channelName,
		ImageCount:  imageCount,
		Quota:       quota,
		Duration:    duration,
		Status:      status,
		Code:        code,
		RequestID:   requestID,
		Message:     message,
	})
}

// recordImageGeneration 从一次图片生成的 buffer 与 error 派生并落库一条观测记录。
// imageCount 为本次实际产出的图片张数（失败时通常为 0）。该调用不应阻断主流程。
func recordImageGeneration(db *sql.DB, user *auth.User, source, model string, buffer *utils.Buffer, imageCount int, err error) {
	if db == nil || user == nil {
		return
	}
	var channelID int
	var channelName string
	var quota, duration float32
	if buffer != nil {
		channelID = buffer.GetChannelID()
		channelName = buffer.GetChannelName()
		duration = buffer.GetDuration()
		quota = buffer.GetRecordQuota()
	}
	RecordImageOutcome(db, user.GetID(db), user.Username, source, model, channelID, channelName, imageCount, quota, duration, err)
}
