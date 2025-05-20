package integration

import (
	"context"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/test/integration/utils"
	"github.com/wso2/identity-customer-data-service/test/setup"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, _ := setup.SetupTestPostgres(ctx)
	//if err != nil {
	//	log.Fatalf("Failed to start test DB: %v", err)
	//}
	defer pg.Container.Terminate(ctx)

	log.Init("DEBUG")
	//logger := log.GetLogger()

	provider.SetTestDB(pg.DB)

	_ = utils.CreateTablesFromFile(pg.DB, "test/setup/schema.sql")

	os.Exit(m.Run())
}
