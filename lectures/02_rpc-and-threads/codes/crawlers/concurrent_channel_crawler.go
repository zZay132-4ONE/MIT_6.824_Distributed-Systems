// (3) Concurrent crawler with channels

package crawlers

// ConcurrentChannel conducts url-fetching concurrently using go channels.
func ConcurrentChannel(url string, fetcher Fetcher) {
	ch := make(chan []string)
	go func() {
		ch <- []string{url}
	}()
	coordinator(ch, fetcher)
}

// worker represents a worker goroutine responsible for fetching urls
func worker(url string, ch chan []string, fetcher Fetcher) {
	urls, err := fetcher.Fetch(url)
	if err != nil {
		ch <- []string{}
	} else {
		ch <- urls
	}
}

// coordinator manages a pool of worker goroutines using channels for communication
func coordinator(ch chan []string, fetcher Fetcher) {
	// Loop over the channel to receive urls and starts new workers for non-retrieved urls
	n := 1
	fetched := make(map[string]bool)
	for urls := range ch {
		for _, u := range urls {
			if fetched[u] == false {
				fetched[u] = true
				n += 1
				go worker(u, ch, fetcher)
			}
		}
		n -= 1
		// Check if all workers have finished
		if n == 0 {
			break
		}
	}
}
