package model

import "gorm.io/gorm"

type TokenRankingQuery struct {
	StartTime int64
	EndTime   int64
	UserID    int
	Username  string
	ModelName string
	Limit     int
}

type TokenRankingItem struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	TokenUsed int64  `json:"token_used"`
}

func baseTokenRankingQuery(query TokenRankingQuery) *gorm.DB {
	db := DB.Table("quota_data").
		Select("user_id, username, sum(token_used) as token_used")
	if query.StartTime > 0 {
		db = db.Where("created_at >= ?", quotaDataBucketStart(query.StartTime))
	}
	if query.EndTime > 0 {
		db = db.Where("created_at <= ?", query.EndTime)
	}
	if query.Username != "" {
		db = db.Where("username = ?", query.Username)
	}
	if query.ModelName != "" {
		db = db.Where("model_name = ?", query.ModelName)
	}
	return db.Group("user_id, username").Having("sum(token_used) > 0")
}

func GetTokenRanking(query TokenRankingQuery) ([]TokenRankingItem, int64, error) {
	var items []TokenRankingItem
	rankingQuery := baseTokenRankingQuery(query).Order("token_used DESC")
	if query.Limit > 0 {
		rankingQuery = rankingQuery.Limit(query.Limit)
	}
	if err := rankingQuery.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := DB.Table("(?) as token_rankings", baseTokenRankingQuery(query)).
		Count(&total)
	if err := countQuery.Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func GetUserTokenRanking(query TokenRankingQuery) (rank int, tokenUsed int64, err error) {
	if query.UserID <= 0 {
		return 0, 0, nil
	}

	type tokenTotalResult struct {
		TokenUsed int64
	}
	var total tokenTotalResult
	userQuery := query
	userQuery.Username = ""
	userQuery.Limit = 0
	db := DB.Table("quota_data").
		Select("coalesce(sum(token_used), 0) as token_used").
		Where("user_id = ?", userQuery.UserID)
	if userQuery.StartTime > 0 {
		db = db.Where("created_at >= ?", quotaDataBucketStart(userQuery.StartTime))
	}
	if userQuery.EndTime > 0 {
		db = db.Where("created_at <= ?", userQuery.EndTime)
	}
	if userQuery.ModelName != "" {
		db = db.Where("model_name = ?", userQuery.ModelName)
	}
	if err = db.Scan(&total).Error; err != nil {
		return 0, 0, err
	}
	tokenUsed = total.TokenUsed
	if tokenUsed <= 0 {
		return 0, 0, nil
	}

	type countResult struct {
		Count int64
	}
	var higher countResult
	rankingQuery := query
	rankingQuery.Username = ""
	rankingQuery.Limit = 0
	subQuery := baseTokenRankingQuery(rankingQuery)
	if err = DB.Table("(?) as token_rankings", subQuery).
		Select("count(*) as count").
		Where("token_used > ?", tokenUsed).
		Scan(&higher).Error; err != nil {
		return 0, 0, err
	}

	return int(higher.Count) + 1, tokenUsed, nil
}
