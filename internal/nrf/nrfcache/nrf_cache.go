// SPDX-FileCopyrightText: 2022 Infosys Limited
//

package nrf_cache

import (
	"container/heap"
	"sync"
	"time"

	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/nrf/logger"
)

const defaultCacheTTl = time.Hour
const defaultNfProfileTTl = time.Minute

type NfProfileItem struct {
	nfProfile  *models.NfProfile
	expiryTime time.Time
	ttl        time.Duration
	index      int // index of the entry in the priority queue
}

// isExpired - returns true if the expiry time has passed.
func (item *NfProfileItem) isExpired() bool {
	return item.expiryTime.Before(time.Now())
}

// updateExpiryTime - sets new expiry time based on the current time
func (item *NfProfileItem) updateExpiryTime() {
	item.expiryTime = time.Now().Add(time.Second * item.ttl)
}

func newNfProfileItem(profile *models.NfProfile, ttl time.Duration) *NfProfileItem {
	item := &NfProfileItem{
		nfProfile: profile,
		ttl:       ttl,
	}
	item.updateExpiryTime()
	return item
}

// NfProfilePriorityQ : Priority Queue to store the profile. Queue is ordered by expiry time
type NfProfilePriorityQ []*NfProfileItem

// Len - Number of entries in the priority queue
func (npq NfProfilePriorityQ) Len() int {
	return len(npq)
}

// Less - Comparator for the sort interface used by the heap.
// entries will be sorted by increasing order of expiry time
func (npq NfProfilePriorityQ) Less(i, j int) bool {
	return npq[i].expiryTime.Before(npq[j].expiryTime)
}

// Swap - implemented for the sort interface used by the heap pkg.
// swaps the element at i and j.
func (npq NfProfilePriorityQ) Swap(i, j int) {
	npq[i], npq[j] = npq[j], npq[i]
	npq[i].index = i
	npq[j].index = j
}

// at - returns the element at index i
func (npq NfProfilePriorityQ) at(index int) *NfProfileItem {
	return npq[index]
}

// push - adds an entry to the priority queue. Invokes heap api to
// push the entry to the correct location in the queue
func (npq *NfProfilePriorityQ) push(item interface{}) {
	heap.Push(npq, item)
}

// update - update fields of existing entry. Invokes heap.Fix to re-establish the ordering.
func (npq *NfProfilePriorityQ) update(item *NfProfileItem, value *models.NfProfile, ttl time.Duration) {
	item.nfProfile = value
	item.ttl = ttl
	item.updateExpiryTime()
	heap.Fix(npq, item.index)
}

// remove -removes an entry at given index.
func (npq *NfProfilePriorityQ) remove(item *NfProfileItem) {
	heap.Remove(npq, item.index)
}

// Push - implemented for heap interface. appends an element to the priority queue
func (npq *NfProfilePriorityQ) Push(item interface{}) {
	n := len(*npq)
	entry := item.(*NfProfileItem)
	entry.index = n
	*npq = append(*npq, entry)
}

// Pop - implemented for heap interface. removes the entry with least expiry time
func (npq *NfProfilePriorityQ) Pop() interface{} {
	old := *npq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*npq = old[0 : n-1]
	return item
}

// newNfProfilePriorityQ - New prority queue for storing NF Profiles.
func newNfProfilePriorityQ() *NfProfilePriorityQ {
	q := &NfProfilePriorityQ{}
	heap.Init(q)
	return q
}

// NrfCache : cache of nf profiles
type NrfCache struct {
	cache map[string]*NfProfileItem // map[nf-instance-id] =*NfProfile

	priorityQ *NfProfilePriorityQ // sorted by expiry time

	evictionTicker *time.Ticker

	done chan struct{}

	nrfDiscoveryQueryCb NrfDiscoveryQueryCb // nrf query callback

	evictionInterval time.Duration // timer interval in which the cache is checked for eviction of expired entries

	mutex sync.RWMutex
}

