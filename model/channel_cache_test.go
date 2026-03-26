package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelCacheTestDB(t *testing.T) func() {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldUsingMySQL := common.UsingMySQL
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldChannelSchedulers := channelSchedulers

	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channel-cache-test.db")), &gorm.Config{})
	require.NoError(t, err)

	DB = testDB
	LOG_DB = testDB
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	channelSchedulers = newChannelSchedulerRegistry()

	common.UsingMySQL = false
	common.MemoryCacheEnabled = true
	initCol()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))

	return func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingPostgreSQL = oldUsingPostgreSQL
		channelSchedulers = oldChannelSchedulers

		common.UsingMySQL = oldUsingMySQL
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		initCol()
	}
}

func createTestChannel(t *testing.T, id int, group string, modelName string, priority int64, weight uint) {
	t.Helper()
	channelWeight := weight
	channelPriority := priority
	channel := &Channel{
		Id:       id,
		Name:     modelName + "-channel",
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Group:    group,
		Models:   modelName,
		Weight:   &channelWeight,
		Priority: &channelPriority,
	}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func TestGetNextSatisfiedChannelExhaustsSamePriorityBeforeFallback(t *testing.T) {
	cleanup := setupChannelCacheTestDB(t)
	defer cleanup()

	createTestChannel(t, 101, "default", "gpt-4o", 10, 100)
	createTestChannel(t, 102, "default", "gpt-4o", 10, 50)
	createTestChannel(t, 103, "default", "gpt-4o", 9, 100)
	InitChannelCache()

	channel, err := GetNextSatisfiedChannel("default", "gpt-4o", nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 101, channel.Id)

	excluded := map[int]struct{}{channel.Id: {}}
	nextChannel, err := GetNextSatisfiedChannel("default", "gpt-4o", excluded)
	require.NoError(t, err)
	require.NotNil(t, nextChannel)
	require.Equal(t, 102, nextChannel.Id)

	excluded[nextChannel.Id] = struct{}{}
	fallbackChannel, err := GetNextSatisfiedChannel("default", "gpt-4o", excluded)
	require.NoError(t, err)
	require.NotNil(t, fallbackChannel)
	require.Equal(t, 103, fallbackChannel.Id)
}

func TestGetNextSatisfiedChannelUsesWeightedRoundRobinWithinSamePriority(t *testing.T) {
	cleanup := setupChannelCacheTestDB(t)
	defer cleanup()

	createTestChannel(t, 111, "default", "gpt-4o", 10, 5)
	createTestChannel(t, 112, "default", "gpt-4o", 10, 3)
	createTestChannel(t, 113, "default", "gpt-4o", 10, 2)
	InitChannelCache()

	selectedIDs := make([]int, 0, 10)
	for range 10 {
		channel, err := GetNextSatisfiedChannel("default", "gpt-4o", nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		selectedIDs = append(selectedIDs, channel.Id)
	}

	require.Equal(t, []int{111, 112, 113, 111, 111, 112, 111, 113, 112, 111}, selectedIDs)
	counts := map[int]int{}
	for _, id := range selectedIDs {
		counts[id]++
	}
	require.Equal(t, 5, counts[111])
	require.Equal(t, 3, counts[112])
	require.Equal(t, 2, counts[113])
}

func TestGetNextSatisfiedChannelReturnsNilWhenAllChannelsExcluded(t *testing.T) {
	cleanup := setupChannelCacheTestDB(t)
	defer cleanup()

	createTestChannel(t, 201, "default", "gpt-4o", 10, 100)
	InitChannelCache()

	channel, err := GetNextSatisfiedChannel("default", "gpt-4o", map[int]struct{}{201: {}})
	require.NoError(t, err)
	require.Nil(t, channel)
}

func TestGetNextSatisfiedChannelUsesWeightedRoundRobinWithoutMemoryCache(t *testing.T) {
	cleanup := setupChannelCacheTestDB(t)
	defer cleanup()

	createTestChannel(t, 211, "default", "gpt-4o", 10, 5)
	createTestChannel(t, 212, "default", "gpt-4o", 10, 3)
	createTestChannel(t, 213, "default", "gpt-4o", 10, 2)
	common.MemoryCacheEnabled = false

	selectedIDs := make([]int, 0, 10)
	for range 10 {
		channel, err := GetNextSatisfiedChannel("default", "gpt-4o", nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		selectedIDs = append(selectedIDs, channel.Id)
	}

	require.Equal(t, []int{211, 212, 213, 211, 211, 212, 211, 213, 212, 211}, selectedIDs)
}
