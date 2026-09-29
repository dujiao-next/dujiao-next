package application

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// CreateCardSecretBatchInput 批量录入卡密输入
type CreateCardSecretBatchInput struct {
	ProductID   uint
	SKUID       uint
	Secrets     []string
	BatchNo     string
	Note        string
	Source      string
	AdminID     uint
	Deduplicate *bool
	IsLoop      bool
}

// CreateCardSecretBatch 批量录入卡密
func (s *Service) CreateCardSecretBatch(input CreateCardSecretBatchInput) (*cardsecretdomain.Batch, int, error) {
	if input.ProductID == 0 {
		return nil, 0, ErrInvalid
	}
	if len(input.Secrets) == 0 {
		return nil, 0, ErrInvalid
	}

	product, err := s.productRepo.GetByID(strings.TrimSpace(strconv.FormatUint(uint64(input.ProductID), 10)))
	if err != nil {
		return nil, 0, ErrProductFetchFailed
	}
	if product == nil {
		return nil, 0, ErrProductNotFound
	}
	sku, err := s.resolveCardSecretSKU(product.ID, input.SKUID)
	if err != nil {
		return nil, 0, err
	}

	normalized := normalizeSecrets(input.Secrets, shouldDeduplicateCardSecrets(input.Deduplicate))
	if len(normalized) == 0 {
		return nil, 0, ErrInvalid
	}
	if s.batchRepo == nil {
		return nil, 0, ErrBatchCreateFailed
	}

	batchNo := strings.TrimSpace(input.BatchNo)
	if batchNo == "" {
		batchNo = generateBatchNo()
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = constants.CardSecretSourceManual
	}

	now := time.Now()
	batch := &cardsecretdomain.Batch{
		ProductID:  input.ProductID,
		SKUID:      sku.ID,
		BatchNo:    batchNo,
		Source:     source,
		TotalCount: len(normalized),
		Note:       strings.TrimSpace(input.Note),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if input.AdminID > 0 {
		batch.CreatedBy = &input.AdminID
	}

	if s.transactions == nil {
		return nil, 0, ErrBatchCreateFailed
	}
	err = s.transactions.Transaction(func(secretRepo cardsecretcontract.Repository, batchRepo cardsecretcontract.BatchRepository) error {
		if err := batchRepo.Create(batch); err != nil {
			return ErrBatchCreateFailed
		}
		items := make([]cardsecretdomain.Secret, 0, len(normalized))
		for _, secret := range normalized {
			items = append(items, cardsecretdomain.Secret{
				ProductID: input.ProductID,
				SKUID:     sku.ID,
				BatchID:   &batch.ID,
				Secret:    secret,
				Status:    cardsecretdomain.StatusAvailable,
				IsLoop:    input.IsLoop,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		if err := secretRepo.CreateBatch(items); err != nil {
			return ErrCreateFailed
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrBatchCreateFailed) {
			return nil, 0, ErrBatchCreateFailed
		}
		return nil, 0, ErrCreateFailed
	}
	return batch, batch.TotalCount, nil
}

// ImportCardSecretCSVInput 导入 CSV 输入
type ImportCardSecretCSVInput struct {
	ProductID   uint
	SKUID       uint
	File        *multipart.FileHeader
	BatchNo     string
	Note        string
	AdminID     uint
	Deduplicate *bool
	IsLoop      bool
}

// ImportCardSecretCSV 从 CSV 导入卡密
func (s *Service) ImportCardSecretCSV(input ImportCardSecretCSVInput) (*cardsecretdomain.Batch, int, error) {
	if input.ProductID == 0 || input.File == nil {
		return nil, 0, ErrInvalid
	}

	file, err := input.File.Open()
	if err != nil {
		return nil, 0, ErrImportFailed
	}
	defer file.Close()

	secrets, err := parseCSVSecrets(file)
	if err != nil {
		return nil, 0, ErrImportFailed
	}
	return s.CreateCardSecretBatch(CreateCardSecretBatchInput{
		ProductID:   input.ProductID,
		SKUID:       input.SKUID,
		Secrets:     secrets,
		BatchNo:     input.BatchNo,
		Note:        input.Note,
		Source:      constants.CardSecretSourceCSV,
		AdminID:     input.AdminID,
		Deduplicate: input.Deduplicate,
		IsLoop:      input.IsLoop,
	})
}

func shouldDeduplicateCardSecrets(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func normalizeSecrets(values []string, deduplicate bool) []string {
	var seen map[string]struct{}
	if deduplicate {
		seen = make(map[string]struct{})
	}
	result := make([]string, 0, len(values))
	for _, val := range values {
		for _, line := range strings.Split(val, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if deduplicate {
				if _, ok := seen[trimmed]; ok {
					continue
				}
				seen[trimmed] = struct{}{}
			}
			result = append(result, trimmed)
		}
	}
	return result
}

func parseCSVSecrets(reader io.Reader) ([]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	var (
		secrets    []string
		headerRead bool
		secretIdx  = 0
	)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 {
			continue
		}
		if !headerRead {
			headerRead = true
			skipRow := false
			for i, col := range record {
				if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(col, "\ufeff")), "secret") {
					secretIdx = i
					skipRow = true
					break
				}
			}
			if skipRow {
				continue
			}
		}
		if secretIdx >= len(record) {
			continue
		}
		secret := strings.TrimSpace(strings.TrimPrefix(record[secretIdx], "\ufeff"))
		if secret == "" {
			continue
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}

func generateBatchNo() string {
	now := time.Now().Format("20060102150405")
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("BATCH-%s-%04d", now, rng.Intn(10000))
}
