package misc

import "sync"

type NMap[kT1, kT2 comparable, vT any] struct {
	m  map[kT1]map[kT2]vT
	mu sync.RWMutex
}

func NewNMap[kT1, kT2 comparable, vT any]() NMap[kT1, kT2, vT] {
	return NMap[kT1, kT2, vT]{
		m: make(map[kT1]map[kT2]vT),
	}
}

func (nm *NMap[kT1, kT2, vT]) Set(key1 kT1, key2 kT2, val vT) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	subMap, ok := nm.m[key1]
	if !ok {
		subMap = make(map[kT2]vT)
		nm.m[key1] = subMap
	}
	subMap[key2] = val
}

func (nm *NMap[kT1, kT2, vT]) Get(key1 kT1, key2 kT2) (val vT, ok bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	subMap, ok := nm.m[key1]
	if !ok {
		return val, false
	}
	val, ok = subMap[key2]
	return
}

func (nm *NMap[kT1, kT2, vT]) DeleteChild(key1 kT1, key2 kT2) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	subMap, ok := nm.m[key1]
	if !ok {
		return
	}
	delete(subMap, key2)
	if len(subMap) == 0 {
		delete(nm.m, key1)
	}
}

func (nm *NMap[kT1, kT2, vT]) Delete(key1 kT1) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	delete(nm.m, key1)
}

func (nm *NMap[kT1, kT2, vT]) GetMapWithMutex() (map[kT1]map[kT2]vT, *sync.RWMutex) {
	return nm.m, &nm.mu
}
