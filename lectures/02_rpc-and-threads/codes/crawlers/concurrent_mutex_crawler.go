// (2) Concurrent Crawler with shared state and Mutex

package crawlers

import "sync"

type fetchState struct {
	fetched map[string]bool
	mu      sync.Mutex
}

// ConcurrentMutex conducts url-fetching concurrently using mutex.
func ConcurrentMutex(url string, fetcher Fetcher, fs *fetchState) {
	if fs.testAndSet(url) {
		return
	}
	urls, err := fetcher.Fetch(url)
	if err != nil {
		return
	}
	var done sync.WaitGroup
	for _, u := range urls {
		done.Add(1)
		go func(u string) {
			defer done.Done()
			ConcurrentMutex(u, fetcher, fs)
		}(u)
	}
	done.Wait()
	return
}

// makeState returns a newly-initialized fetchState instance.
func makeState() *fetchState {
	return &fetchState{fetched: make(map[string]bool)}
}

// testAndSet checks the current state associated with a URL, sets it to true, and returns the original state.
func (fs *fetchState) testAndSet(url string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	r := fs.fetched[url]
	fs.fetched[url] = true
	return r
}
