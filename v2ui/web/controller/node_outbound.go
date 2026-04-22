package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"x-ui/web/entity"
	"x-ui/web/service"
	"x-ui/web/session"
)

type NodeOutboundController struct {
	nodeOutboundService service.NodeOutboundService
	xrayService         service.XrayService
}

func NewNodeOutboundController(g *gin.RouterGroup) *NodeOutboundController {
	a := &NodeOutboundController{}
	a.initRouter(g)
	return a
}

func (a *NodeOutboundController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/nodeOutbounds")
	g.POST("/list", a.getNodeOutboundList)
	g.POST("/save", a.saveNodeOutbound)
	g.POST("/toggle", a.toggleNodeOutbound)
	g.POST("/del/:inboundId", a.deleteNodeOutbound)
	g.POST("/latency", a.getNodeOutboundLatency)
}

func (a *NodeOutboundController) getNodeOutboundList(c *gin.Context) {
	user := session.GetLoginUser(c)
	list, err := a.nodeOutboundService.GetNodeOutboundList(user.Id)
	if err != nil {
		jsonMsg(c, "获取", err)
		return
	}
	jsonObj(c, list, nil)
}

func (a *NodeOutboundController) saveNodeOutbound(c *gin.Context) {
	req := &entity.SaveNodeOutboundReq{}
	err := c.ShouldBind(req)
	if err != nil {
		jsonMsg(c, "保存", err)
		return
	}

	user := session.GetLoginUser(c)
	err = a.nodeOutboundService.Save(req, user.Id)
	jsonMsg(c, "保存", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *NodeOutboundController) toggleNodeOutbound(c *gin.Context) {
	req := &entity.ToggleNodeOutboundReq{}
	err := c.ShouldBind(req)
	if err != nil {
		jsonMsg(c, "切换", err)
		return
	}

	user := session.GetLoginUser(c)
	err = a.nodeOutboundService.Toggle(req.InboundId, req.Enable, user.Id)
	jsonMsg(c, "切换", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *NodeOutboundController) deleteNodeOutbound(c *gin.Context) {
	inboundId, err := strconv.Atoi(c.Param("inboundId"))
	if err != nil {
		jsonMsg(c, "删除", err)
		return
	}

	user := session.GetLoginUser(c)
	err = a.nodeOutboundService.DeleteByInboundId(inboundId, user.Id)
	jsonMsg(c, "删除", err)
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *NodeOutboundController) getNodeOutboundLatency(c *gin.Context) {
	req := &entity.NodeOutboundLatencyReq{}
	err := c.ShouldBind(req)
	if err != nil {
		jsonMsg(c, "检测延迟", err)
		return
	}

	user := session.GetLoginUser(c)
	result, err := a.nodeOutboundService.CheckLatency(req, user.Id)
	if err != nil {
		jsonMsg(c, "检测延迟", err)
		return
	}
	jsonObj(c, result, nil)
}