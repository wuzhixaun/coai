package photo

import (
	"database/sql"
	"net/http"
	"time"

	"chat/utils"

	"github.com/gin-gonic/gin"
)

// ── 一致性身份 CRUD ──────────────────────────────────────────
//
// 身份 = 一组参考图(引用 photo_images) + 锁定 seed + 主体描述 prompt。
// 出图时注入这三者，使「同一商品/模特」跨场景、跨功能保持主体一致。

// lockedSeed 在身份创建时生成一个稳定 seed；同一身份后续复用该 seed 即可获得可复现、
// 更一致的生成结果（绝对值不重要，复用同值才是关键）。
func lockedSeed() int {
	return int(time.Now().UnixNano() % 2147483647)
}

// resolveRefImageUrls 把 ref_image_ids 解析为可展示的 url，并顺带校验归属。
// 返回的 url 顺序与传入 id 一致；无法解析（不存在/非本人）的 id 会被跳过。
func resolveRefImageUrls(db *sql.DB, ids []string, userID int64) []string {
	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		if img, err := queryImageByID(db, id, userID); err == nil && img != nil {
			urls = append(urls, img.Url)
		}
	}
	return urls
}

func queryIdentityByID(db *sql.DB, id string, userID int64) (*IdentityInfo, error) {
	var it IdentityInfo
	var refJSON string
	var subject sql.NullString
	var createdAt string
	err := db.QueryRow(`
		SELECT id, type, name, ref_image_ids, seed, subject_prompt, created_at
		FROM photo_identity WHERE id = ? AND user_id = ?
	`, id, userID).Scan(&it.Id, &it.Type, &it.Name, &refJSON, &it.Seed, &subject, &createdAt)
	if err != nil {
		return nil, err
	}
	it.RefImageIds = parseJSONStringArray(refJSON)
	it.SubjectPrompt = nullToString(subject)
	it.CreatedAt = createdAt
	return &it, nil
}

func CreateIdentityAPI(c *gin.Context) {
	var req CreateIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请求参数错误"})
		return
	}
	if req.Type != IdentityTypeModel {
		req.Type = IdentityTypeProduct
	}

	db := getDBFromCtx(c)
	userID := getUserID(c)

	// 校验参考图均属于当前用户，并解析出展示 url
	urls := resolveRefImageUrls(db, req.RefImageIds, userID)
	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参考图无效或不属于当前用户"})
		return
	}

	id := generateImageID()
	seed := lockedSeed()
	_, err := db.Exec(`INSERT INTO photo_identity
		(id, user_id, type, name, ref_image_ids, seed, subject_prompt)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID, req.Type, req.Name, utils.Marshal(req.RefImageIds), seed, req.SubjectPrompt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建失败"})
		return
	}

	c.JSON(http.StatusOK, IdentityInfo{
		Id: id, Type: req.Type, Name: req.Name, RefImageIds: req.RefImageIds,
		RefImageUrls: urls, Seed: seed, SubjectPrompt: req.SubjectPrompt,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func ListIdentitiesAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)

	typeFilter := c.Query("type")
	query := `SELECT id, type, name, ref_image_ids, seed, subject_prompt, created_at
		FROM photo_identity WHERE user_id = ?`
	args := []interface{}{userID}
	if typeFilter == IdentityTypeProduct || typeFilter == IdentityTypeModel {
		query += " AND type = ?"
		args = append(args, typeFilter)
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询失败"})
		return
	}
	defer rows.Close()

	list := make([]IdentityInfo, 0)
	for rows.Next() {
		var it IdentityInfo
		var refJSON string
		var subject sql.NullString
		var createdAt string
		if err := rows.Scan(&it.Id, &it.Type, &it.Name, &refJSON, &it.Seed, &subject, &createdAt); err != nil {
			continue
		}
		it.RefImageIds = parseJSONStringArray(refJSON)
		it.SubjectPrompt = nullToString(subject)
		it.CreatedAt = createdAt
		it.RefImageUrls = resolveRefImageUrls(db, it.RefImageIds, userID)
		list = append(list, it)
	}
	c.JSON(http.StatusOK, list)
}

func DeleteIdentityAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)
	res, err := db.Exec("DELETE FROM photo_identity WHERE id = ? AND user_id = ?", c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "身份不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": c.Param("id")})
}
