package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type InvoiceSetting struct {
	Enabled        bool    `json:"enabled"`         // 是否启用发票功能
	MinAmount      float64 `json:"min_amount"`      // 最低开票金额（元）
	DefaultContent string  `json:"default_content"` // 默认发票内容
	MaxHeaders     int     `json:"max_headers"`     // 每用户最多保存几个抬头模板
}

// 默认配置
var invoiceSetting = InvoiceSetting{
	Enabled:        false,
	MinAmount:      500,
	DefaultContent: "*信息技术服务*技术服务费",
	MaxHeaders:     5,
}

func init() {
	config.GlobalConfig.Register("invoice_setting", &invoiceSetting)
}

func GetInvoiceSetting() *InvoiceSetting {
	return &invoiceSetting
}
