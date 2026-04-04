package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	InvoiceStatusPending  = 1 // 待审核
	InvoiceStatusApproved = 2 // 已通过
	InvoiceStatusRejected = 3 // 已拒绝
	InvoiceStatusCanceled = 4 // 已撤销
	InvoiceStatusSent     = 5 // 已发送
)

// InvoiceActiveStatuses 这些状态下关联的 TopUp 视为"被占用"
var InvoiceActiveStatuses = []int{InvoiceStatusPending, InvoiceStatusApproved, InvoiceStatusSent}

// ---------------------------------------------------------------------------
// Invoice — 发票申请
// ---------------------------------------------------------------------------

type Invoice struct {
	Id             int     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId         int     `json:"user_id" gorm:"index"`
	Email          string  `json:"email" gorm:"type:varchar(100)"`
	CompanyName    string  `json:"company_name" gorm:"type:varchar(255)"`
	TaxNumber      string  `json:"tax_number" gorm:"type:varchar(50)"`
	Amount         float64 `json:"amount"`
	Content        string  `json:"content" gorm:"type:varchar(255)"`
	Remark         string  `json:"remark" gorm:"type:varchar(255)"`
	Status         int     `json:"status" gorm:"default:1;index"`
	RejectReason   string  `json:"reject_reason" gorm:"type:varchar(255)"`
	InvoiceFileUrl string  `json:"invoice_file_url" gorm:"type:varchar(500)"`
	AdminId        int     `json:"admin_id"`
	CreateTime     int64   `json:"create_time"`
	UpdateTime     int64   `json:"update_time"`
}

// ---------------------------------------------------------------------------
// InvoiceItem — 发票与充值记录的关联表（替代改 TopUp 结构体）
// ---------------------------------------------------------------------------

type InvoiceItem struct {
	Id        int     `json:"id" gorm:"primaryKey;autoIncrement"`
	InvoiceId int     `json:"invoice_id" gorm:"index"`
	TopUpId   int     `json:"topup_id" gorm:"uniqueIndex:idx_active_topup"`
	Amount    float64 `json:"amount"` // 冗余存该笔 TopUp 的 money，用于审计
}

// ---------------------------------------------------------------------------
// InvoiceHeader — 用户的发票抬头模板
// ---------------------------------------------------------------------------

