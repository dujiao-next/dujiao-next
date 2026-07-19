package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/upstream"
	"github.com/shopspring/decimal"
)

var (
	ErrReconciliationJobNotFound  = errors.New("reconciliation job not found")
	ErrReconciliationItemNotFound = errors.New("reconciliation item not found")
	ErrReconciliationJobRunning   = errors.New("reconciliation job is already running")
)

// ReconciliationService 对账服务
type ReconciliationService struct {
	jobRepo     repository.ReconciliationJobRepository
	itemRepo    repository.ReconciliationItemRepository
	procRepo    repository.ProcurementOrderRepository
	connSvc     *SiteConnectionService
	queueClient *queue.Client
	notifySvc   *NotificationService
}

// NewReconciliationService 创建对账服务
func NewReconciliationService(
	jobRepo repository.ReconciliationJobRepository,
	itemRepo repository.ReconciliationItemRepository,
	procRepo repository.ProcurementOrderRepository,
	connSvc *SiteConnectionService,
	queueClient *queue.Client,
	notifySvc *NotificationService,
) *ReconciliationService {
	return &ReconciliationService{
		jobRepo:     jobRepo,
		itemRepo:    itemRepo,
		procRepo:    procRepo,
		connSvc:     connSvc,
		queueClient: queueClient,
		notifySvc:   notifySvc,
	}
}

// RunReconciliationInput 发起对账任务的入参
type RunReconciliationInput struct {
	ConnectionID   uint      `json:"connection_id" binding:"required"`
	Type           string    `json:"type" binding:"required"`
	TimeRangeStart time.Time `json:"time_range_start" binding:"required"`
	TimeRangeEnd   time.Time `json:"time_range_end" binding:"required"`
}

// CreateAndEnqueue 创建对账任务并入队执行
func (s *ReconciliationService) CreateAndEnqueue(input RunReconciliationInput) (*models.ReconciliationJob, error) {
	job := &models.ReconciliationJob{
		ConnectionID:   input.ConnectionID,
		Type:           input.Type,
		Status:         constants.ReconciliationJobStatusPending,
		TimeRangeStart: input.TimeRangeStart,
		TimeRangeEnd:   input.TimeRangeEnd,
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, fmt.Errorf("create reconciliation job: %w", err)
	}

	if s.queueClient != nil {
		if err := s.queueClient.EnqueueReconciliationRun(queue.ReconciliationRunPayload{
			JobID: job.ID,
		}); err != nil {
			logger.Warnw("reconciliation_enqueue_failed", "job_id", job.ID, "error", err)
		}
	}

	return job, nil
}

// Execute 执行对账任务（由 worker 调用）
func (s *ReconciliationService) Execute(ctx context.Context, jobID uint) error {
	job, err := s.jobRepo.GetByID(jobID)
	if err != nil {
		return fmt.Errorf("get reconciliation job: %w", err)
	}
	if job.Status == constants.ReconciliationJobStatusRunning {
		return ErrReconciliationJobRunning
	}
	if job.Status == constants.ReconciliationJobStatusCompleted {
		return nil // 已完成，不重复执行
	}

	now := time.Now()
	job.Status = constants.ReconciliationJobStatusRunning
	job.StartedAt = &now
	if err := s.jobRepo.Update(job); err != nil {
		return fmt.Errorf("update job status to running: %w", err)
	}

	if err := s.executeReconciliation(ctx, job); err != nil {
		finishedAt := time.Now()
		job.Status = constants.ReconciliationJobStatusFailed
		job.FinishedAt = &finishedAt
		resultJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
		job.ResultJSON = string(resultJSON)
		_ = s.jobRepo.Update(job)
		return fmt.Errorf("execute reconciliation: %w", err)
	}

	finishedAt := time.Now()
	job.Status = constants.ReconciliationJobStatusCompleted
	job.FinishedAt = &finishedAt
	_ = s.jobRepo.Update(job)

	// 如果有差异项，发送通知
	if job.MismatchedCount > 0 {
		s.sendMismatchNotification(job)
	}

	return nil
}

