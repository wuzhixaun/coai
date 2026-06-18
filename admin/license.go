package admin

import (
	"chat/globals"
	"chat/utils"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type LicenseModuleItem struct {
	Id     string `json:"id"`
	Price  int    `json:"price"`
	Bought bool   `json:"bought"`
}

type LicenseResponse struct {
	Domain  string             `json:"domain"`
	Digest  string             `json:"digest"`
	Modules []LicenseModuleItem `json:"modules"`
}

func GetLicenseAPI(c *gin.Context) {
	db := utils.GetDBFromContext(c)

	// 1. 获取 domain
	domain := viper.GetString("system.general.backend")
	if domain == "" {
		domain = c.Request.Host
	}

	// 2. 检查是否存在 enterprise 订阅用户
	var enterpriseCount int
	if err := globals.QueryRowDb(db, `
		SELECT COUNT(*) FROM subscription
		WHERE enterprise = TRUE AND expired_at > NOW()
	`).Scan(&enterpriseCount); err != nil {
		enterpriseCount = 0
	}

	hasPro := enterpriseCount > 0

	// 3. 生成 digest (domain + subscription status 的哈希)
	secret := viper.GetString("secret")
	digest := utils.Sha2Encrypt(fmt.Sprintf("%s:%s:%t", domain, secret, hasPro))

	// 4. 构建模块列表
	modules := []LicenseModuleItem{
		{Id: "coai-pro", Price: -1, Bought: hasPro},
		{Id: "afdian", Price: 1000, Bought: hasPro},
		{Id: "paypal", Price: 2000, Bought: hasPro},
		{Id: "stripe", Price: 2000, Bought: hasPro},
		{Id: "digital", Price: 50000, Bought: hasPro},
	}

	c.JSON(http.StatusOK, LicenseResponse{
		Domain:  domain,
		Digest:  digest[:8],
		Modules: modules,
	})
}
