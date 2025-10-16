package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	engine "github.com/vineetjain/game_engine/game_engine"
)

type Server struct {
	engine *engine.GameEngine
	mux    *http.ServeMux
}

func New(e *engine.GameEngine) *Server {
	s := &Server{engine: e, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// winner: quick check
	s.mux.HandleFunc("/winner", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.engine.Winner()
		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			Found  bool  `json:"found"`
			UserID int64 `json:"user_id,omitempty"`
		}{Found: ok, UserID: id}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// NEW: /metrics – live counters
	s.mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.engine.StatsSnapshot())
	})

	s.mux.HandleFunc("/submit", s.handleSubmit)
}

func (s *Server) Handler() http.Handler { return s.mux }

type submitReq struct {
	UserID  int64 `json:"user_id"`
	Correct bool  `json:"correct"`
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req submitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := validate(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.engine.Submit(engine.Submission{
		UserID:  req.UserID,
		Correct: req.Correct,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"queued_at": time.Now().UTC(),
		"status":    "accepted",
	})
}

func validate(req submitReq) error {
	if req.UserID <= 0 {
		return errors.New("user_id must be positive")
	}
	return nil
}
