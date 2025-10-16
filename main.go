package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	apiserver "github.com/vineetjain/game_engine/api_server"
	engine "github.com/vineetjain/game_engine/game_engine"
	mock "github.com/vineetjain/game_engine/mock_api"
)

func main() {
	var (
		port     = flag.String("port", "8080", "http listen port")
		runMock  = flag.Bool("run-mock", true, "start mock user engine")
		nUsers   = flag.Int("n", 1000, "number of mock users")
		minLagMs = flag.Int("min-lag-ms", 10, "min simulated network lag (ms)")
		maxLagMs = flag.Int("max-lag-ms", 1000, "max simulated network lag (ms)")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1) start game engine
	engine := engine.New(2048) // generous buffer for bursts
	engine.Start(ctx)

	// 2) start API server
	srv := apiserver.New(engine)
	httpSrv := &http.Server{
		Addr:         ":" + *port,
		Handler:      srv.Handler(),
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("[API] listening on :%s", *port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// 3) optionally run the mock user engine
	if *runMock {
		// brief warm-up so the listener is ready
		time.Sleep(150 * time.Millisecond)
		go func() {
			baseURL := "http://localhost:" + *port
			opts := mock.Options{
				N:            *nUsers,
				BaseURL:      baseURL,
				MinDelay:     time.Duration(*minLagMs) * time.Millisecond,
				MaxDelay:     time.Duration(*maxLagMs) * time.Millisecond,
				CorrectRatio: 0.25,
			}
			if err := mock.Run(ctx, opts); err != nil {
				log.Printf("[MOCK] run error: %v", err)
			}
		}()
	}

	// log metrics when a winner is declared (channel-driven, no polling)
	go func() {
		for id := range engine.WinnerEvents() {
			s := engine.StatsSnapshot()
			log.Printf("[METRICS] winner is user_id=%d \n total=%d correct=%d incorrect=%d",
				id, s.Total, s.Correct, s.Incorrect)
		}
	}()

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Printf("[MAIN] shutting down...")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	_ = httpSrv.Shutdown(shutdownCtx)
	cancel()
	time.Sleep(150 * time.Millisecond)
}
