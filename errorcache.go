package sweet

type ErrorCache struct {
	Updates  chan *ErrorCacheUpdate
	Requests chan *ErrorCacheRequest
}
type ErrorCacheUpdate struct {
	Hostname     string
	ErrorMessage string
}

type ErrorCacheRequest struct {
	Hostname string
	Response chan string
}

////
func RunErrorCache() (ErrorCache, error) {
	cache := make(map[string]string)

	ec := new(ErrorCache)
	ec.Updates = make(chan *ErrorCacheUpdate)
	ec.Requests = make(chan *ErrorCacheRequest)

	go func() {
		for {
			select {
			case update := <-ec.Updates:
				if len(update.ErrorMessage) > 0 { // no errors - remove from cache
					cache[update.Hostname] = update.ErrorMessage
				} else { // errors - add to cache
					delete(cache, update.Hostname)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case request := <-ec.Requests:
				conf, exists := cache[request.Hostname]
				if exists {
					request.Response <- conf
				} else {
					request.Response <- ""
				}
			}
		}
	}()

	return *ec, nil
}