// executeReconciliation 执行单次对账主流程并回填任务统计结果。
func (s *ReconciliationService) executeReconciliation(ctx context.Context, job *models.ReconciliationJob) error {
	// 获取连接信息和适配器
	conn, err := s.connSvc.GetByID(job.ConnectionID)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}
	adapter, err := s.connSvc.GetAdapter(conn)
	if err != nil {
		return fmt.Errorf("get adapter: %w", err)
	}

	// 查询时间范围内的采购单
	procOrders, err := s.procRepo.ListByConnectionAndTimeRange(
		job.ConnectionID, job.TimeRangeStart, job.TimeRangeEnd,
	)
	if err != nil {
		return fmt.Errorf("list procurement orders: %w", err)
	}

	var mismatchItems []models.ReconciliationItem
	var skippedCount int
	var errorCount int

	for _, po := range procOrders {
		if po.UpstreamOrderID == 0 {
			skippedCount++
			continue
		}

		// 查询上游订单状态
		upstreamDetail, err := adapter.GetOrder(ctx, po.UpstreamOrderID)
		if err != nil {
			logger.Warnw("reconciliation_get_upstream_order_failed",
				"job_id", job.ID, "procurement_id", po.ID,
				"upstream_order_id", po.UpstreamOrderID, "error", err)
			errorCount++
			continue
		}

		item := s.compareOrder(job, &po, upstreamDetail)
		if item != nil {
			mismatchItems = append(mismatchItems, *item)
		}
	}

	// 批量写入差异项
	if len(mismatchItems) > 0 {
		if err := s.itemRepo.BatchCreate(mismatchItems); err != nil {
			return fmt.Errorf("batch create reconciliation items: %w", err)
		}
	}

	// TotalCount 只统计实际参与对比的采购单（排除无上游订单和查询失败的）
	comparedCount := len(procOrders) - skippedCount - errorCount
	job.TotalCount = comparedCount
	job.MismatchedCount = len(mismatchItems)
	job.MatchedCount = comparedCount - job.MismatchedCount

	resultJSON, _ := json.Marshal(map[string]any{
		"total":      job.TotalCount,
		"matched":    job.MatchedCount,
		"mismatched": job.MismatchedCount,
		"skipped":    skippedCount,
		"errors":     errorCount,
	})
	job.ResultJSON = string(resultJSON)

	return nil
}

// compareOrder 对比本地采购单与上游订单，返回差异项（无差异返回 nil）。
func (s *ReconciliationService) compareOrder(job *models.ReconciliationJob, po *models.ProcurementOrder, detail *upstream.UpstreamOrderDetail) *models.ReconciliationItem {
	checkStatus := job.Type == constants.ReconciliationTypeStatus || job.Type == constants.ReconciliationTypeFull
	checkAmount := job.Type == constants.ReconciliationTypeAmount || job.Type == constants.ReconciliationTypeFull

	statusMismatch := false
	if checkStatus {
		statusMismatch = !isStatusConsistent(po.Status, detail.Status)
	}

	// 金额对比：本地记录的采购价 vs 上游实际返回的金额
	// 注意：不能拿 LocalSellAmount（本地售价）和 UpstreamAmount（采购价）比，两者有利润差是正常的
	amountMismatch := false
	var upstreamActualAmount models.Money
	if checkAmount && detail.Amount != "" {
		upstreamDecimal, parseErr := decimal.NewFromString(detail.Amount)
		if parseErr == nil && upstreamDecimal.IsPositive() && po.UpstreamAmount.IsPositive() {
			upstreamActualAmount = models.NewMoneyFromDecimal(upstreamDecimal)
			amountMismatch = !po.UpstreamAmount.Equal(upstreamDecimal)
		}
	}

	var mismatchType string
	if statusMismatch && amountMismatch {
		mismatchType = constants.MismatchTypeBoth
	} else if statusMismatch {
		mismatchType = constants.MismatchTypeStatus
	} else if amountMismatch {
		mismatchType = constants.MismatchTypeAmount
	}

	if mismatchType == "" {
		return nil
	}

	return &models.ReconciliationItem{
		JobID:              job.ID,
		ProcurementOrderID: po.ID,
		LocalOrderNo:       po.LocalOrderNo,
		UpstreamOrderNo:    po.UpstreamOrderNo,
		LocalStatus:        po.Status,
		UpstreamStatus:     detail.Status,
		LocalAmount:        po.UpstreamAmount,
		UpstreamAmount:     upstreamActualAmount,
		MismatchType:       mismatchType,
	}
}

