package service

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/web/entity"
)

type NodeOutboundService struct {
}

func (s *NodeOutboundService) GetNodeOutboundList(userId int) ([]entity.NodeOutboundItem, error) {
	db := database.GetDB()

	var inbounds []*model.Inbound
	err := db.Model(&model.Inbound{}).
		Where("user_id = ?", userId).
		Order("id asc").
		Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	var configs []*model.InboundOutbound
	err = db.Model(&model.InboundOutbound{}).
		Where("user_id = ?", userId).
		Find(&configs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	confMap := map[int]*model.InboundOutbound{}
	for _, conf := range configs {
		confMap[conf.InboundId] = conf
	}

	res := make([]entity.NodeOutboundItem, 0, len(inbounds))
	for _, inbound := range inbounds {
		item := entity.NodeOutboundItem{
			InboundId:        inbound.Id,
			Remark:           inbound.Remark,
			Protocol:         string(inbound.Protocol),
			Port:             inbound.Port,
			Tag:              inbound.Tag,
			Up:               inbound.Up,
			Down:             inbound.Down,
			Total:            inbound.Total,
			ExpiryTime:       inbound.ExpiryTime,
			OutboundProtocol: "socks5",
			HasConfig:        false,
			CanEnable:        false,
		}

		if conf, ok := confMap[inbound.Id]; ok {
			item.OutboundId = conf.Id
			item.OutboundEnable = conf.Enable
			item.OutboundProtocol = conf.Protocol
			item.OutboundAddress = conf.Address
			item.OutboundPort = conf.Port
			item.OutboundUsername = conf.Username
			item.OutboundPassword = conf.Password
			item.HasConfig = s.CanEnable(conf)
			item.CanEnable = s.CanEnable(conf)
		}

		res = append(res, item)
	}

	return res, nil
}

func (s *NodeOutboundService) GetByInboundId(inboundId int) (*model.InboundOutbound, error) {
	db := database.GetDB()
	conf := &model.InboundOutbound{}
	err := db.Model(&model.InboundOutbound{}).
		Where("inbound_id = ?", inboundId).
		First(conf).Error
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (s *NodeOutboundService) Save(req *entity.SaveNodeOutboundReq, userId int) error {
	if req.InboundId <= 0 {
		return errors.New("入站节点不存在")
	}
	if req.Protocol == "" {
		req.Protocol = "socks5"
	}
	if strings.TrimSpace(req.Address) == "" {
		return errors.New("代理IP/地址不能为空")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return errors.New("代理端口不正确")
	}
	if !s.isAuthPairValid(req.Username, req.Password) {
		return errors.New("用户名和密码必须同时填写或同时留空")
	}
	if req.Enable && !s.isReqValid(req) {
		return errors.New("请先填写完整的 socks5 信息")
	}

	db := database.GetDB()

	inbound := &model.Inbound{}
	err := db.Model(&model.Inbound{}).
		Where("id = ? and user_id = ?", req.InboundId, userId).
		First(inbound).Error
	if err != nil {
		return errors.New("入站节点不存在")
	}

	conf := &model.InboundOutbound{}
	err = db.Model(&model.InboundOutbound{}).
		Where("inbound_id = ?", req.InboundId).
		First(conf).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if err == gorm.ErrRecordNotFound {
		conf = &model.InboundOutbound{
			UserId:    userId,
			InboundId: req.InboundId,
		}
	}

	conf.Protocol = "socks5"
	conf.Address = strings.TrimSpace(req.Address)
	conf.Port = req.Port
	conf.Username = strings.TrimSpace(req.Username)
	conf.Password = strings.TrimSpace(req.Password)

	if s.isReqValid(req) {
		conf.Enable = req.Enable
	} else {
		conf.Enable = false
	}

	return db.Save(conf).Error
}

func (s *NodeOutboundService) Toggle(inboundId int, enable bool, userId int) error {
	db := database.GetDB()

	inbound := &model.Inbound{}
	err := db.Model(&model.Inbound{}).
		Where("id = ? and user_id = ?", inboundId, userId).
		First(inbound).Error
	if err != nil {
		return errors.New("入站节点不存在")
	}

	conf := &model.InboundOutbound{}
	err = db.Model(&model.InboundOutbound{}).
		Where("inbound_id = ?", inboundId).
		First(conf).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("请先配置代理信息")
		}
		return err
	}

	if enable && !s.CanEnable(conf) {
		return errors.New("请先填写完整的 socks5 信息")
	}

	conf.Enable = enable
	return db.Save(conf).Error
}

func (s *NodeOutboundService) DeleteByInboundId(inboundId int, userId int) error {
	db := database.GetDB()

	inbound := &model.Inbound{}
	err := db.Model(&model.Inbound{}).
		Where("id = ? and user_id = ?", inboundId, userId).
		First(inbound).Error
	if err != nil {
		return errors.New("入站节点不存在")
	}

	return db.Where("inbound_id = ? and user_id = ?", inboundId, userId).
		Delete(&model.InboundOutbound{}).Error
}

