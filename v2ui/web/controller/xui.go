package controller

import (
	"github.com/gin-gonic/gin"
)

type XUIController struct {
	BaseController
	inboundController      *InboundController
	nodeOutboundController *NodeOutboundController
	settingController      *SettingController
}

func NewXUIController(g *gin.RouterGroup) *XUIController {
	a := &XUIController{}
	a.initRouter(g)
	return a
}

func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xui")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/nodeOutbounds", a.nodeOutbounds)
	g.GET("/setting", a.setting)

	a.inboundController = NewInboundController(g)
	a.nodeOutboundController = NewNodeOutboundController(g)
	a.settingController = NewSettingController(g)
}

func (a *XUIController) index(c *gin.Context) {
	html(c, "index.html", "系统状态", nil)
}

func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "入站列表", nil)
}

func (a *XUIController) nodeOutbounds(c *gin.Context) {
	html(c, "node_outbounds.html", "节点出口", nil)
}

func (a *XUIController) setting(c *gin.Context) {
	html(c, "setting.html", "设置", nil)
}
