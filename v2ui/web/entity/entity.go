package entity

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"time"

	"x-ui/util/common"
	"x-ui/xray"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

type Pager struct {
	Current  int         `json:"current"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	OrderBy  string      `json:"order_by"`
	Desc     bool        `json:"desc"`
	Key      string      `json:"key"`
	List     interface{} `json:"list"`
}

type AllSetting struct {
	WebListen          string `json:"webListen" form:"webListen"`
	WebPort            int    `json:"webPort" form:"webPort"`
	WebCertFile        string `json:"webCertFile" form:"webCertFile"`
	WebKeyFile         string `json:"webKeyFile" form:"webKeyFile"`
	WebBasePath        string `json:"webBasePath" form:"webBasePath"`
	XrayTemplateConfig string `json:"xrayTemplateConfig" form:"xrayTemplateConfig"`
	TimeLocation       string `json:"timeLocation" form:"timeLocation"`
}

func (s *AllSetting) CheckValid() error {
	if s.WebListen != "" {
		ip := net.ParseIP(s.WebListen)
		if ip == nil {
			return common.NewError("web listen is not valid ip:", s.WebListen)
		}
	}
	if s.WebPort <= 0 || s.WebPort > 65535 {
		return common.NewError("web port is not a valid port:", s.WebPort)
	}
	if s.WebCertFile != "" || s.WebKeyFile != "" {
		_, err := tls.LoadX509KeyPair(s.WebCertFile, s.WebKeyFile)
		if err != nil {
			return common.NewErrorf("cert file <%v> or key file <%v> invalid: %v", s.WebCertFile, s.WebKeyFile, err)
		}
	}
	if !strings.HasPrefix(s.WebBasePath, "/") {
		s.WebBasePath = "/" + s.WebBasePath
	}
	if !strings.HasSuffix(s.WebBasePath, "/") {
		s.WebBasePath += "/"
	}

	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(s.XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}
	_, err = time.LoadLocation(s.TimeLocation)
	if err != nil {
		return common.NewError("time location not exist:", s.TimeLocation)
	}
	return nil
}

type BatchAddInboundReq struct {
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	Total      int64  `json:"total" form:"total"`
	Remark     string `json:"remark" form:"remark"`
	Enable     bool   `json:"enable" form:"enable"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`

	Reset         bool  `json:"reset" form:"reset"`
	ResetDay      int   `json:"resetDay" form:"resetDay"`
	LastResetTime int64 `json:"lastResetTime" form:"lastResetTime"`

	Listen         string `json:"listen" form:"listen"`
	Port           int    `json:"port" form:"port"`
	Protocol       string `json:"protocol" form:"protocol"`
	Settings       string `json:"settings" form:"settings"`
	StreamSettings string `json:"streamSettings" form:"streamSettings"`
	Sniffing       string `json:"sniffing" form:"sniffing"`

	BatchSocks5Text string `json:"batchSocks5Text" form:"batchSocks5Text"`
}

type NodeOutboundItem struct {
	InboundId  int    `json:"inboundId"`
	Remark     string `json:"remark"`
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	Tag        string `json:"tag"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	Total      int64  `json:"total"`
	ExpiryTime int64  `json:"expiryTime"`

	OutboundId       int    `json:"outboundId"`
	OutboundEnable   bool   `json:"outboundEnable"`
	OutboundProtocol string `json:"outboundProtocol"`
	OutboundAddress  string `json:"outboundAddress"`
	OutboundPort     int    `json:"outboundPort"`
	OutboundUsername string `json:"outboundUsername"`
	OutboundPassword string `json:"outboundPassword"`
	HasConfig        bool   `json:"hasConfig"`
	CanEnable        bool   `json:"canEnable"`
}

type SaveNodeOutboundReq struct {
	InboundId int    `json:"inboundId" form:"inboundId"`
	Enable    bool   `json:"enable" form:"enable"`
	Protocol  string `json:"protocol" form:"protocol"`
	Address   string `json:"address" form:"address"`
	Port      int    `json:"port" form:"port"`
	Username  string `json:"username" form:"username"`
	Password  string `json:"password" form:"password"`
}

type ToggleNodeOutboundReq struct {
	InboundId int  `json:"inboundId" form:"inboundId"`
	Enable    bool `json:"enable" form:"enable"`
}

type NodeOutboundLatencyReq struct {
	InboundId int    `json:"inboundId" form:"inboundId"`
	Address   string `json:"address" form:"address"`
	Port      int    `json:"port" form:"port"`
}

type NodeOutboundLatencyResult struct {
	Address     string `json:"address"`
	ResolvedIP  string `json:"resolvedIp"`
	IPLatency   string `json:"ipLatency"`
	PortLatency string `json:"portLatency"`
	IPStatus    string `json:"ipStatus"`
	PortStatus  string `json:"portStatus"`
	IPError     string `json:"ipError"`
	PortError   string `json:"portError"`
}