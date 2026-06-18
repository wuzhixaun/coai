package admin

import (
	"chat/globals"
	"chat/utils"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PaymentOrderData struct {
	Total   int64          `json:"total"`
	Data    []PaymentOrder `json:"data"`
}

type PaymentOrder struct {
	UserId    int     `json:"user_id"`
	Username  string  `json:"username"`
	Type      string  `json:"type"`
	Service   string  `json:"service"`
	Amount    float32 `json:"amount"`
	OrderId   string  `json:"order_id"`
	Name      string  `json:"name"`
	Device    string  `json:"device"`
	State     bool    `json:"state"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func queryPaymentRows(db *sql.DB, search string, pageSize, offset int) (*sql.Rows, error) {
	if search != "" {
		return globals.QueryDb(db, `
			SELECT user_id, username, type, service, amount, order_id, name, device, state, created_at, updated_at
			FROM payment WHERE order_id LIKE ? OR username LIKE ?
			ORDER BY id DESC LIMIT ? OFFSET ?
		`, "%"+search+"%", "%"+search+"%", pageSize, offset)
	}
	return globals.QueryDb(db, `
		SELECT user_id, username, type, service, amount, order_id, name, device, state, created_at, updated_at
		FROM payment ORDER BY id DESC LIMIT ? OFFSET ?
	`, pageSize, offset)
}

func PaymentListAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)
	page, _ := strconv.Atoi(c.Query("page"))
	search := c.Query("search")
	if page < 0 {
		page = 0
	}
	pageSize := 20
	offset := page * pageSize

	var total int64
	if search != "" {
		globals.QueryRowDb(db, "SELECT COUNT(*) FROM payment WHERE order_id LIKE ? OR username LIKE ?",
			"%"+search+"%", "%"+search+"%").Scan(&total)
	} else {
		globals.QueryRowDb(db, "SELECT COUNT(*) FROM payment").Scan(&total)
	}

	rows, err := queryPaymentRows(db, search, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusOK, PaymentOrderData{Total: 0, Data: []PaymentOrder{}})
		return
	}
	defer rows.Close()

	orders := make([]PaymentOrder, 0)
	for rows.Next() {
		var order PaymentOrder
		rows.Scan(&order.UserId, &order.Username, &order.Type, &order.Service,
			&order.Amount, &order.OrderId, &order.Name, &order.Device,
			&order.State, &order.CreatedAt, &order.UpdatedAt)
		orders = append(orders, order)
	}

	c.JSON(http.StatusOK, PaymentOrderData{Total: total, Data: orders})
}

func PaymentRecheckAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)
	orderId := c.Query("order")

	var state bool
	if err := globals.QueryRowDb(db, "SELECT state FROM payment WHERE order_id = ?", orderId).Scan(&state); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":      false,
			"order_state": false,
			"is_changed":  false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      true,
		"order_state": state,
		"is_changed":  false,
	})
}
