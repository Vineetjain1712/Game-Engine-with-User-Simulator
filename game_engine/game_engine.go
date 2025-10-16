package engine

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
)

type Submission struct {
	UserID  int64 `json:"user_id"`
	Correct bool  `json:"correct"`
}

type Stats struct {
	Total     uint64 `json:"total"`
	Correct   uint64 `json:"correct"`
	Incorrect uint64 `json:"incorrect"`
	WinnerID  int64  `json:"winner_id,omitempty"`
}

// GameEngine evaluates submissions and declares exactly one winner.
// Event-driven: a single goroutine consumes from subCh.
type GameEngine struct {
	// winner + one-time announce
	winnerID   atomic.Int64
	announceDo sync.Once

	// metrics
	total     atomic.Uint64
	correct   atomic.Uint64
	incorrect atomic.Uint64

	// channels
	subCh chan Submission
	done  chan struct{}

	// optional: winner event (send once)
	winnerOnce sync.Once
	winnerCh   chan int64
}

func New(buffer int) *GameEngine {
	return &GameEngine{
		subCh:    make(chan Submission, buffer),
		done:     make(chan struct{}),
		winnerCh: make(chan int64, 1),
	}
}

// Start spins the evaluation loop until ctx is done.
func (g *GameEngine) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case s := <-g.subCh:
				g.eval(s)
			case <-ctx.Done():
				close(g.done)
				return
			}
		}
	}()
}

func (g *GameEngine) eval(s Submission) {
	g.total.Add(1)

	if s.Correct {
		g.correct.Add(1)

		// CAS ensures exactly one winner under concurrency.
		if g.winnerID.Load() == 0 && g.winnerID.CompareAndSwap(0, s.UserID) {
			g.announceDo.Do(func() {
				log.Printf("[GAME] Winner is user_id=%d 🎉", s.UserID)
			})
			g.winnerOnce.Do(func() {
				g.winnerCh <- s.UserID
				close(g.winnerCh)
			})
		}
		return
	}

	g.incorrect.Add(1)
}

// Submit forwards a submission for real-time evaluation.
func (g *GameEngine) Submit(s Submission) {
	g.subCh <- s
}

// Winner returns (id, ok).
func (g *GameEngine) Winner() (int64, bool) {
	id := g.winnerID.Load()
	return id, id != 0
}

// WinnerEvents returns a read-only channel that will receive the winner's id once.
func (g *GameEngine) WinnerEvents() <-chan int64 { return g.winnerCh }

// StatsSnapshot atomically snapshots current metrics.
func (g *GameEngine) StatsSnapshot() Stats {
	return Stats{
		Total:     g.total.Load(),
		Correct:   g.correct.Load(),
		Incorrect: g.incorrect.Load(),
		WinnerID:  g.winnerID.Load(),
	}
}