// isStatusConsistent 判断本地采购单状态与上游状态是否一致。
//
// 状态对照说明（本地采购单 -> 上游订单）：
// 1) completed / fulfilled
//   - 对应上游 delivered / completed / fulfilled（已交付）。
//   - 同时兼容上游 refunded / partially_refunded（先交付后退款的场景仍视为一致）。
//
// 2) canceled
//   - 对应上游 canceled / cancelled（取消）。
//   - 同时兼容上游 refunded / partially_refunded（本地下取消、上游退款的补偿链路）。
//
// 3) pending
//   - 对应上游 pending / paid（待处理到待执行窗口）。
//
// 4) submitted / accepted
//   - 对应上游 paid / processing / accepted（已提交到处理中窗口）。
//
// 5) failed / rejected
//   - 对应上游 failed / rejected（失败态一致）。
//
// 6) 历史兼容状态 "fulfilling"
//   - 对应上游 fulfilling / processing / paid（处理中窗口）。
//
// 说明：
// - 上游状态会先做小写与空白归一化。
// - 对于未命中的状态，默认要求两边字符串完全一致。
func isStatusConsistent(localStatus, upstreamStatus string) bool {
	localStatus = strings.ToLower(strings.TrimSpace(localStatus))
	upstreamStatus = strings.ToLower(strings.TrimSpace(upstreamStatus))

	switch localStatus {
	case constants.ProcurementStatusDelivered, constants.ProcurementStatusCompleted, constants.ProcurementStatusFulfilled:
		return upstreamStatus == "completed" ||
			upstreamStatus == "delivered" ||
			upstreamStatus == "fulfilled" ||
			upstreamStatus == "refunded" ||
			upstreamStatus == "partially_refunded"
	case constants.ProcurementStatusCanceled:
		return upstreamStatus == "canceled" || upstreamStatus == "cancelled" || upstreamStatus == "refunded" || upstreamStatus == "partially_refunded"
	case constants.ProcurementStatusPending:
		return upstreamStatus == "pending" || upstreamStatus == "paid"
	case constants.ProcurementStatusSubmitted, constants.ProcurementStatusAccepted:
		return upstreamStatus == "paid" || upstreamStatus == "processing" || upstreamStatus == "accepted"
	case constants.ProcurementStatusFailed, constants.ProcurementStatusRejected:
		return upstreamStatus == "failed" || upstreamStatus == "rejected"
	case "fulfilling":
		return upstreamStatus == "fulfilling" || upstreamStatus == "processing" || upstreamStatus == "paid"
	default:
		return localStatus == upstreamStatus
	}
}

// sendMismatchNotification 发送对账差异通知到通知中心。
func (s *ReconciliationService) sendMismatchNotification(job *models.ReconciliationJob) {
	if s.notifySvc == nil {
		return
	}
	_ = s.notifySvc.Enqueue(NotificationEnqueueInput{
		EventType: constants.NotificationEventExceptionAlert,
		BizType:   constants.NotificationBizTypeReconciliation,
		BizID:     job.ID,
		Data: map[string]any{
			"message":          fmt.Sprintf("对账任务 #%d 完成，发现 %d 项差异", job.ID, job.MismatchedCount),
			"job_id":           job.ID,
			"connection_id":    job.ConnectionID,
			"total_count":      job.TotalCount,
			"mismatched_count": job.MismatchedCount,
		},
	})
}

// GetJob 获取对账任务
func (s *ReconciliationService) GetJob(id uint) (*models.ReconciliationJob, error) {
	return s.jobRepo.GetByID(id)
}

// ListJobs 对账任务列表
func (s *ReconciliationService) ListJobs(filter repository.ReconciliationJobListFilter) ([]models.ReconciliationJob, int64, error) {
	return s.jobRepo.List(filter)
}

// GetJobItems 获取对账任务明细
func (s *ReconciliationService) GetJobItems(jobID uint, page, pageSize int) ([]models.ReconciliationItem, int64, error) {
	return s.itemRepo.ListByJobID(jobID, page, pageSize)
}

// ResolveItem 处理对账差异项
func (s *ReconciliationService) ResolveItem(itemID uint, adminID uint, remark string) error {
	item, err := s.itemRepo.GetByID(itemID)
	if err != nil {
		return ErrReconciliationItemNotFound
	}
	now := time.Now()
	item.Resolved = true
	item.ResolvedBy = &adminID
	item.ResolvedAt = &now
	item.Remark = remark
	return s.itemRepo.Update(item)
}
