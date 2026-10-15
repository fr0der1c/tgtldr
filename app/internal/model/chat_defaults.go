package model

import (
	"fmt"
	"strings"
	"time"
)

// ChatDefaults 是新同步群组的初始配置，也可一键覆盖到全部已有群组。
type ChatDefaults struct {
	DeliveryMode     DeliveryMode `json:"deliveryMode"`
	SummaryTimeLocal string       `json:"summaryTimeLocal"`
	ModelOverride    string       `json:"modelOverride"`
	KeepBotMessages  bool         `json:"keepBotMessages"`
}

// DefaultChatDefaults 与引入该配置前新群组的写死默认值保持一致。
func DefaultChatDefaults() ChatDefaults {
	return ChatDefaults{
		DeliveryMode:     DeliveryModeDashboard,
		SummaryTimeLocal: "09:00",
		KeepBotMessages:  true,
	}
}

// Normalize 去除首尾空白并校验取值，返回可直接落库的配置。
func (d ChatDefaults) Normalize() (ChatDefaults, error) {
	d.SummaryTimeLocal = strings.TrimSpace(d.SummaryTimeLocal)
	d.ModelOverride = strings.TrimSpace(d.ModelOverride)
	if d.DeliveryMode != DeliveryModeDashboard && d.DeliveryMode != DeliveryModeBot {
		return ChatDefaults{}, fmt.Errorf("invalid delivery mode %q", d.DeliveryMode)
	}
	parsed, err := time.Parse("15:04", d.SummaryTimeLocal)
	if err != nil {
		return ChatDefaults{}, fmt.Errorf("invalid summary time %q", d.SummaryTimeLocal)
	}
	d.SummaryTimeLocal = parsed.Format("15:04")
	return d, nil
}
