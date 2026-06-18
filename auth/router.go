package auth

import "github.com/gin-gonic/gin"

func Register(app *gin.RouterGroup) {
	app.Any("/", IndexAPI)
	app.POST("/verify", VerifyAPI)
	app.POST("/reset", ResetAPI)
	app.POST("/register", RegisterAPI)
	app.POST("/login", LoginAPI)
	app.POST("/state", StateAPI)
	app.GET("/apikey", KeyAPI)
	app.GET("/userinfo", UserInfoAPI)
	app.POST("/resetkey", ResetKeyAPI)
	app.GET("/package", PackageAPI)
	app.GET("/quota", QuotaAPI)
	app.POST("/buy", BuyAPI)
	app.POST("/payment/create", CreatePaymentAPI)
	app.GET("/payment/check/:order", CheckPaymentAPI)
	app.Any("/payment/epay/notify", EPayNotifyAPI)
	app.Any("/payment/epay/return", EPayReturnAPI)
	app.POST("/payment/stripe/webhook", StripeWebhookAPI)
	app.GET("/subscription", SubscriptionAPI)
	app.POST("/subscribe", SubscribeAPI)
	app.GET("/invite", InviteAPI)
	app.GET("/redeem", RedeemAPI)
}
