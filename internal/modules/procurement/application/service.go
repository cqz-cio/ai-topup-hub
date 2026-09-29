package application

import procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"

type Options struct {
	Repository         procurementcontract.Repository
	Orders             procurementcontract.OrderRepository
	ProductMappings    procurementcontract.ProductMappingReader
	SKUMappings        procurementcontract.SKUMappingReader
	Connections        procurementcontract.ConnectionProvider
	Queue              procurementcontract.Enqueuer
	OrderLifecycle     procurementcontract.OrderLifecycle
	DownstreamCallback procurementcontract.DownstreamCallbackEnqueuer
	BotNotifier        procurementcontract.BotFulfillmentNotifier
	Notifications      procurementcontract.FailureNotifier
}

type Service struct {
	procRepo           procurementcontract.Repository
	orderRepo          procurementcontract.OrderRepository
	mappingRepo        procurementcontract.ProductMappingReader
	skuMapRepo         procurementcontract.SKUMappingReader
	connections        procurementcontract.ConnectionProvider
	queue              procurementcontract.Enqueuer
	orderLifecycle     procurementcontract.OrderLifecycle
	downstreamCallback procurementcontract.DownstreamCallbackEnqueuer
	botNotifier        procurementcontract.BotFulfillmentNotifier
	notifications      procurementcontract.FailureNotifier
}

var _ procurementcontract.UseCase = (*Service)(nil)

func NewService(options Options) *Service {
	return &Service{
		procRepo: options.Repository, orderRepo: options.Orders,
		mappingRepo: options.ProductMappings, skuMapRepo: options.SKUMappings,
		connections: options.Connections, queue: options.Queue,
		orderLifecycle: options.OrderLifecycle, downstreamCallback: options.DownstreamCallback,
		botNotifier: options.BotNotifier, notifications: options.Notifications,
	}
}
