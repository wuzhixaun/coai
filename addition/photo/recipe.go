package photo

import (
	"net/http"
	"time"

	"chat/utils"

	"github.com/gin-gonic/gin"
)

// ── 配方（保存/复用工作流）──────────────────────────────────
//
// 配方 = 用户保存的命名工作流（有序步骤）。提交时复用 /photo/workflow 的自定义 steps，
// 让运营把一套标准操作沉淀下来一键复跑。

type RecipeInfo struct {
	Id        string         `json:"id"`
	Name      string         `json:"name"`
	Steps     []WorkflowStep `json:"steps"`
	CreatedAt string         `json:"created_at"`
}

type CreateRecipeRequest struct {
	Name  string         `json:"name" binding:"required"`
	Steps []WorkflowStep `json:"steps" binding:"required,min=1"`
}

func CreateRecipeAPI(c *gin.Context) {
	var req CreateRecipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "请求参数错误"})
		return
	}

	db := getDBFromCtx(c)
	userID := getUserID(c)
	id := generateImageID()
	if _, err := db.Exec(`INSERT INTO photo_recipe (id, user_id, name, steps) VALUES (?, ?, ?, ?)`,
		id, userID, req.Name, utils.Marshal(req.Steps)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, RecipeInfo{Id: id, Name: req.Name, Steps: req.Steps, CreatedAt: time.Now().Format(time.RFC3339)})
}

func ListRecipesAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)
	rows, err := db.Query(`SELECT id, name, steps, created_at FROM photo_recipe WHERE user_id = ? ORDER BY created_at DESC LIMIT 200`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询失败"})
		return
	}
	defer rows.Close()

	list := make([]RecipeInfo, 0)
	for rows.Next() {
		var it RecipeInfo
		var stepsJSON, createdAt string
		if err := rows.Scan(&it.Id, &it.Name, &stepsJSON, &createdAt); err != nil {
			continue
		}
		if steps, e := utils.UnmarshalString[[]WorkflowStep](stepsJSON); e == nil {
			it.Steps = steps
		}
		it.CreatedAt = createdAt
		list = append(list, it)
	}
	c.JSON(http.StatusOK, list)
}

func DeleteRecipeAPI(c *gin.Context) {
	db := getDBFromCtx(c)
	userID := getUserID(c)
	res, err := db.Exec("DELETE FROM photo_recipe WHERE id = ? AND user_id = ?", c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除失败"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "配方不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": c.Param("id")})
}
