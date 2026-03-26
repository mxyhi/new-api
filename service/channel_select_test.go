package service

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectTestDB(t *testing.T) func() {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldUsingMySQL := common.UsingMySQL
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldAutoGroups := setting.AutoGroups2JsonString()
	oldUserUsableGroups := setting.UserUsableGroups2JSONString()

	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channel-select-test.db")), &gorm.Config{})
	require.NoError(t, err)

	model.DB = testDB
	model.LOG_DB = testDB
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	common.MemoryCacheEnabled = true
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["group-a","group-b"]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"group-a":"Group A","group-b":"Group B"}`))
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.InitChannelCache()

	return func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.UsingMySQL = oldUsingMySQL
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(oldAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUserUsableGroups))
		model.InitChannelCache()
	}
}

func createServiceTestChannel(t *testing.T, id int, group string, modelName string, priority int64) {
	t.Helper()
	weight := uint(100)
	channelPriority := priority
	channel := &model.Channel{
		Id:       id,
		Name:     group + "-channel",
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Group:    group,
		Models:   modelName,
		Weight:   &weight,
		Priority: &channelPriority,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestCacheGetNextSatisfiedChannelAutoGroupExhaustsCurrentGroupFirst(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	createServiceTestChannel(t, 301, "group-a", "gpt-4o", 10)
	createServiceTestChannel(t, 302, "group-a", "gpt-4o", 10)
	createServiceTestChannel(t, 303, "group-b", "gpt-4o", 10)
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"301"})

	channel, group, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "auto",
		ModelName:  "gpt-4o",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "group-a", group)
	require.Equal(t, 302, channel.Id)
}

func TestCacheGetNextSatisfiedChannelAutoGroupFallsBackAfterCurrentGroupExhausted(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	createServiceTestChannel(t, 401, "group-a", "gpt-4o", 10)
	createServiceTestChannel(t, 402, "group-b", "gpt-4o", 10)
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"401"})

	channel, group, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "auto",
		ModelName:  "gpt-4o",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "group-b", group)
	require.Equal(t, 402, channel.Id)
}

