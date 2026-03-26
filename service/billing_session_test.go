package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func ensureBillingSubscriptionTables(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec(`CREATE TABLE IF NOT EXISTS subscription_plans (
		id integer primary key,
		title varchar(128) NOT NULL,
		subtitle varchar(255) DEFAULT '',
		price_amount decimal(10,6) NOT NULL DEFAULT 0,
		currency varchar(8) NOT NULL DEFAULT 'USD',
		duration_unit varchar(16) NOT NULL DEFAULT 'month',
		duration_value integer NOT NULL DEFAULT 1,
		custom_seconds bigint NOT NULL DEFAULT 0,
		enabled numeric DEFAULT 1,
		sort_order integer DEFAULT 0,
		stripe_price_id varchar(128) DEFAULT '',
		creem_product_id varchar(128) DEFAULT '',
		purchase_link varchar(1024) DEFAULT '',
		max_purchase_per_user integer DEFAULT 0,
		upgrade_group varchar(64) DEFAULT '',
		total_amount bigint NOT NULL DEFAULT 0,
		quota_reset_period varchar(16) DEFAULT 'never',
		quota_reset_custom_seconds bigint DEFAULT 0,
		created_at bigint,
		updated_at bigint
	)`).Error)
	require.NoError(t, model.DB.Exec(`CREATE TABLE IF NOT EXISTS subscription_pre_consume_records (
		id integer primary key,
		request_id varchar(64) UNIQUE,
		user_id integer,
		user_subscription_id integer,
		pre_consumed bigint NOT NULL DEFAULT 0,
		status varchar(32),
		created_at bigint,
		updated_at bigint
	)`).Error)
}

func seedPlanWithSubscription(t *testing.T, userID, planID, subID int, group string, total, used int64) {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Id:            planID,
		Title:         "test-plan-" + group,
		PriceAmount:   9.9,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		UpgradeGroup:  group,
		TotalAmount:   total,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	now := time.Now().Unix()
	sub := &model.UserSubscription{
		Id:           subID,
		UserId:       userID,
		PlanId:       planID,
		AmountTotal:  total,
		AmountUsed:   used,
		Status:       "active",
		StartTime:    now,
		EndTime:      now + 3600,
		UpgradeGroup: group,
		CreatedAt:    common.GetTimestamp(),
		UpdatedAt:    common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func newBillingTestRelayInfo(userID int, group, pref string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestId:       "req-" + time.Now().Format("150405.000000"),
		UserId:          userID,
		UsingGroup:      group,
		UserGroup:       group,
		OriginModelName: "gpt-4o",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: pref,
		},
	}
}

func TestNewBillingSessionSubscriptionFirstFallsBackToWalletOnGroupMismatch(t *testing.T) {
	truncate(t)
	ensureBillingSubscriptionTables(t)
	seedUser(t, 1, 100)
	seedPlanWithSubscription(t, 1, 11, 21, "vip", 100, 0)

	ctx, _ := gin.CreateTestContext(nil)
	session, apiErr := NewBillingSession(ctx, newBillingTestRelayInfo(1, "default", "subscription_first"), 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceWallet, session.funding.Source())
	require.Equal(t, 90, getUserQuota(t, 1))
	require.EqualValues(t, 0, getSubscriptionUsed(t, 21))
}
func TestNewBillingSessionUsesAutoGroupForSubscriptionMatch(t *testing.T) {
	truncate(t)
	ensureBillingSubscriptionTables(t)
	seedUser(t, 1, 100)
	seedPlanWithSubscription(t, 1, 14, 24, "vip", 100, 0)

	ctx, _ := gin.CreateTestContext(nil)
	relayInfo := newBillingTestRelayInfo(1, "auto", "subscription_first")
	ctx.Set("auto_group", "vip")
	_ = relayhelper.HandleGroupRatio(ctx, relayInfo)

	session, apiErr := NewBillingSession(ctx, relayInfo, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, "vip", relayInfo.UsingGroup)
	require.Equal(t, BillingSourceSubscription, session.funding.Source())
	require.Equal(t, 100, getUserQuota(t, 1))
	require.EqualValues(t, 10, getSubscriptionUsed(t, 24))
}


func TestNewBillingSessionSubscriptionOnlyRejectsGroupMismatch(t *testing.T) {
	truncate(t)
	ensureBillingSubscriptionTables(t)
	seedUser(t, 1, 100)
	seedPlanWithSubscription(t, 1, 12, 22, "vip", 100, 0)

	ctx, _ := gin.CreateTestContext(nil)
	session, apiErr := NewBillingSession(ctx, newBillingTestRelayInfo(1, "default", "subscription_only"), 10)
	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.Equal(t, 100, getUserQuota(t, 1))
	require.EqualValues(t, 0, getSubscriptionUsed(t, 22))
}

func TestNewBillingSessionWalletFirstFallsBackToSubscriptionWhenGroupMatches(t *testing.T) {
	truncate(t)
	ensureBillingSubscriptionTables(t)
	seedUser(t, 1, 0)
	seedPlanWithSubscription(t, 1, 13, 23, "vip", 100, 0)

	ctx, _ := gin.CreateTestContext(nil)
	session, apiErr := NewBillingSession(ctx, newBillingTestRelayInfo(1, "vip", "wallet_first"), 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceSubscription, session.funding.Source())
	require.Equal(t, 0, getUserQuota(t, 1))
	require.EqualValues(t, 10, getSubscriptionUsed(t, 23))
}