type InvoiceHeader struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId      int    `json:"user_id" gorm:"index"`
	CompanyName string `json:"company_name" gorm:"type:varchar(255)"`
	TaxNumber   string `json:"tax_number" gorm:"type:varchar(50)"`
	IsDefault   bool   `json:"is_default" gorm:"default:false"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
}

// ===========================================================================
// Invoice CRUD
// ===========================================================================

func CreateInvoice(tx *gorm.DB, invoice *Invoice) error {
	invoice.CreateTime = common.GetTimestamp()
	invoice.UpdateTime = invoice.CreateTime
	return tx.Create(invoice).Error
}

func GetInvoiceById(tx *gorm.DB, id int) (*Invoice, error) {
	var invoice Invoice
	err := tx.Where("id = ?", id).First(&invoice).Error
	return &invoice, err
}

func GetInvoiceByIdAndUserId(tx *gorm.DB, id int, userId int) (*Invoice, error) {
	var invoice Invoice
	err := tx.Where("id = ? AND user_id = ?", id, userId).First(&invoice).Error
	return &invoice, err
}

func GetUserInvoices(userId int, pageInfo *common.PageInfo) ([]*Invoice, int64, error) {
	var invoices []*Invoice
	var total int64
	query := DB.Model(&Invoice{}).Where("user_id = ?", userId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&invoices).Error
	return invoices, total, err
}

func GetAllInvoices(pageInfo *common.PageInfo, status int, keyword string) ([]*Invoice, int64, error) {
	var invoices []*Invoice
	var total int64
	query := DB.Model(&Invoice{})
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		// 转义 SQL 通配符，防止搜索攻击
		keyword = strings.ReplaceAll(keyword, "%", "\\%")
		keyword = strings.ReplaceAll(keyword, "_", "\\_")
		like := "%" + keyword + "%"
		query = query.Where("company_name LIKE ? ESCAPE '\\' OR tax_number LIKE ? ESCAPE '\\'", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&invoices).Error
	return invoices, total, err
}

func UpdateInvoiceStatus(tx *gorm.DB, id int, status int, updates map[string]interface{}) error {
	updates["status"] = status
	updates["update_time"] = common.GetTimestamp()
	return tx.Model(&Invoice{}).Where("id = ?", id).Updates(updates).Error
}

// ===========================================================================
// InvoiceItem — 关联表操作
// ===========================================================================

func BatchCreateInvoiceItems(tx *gorm.DB, items []*InvoiceItem) error {
	return tx.Create(&items).Error
}

func DeleteInvoiceItemsByInvoiceId(tx *gorm.DB, invoiceId int) error {
	return tx.Where("invoice_id = ?", invoiceId).Delete(&InvoiceItem{}).Error
}

func GetInvoiceItemsByInvoiceId(invoiceId int) ([]*InvoiceItem, error) {
	var items []*InvoiceItem
	err := DB.Where("invoice_id = ?", invoiceId).Find(&items).Error
	return items, err
}

// GetInvoicedTopUpIds 查询哪些 TopUpId 已被活跃发票占用（待审核或已通过）
func GetInvoicedTopUpIds(tx *gorm.DB, topUpIds []int) ([]int, error) {
	var usedIds []int
	err := tx.Model(&InvoiceItem{}).
		Joins("JOIN invoices ON invoice_items.invoice_id = invoices.id").
		Where("invoice_items.top_up_id IN ? AND invoices.status IN ?", topUpIds, InvoiceActiveStatuses).
		Pluck("invoice_items.top_up_id", &usedIds).Error
	return usedIds, err
}

// GetUserInvoicedTopUpIds 查询某用户所有已被活跃发票占用的 TopUp ID（无需传 ID 列表）
func GetUserInvoicedTopUpIds(userId int) ([]int, error) {
	var usedIds []int
	err := DB.Model(&InvoiceItem{}).
		Joins("JOIN invoices ON invoice_items.invoice_id = invoices.id").
		Joins("JOIN top_ups ON invoice_items.top_up_id = top_ups.id").
		Where("top_ups.user_id = ? AND invoices.status IN ?", userId, InvoiceActiveStatuses).
		Pluck("invoice_items.top_up_id", &usedIds).Error
	return usedIds, err
}

// ===========================================================================
// TopUp 只读查询（不写 topup 表，不改 topup.go）
// ===========================================================================

// GetTopUpsByIdsForUpdate 带行锁查充值记录（SQLite 不支持 FOR UPDATE，退化为普通查询）
func GetTopUpsByIdsForUpdate(tx *gorm.DB, ids []int, userId int) ([]*TopUp, error) {
	var topups []*TopUp
	query := tx.Where("id IN ? AND user_id = ?", ids, userId)
	if !common.UsingSQLite {
		query = query.Set("gorm:query_option", "FOR UPDATE")
	}
	err := query.Find(&topups).Error
	return topups, err
}

// GetAvailableTopUpsForInvoice 获取用户可开票的充值记录（状态 success 且未被占用）
func GetAvailableTopUpsForInvoice(userId int, pageInfo *common.PageInfo) ([]*TopUp, int64, error) {
	var topups []*TopUp
	var total int64

	// 子查询：找出已被活跃发票占用的 topup_id
	subQuery := DB.Model(&InvoiceItem{}).
		Select("invoice_items.top_up_id").
		Joins("JOIN invoices ON invoice_items.invoice_id = invoices.id").
		Where("invoices.status IN ?", InvoiceActiveStatuses)

	query := DB.Model(&TopUp{}).
		Where("user_id = ? AND status = ?", userId, common.TopUpStatusSuccess).
		Where("id NOT IN (?)", subQuery)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&topups).Error
	return topups, total, err
}

// ===========================================================================
// InvoiceHeader CRUD
// ===========================================================================

func GetUserInvoiceHeaders(userId int) ([]*InvoiceHeader, error) {
	var headers []*InvoiceHeader
	err := DB.Where("user_id = ?", userId).Order("is_default desc, id desc").Find(&headers).Error
	return headers, err
}

func CountUserInvoiceHeaders(userId int) (int64, error) {
	var count int64
	err := DB.Model(&InvoiceHeader{}).Where("user_id = ?", userId).Count(&count).Error
	return count, err
}

func CreateInvoiceHeader(header *InvoiceHeader) error {
	header.CreateTime = common.GetTimestamp()
	header.UpdateTime = header.CreateTime
	return DB.Create(header).Error
}

// CreateInvoiceHeaderTx 事务版本
func CreateInvoiceHeaderTx(tx *gorm.DB, header *InvoiceHeader) error {
	header.CreateTime = common.GetTimestamp()
	header.UpdateTime = header.CreateTime
	return tx.Create(header).Error
}

func UpdateInvoiceHeader(userId, headerId int, updates map[string]interface{}) error {
	updates["update_time"] = common.GetTimestamp()
	return DB.Model(&InvoiceHeader{}).
		Where("id = ? AND user_id = ?", headerId, userId).
		Updates(updates).Error
}

// UpdateInvoiceHeaderTx 事务版本
func UpdateInvoiceHeaderTx(tx *gorm.DB, userId, headerId int, updates map[string]interface{}) error {
	updates["update_time"] = common.GetTimestamp()
	return tx.Model(&InvoiceHeader{}).
		Where("id = ? AND user_id = ?", headerId, userId).
		Updates(updates).Error
}

func DeleteInvoiceHeader(userId, headerId int) error {
	return DB.Where("id = ? AND user_id = ?", headerId, userId).Delete(&InvoiceHeader{}).Error
}

// ClearDefaultHeaders 清除用户所有默认标记（设新默认前调用）
func ClearDefaultHeaders(tx *gorm.DB, userId int) error {
	return tx.Model(&InvoiceHeader{}).
		Where("user_id = ? AND is_default = ?", userId, true).
		Update("is_default", false).Error
}

// ===========================================================================
// TopUp 删除（仅 pending 状态，不碰上游 topup.go）
// ===========================================================================

// DeletePendingTopUp 删除用户自己的 pending 充值记录
func DeletePendingTopUp(userId, topUpId int) error {
	result := DB.Where("id = ? AND user_id = ? AND status = ?", topUpId, userId, common.TopUpStatusPending).Delete(&TopUp{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