func (s *NodeOutboundService) GetEnabledConfigs() ([]*model.InboundOutbound, error) {
	db := database.GetDB()

	var configs []*model.InboundOutbound
	err := db.Model(&model.InboundOutbound{}).
		Where("enable = 1").
		Order("inbound_id asc").
		Find(&configs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	logger.Info("node outbound raw enabled configs count: ", len(configs))

	res := make([]*model.InboundOutbound, 0, len(configs))
	for _, conf := range configs {
		if conf == nil {
			logger.Warning("node outbound config is nil")
			continue
		}

		logger.Info(
			"node outbound raw config: id=", conf.Id,
			", inboundId=", conf.InboundId,
			", enable=", conf.Enable,
			", address=", conf.Address,
			", port=", conf.Port,
			", username_len=", len(strings.TrimSpace(conf.Username)),
			", password_len=", len(strings.TrimSpace(conf.Password)),
		)

		if s.CanEnable(conf) {
			res = append(res, conf)
		} else {
			logger.Warning("node outbound config filtered by CanEnable: id=", conf.Id, ", inboundId=", conf.InboundId)
		}
	}

	logger.Info("node outbound valid enabled configs count: ", len(res))
	return res, nil
}

func (s *NodeOutboundService) CanEnable(conf *model.InboundOutbound) bool {
	if conf == nil {
		return false
	}
	if strings.TrimSpace(conf.Address) == "" {
		return false
	}
	if conf.Port <= 0 || conf.Port > 65535 {
		return false
	}
	if !s.isAuthPairValid(conf.Username, conf.Password) {
		return false
	}
	return true
}

func (s *NodeOutboundService) isReqValid(req *entity.SaveNodeOutboundReq) bool {
	if req == nil {
		return false
	}
	if strings.TrimSpace(req.Address) == "" {
		return false
	}
	if req.Port <= 0 || req.Port > 65535 {
		return false
	}
	if !s.isAuthPairValid(req.Username, req.Password) {
		return false
	}
	return true
}

func (s *NodeOutboundService) isAuthPairValid(username, password string) bool {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" && password == "" {
		return true
	}
	if username != "" && password != "" {
		return true
	}
	return false
}

func (s *NodeOutboundService) CheckLatency(req *entity.NodeOutboundLatencyReq, userId int) (*entity.NodeOutboundLatencyResult, error) {
	if req == nil || req.InboundId <= 0 {
		return nil, errors.New("入站节点不存在")
	}
	if strings.TrimSpace(req.Address) == "" {
		return nil, errors.New("代理IP/地址不能为空")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return nil, errors.New("代理端口不正确")
	}

	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(&model.Inbound{}).
		Where("id = ? and user_id = ?", req.InboundId, userId).
		First(inbound).Error
	if err != nil {
		return nil, errors.New("入站节点不存在")
	}

	address := strings.TrimSpace(req.Address)
	result := &entity.NodeOutboundLatencyResult{Address: address}

	resolvedIP := address
	if ip := net.ParseIP(address); ip == nil {
		ips, err := net.LookupIP(address)
		if err != nil || len(ips) == 0 {
			result.IPStatus = "失败"
			result.PortStatus = "失败"
			result.IPLatency = "检测失败"
			result.PortLatency = "检测失败"
			msg := "域名解析失败"
			if err != nil {
				msg = err.Error()
			}
			result.IPError = msg
			result.PortError = msg
			return result, nil
		}
		resolvedIP = ips[0].String()
	}
	result.ResolvedIP = resolvedIP

	ipLatency, ipErr := s.measurePingLatency(resolvedIP)
	if ipErr != nil {
		result.IPStatus = "失败"
		result.IPLatency = "检测失败"
		result.IPError = ipErr.Error()
	} else {
		result.IPStatus = "成功"
		result.IPLatency = ipLatency
	}

	portLatency, portErr := s.measureTCPLatency(resolvedIP, req.Port)
	if portErr != nil {
		result.PortStatus = "失败"
		result.PortLatency = "检测失败"
		result.PortError = portErr.Error()
	} else {
		result.PortStatus = "成功"
		result.PortLatency = portLatency
	}

	return result, nil
}

func (s *NodeOutboundService) measurePingLatency(ip string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1500", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "2", ip)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ping失败: %s", strings.TrimSpace(string(output)))
	}
	text := string(output)
	re := regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", errors.New("无法解析 ping 延迟")
	}
	return matches[1] + " ms", nil
}

func (s *NodeOutboundService) measureTCPLatency(ip string, port int) (string, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 2*time.Second)
	if err != nil {
		return "", err
	}
	_ = conn.Close()
	return fmt.Sprintf("%.2f ms", float64(time.Since(start).Microseconds())/1000), nil
}