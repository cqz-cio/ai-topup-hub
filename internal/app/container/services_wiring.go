package container

// wireServiceDependencies 收口构造后才能建立的双向或延迟依赖。
func (c *Container) wireServiceDependencies() {
	c.UserAuthService.SetMemberLevelService(c.MemberLevelService)
	c.OrderRefundService.SetResellerAccounting(c.ResellerAccountingLedger)
	c.PaymentService.SetMemberLevelService(c.MemberLevelService)
	c.PaymentService.SetProcurementService(c.ProcurementOrderService)
	c.PaymentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.FulfillmentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
}
