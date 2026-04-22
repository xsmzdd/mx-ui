package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/web/entity"
	"x-ui/xray"
)

type InboundService struct {
}

func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) GetAllInbounds() ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) checkPortExist(port int, ignoreId int) (bool, error) {
	db := database.GetDB()
	db = db.Model(model.Inbound{}).Where("port = ?", port)
	if ignoreId > 0 {
		db = db.Where("id != ?", ignoreId)
	}
	var count int64
	err := db.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *InboundService) AddInbound(inbound *model.Inbound) error {
	exist, err := s.checkPortExist(inbound.Port, 0)
	if err != nil {
		return err
	}
	if exist {
		return common.NewError("端口已存在:", inbound.Port)
	}

	if inbound.Reset && inbound.ResetDay > 0 && inbound.LastResetTime <= 0 {
		inbound.LastResetTime = time.Now().Unix() * 1000
	}

	db := database.GetDB()
	return db.Save(inbound).Error
}

func (s *InboundService) AddInbounds(inbounds []*model.Inbound) error {
	for _, inbound := range inbounds {
		exist, err := s.checkPortExist(inbound.Port, 0)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("端口已存在:", inbound.Port)
		}
		if inbound.Reset && inbound.ResetDay > 0 && inbound.LastResetTime <= 0 {
			inbound.LastResetTime = time.Now().Unix() * 1000
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	var err error
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, inbound := range inbounds {
		err = tx.Save(inbound).Error
		if err != nil {
			return err
		}
	}
	return nil
}

type batchSocks5Item struct {
	Address  string
	Port     int
	Username string
	Password string
}

func (s *InboundService) AddBatchInbound(req *entity.BatchAddInboundReq, userId int) error {
	if req == nil {
		return common.NewError("批量添加参数不能为空")
	}
	if req.Port <= 0 || req.Port > 65535 {
		return common.NewError("端口不正确:", req.Port)
	}

	items, err := s.parseBatchSocks5Text(req.BatchSocks5Text)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return common.NewError("请至少输入一行 socks5 代理")
	}

	endPort := req.Port + len(items) - 1
	if endPort > 65535 {
		return common.NewError("批量添加后的端口超出范围:", endPort)
	}

	inbounds := make([]*model.Inbound, 0, len(items))
	outbounds := make([]*model.InboundOutbound, 0, len(items))
	for idx, item := range items {
		port := req.Port + idx
		exist, err := s.checkPortExist(port, 0)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("端口已存在:", port)
		}

		inbound := &model.Inbound{
			UserId:         userId,
			Up:             req.Up,
			Down:           req.Down,
			Total:          req.Total,
			Remark:         req.Remark,
			Enable:         req.Enable,
			ExpiryTime:     req.ExpiryTime,
			Reset:          req.Reset,
			ResetDay:       req.ResetDay,
			LastResetTime:  req.LastResetTime,
			Listen:         req.Listen,
			Port:           port,
			Protocol:       model.Protocol(req.Protocol),
			Settings:       req.Settings,
			StreamSettings: req.StreamSettings,
			Tag:            fmt.Sprintf("inbound-%v", port),
			Sniffing:       req.Sniffing,
		}
		if inbound.Reset && inbound.ResetDay > 0 && inbound.LastResetTime <= 0 {
			inbound.LastResetTime = time.Now().Unix() * 1000
		}

		outbound := &model.InboundOutbound{
			UserId:   userId,
			Enable:   true,
			Protocol: "socks5",
			Address:  item.Address,
			Port:     item.Port,
			Username: item.Username,
			Password: item.Password,
		}

		inbounds = append(inbounds, inbound)
		outbounds = append(outbounds, outbound)
	}

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	for idx, inbound := range inbounds {
		err = tx.Save(inbound).Error
		if err != nil {
			return err
		}
		outbounds[idx].InboundId = inbound.Id
		err = tx.Save(outbounds[idx]).Error
		if err != nil {
			return err
		}
	}

	return tx.Commit().Error
}

