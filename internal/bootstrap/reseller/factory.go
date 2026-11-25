package resellerbootstrap

import (
	"os"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/modules/reseller/infrastructure/domainhttps"
	adminhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/admin"
	userhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/user"
)

type Handlers struct {
	User                *userhttp.UserHandler
	UserProductSetting  *userhttp.UserProductSettingHandler
	UserFinance         *userhttp.UserFinanceHandler
	UserOrder           *userhttp.UserOrderHandler
	AdminManagement     *adminhttp.AdminManagementHandler
	AdminProfileDetail  *adminhttp.AdminProfileDetailHandler
	AdminSiteConfig     *adminhttp.AdminSiteConfigHandler
	AdminProductSetting *adminhttp.AdminProductSettingHandler
	AdminOperations     *adminhttp.AdminOperationsHandler
	AdminFinance        *adminhttp.AdminFinanceHandler
}

func New(c *container.Container) Handlers {
	return Handlers{
		User: userhttp.NewUserHandler(
			c.ResellerManagementService,
			c.ResellerSiteConfigService,
			c.UploadService,
		),
		UserProductSetting: userhttp.NewUserProductSettingHandler(c.ResellerProductSettingService),
		UserFinance: userhttp.NewUserFinanceHandler(
			c.ResellerAccountingQuery,
			c.ResellerAccountingWithdraw,
		),
		UserOrder: userhttp.NewUserOrderHandler(c.ResellerOrderService),
		AdminManagement: adminhttp.NewAdminManagementHandler(
			c.ResellerManagementService, c.ResellerStore, c.AuthzAuditService,
			domainHTTPSProvisioner(),
		),
		AdminProfileDetail: adminhttp.NewAdminProfileDetailHandler(
			c.ResellerStore, c.ResellerProductSettingService, c.ResellerAccountingQuery, c.ResellerOrderService,
		),
		AdminSiteConfig: adminhttp.NewAdminSiteConfigHandler(
			c.ResellerSiteConfigService, c.ResellerStore, c.AuthzAuditService,
		),
		AdminProductSetting: adminhttp.NewAdminProductSettingHandler(
			c.ResellerProductSettingService, c.AuthzAuditService,
		),
		AdminOperations: adminhttp.NewAdminOperationsHandler(c.ResellerOperationsService),
		AdminFinance: adminhttp.NewAdminFinanceHandler(
			c.ResellerAccountingQuery,
			c.ResellerAccountingWithdraw,
		),
	}
}

func domainHTTPSProvisioner() adminhttp.DomainHTTPSProvisioner {
	if path := os.Getenv("RESELLER_HTTPS_SOCKET"); path != "" {
		return domainhttps.Worker{SocketPath: path}
	}
	return nil
}
