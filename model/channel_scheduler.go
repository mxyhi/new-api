package model

import (
	"fmt"
	"sort"
	"sync"
)

type channelPriorityBucket struct {
	priority   int64
	channelIDs []int
	weights    map[int]int
}

type channelSchedulerState struct {
	mu              sync.Mutex
	channelIDs      []int
	effectiveWeight map[int]int
	currentWeight   map[int]int
}

func newChannelSchedulerState(channelIDs []int, weights map[int]int) *channelSchedulerState {
	effectiveWeight := make(map[int]int, len(channelIDs))
	currentWeight := make(map[int]int, len(channelIDs))
	for _, channelID := range channelIDs {
		weight := normalizeSchedulerWeight(weights[channelID])
		effectiveWeight[channelID] = weight
		currentWeight[channelID] = 0
	}
	return &channelSchedulerState{
		channelIDs:      append([]int(nil), channelIDs...),
		effectiveWeight: effectiveWeight,
		currentWeight:   currentWeight,
	}
}

func normalizeSchedulerWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}

func (s *channelSchedulerState) next(excludedChannelIDs map[int]struct{}) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	selectedChannelID := 0
	selectedCurrentWeight := 0
	totalWeight := 0
	for _, channelID := range s.channelIDs {
		if _, excluded := excludedChannelIDs[channelID]; excluded {
			continue
		}
		weight := s.effectiveWeight[channelID]
		s.currentWeight[channelID] += weight
		totalWeight += weight
		if selectedChannelID == 0 || s.currentWeight[channelID] > selectedCurrentWeight {
			selectedChannelID = channelID
			selectedCurrentWeight = s.currentWeight[channelID]
		}
	}
	if selectedChannelID == 0 {
		return 0
	}
	s.currentWeight[selectedChannelID] -= totalWeight
	return selectedChannelID
}

type channelSchedulerRegistry struct {
	mu         sync.RWMutex
	schedulers map[string]*channelSchedulerState
}

func newChannelSchedulerRegistry() *channelSchedulerRegistry {
	return &channelSchedulerRegistry{schedulers: make(map[string]*channelSchedulerState)}
}

func (r *channelSchedulerRegistry) replace(schedulers map[string]*channelSchedulerState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.schedulers = schedulers
}

func (r *channelSchedulerRegistry) getOrCreate(key string, channelIDs []int, weights map[int]int) *channelSchedulerState {
	r.mu.RLock()
	scheduler, ok := r.schedulers[key]
	r.mu.RUnlock()
	if ok {
		return scheduler
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if scheduler, ok = r.schedulers[key]; ok {
		return scheduler
	}
	scheduler = newChannelSchedulerState(channelIDs, weights)
	r.schedulers[key] = scheduler
	return scheduler
}

type prioritizedChannelBuckets struct {
	priorities []int64
	buckets    map[int64]channelPriorityBucket
}

func buildPrioritizedChannelBuckets(channelIDs []int, channelByID map[int]*Channel) prioritizedChannelBuckets {
	bucketMap := make(map[int64]channelPriorityBucket)
	for _, channelID := range channelIDs {
		channel := channelByID[channelID]
		priority := channel.GetPriority()
		bucket := bucketMap[priority]
		bucket.priority = priority
		bucket.channelIDs = append(bucket.channelIDs, channelID)
		if bucket.weights == nil {
			bucket.weights = make(map[int]int)
		}
		bucket.weights[channelID] = normalizeSchedulerWeight(channel.GetWeight())
		bucketMap[priority] = bucket
	}
	priorities := make([]int64, 0, len(bucketMap))
	for priority := range bucketMap {
		priorities = append(priorities, priority)
		bucket := bucketMap[priority]
		sort.Ints(bucket.channelIDs)
		bucketMap[priority] = bucket
	}
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i] > priorities[j]
	})
	return prioritizedChannelBuckets{priorities: priorities, buckets: bucketMap}
}

func prioritizedBucketsContainChannelID(buckets prioritizedChannelBuckets, channelID int) bool {
	for _, priority := range buckets.priorities {
		bucket := buckets.buckets[priority]
		for _, id := range bucket.channelIDs {
			if id == channelID {
				return true
			}
		}
	}
	return false
}

func removeChannelIDFromPrioritizedBuckets(buckets prioritizedChannelBuckets, channelID int) prioritizedChannelBuckets {
	updatedBuckets := prioritizedChannelBuckets{
		priorities: make([]int64, 0, len(buckets.priorities)),
		buckets:    make(map[int64]channelPriorityBucket, len(buckets.buckets)),
	}
	for _, priority := range buckets.priorities {
		bucket := buckets.buckets[priority]
		filteredChannelIDs := make([]int, 0, len(bucket.channelIDs))
		filteredWeights := make(map[int]int, len(bucket.weights))
		for _, id := range bucket.channelIDs {
			if id == channelID {
				continue
			}
			filteredChannelIDs = append(filteredChannelIDs, id)
			filteredWeights[id] = bucket.weights[id]
		}
		if len(filteredChannelIDs) == 0 {
			continue
		}
		updatedBuckets.priorities = append(updatedBuckets.priorities, priority)
		updatedBuckets.buckets[priority] = channelPriorityBucket{
			priority:   priority,
			channelIDs: filteredChannelIDs,
			weights:    filteredWeights,
		}
	}
	return updatedBuckets
}

func rebuildSchedulersFromBuckets(groupedBuckets map[string]map[string]prioritizedChannelBuckets) map[string]*channelSchedulerState {
	schedulers := make(map[string]*channelSchedulerState)
	for group, modelBuckets := range groupedBuckets {
		for model, buckets := range modelBuckets {
			for _, priority := range buckets.priorities {
				bucket := buckets.buckets[priority]
				schedulers[buildChannelSchedulerKey(group, model, priority)] = newChannelSchedulerState(bucket.channelIDs, bucket.weights)
			}
		}
	}
	return schedulers
}

func buildChannelSchedulerKey(group string, model string, priority int64) string {
	return fmt.Sprintf("%s|%s|%d", group, model, priority)
}