func (s *InboundService) parseBatchSocks5Text(text string) ([]batchSocks5Item, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	items := make([]batchSocks5Item, 0, len(lines))

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 4 {
			return nil, common.NewError("socks5 格式不正确:", line)
		}

		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || port <= 0 || port > 65535 {
			return nil, common.NewError("socks5 端口不正确:", line)
		}

		item := batchSocks5Item{
			Address:  strings.TrimSpace(parts[0]),
			Port:     port,
			Username: strings.TrimSpace(parts[2]),
			Password: strings.TrimSpace(parts[3]),
		}
		if item.Address == "" {
			return nil, common.NewError("socks5 地址不能为空:", line)
		}
		if !((item.Username == "" && item.Password == "") || (item.Username != "" && item.Password != "")) {
			return nil, common.NewError("socks5 用户名和密码必须同时填写或同时留空:", line)
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *InboundService) DelInbound(id int) error {
	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Where("inbound_id = ?", id).Delete(&model.InboundOutbound{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(model.Inbound{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *InboundService) GetInbound(id int) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}
	err := db.Model(model.Inbound{}).First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

func (s *InboundService) UpdateInbound(inbound *model.Inbound) error {
	exist, err := s.checkPortExist(inbound.Port, inbound.Id)
	if err != nil {
		return err
	}
	if exist {
		return common.NewError("端口已存在:", inbound.Port)
	}

	oldInbound, err := s.GetInbound(inbound.Id)
	if err != nil {
		return err
	}

	oldInbound.Up = inbound.Up
	oldInbound.Down = inbound.Down
	oldInbound.Total = inbound.Total
	oldInbound.Remark = inbound.Remark
	oldInbound.Enable = inbound.Enable
	oldInbound.ExpiryTime = inbound.ExpiryTime

	oldInbound.Reset = inbound.Reset
	oldInbound.ResetDay = inbound.ResetDay
	if inbound.Reset && inbound.ResetDay > 0 {
		if oldInbound.LastResetTime <= 0 {
			oldInbound.LastResetTime = time.Now().Unix() * 1000
		}
	} else {
		oldInbound.LastResetTime = 0
	}

	oldInbound.Listen = inbound.Listen
	oldInbound.Port = inbound.Port
	oldInbound.Protocol = inbound.Protocol
	oldInbound.Settings = inbound.Settings
	oldInbound.StreamSettings = inbound.StreamSettings
	oldInbound.Sniffing = inbound.Sniffing
	oldInbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)

	db := database.GetDB()
	return db.Save(oldInbound).Error
}

func (s *InboundService) AddTraffic(traffics []*xray.Traffic) (err error) {
	if len(traffics) == 0 {
		return nil
	}
	db := database.GetDB()
	db = db.Model(model.Inbound{})
	tx := db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	for _, traffic := range traffics {
		if traffic.IsInbound {
			err = tx.Where("tag = ?", traffic.Tag).
				UpdateColumn("up", gorm.Expr("up + ?", traffic.Up)).
				UpdateColumn("down", gorm.Expr("down + ?", traffic.Down)).
				Error
			if err != nil {
				return
			}
		}
	}
	return
}

func (s *InboundService) DisableInvalidInbounds() (int64, error) {
	db := database.GetDB()
	now := time.Now().Unix() * 1000
	result := db.Model(model.Inbound{}).
		Where("((total > 0 and up + down >= total) or (expiry_time > 0 and expiry_time <= ?)) and enable = ?", now, true).
		Update("enable", false)
	err := result.Error
	count := result.RowsAffected
	return count, err
}

func (s *InboundService) ResetDueInbounds() error {
	db := database.GetDB()

	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("reset = ? AND reset_day > 0", true).
		Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	now := time.Now().Unix() * 1000

	for _, inbound := range inbounds {
		if inbound.LastResetTime <= 0 {
			inbound.LastResetTime = now
			if err := db.Save(inbound).Error; err != nil {
				return err
			}
			continue
		}

		interval := int64(inbound.ResetDay) * 24 * 60 * 60 * 1000
		if interval <= 0 {
			continue
		}

		if now-inbound.LastResetTime >= interval {
			inbound.Up = 0
			inbound.Down = 0
			inbound.LastResetTime = now

			if err := db.Save(inbound).Error; err != nil {
				return err
			}
		}
	}

	return nil
}