package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"OrderAI/models"
	"OrderAI/repository"
)

// 日志写入队列长度；队列满时丢弃日志并打印告警，不阻塞业务请求。
const operationLogQueueSize = 512

// 导出操作日志的最大条数。
const operationLogExportLimit = 20000

// OperationLogService 关键操作日志：异步落库 + 查询 + 清理。
type OperationLogService struct {
	Repo  *repository.OperationLogRepository
	queue chan *models.OperationLog
	done  chan struct{}
	once  sync.Once
}

func NewOperationLogService(repo *repository.OperationLogRepository) *OperationLogService {
	s := &OperationLogService{
		Repo:  repo,
		queue: make(chan *models.OperationLog, operationLogQueueSize),
		done:  make(chan struct{}),
	}
	go s.worker()
	return s
}

// worker 串行消费队列，避免每次请求都同步等待数据库。
func (s *OperationLogService) worker() {
	for {
		select {
		case item := <-s.queue:
			s.write(item)
		case <-s.done:
			for {
				select {
				case item := <-s.queue:
					s.write(item)
				default:
					return
				}
			}
		}
	}
}

func (s *OperationLogService) write(item *models.OperationLog) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Repo.Create(ctx, item); err != nil {
		log.Printf("写入操作日志失败: %v", err)
	}
}

// Record 记录一条操作日志；队列满时丢弃，不阻塞调用方。
func (s *OperationLogService) Record(item *models.OperationLog) {
	if item == nil {
		return
	}
	if item.Operator == "" {
		item.Operator = models.DefaultOperator
	}
	if item.ActionName == "" {
		item.ActionName = models.OperationActionName(item.Action)
	}

	select {
	case s.queue <- item:
	default:
		log.Printf("操作日志队列已满，丢弃: %s", item.Description)
	}
}

// Close 停止后台写入，并落库队列中剩余日志。
func (s *OperationLogService) Close() {
	s.once.Do(func() {
		close(s.done)
	})
}

// List 分页查询操作日志。
func (s *OperationLogService) List(
	ctx context.Context,
	filter models.OperationLogFilter,
) ([]models.OperationLog, int64, error) {
	return s.Repo.List(ctx, filter)
}

// Get 查询单条日志。
func (s *OperationLogService) Get(
	ctx context.Context,
	id uint64,
) (*models.OperationLog, error) {
	if id == 0 {
		return nil, fmt.Errorf("日志 ID 无效")
	}
	item, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("操作日志不存在")
	}
	return item, nil
}

// Summary 概览统计。
func (s *OperationLogService) Summary(
	ctx context.Context,
) (*models.OperationLogSummary, error) {
	return s.Repo.Summary(ctx)
}

// Modules 模块筛选项。
func (s *OperationLogService) Modules(
	ctx context.Context,
) ([]models.OperationLogOption, error) {
	return s.Repo.Modules(ctx)
}

// Actions 动作筛选项。
func (s *OperationLogService) Actions(
	ctx context.Context,
) ([]models.OperationLogOption, error) {
	return s.Repo.Actions(ctx)
}

// Cleanup 清理 beforeDays 天之前的日志，返回删除条数。
func (s *OperationLogService) Cleanup(
	ctx context.Context,
	beforeDays int,
) (int64, error) {
	if beforeDays < 1 {
		return 0, fmt.Errorf("保留天数必须大于 0")
	}
	if beforeDays > 3650 {
		return 0, fmt.Errorf("保留天数过大")
	}
	return s.Repo.DeleteBefore(ctx, time.Now().AddDate(0, 0, -beforeDays))
}

// ExportCSV 按查询条件导出日志（带 BOM，便于 Excel 直接打开）。
func (s *OperationLogService) ExportCSV(
	ctx context.Context,
	filter models.OperationLogFilter,
) ([]byte, error) {
	items, err := s.Repo.Export(ctx, filter, operationLogExportLimit)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(buf)
	header := []string{
		"时间", "模块", "操作", "操作摘要", "操作对象",
		"操作人", "结果", "状态码", "耗时(ms)", "IP", "接口", "错误信息",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, item := range items {
		result := "失败"
		if item.Success {
			result = "成功"
		}
		row := []string{
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			item.ModuleName,
			item.ActionName,
			item.Description,
			item.TargetID,
			item.Operator,
			result,
			strconv.Itoa(item.Status),
			strconv.FormatInt(item.DurationMs, 10),
			item.IP,
			item.Method + " " + item.Path,
			item.ErrorMsg,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
