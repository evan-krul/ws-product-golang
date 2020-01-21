package rate_limit

import (
	"fmt"
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"sync"
	"time"
)

/**
Struct for visited map
*/
type user struct {
	limiter     *rate.Limiter
	lastVisited time.Time
}

var (
	// Map of visitors to site (key: IP, value: {limiter, lastVisitedTime})
	visitors = make(map[string]*user)

	visitMutex sync.Mutex
)

/**
create a Go routine to regularly cleans up the visited map
*/
func init() {
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				err := cleanUpVisitorsMap()
				if err != nil {
					fmt.Println(err)
					close(quit)
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

/**
Adapted from: https://www.alexedwards.net/blog/how-to-rate-limit-http-requests
Implementation of a decorator pattern to act as middleware for rate limiter
*/

func LimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// get user IP
		ip, _, err := net.SplitHostPort(request.RemoteAddr)
		if err != nil {
			http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		}

		// Check if IP has exceeded their rate limit
		// If they have send StatusTooManyRequests
		// Else continue to next part of service handler (decorator)
		if !checkUser(ip).Allow() {
			http.Error(responseWriter, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(responseWriter, request)
	})
}

/**
Checks map for IP
If IP does not exist in map, add a record with a new limiter and time else update visited time
*/
func checkUser(ip string) *rate.Limiter {
	visitMutex.Lock()
	// When function returns, unlock visit_mutex
	defer visitMutex.Unlock()

	// if this is a new user, create a new map entry
	visitor, exists := visitors[ip]
	if !exists {
		// Limiter sets rate limit parameters`
		// If this was deployed on docker I would make this an environment variable to configure
		limiter := rate.NewLimiter(1, 5)
		visitors[ip] = &user{
			limiter:     limiter,
			lastVisited: time.Now(),
		}
		return limiter
	}

	visitor.lastVisited = time.Now()
	return visitor.limiter
}

/**
Unlike my Node rate limiter that uses Redis this does not expire entries so on an interval I clean up the map
*/
func cleanUpVisitorsMap() error {
	visitMutex.Lock()
	defer visitMutex.Unlock()
	// check each entry in visitors map
	// if that IP has not visited in past 5 minutes delete the entry to maintain small map
	for ip, visitor := range visitors {
		if time.Now().Sub(visitor.lastVisited) > 5*time.Minute {
			delete(visitors, ip)
		}
	}
	return nil
}
