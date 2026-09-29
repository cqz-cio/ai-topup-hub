package container

// initServices 按依赖顺序装配各阶段 Service。
func (c *Container) initServices() {
	c.initPolicyAndSettingServices()
	c.loadRuntimeSettings()
	c.initIdentityAndCatalogServices()
	c.initApplicationServices()
	c.initIntegrationServices()
	c.wireServiceDependencies()
}
