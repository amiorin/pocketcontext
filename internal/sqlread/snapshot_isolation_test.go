package sqlread

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestSnapshotConcurrentRequesters(t *testing.T) {
	_, path, cfg, sc := snapshotFixture(t)
	sc.MaxConcurrent = 2 // Exercise both independent snapshots and the waiting queue.
	source := newTestSnapshot(t, path, cfg, sc)
	start := make(chan struct{})
	failures := make(chan error, 12)
	var requests sync.WaitGroup
	for i := 0; i < 12; i++ {
		requests.Add(1)
		go func(i int) {
			defer requests.Done()
			<-start
			user, want := "alice", int64(300)
			if i%2 != 0 {
				user, want = "bob", int64(900)
			}
			result, _, err := source.Query(context.Background(), user,
				"WITH visible AS (SELECT salary FROM pay) SELECT sum(salary) FROM visible")
			if err != nil {
				failures <- err
				return
			}
			if len(result.Rows) != 1 || len(result.Rows[0]) != 1 || result.Rows[0][0] != want {
				failures <- fmt.Errorf("requester %s received %#v; expected %d", user, result.Rows, want)
			}
		}(i)
	}
	close(start)
	requests.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	cleanSnapshot(t, source)
}
