package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// PaymentService 负责收款登记，并同步更新 PI 的已收款金额。
type PaymentService struct {
	Repo     *repository.PaymentRepository
	PIRepo   *repository.ProformaInvoiceRepository
	Settings *SettingsService
}

func NewPaymentService(
	repo *repository.PaymentRepository,
	piRepo *repository.ProformaInvoiceRepository,
	settings *SettingsService,
) *PaymentService {
	return &PaymentService{Repo: repo, PIRepo: piRepo, Settings: settings}
}

func (s *PaymentService) List(
	ctx context.Context,
	customerName, keyword string,
) ([]models.Payment, error) {
	return s.Repo.List(ctx, customerName, keyword)
}

func (s *PaymentService) Get(
	ctx context.Context,
	id uint64,
) (*models.Payment, error) {
	return s.Repo.GetByID(ctx, id)
}

func (s *PaymentService) Customers(ctx context.Context) ([]string, error) {
	return s.Repo.DistinctCustomers(ctx)
}

func (s *PaymentService) Create(
	ctx context.Context,
	p *models.Payment,
) error {
	if err := s.prepare(ctx, p); err != nil {
		return err
	}
	if err := s.Repo.Create(ctx, p); err != nil {
		return err
	}
	return s.syncPI(ctx, p.PIID)
}

func (s *PaymentService) Update(
	ctx context.Context,
	p *models.Payment,
) error {
	if p.ID == 0 {
		return fmt.Errorf("收款记录 ID 无效")
	}

	// 记录修改前的归属 PI，便于两个 PI 都重算。
	existing, err := s.Repo.GetByID(ctx, p.ID)
	if err != nil {
		return err
	}

	if err := s.prepare(ctx, p); err != nil {
		return err
	}
	if err := s.Repo.Update(ctx, p); err != nil {
		return err
	}

	if err := s.syncPI(ctx, existing.PIID); err != nil {
		return err
	}
	return s.syncPI(ctx, p.PIID)
}

func (s *PaymentService) Delete(ctx context.Context, id uint64) error {
	if id == 0 {
		return fmt.Errorf("收款记录 ID 无效")
	}

	existing, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	return s.syncPI(ctx, existing.PIID)
}

func (s *PaymentService) prepare(ctx context.Context, p *models.Payment) error {
	p.CustomerName = strings.TrimSpace(p.CustomerName)
	if p.CustomerName == "" {
		return fmt.Errorf("请填写客户名称")
	}
	if p.Amount <= 0 {
		return fmt.Errorf("收款金额必须大于0")
	}
	if p.Currency == "" {
		if s.Settings != nil {
			p.Currency = s.Settings.DefaultCurrency(ctx)
		} else {
			p.Currency = "USD"
		}
	}
	if p.PaidDate.Time.IsZero() {
		p.PaidDate = models.Date{Time: time.Now()}
	}
	return nil
}

// syncPI 重算某 PI 的已收款金额，并同步其累计未收。
func (s *PaymentService) syncPI(ctx context.Context, piID *uint64) error {
	if piID == nil || s.PIRepo == nil {
		return nil
	}
	paid, err := s.Repo.SumByPI(ctx, *piID)
	if err != nil {
		return err
	}
	return s.PIRepo.SetPaidAmount(ctx, *piID, round2(paid))
}
