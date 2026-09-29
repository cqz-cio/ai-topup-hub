package application

import (
	"strconv"
	"strings"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
)

type ServiceOptions struct {
	Secrets      cardsecretcontract.Repository
	Batches      cardsecretcontract.BatchRepository
	Transactions cardsecretcontract.UnitOfWork
	Products     cardsecretcontract.ProductRepository
	ProductSKUs  cardsecretcontract.ProductSKURepository
}

// Service 卡密库存服务。
type Service struct {
	secretRepo     cardsecretcontract.Repository
	batchRepo      cardsecretcontract.BatchRepository
	transactions   cardsecretcontract.UnitOfWork
	productRepo    cardsecretcontract.ProductRepository
	productSKURepo cardsecretcontract.ProductSKURepository
}

func NewService(options ServiceOptions) *Service {
	return &Service{
		secretRepo:     options.Secrets,
		batchRepo:      options.Batches,
		transactions:   options.Transactions,
		productRepo:    options.Products,
		productSKURepo: options.ProductSKUs,
	}
}

func (s *Service) resolveCardSecretSKU(productID, rawSKUID uint) (*productdomain.ProductSKU, error) {
	if productID == 0 || s.productSKURepo == nil {
		return nil, ErrProductSKUInvalid
	}
	product, err := s.productRepo.GetByID(strings.TrimSpace(strconv.FormatUint(uint64(productID), 10)))
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrProductNotFound
	}
	skus, err := s.productSKURepo.ListByProduct(productID, false)
	if err != nil {
		return nil, err
	}
	activeSKUs := make([]productdomain.ProductSKU, 0, len(skus))
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		activeSKUs = append(activeSKUs, sku)
	}
	if rawSKUID > 0 {
		sku, err := s.productSKURepo.GetByID(rawSKUID)
		if err != nil {
			return nil, err
		}
		if sku == nil || sku.ProductID != productID {
			return nil, ErrProductSKUInvalid
		}
		if strings.TrimSpace(product.FulfillmentType) == constants.FulfillmentTypeAuto && !sku.IsActive {
			return nil, ErrProductSKUInvalid
		}
		return sku, nil
	}

	if strings.TrimSpace(product.FulfillmentType) == constants.FulfillmentTypeAuto {
		switch len(activeSKUs) {
		case 0:
		case 1:
			return &activeSKUs[0], nil
		default:
			return nil, ErrProductSKURequired
		}
	}

	defaultSKU, err := s.productSKURepo.GetByProductAndCode(productID, productdomain.DefaultSKUCode)
	if err != nil {
		return nil, err
	}
	if defaultSKU != nil {
		return defaultSKU, nil
	}
	if len(skus) == 1 {
		return &skus[0], nil
	}
	return nil, ErrProductSKURequired
}

func normalizeCardSecretIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
