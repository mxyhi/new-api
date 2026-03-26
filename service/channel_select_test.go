package service

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
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

func TestCacheGetNextSatisfiedChannelAutoGroupKeepsWeightedRoundRobinWithinCurrentGroup(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	weightA := uint(5)
	priority := int64(10)
	channelA := &model.Channel{
		Id:       501,
		Name:     "group-a-channel-1",
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Group:    "group-a",
		Models:   "gpt-4o",
		Weight:   &weightA,
		Priority: &priority,
	}
	require.NoError(t, model.DB.Create(channelA).Error)
	require.NoError(t, channelA.AddAbilities(nil))

	weightB := uint(3)
	channelB := &model.Channel{
		Id:       502,
		Name:     "group-a-channel-2",
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Group:    "group-a",
		Models:   "gpt-4o",
		Weight:   &weightB,
		Priority: &priority,
	}
	require.NoError(t, model.DB.Create(channelB).Error)
	require.NoError(t, channelB.AddAbilities(nil))

	weightNextGroup := uint(9)
	channelNextGroup := &model.Channel{
		Id:       503,
		Name:     "group-b-channel-1",
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Group:    "group-b",
		Models:   "gpt-4o",
		Weight:   &weightNextGroup,
		Priority: &priority,
	}
	require.NoError(t, model.DB.Create(channelNextGroup).Error)
	require.NoError(t, channelNextGroup.AddAbilities(nil))
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	first, group, err := CacheGetNextSatisfiedChannel(&RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, "group-a", group)
	require.Equal(t, 501, first.Id)

	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"501"})
	second, group, err := CacheGetNextSatisfiedChannel(&RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: "gpt-4o", Retry: common.GetPointer(1)})
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, "group-a", group)
	require.Equal(t, 502, second.Id)

	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"501", "502"})
	third, group, err := CacheGetNextSatisfiedChannel(&RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: "gpt-4o", Retry: common.GetPointer(2)})
	require.NoError(t, err)
	require.NotNil(t, third)
	require.Equal(t, "group-b", group)
	require.Equal(t, 503, third.Id)
}
