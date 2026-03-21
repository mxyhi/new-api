package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserSettingsControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUserForSettingsTest(t *testing.T, db *gorm.DB, userID int, settings dto.UserSetting) {
	t.Helper()

	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("user_%d", userID),
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}
	user.SetSetting(settings)

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
}

func TestUpdateUserSettingPreservesRecordIPLogWhenRequestOmitsField(t *testing.T) {
	db := setupUserSettingsControllerTestDB(t)
	seedUserForSettingsTest(t, db, 1, dto.UserSetting{
		NotifyType:            dto.NotifyTypeEmail,
		QuotaWarningThreshold: 1000,
		RecordIpLog:           true,
	})

	body := map[string]interface{}{
		"notify_type":                    dto.NotifyTypeEmail,
		"quota_warning_threshold":        2000,
		"accept_unset_model_ratio_model": true,
		"notification_email":             "user@example.com",
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", body, 1)
	UpdateUserSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var user model.User
	if err := db.Where("id = ?", 1).First(&user).Error; err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}

	if !user.GetSetting().RecordIpLog {
		t.Fatalf("expected record_ip_log to be preserved when request omits field")
	}
}
