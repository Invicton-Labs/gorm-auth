package gormauth

import (
	"context"

	"github.com/Invicton-Labs/go-stackerr"
	gormaws "github.com/Invicton-Labs/gorm-auth/aws"
	gormawsiam "github.com/Invicton-Labs/gorm-auth/aws/iam-auth"
	gormauthmodels "github.com/Invicton-Labs/gorm-auth/models"
	gormauthmysql "github.com/Invicton-Labs/gorm-auth/mysql"
	"gorm.io/gorm"
)

func GetMysqlGorm[authType gormauthmysql.MysqlAuthTypes](
	ctx context.Context,
	input gormauthmysql.GetMysqlGormInputWithAuth[authType],
) (*gorm.DB, stackerr.Error) {
	if input, ok := any(input).(gormauthmysql.GetMysqlGormInputWithAuth[gormawsiam.RdsIamAuth]); ok {
		return gormaws.GetRdsIamMysqlGorm(ctx, input)
	} else if input, ok := any(input.AuthSettings).(gormauthmysql.GetMysqlGormInputWithAuth[gormawsiam.RdsIamAuthWithReadOnly]); ok {
		return gormaws.GetRdsIamMysqlGorm(ctx, input)
	} else if input, ok := any(input.AuthSettings).(gormauthmysql.GetMysqlGormInputWithAuth[gormauthmodels.MysqlSecretPassword]); ok {
		return gormauthmysql.GetMysqlGormPassword(ctx, input)
	} else if input, ok := any(input.AuthSettings).(gormauthmysql.GetMysqlGormInputWithAuth[gormauthmodels.MysqlSecretPasswordWithReadOnly]); ok {
		return gormauthmysql.GetMysqlGormPassword(ctx, input)
	} else {
		return nil, stackerr.Errorf("Input is of unexpected type")
	}
}
