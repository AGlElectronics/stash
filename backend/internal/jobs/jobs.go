// Package jobs runs Stash's background work on exactly one replica. The
// replica that holds a PostgreSQL advisory lock is the leader; it runs the
// periodic jobs and the Telegram long poller. When it stops or loses its
// database connection, the lock is released and another replica takes
// over within RetryInterval.
package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// LockKey is the advisory lock that elects the leader.
const LockKey int64 = 0x57a5_4_1ead

// RetryInterval is how often a follower tries to become the leader, and
// how often the leader checks it still holds the lock.
var RetryInterval = 30 * time.Second

// Job runs every Every while this replica is the leader, first right after
// it becomes the leader.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(ctx context.Context) error
}

// Runner is long-running work for the leader only (e.g. the Telegram
// poller); it must return when ctx ends.
type Runner func(ctx context.Context)

// Leader elects a leader and runs jobs and runners on it.
type Leader struct {
	pool    *pgxpool.Pool
	log     zerolog.Logger
	jobs    []Job
	runners []Runner
}

func NewLeader(pool *pgxpool.Pool, log zerolog.Logger, jobs []Job, runners ...Runner) *Leader {
	return &Leader{pool: pool, log: log, jobs: jobs, runners: runners}
}

// Run blocks until ctx ends.
func (l *Leader) Run(ctx context.Context) {
	for ctx.Err() == nil {
		l.lead(ctx)
		select {
		case <-ctx.Done():
		case <-time.After(RetryInterval):
		}
	}
}

// lead tries to take the lock and, if it gets it, runs everything until the
// lock connection fails or ctx ends.
func (l *Leader) lead(ctx context.Context) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, LockKey).Scan(&got); err != nil || !got {
		return
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, LockKey)
	}()
	l.log.Info().Msg("jobs: this replica is the leader")

	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{}, len(l.jobs)+len(l.runners))
	for _, j := range l.jobs {
		go func(j Job) {
			defer func() { done <- struct{}{} }()
			l.loop(lctx, j)
		}(j)
	}
	for _, r := range l.runners {
		go func(r Runner) {
			defer func() { done <- struct{}{} }()
			r(lctx)
		}(r)
	}

	t := time.NewTicker(RetryInterval)
	defer t.Stop()
	for lctx.Err() == nil {
		select {
		case <-lctx.Done():
		case <-t.C:
			if err := conn.Ping(lctx); err != nil {
				l.log.Warn().Err(err).Msg("jobs: lost the leader connection")
				cancel()
			}
		}
	}
	for range len(l.jobs) + len(l.runners) {
		<-done
	}
}

func (l *Leader) loop(ctx context.Context, j Job) {
	t := time.NewTicker(j.Every)
	defer t.Stop()
	for {
		start := time.Now()
		if err := j.Run(ctx); err != nil && ctx.Err() == nil {
			l.log.Error().Err(err).Str("job", j.Name).Msg("jobs: run failed")
		} else if ctx.Err() == nil {
			l.log.Debug().Str("job", j.Name).Dur("took", time.Since(start)).Msg("jobs: run done")
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
