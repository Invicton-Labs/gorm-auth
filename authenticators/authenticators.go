package authenticators

import (
	"context"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/Invicton-Labs/gorm-auth/dialectors"
	"github.com/go-sql-driver/mysql"
)

type AuthenticationSettings interface {
	// UpdateConfigWithAuth adds authentication parameters
	// to an existing mysql.Config struct.
	UpdateConfigWithAuth(ctx context.Context, config mysql.Config) (*mysql.Config, stackerr.Error)
	// UpdateDialectorSettings updates the dialector settings with
	// override values required by this authentication method
	UpdateDialectorSettings(dialectors.MysqlDialectorInput) (dialectors.MysqlDialectorInput, stackerr.Error)
}
