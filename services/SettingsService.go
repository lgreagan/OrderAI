package services

import (
	"context"
	"strconv"
	"strings"

	"OrderAI/models"
	"OrderAI/repository"
)

// 系统设置键名。
const (
	SettingDefaultCurrency   = "default_currency"
	SettingDefaultWarehouse  = "default_warehouse"
	SettingPITerms           = "pi_terms"
	SettingPIValidDays       = "pi_valid_days"
	SettingStockNoteHeader   = "stock_note_header"
	SettingLowStockThreshold = "low_stock_threshold"
	SettingExchangeRate      = "exchange_rate"

	// 截图 / 打印时的中英文对照字典，值是一行一条的「英文=中文」文本。
	SettingI18NCountry  = "i18n_country"
	SettingI18NShipping = "i18n_shipping"
)

// DefaultExchangeRate 未维护月度汇率时使用的默认汇率。
const DefaultExchangeRate = 7.1

// monthlyExchangeRatePrefix 是月度汇率设置键前缀，完整键形如 exchange_rate_2026-09。
const monthlyExchangeRatePrefix = SettingExchangeRate + "_"

// MonthExchangeRateKey 返回某个月（yyyy-MM）的汇率设置键。
func MonthExchangeRateKey(month string) string {
	return monthlyExchangeRatePrefix + month
}

// ParseMonthExchangeRateKey 解析月度汇率设置键，非月度汇率返回 false。
func ParseMonthExchangeRateKey(key string) (string, bool) {
	if !strings.HasPrefix(key, monthlyExchangeRatePrefix) {
		return "", false
	}

	month := strings.TrimPrefix(key, monthlyExchangeRatePrefix)
	if len(month) != 7 || month[4] != '-' {
		return "", false
	}

	return month, true
}

// SettingsService 读写系统设置，并提供带默认值的取值方法。
type SettingsService struct {
	Repo *repository.AppSettingRepository
}

func NewSettingsService(repo *repository.AppSettingRepository) *SettingsService {
	return &SettingsService{Repo: repo}
}

func (s *SettingsService) All(ctx context.Context) ([]models.AppSetting, error) {
	return s.Repo.All(ctx)
}

// Get 读取设置值（不存在返回空串）。
func (s *SettingsService) Get(ctx context.Context, key string) string {
	value, err := s.Repo.Get(ctx, key)
	if err != nil {
		return ""
	}
	return value
}

// Save 批量保存设置。
func (s *SettingsService) Save(
	ctx context.Context,
	settings map[string]string,
) error {
	for key, value := range settings {
		if err := s.Repo.Upsert(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// DefaultCurrency 默认币种。
func (s *SettingsService) DefaultCurrency(ctx context.Context) string {
	return firstNonEmpty(strings.TrimSpace(s.Get(ctx, SettingDefaultCurrency)), "USD")
}

// DefaultWarehouse 默认仓库。
func (s *SettingsService) DefaultWarehouse(ctx context.Context) string {
	return firstNonEmpty(strings.TrimSpace(s.Get(ctx, SettingDefaultWarehouse)), "default")
}

// PITerms PI 付款条款。
func (s *SettingsService) PITerms(ctx context.Context) string {
	return s.Get(ctx, SettingPITerms)
}

// StockNoteHeader 库存说明默认抬头。
func (s *SettingsService) StockNoteHeader(ctx context.Context) string {
	return s.Get(ctx, SettingStockNoteHeader)
}

// I18NDictionary 返回截图 / 打印使用的中英文对照字典原文。
//
// 空字符串表示还没维护，由前端内置字典兜底。
func (s *SettingsService) I18NDictionary(ctx context.Context) (country, shipping string) {
	return s.Get(ctx, SettingI18NCountry), s.Get(ctx, SettingI18NShipping)
}

// LowStockThreshold 低库存阈值。
func (s *SettingsService) LowStockThreshold(ctx context.Context) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(s.Get(ctx, SettingLowStockThreshold)), 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// PIValidDays PI 有效期天数。
func (s *SettingsService) PIValidDays(ctx context.Context) int {
	value, err := strconv.Atoi(strings.TrimSpace(s.Get(ctx, SettingPIValidDays)))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

// ExchangeRate 默认人民币汇率（1 外币 = N CNY）。
//
// 用于还没维护月度汇率的月份：财务报表按 PI 签发月份取月度汇率，
// 取不到时回落到该默认值。
func (s *SettingsService) ExchangeRate(ctx context.Context) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(s.Get(ctx, SettingExchangeRate)), 64)
	if err != nil || value <= 0 {
		return DefaultExchangeRate
	}
	return value
}

// MonthlyExchangeRates 返回已维护的月度汇率（yyyy-MM -> 汇率）。
//
// 键为 exchange_rate_yyyy-MM，因此不需要额外的表结构即可按月存档。
func (s *SettingsService) MonthlyExchangeRates(
	ctx context.Context,
) (map[string]float64, error) {
	items, err := s.Repo.All(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]float64{}
	for _, item := range items {
		month, ok := ParseMonthExchangeRateKey(item.Key)
		if !ok {
			continue
		}

		value, err := strconv.ParseFloat(strings.TrimSpace(item.Value), 64)
		if err != nil || value <= 0 {
			continue
		}
		result[month] = value
	}

	return result, nil
}

// Delete 删除一个设置项。
func (s *SettingsService) Delete(ctx context.Context, key string) error {
	return s.Repo.Delete(ctx, strings.TrimSpace(key))
}
