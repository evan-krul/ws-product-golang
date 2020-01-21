package main

import (
	"eq/rate_limit"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

/**
Struct to keep track of individual count
- Lockable with mutex
*/
type counter struct {
	sync.Mutex
	View  int
	Click int
}

/**
Struct for map of counts
- Lockable with mutex (for syncing to backing store)
*/
type counters struct {
	sync.Mutex
	countersMap map[string]*counter
}

var (
	// Map of the counters in a counters struct
	countersStruct = counters{
		countersMap: make(map[string]*counter),
	}

	// content options available
	content = []string{"sports", "entertainment", "business", "education"}
)

/**
Default handler
route: /
*/
func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome to EQ Works 😎")
}

/**
Handler for view requests
route: /view
*/
func viewHandler(w http.ResponseWriter, r *http.Request) {
	// Pick random data item
	data := content[rand.Intn(len(content))]
	// generate time key
	timeKey := time.Now().Format("2006-1-2 15:04")
	// Key for map
	clickMapKey := fmt.Sprintf("%s:%s", data, timeKey)

	// If they key does not exist in the map we create a new counter
	if countersStruct.countersMap[clickMapKey] == nil {
		countersStruct.countersMap[clickMapKey] = &counter{}
	}
	// increment count
	countersStruct.countersMap[clickMapKey].Lock()
	countersStruct.countersMap[clickMapKey].View += 1
	countersStruct.countersMap[clickMapKey].Unlock()

	err := processRequest(r)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(400)
		return
	}

	// simulate random click call
	if rand.Intn(100) < 50 {
		err = processClick(clickMapKey)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(400)
			return
		}
	}
}

func processRequest(r *http.Request) error {
	time.Sleep(time.Duration(rand.Int31n(50)) * time.Millisecond)
	return nil
}

/**
Increase click value for counter in map
*/
func processClick(mapKey string) error {
	countersStruct.countersMap[mapKey].Lock()
	countersStruct.countersMap[mapKey].Click += 1
	countersStruct.countersMap[mapKey].Unlock()

	return nil
}

/**
Stats handler not implemented
*/
func statsHandler(w http.ResponseWriter, r *http.Request) {
	if !isAllowed() {
		w.WriteHeader(429)
		return
	}
}

func isAllowed() bool {
	return true
}

/**
Mock function to upload counter info to a backing store ie elastic or redis
Called on an 5 second interval
*/
func uploadCounters() error {
	countersStruct.Lock()
	// Copy map to backing store
	// To do this I would copy the map in the mutex, clear it,
	//then after I have unlocked the mutex I would upload to a service like elastic search

	// Clear map to prevent it from getting to large
	// This just dereferences the old map and makes a new one. Go's GC will clear the old one on when it is ready
	countersStruct.countersMap = make(map[string]*counter)
	countersStruct.Unlock()

	// Upload map copy here
	return nil
}

func main() {
	httpMux := http.NewServeMux()
	// Declare routes and their handlers
	httpMux.HandleFunc("/", welcomeHandler)
	httpMux.HandleFunc("/view/", viewHandler)
	httpMux.HandleFunc("/stats/", statsHandler)

	log.Fatal(http.ListenAndServe(":8080", rate_limit.LimitMiddleware(httpMux)))
}

/**
Backing store ticker runs as routine on 5 second interval
*/
func init() {
	// uses ticker channel, adapted from: https://stackoverflow.com/questions/16466320/is-there-a-way-to-do-repetitive-tasks-at-intervals
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				err := uploadCounters()
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
