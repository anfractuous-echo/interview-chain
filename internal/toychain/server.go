package toychain

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"interview-chain/internal/transaction"
)

type Server struct {
	ledger *Ledger
	mux    *http.ServeMux
}

func NewServer(ledger *Ledger) *Server {
	s := &Server{ledger: ledger, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /transactions", s.postTransaction)
	s.mux.HandleFunc("GET /transactions/{id}", s.getTransaction)
	s.mux.HandleFunc("GET /accounts/{address}", s.getAccount)
	s.mux.HandleFunc("GET /blocks", s.getBlocks)
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return s
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) postTransaction(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var tx transaction.Signed
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tx); err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction JSON")
		return
	}
	result, err := s.ledger.Submit(tx)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnknownAccount):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrInsufficientFunds), errors.Is(err, ErrBalanceOverflow), errors.Is(err, ErrTransactionConflict):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	status := http.StatusAccepted
	if result.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) getTransaction(w http.ResponseWriter, r *http.Request) {
	result, err := s.ledger.Transaction(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getAccount(w http.ResponseWriter, r *http.Request) {
	account, err := s.ledger.Account(r.PathValue("address"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) getBlocks(w http.ResponseWriter, r *http.Request) {
	after, err := parseInt64(r.URL.Query().Get("after_height"), 0)
	if err != nil || after < 0 {
		writeError(w, http.StatusBadRequest, "invalid after_height")
		return
	}
	limit64, err := parseInt64(r.URL.Query().Get("limit"), 100)
	if err != nil || limit64 < 1 || limit64 > 1000 {
		writeError(w, http.StatusBadRequest, "limit must be between 1 and 1000")
		return
	}
	writeJSON(w, http.StatusOK, s.ledger.BlocksAfter(after, int(limit64)))
}

func parseInt64(raw string, fallback int64) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse integer: %w", err)
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
