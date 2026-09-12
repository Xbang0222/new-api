package dto

type UpstreamDTO struct {
	ID       int    `json:"id,omitempty"`
	Name     string `json:"name" binding:"required"`
	BaseURL  string `json:"base_url" binding:"required"`
	Endpoint string `json:"endpoint"`
}

type UpstreamRequest struct {
	ChannelIDs  []int64       `json:"channel_ids"`
	Upstreams   []UpstreamDTO `json:"upstreams"`
	Timeout     int           `json:"timeout"`
	CatalogOnly bool          `json:"catalog_only"`
}

// TestResult 上游测试连通性结果
type TestResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// DifferenceItem 差异项
// Current 为本地值，可能为 nil
// Upstreams 为各渠道的上游值，具体数值 / "same" / nil

type DifferenceItem struct {
	Current    any             `json:"current"`
	Upstreams  map[string]any  `json:"upstreams"`
	Confidence map[string]bool `json:"confidence"`
}

type UpstreamPricingTier struct {
	Label           string   `json:"label"`
	Condition       string   `json:"condition,omitempty"`
	InputPrice      *float64 `json:"input_price,omitempty"`
	OutputPrice     *float64 `json:"output_price,omitempty"`
	CacheReadPrice  *float64 `json:"cache_read_price,omitempty"`
	CacheWritePrice *float64 `json:"cache_write_price,omitempty"`
}

type UpstreamPricingItem struct {
	Key              string                 `json:"key"`
	SourceName       string                 `json:"source_name"`
	ModelName        string                 `json:"model_name"`
	ModelID          string                 `json:"model_id,omitempty"`
	ProviderName     string                 `json:"provider_name,omitempty"`
	ProviderID       string                 `json:"provider_id,omitempty"`
	Type             string                 `json:"type,omitempty"`
	InputPrice       *float64               `json:"input_price,omitempty"`
	OutputPrice      *float64               `json:"output_price,omitempty"`
	CacheReadPrice   *float64               `json:"cache_read_price,omitempty"`
	CacheWritePrice  *float64               `json:"cache_write_price,omitempty"`
	Context          int64                  `json:"context,omitempty"`
	InputLimit       int64                  `json:"input_limit,omitempty"`
	OutputLimit      int64                  `json:"output_limit,omitempty"`
	InputModalities  []string               `json:"input_modalities,omitempty"`
	OutputModalities []string               `json:"output_modalities,omitempty"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Knowledge        string                 `json:"knowledge,omitempty"`
	ReleaseDate      string                 `json:"release_date,omitempty"`
	LastUpdated      string                 `json:"last_updated,omitempty"`
	Description      string                 `json:"description,omitempty"`
	BillingMode      string                 `json:"billing_mode,omitempty"`
	BillingExpr      string                 `json:"billing_expr,omitempty"`
	SyncValues       map[string]interface{} `json:"sync_values"`
	Tiers            []UpstreamPricingTier  `json:"tiers,omitempty"`
}

type SyncableChannel struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	Status  int    `json:"status"`
	Type    int    `json:"type"`
}
