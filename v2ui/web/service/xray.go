package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/atomic"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/json_util"
	"x-ui/xray"
)

var p *xray.Process
var lock sync.Mutex
var isNeedXrayRestart atomic.Bool
var result string

type XrayService struct {
	inboundService      InboundService
	nodeOutboundService NodeOutboundService
	settingService      SettingService
}

func (s *XrayService) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

func (s *XrayService) GetXrayErr() error {
	if p == nil {
		return nil
	}
	return p.GetErr()
}

func (s *XrayService) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}
	result = p.GetResult()
	return result
}

func (s *XrayService) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}

	enabledNodeOutbounds, err := s.nodeOutboundService.GetEnabledConfigs()
	if err != nil {
		return nil, err
	}
	logger.Info("xray enabled node outbounds count: ", len(enabledNodeOutbounds))

	if len(enabledNodeOutbounds) == 0 {
		xrayConfig.OutboundConfigs, err = s.ensureDirectOutbound(xrayConfig.OutboundConfigs)
		if err != nil {
			return nil, err
		}
		return xrayConfig, nil
	}

	inboundMap := map[int]*model.Inbound{}
	for _, inbound := range inbounds {
		inboundMap[inbound.Id] = inbound
	}

	xrayConfig.OutboundConfigs, err = s.mergeNodeProxyOutbounds(xrayConfig.OutboundConfigs, enabledNodeOutbounds)
	if err != nil {
		return nil, err
	}
	logger.Info("xray merged node proxy outbounds")

	xrayConfig.RouterConfig, err = s.mergeNodeProxyRouting(xrayConfig.RouterConfig, enabledNodeOutbounds, inboundMap)
	if err != nil {
		return nil, err
	}
	logger.Info("xray merged node proxy routing")

	return xrayConfig, nil
}

func (s *XrayService) ensureDirectOutbound(origin json_util.RawMessage) (json_util.RawMessage, error) {
	var outbounds []map[string]interface{}
	if len(origin) > 0 && string(origin) != "null" {
		if err := json.Unmarshal(origin, &outbounds); err != nil {
			return nil, err
		}
	}

	hasDirect := false
	for _, outbound := range outbounds {
		protocol, _ := outbound["protocol"].(string)
		tag, _ := outbound["tag"].(string)

		if protocol == "freedom" {
			if tag == "" {
				outbound["tag"] = "direct"
				tag = "direct"
			}
			if tag == "direct" {
				hasDirect = true
			}
		}
	}

	if !hasDirect {
		outbounds = append([]map[string]interface{}{
			{
				"protocol": "freedom",
				"settings": map[string]interface{}{},
				"tag":      "direct",
			},
		}, outbounds...)
	}

	b, err := json.Marshal(outbounds)
	if err != nil {
		return nil, err
	}
	return json_util.RawMessage(b), nil
}

func (s *XrayService) mergeNodeProxyOutbounds(origin json_util.RawMessage, configs []*model.InboundOutbound) (json_util.RawMessage, error) {
	var outbounds []map[string]interface{}
	if len(origin) > 0 && string(origin) != "null" {
		if err := json.Unmarshal(origin, &outbounds); err != nil {
			return nil, err
		}
	}

	filtered := make([]map[string]interface{}, 0, len(outbounds))
	hasDirect := false

	for _, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		protocol, _ := outbound["protocol"].(string)

		if strings.HasPrefix(tag, "node-proxy-") {
			continue
		}

		if protocol == "freedom" {
			if tag == "" {
				outbound["tag"] = "direct"
				tag = "direct"
			}
			if tag == "direct" {
				hasDirect = true
			}
		}

		filtered = append(filtered, outbound)
	}

	if !hasDirect {
		filtered = append([]map[string]interface{}{
			{
				"protocol": "freedom",
				"settings": map[string]interface{}{},
				"tag":      "direct",
			},
		}, filtered...)
	}

	for _, conf := range configs {
		filtered = append(filtered, s.buildNodeProxyOutbound(conf))
	}

	b, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	return json_util.RawMessage(b), nil
}

func (s *XrayService) mergeNodeProxyRouting(origin json_util.RawMessage, configs []*model.InboundOutbound, inboundMap map[int]*model.Inbound) (json_util.RawMessage, error) {
	routing := map[string]interface{}{}
	if len(origin) > 0 && string(origin) != "null" {
		if err := json.Unmarshal(origin, &routing); err != nil {
			return nil, err
		}
	}

	rawRules, ok := routing["rules"].([]interface{})
	if !ok {
		rawRules = []interface{}{}
	}

	filteredRules := make([]interface{}, 0, len(rawRules))
	for _, rule := range rawRules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		outboundTag, _ := ruleMap["outboundTag"].(string)
		if strings.HasPrefix(outboundTag, "node-proxy-") {
			continue
		}
		filteredRules = append(filteredRules, ruleMap)
	}

	for _, conf := range configs {
		inbound := inboundMap[conf.InboundId]
		if inbound == nil || !inbound.Enable {
			continue
		}
		if strings.TrimSpace(inbound.Tag) == "" {
			return nil, errors.New("inbound tag is empty")
		}

		udpRule, tcpRule := s.buildNodeProxyRoutingRules(inbound.Tag, conf.InboundId)
		filteredRules = append(filteredRules, udpRule, tcpRule)
	}

	routing["rules"] = filteredRules
	b, err := json.Marshal(routing)
	if err != nil {
		return nil, err
	}
	return json_util.RawMessage(b), nil
}

func (s *XrayService) buildNodeProxyOutbound(conf *model.InboundOutbound) map[string]interface{} {
	server := map[string]interface{}{
		"address": conf.Address,
		"port":    conf.Port,
	}

	if strings.TrimSpace(conf.Username) != "" && strings.TrimSpace(conf.Password) != "" {
		server["users"] = []map[string]interface{}{
			{
				"user":  conf.Username,
				"pass":  conf.Password,
				"level": 0,
			},
		}
	}

	return map[string]interface{}{
		"tag":      fmt.Sprintf("node-proxy-%d", conf.InboundId),
		"protocol": "socks",
		"settings": map[string]interface{}{
			"servers": []map[string]interface{}{server},
		},
	}
}

func (s *XrayService) buildNodeProxyRoutingRules(inboundTag string, inboundId int) (map[string]interface{}, map[string]interface{}) {
        udpRule := map[string]interface{}{
                "type":        "field",
                "inboundTag":  []string{inboundTag},
                "network":     "udp",
                "outboundTag": "direct",
        }

        tcpRule := map[string]interface{}{
                "type":        "field",
                "inboundTag":  []string{inboundTag},
                "network":     "tcp",
                "outboundTag": fmt.Sprintf("node-proxy-%d", inboundId),
        }

        return udpRule, tcpRule
}

func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, error) {
	if !s.IsXrayRunning() {
		return nil, errors.New("xray is not running")
	}
	return p.GetTraffic(true)
}

func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()

	logger.Debug("restart xray, force:", isForce)
	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if p != nil && p.IsRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) {
			logger.Debug("not need to restart xray")
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	return p.Start()
}

func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()

	logger.Debug("stop xray")
	if s.IsXrayRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CAS(true, false)
}
