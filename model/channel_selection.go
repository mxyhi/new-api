package model

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type channelLookupFunc func(channelID int) (*Channel, error)

type prioritizedCandidateBuckets struct {
	buckets prioritizedChannelBuckets
	lookup  channelLookupFunc
}

func selectNextChannelFromBuckets(group string, model string, candidates prioritizedCandidateBuckets, excludedChannelIDs map[int]struct{}) (*Channel, error) {
	for _, priority := range candidates.buckets.priorities {
		bucket := candidates.buckets.buckets[priority]
		channelID := channelSchedulers.getOrCreate(buildChannelSchedulerKey(group, model, priority), bucket.channelIDs, bucket.weights).next(excludedChannelIDs)
		if channelID == 0 {
			continue
		}
		channel, err := candidates.lookup(channelID)
		if err != nil {
			return nil, err
		}
		if channel == nil {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		return channel, nil
	}
	return nil, nil
}

func getCachedPrioritizedBuckets(group string, model string) prioritizedChannelBuckets {
	buckets := group2model2channels[group][model]
	if len(buckets.priorities) > 0 {
		return buckets
	}
	normalizedModel := ratio_setting.FormatMatchingModelName(model)
	if normalizedModel == "" || normalizedModel == model {
		return prioritizedChannelBuckets{}
	}
	return group2model2channels[group][normalizedModel]
}

func getCachedChannelLookup() channelLookupFunc {
	return func(channelID int) (*Channel, error) {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, nil
		}
		return channel, nil
	}
}

func buildAbilityPrioritizedBuckets(abilities []Ability) prioritizedChannelBuckets {
	if len(abilities) == 0 {
		return prioritizedChannelBuckets{}
	}
	maxPriority := abilities[0].GetPriority()
	for _, ability := range abilities[1:] {
		priority := ability.GetPriority()
		if priority > maxPriority {
			maxPriority = priority
		}
	}
	channelIDs := make([]int, 0, len(abilities))
	weights := make(map[int]int, len(abilities))
	for _, ability := range abilities {
		if ability.GetPriority() != maxPriority {
			continue
		}
		channelIDs = append(channelIDs, ability.ChannelId)
		weights[ability.ChannelId] = normalizeSchedulerWeight(int(ability.Weight))
	}
	if len(channelIDs) == 0 {
		return prioritizedChannelBuckets{}
	}
	sort.Ints(channelIDs)
	return prioritizedChannelBuckets{
		priorities: []int64{maxPriority},
		buckets: map[int64]channelPriorityBucket{
			maxPriority: {
				priority:   maxPriority,
				channelIDs: channelIDs,
				weights:    weights,
			},
		},
	}
}

func getDBChannelLookup() channelLookupFunc {
	return func(channelID int) (*Channel, error) {
		channel := Channel{}
		if err := DB.First(&channel, "id = ?", channelID).Error; err != nil {
			return nil, err
		}
		return &channel, nil
	}
}
