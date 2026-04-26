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

// 核心修复：非 auto 分组下所有渠道都试过时，fallback 重选已用渠道。
// 这模拟用户场景：deepseek-v4-pro 在 default 分组只有 2 个渠道，
// 上游 429 同时打到两个渠道时，重试到第 3 次时不应该报"无可用渠道"，
// 而应该再次允许选择前面试过的渠道（限流是瞬时的，可能已经恢复）。
func TestCacheGetNextSatisfiedChannelNonAutoFallbackWhenAllChannelsExhausted(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	createServiceTestChannel(t, 601, "default", "deepseek-v4-pro", 10)
	createServiceTestChannel(t, 602, "default", "deepseek-v4-pro", 10)
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	// 模拟前两次 retry 已经把 601 和 602 都用过了
	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"601", "602"})

	channel, group, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "deepseek-v4-pro",
		Retry:      common.GetPointer(2),
	})
	require.NoError(t, err)
	// 修复前：channel == nil，触发"分组 default 下模型 ... 的可用渠道不存在（retry）"
	// 修复后：fallback 后能从 [601, 602] 中重选一个
	require.NotNil(t, channel, "fallback should reselect from used channels when all exhausted")
	require.Equal(t, "default", group)
	require.Contains(t, []int{601, 602}, channel.Id)
}

// 边界：没有任何 used channel 时正常选第一个，不触发 fallback。
func TestCacheGetNextSatisfiedChannelNonAutoFirstCallNoUsedChannels(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	createServiceTestChannel(t, 701, "default", "model-x", 10)
	createServiceTestChannel(t, 702, "default", "model-x", 10)
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	channel, group, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "model-x",
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, "default", group)
	require.Contains(t, []int{701, 702}, channel.Id)
}

// 边界：当前 group 真的没有任何渠道时返回 nil，fallback 不会无限循环。
func TestCacheGetNextSatisfiedChannelNonAutoNoChannelsAtAll(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	// 不创建任何 default 分组的渠道
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"999"})

	channel, group, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "nonexistent-model",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.Nil(t, channel)
	require.Equal(t, "default", group)
}

// 第一次正常排除已用渠道，仍能选到剩余可用渠道（不触发 fallback）。
func TestCacheGetNextSatisfiedChannelNonAutoExcludesUsedWhenOthersAvailable(t *testing.T) {
	cleanup := setupChannelSelectTestDB(t)
	defer cleanup()

	createServiceTestChannel(t, 801, "default", "model-y", 10)
	createServiceTestChannel(t, 802, "default", "model-y", 10)
	createServiceTestChannel(t, 803, "default", "model-y", 10)
	model.InitChannelCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyUsedChannels, []string{"801"})

	channel, _, err := CacheGetNextSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "model-y",
		Retry:      common.GetPointer(1),
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.NotEqual(t, 801, channel.Id, "should not pick the already-used channel when others are available")
}