// handleLookup - Checks if the cache has nf cache entry corresponding to the parameters specified.
// If entry does not exist, perform nrf discovery query. To avoid concurrency issues,
// nrf discovery query is mutex protected.
func (c *NrfCache) handleLookup(nrfUri string, targetNfType, requestNfType models.NfType, param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (models.SearchResult, error) {
	var searchResult models.SearchResult
	var err error

	c.mutex.RLock()
	searchResult.NfInstances = c.get(param)
	c.mutex.RUnlock()

	if len(searchResult.NfInstances) == 0 {
		logger.UtilLog.Tracef("Cache miss for nftype %s", targetNfType)

		c.mutex.Lock()
		defer c.mutex.Unlock()
		searchResult.NfInstances = c.get(param)
		if len(searchResult.NfInstances) == 0 {
			searchResult, err = c.nrfDiscoveryQueryCb(nrfUri, targetNfType, requestNfType, param)
			if err != nil {
				return searchResult, err
			}

			for i := 0; i < len(searchResult.NfInstances); i++ {
				c.set(&searchResult.NfInstances[i], time.Duration(searchResult.ValidityPeriod))
			}
		}
	}
	return searchResult, err
}

// set - Adds nf profile entry to the map and the priority queue
func (c *NrfCache) set(nfProfile *models.NfProfile, ttl time.Duration) {
	if ttl == 0 {
		ttl = defaultNfProfileTTl
	}

	item, exists := c.cache[nfProfile.NfInstanceId]
	if exists {
		// if item.isExpired()
		c.priorityQ.update(item, nfProfile, ttl)
	} else {
		newItem := newNfProfileItem(nfProfile, ttl)
		c.cache[nfProfile.NfInstanceId] = newItem
		c.priorityQ.push(newItem)
	}
}

// get - checks if nf profile corresponding to the search opts exist in the cache.
func (c *NrfCache) get(opts *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) []models.NfProfile {
	var nfProfiles []models.NfProfile

	for _, element := range c.cache {
		if !element.isExpired() {
			if opts != nil {
				cb, ok := matchFilters[element.nfProfile.NfType]
				if ok {
					matchFound, err := cb(element.nfProfile, opts)
					if err != nil {
						logger.UtilLog.Errorf("match filter returned error %v", err)
					} else if matchFound {
						nfProfiles = append(nfProfiles, *(element.nfProfile))
					}
				}
			} else {
				nfProfiles = append(nfProfiles, *(element.nfProfile))
			}
		}
	}
	return nfProfiles
}

// removeByNfInstanceId - removes nf profile with nfInstanceId from the cache and queue
func (c *NrfCache) removeByNfInstanceId(nfInstanceId string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	NfProfileItem, rc := c.cache[nfInstanceId]
	if rc {
		c.remove(NfProfileItem)
	}
	return rc
}

// remove -
func (c *NrfCache) remove(item *NfProfileItem) {
	c.priorityQ.remove(item)
	delete(c.cache, item.nfProfile.NfInstanceId)
}

// cleanupExpiredItems - removes the profiles with expired TTLs
func (c *NrfCache) cleanupExpiredItems() {
	logger.UtilLog.Infoln("nrf cache: cleanup expired items")

	for item := c.priorityQ.at(0); item.isExpired(); {

		logger.UtilLog.Tracef("evicted nf instance %s", item.nfProfile.NfInstanceId)

		c.remove(item)
		if c.priorityQ.Len() == 0 {
			break
		} else {
			item = c.priorityQ.at(0)
		}
	}
}

// purge - release the cache and its resources.
func (c *NrfCache) purge() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	close(c.done)
	c.priorityQ = newNfProfilePriorityQ()
	c.cache = make(map[string]*NfProfileItem)
	c.evictionTicker.Stop()
}

func (c *NrfCache) startExpiryProcessing() {
	for {
		select {
		case <-c.evictionTicker.C:
			c.mutex.Lock()
			if c.priorityQ.Len() == 0 {
				c.mutex.Unlock()
				continue
			}

			c.cleanupExpiredItems()
			c.mutex.Unlock()

		case <-c.done:
			return
		}
	}
}

func NewNrfCache(duration time.Duration, dbqueryCb NrfDiscoveryQueryCb) *NrfCache {
	cache := &NrfCache{
		cache:               make(map[string]*NfProfileItem),
		priorityQ:           newNfProfilePriorityQ(),
		evictionInterval:    defaultCacheTTl,
		nrfDiscoveryQueryCb: dbqueryCb,
		done:                make(chan struct{}),
	}

	cache.evictionTicker = time.NewTicker(duration)

	go cache.startExpiryProcessing()

	return cache
}

type NrfMasterCache struct {
	nrfDiscoveryQueryCb NrfDiscoveryQueryCb
	nfTypeToCacheMap    map[models.NfType]*NrfCache
	evictionInterval    time.Duration
	mutex               sync.Mutex
}

func (c *NrfMasterCache) GetNrfCacheInstance(targetNfType models.NfType) *NrfCache {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	cache, exists := c.nfTypeToCacheMap[targetNfType]
	if exists == false {
		logger.UtilLog.Infof("Creating cache for nftype %v", targetNfType)

		cache = NewNrfCache(c.evictionInterval, c.nrfDiscoveryQueryCb)
		c.nfTypeToCacheMap[targetNfType] = cache
	}
	return cache
}

func (c *NrfMasterCache) clearNrfMasterCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for k, cache := range c.nfTypeToCacheMap {
		cache.purge()
		delete(c.nfTypeToCacheMap, k)
	}
}

func (c *NrfMasterCache) removeNfProfile(nfInstanceId string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var ok bool
	for _, nrfCache := range c.nfTypeToCacheMap {
		if ok = nrfCache.removeByNfInstanceId(nfInstanceId); ok {
			break
		}
	}
	return ok
}

var masterCache *NrfMasterCache

type NrfDiscoveryQueryCb func(nrfUri string, targetNfType, requestNfType models.NfType, param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (models.SearchResult, error)

func InitNrfCaching(interval time.Duration, cb NrfDiscoveryQueryCb) {
	m := &NrfMasterCache{
		nfTypeToCacheMap:    make(map[models.NfType]*NrfCache),
		evictionInterval:    interval,
		nrfDiscoveryQueryCb: cb,
	}
	masterCache = m
}

func disableNrfCaching() {
	masterCache.clearNrfMasterCache()
	masterCache = nil
}

func SearchNFInstances(nrfUri string, targetNfType, requestNfType models.NfType, param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (models.SearchResult, error) {

	logger.UtilLog.Traceln("SearchNFInstances nrf cache")

	var searchResult models.SearchResult
	var err error

	c := masterCache.GetNrfCacheInstance(targetNfType)
	if c != nil {
		searchResult, err = c.handleLookup(nrfUri, targetNfType, requestNfType, param)
	} else {
		logger.UtilLog.Infoln("Failed to find cache for nftype")
	}

	for _, np := range searchResult.NfInstances {
		logger.UtilLog.Tracef("%v", np)
	}

	return searchResult, err

}

func RemoveNfProfileFromNrfCache(nfInstanceId string) bool {
	return masterCache.removeNfProfile(nfInstanceId)
}
