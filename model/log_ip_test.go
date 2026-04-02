package model

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogIPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldLogConsumeEnabled := common.LogConsumeEnabled

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.LogConsumeEnabled = true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&User{}, &Log{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.LogConsumeEnabled = oldLogConsumeEnabled
		_ = sqlDB.Close()
	})

	return db
}

func seedUserWithIPSetting(t *testing.T, db *gorm.DB, userID int, recordIP bool) {
	t.Helper()

	user := &User{
		Id:       userID,
		Username: fmt.Sprintf("user_%d", userID),
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
	}
	user.SetSetting(dto.UserSetting{
		RecordIpLog: recordIP,
	})

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
}

func newLogTestContext(remoteAddr string, username string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	request.RemoteAddr = remoteAddr
	ctx.Request = request
	ctx.Set("username", username)
	return ctx
}

func getLatestLog(t *testing.T) *Log {
	t.Helper()

	var log Log
	if err := LOG_DB.Order("id desc").First(&log).Error; err != nil {
		t.Fatalf("failed to load log: %v", err)
	}
	return &log
}

func TestRecordConsumeLogAlwaysRecordsIP(t *testing.T) {
	db := setupLogIPTestDB(t)
	seedUserWithIPSetting(t, db, 1, false)

	ctx := newLogTestContext("203.0.113.10:12345", "consume_user")

	RecordConsumeLog(ctx, 1, RecordConsumeLogParams{
		ChannelId:        1,
		PromptTokens:     10,
		CompletionTokens: 20,
		ModelName:        "gpt-test",
		TokenName:        "token-a",
		Quota:            100,
		Content:          "consume",
		TokenId:          2,
		UseTimeSeconds:   3,
		IsStream:         false,
		Group:            "default",
		Other:            map[string]interface{}{},
	})

	log := getLatestLog(t)
	if log.Ip != "203.0.113.10" {
		t.Fatalf("expected consume log ip to be recorded, got %q", log.Ip)
	}
}

func TestRecordErrorLogAlwaysRecordsIP(t *testing.T) {
	db := setupLogIPTestDB(t)
	seedUserWithIPSetting(t, db, 2, false)

	ctx := newLogTestContext("198.51.100.8:54321", "error_user")

	RecordErrorLog(
		ctx,
		2,
		1,
		"gpt-test",
		"token-b",
		"upstream error",
		3,
		4,
		false,
		"default",
		map[string]interface{}{},
	)

	log := getLatestLog(t)
	if log.Ip != "198.51.100.8" {
		t.Fatalf("expected error log ip to be recorded, got %q", log.Ip)
	}
}
