package buildmgr

import (
	"context"
	"log"
	"sync"

	"github.com/glennswest/rosecicd/internal/builder"
	"github.com/glennswest/rosecicd/internal/config"
)

// BuildJob is a unit of work for a builder queue.
type BuildJob struct {
	Build    *Build
	Spec     builder.BuildSpec
	Repo     config.RepoConfig
}

// BuildQueue is a per-builder FIFO queue. One build runs at a time.
type BuildQueue struct {
	builder  Builder
	mu       sync.Mutex
	queue    []*BuildJob
	running  *BuildJob
	jobCh    chan struct{}
	cancel   context.CancelFunc
}

// NewBuildQueue creates a queue backed by the given builder.
func NewBuildQueue(b Builder) *BuildQueue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &BuildQueue{
		builder: b,
		jobCh:   make(chan struct{}, 1),
		cancel:  cancel,
	}
	go q.processLoop(ctx)
	return q
}

// Enqueue adds a job to the queue and returns the queue position (1-based, 0 means running immediately).
func (q *BuildQueue) Enqueue(job *BuildJob) int {
	q.mu.Lock()
	q.queue = append(q.queue, job)
	pos := len(q.queue)
	if q.running != nil {
		pos++ // account for currently running job
	}
	q.mu.Unlock()

	// Signal the process loop
	select {
	case q.jobCh <- struct{}{}:
	default:
	}
	return pos
}

// QueuePosition returns 0 if running, 1+ if waiting, -1 if not found.
func (q *BuildQueue) QueuePosition(buildID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running != nil && q.running.Build.ID == buildID {
		return 0
	}
	for i, j := range q.queue {
		if j.Build.ID == buildID {
			return i + 1
		}
	}
	return -1
}

// QueueStatus returns the currently running job and the waiting queue.
func (q *BuildQueue) QueueStatus() (running *BuildJob, queued []*BuildJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	queued = make([]*BuildJob, len(q.queue))
	copy(queued, q.queue)
	return q.running, queued
}

// QueueDepth returns the number of waiting jobs (not including the running one).
func (q *BuildQueue) QueueDepth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// Stop shuts down the queue.
func (q *BuildQueue) Stop() {
	q.cancel()
}

func (q *BuildQueue) processLoop(ctx context.Context) {
	for {
		// Grab the next job
		q.mu.Lock()
		if len(q.queue) == 0 {
			q.mu.Unlock()
			// Wait for a signal or cancellation
			select {
			case <-ctx.Done():
				return
			case <-q.jobCh:
				continue
			}
		}
		job := q.queue[0]
		q.queue = q.queue[1:]
		q.running = job
		q.mu.Unlock()

		// Execute the build
		q.runJob(ctx, job)

		q.mu.Lock()
		q.running = nil
		q.mu.Unlock()
	}
}

func (q *BuildQueue) runJob(ctx context.Context, job *BuildJob) {
	job.Build.Status = StatusRunning
	job.Build.BuilderName = q.builder.Name()
	log.Printf("[queue/%s] starting build %s for %s", q.builder.Name(), job.Build.ID, job.Build.RepoName)

	logs, err := q.builder.Run(ctx, job.Spec, job.Build.ID)

	job.Build.Logs = logs
	if err != nil {
		job.Build.Status = StatusFailed
		job.Build.Error = err.Error()
		log.Printf("[queue/%s] build %s failed: %v", q.builder.Name(), job.Build.ID, err)
	} else {
		job.Build.Status = StatusSuccess
		log.Printf("[queue/%s] build %s succeeded", q.builder.Name(), job.Build.ID)
	}
}
