package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/AGlElectronics/stash/backend/internal/testdb"
)

func TestOnlyOneLeaderRunsJobs(t *testing.T) {
	pool := testdb.New(t)
	RetryInterval = 100 * time.Millisecond
	var runsA, runsB atomic.Int32
	job := func(n *atomic.Int32) []Job {
		return []Job{{Name: "count", Every: 20 * time.Millisecond, Run: func(context.Context) error {
			n.Add(1)
			return nil
		}}}
	}
	ctxA, stopA := context.WithCancel(context.Background())
	ctxB, stopB := context.WithCancel(context.Background())
	defer stopB()
	go NewLeader(pool, zerolog.Nop(), job(&runsA)).Run(ctxA)
	time.Sleep(150 * time.Millisecond)
	go NewLeader(pool, zerolog.Nop(), job(&runsB)).Run(ctxB)
	time.Sleep(300 * time.Millisecond)

	if runsA.Load() == 0 || runsB.Load() != 0 {
		t.Fatalf("while A leads: A=%d B=%d, want only A", runsA.Load(), runsB.Load())
	}

	stopA()
	deadline := time.Now().Add(3 * time.Second)
	for runsB.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if runsB.Load() == 0 {
		t.Fatal("B did not take over after A stopped")
	}
}
