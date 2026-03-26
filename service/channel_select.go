package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

func GetUsedChannelIDs(c *gin.Context) map[int]struct{} {
	usedChannels := common.GetContextKeyStringSlice(c, constant.ContextKeyUsedChannels)
	if len(usedChannels) == 0 {
		return nil
	}
	usedChannelIDs := make(map[int]struct{}, len(usedChannels))
	for _, channelIDText := range usedChannels {
		var channelID int
		if _, err := fmt.Sscanf(channelIDText, "%d", &channelID); err != nil {
			continue
		}
		usedChannelIDs[channelID] = struct{}{}
	}
	if len(usedChannelIDs) == 0 {
		return nil
	}
	return usedChannelIDs
}

// CacheGetNextSatisfiedChannel returns the next channel to try according to
// priority layering + weighted round robin within the same priority.
// 按优先级分层，并在同优先级层内执行加权轮询，获取下一个应尝试的渠道。
//
// For "auto" tokenGroup:
// 对于 "auto" tokenGroup：
//
//   - Each group will exhaust all remaining channels before moving to the next group.
//     Channel selection inside a group follows: exhaust same-priority channels first,
//     then fallback to lower priorities.
//     每个分组都会先耗尽当前剩余渠道，之后才会切换到下一个分组。
//     组内选路遵循：先在当前最高优先级层内按权重轮询并穷尽该层，再降级到更低优先级。
//
//   - Uses ContextKeyAutoGroupIndex to track the current group index.
//     使用 ContextKeyAutoGroupIndex 跟踪当前分组索引。
//
//   - Used channels are tracked in request context, and excluded from subsequent retries.
//     已尝试过的渠道会记录在请求上下文里，并在后续重试时排除。

func CacheGetNextSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	var channel *model.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.TokenGroup == "auto" {
		if len(setting.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := GetUserAutoGroup(userGroup)
		startGroupIndex := 0
		crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
		usedChannelIDs := GetUsedChannelIDs(param.Ctx)

		if lastGroupIndex, exists := common.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, retry: %d", autoGroup, param.GetRetry())

			channel, err = model.GetNextSatisfiedChannel(autoGroup, param.ModelName, usedChannelIDs)
			if err != nil {
				return nil, autoGroup, err
			}
			if channel == nil {
				// 当前分组没有剩余可用渠道，尝试下一个分组。
				// 这里的“没有剩余渠道”已经隐含了：同优先级已穷尽，且低优先级也已穷尽。
				logger.LogDebug(param.Ctx, "No remaining channel in group %s for model %s, trying next group", autoGroup, param.ModelName)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, param.GetRetry())
				if crossGroupRetry && i > startGroupIndex {
					param.ResetRetryNextTry()
				}
				continue
			}

			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			selectGroup = autoGroup
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)
			break
		}
	} else {
		channel, err = model.GetNextSatisfiedChannel(param.TokenGroup, param.ModelName, GetUsedChannelIDs(param.Ctx))
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}

	return channel, selectGroup, nil
}
