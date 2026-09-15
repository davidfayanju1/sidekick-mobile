// Package httpapi — Phase 7 finance handlers.
package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sidekick/backend/internal/finance"
	"github.com/sidekick/backend/internal/webhook"
)

func (s *Server) registerFinanceRoutes(mux *http.ServeMux) {
	mux.Handle("GET /me/wallet", s.requireAuth(http.HandlerFunc(s.handleWallet)))
	mux.Handle("GET /me/transactions", s.requireAuth(http.HandlerFunc(s.handleTransactions)))
	mux.Handle("GET /me/payment-methods", s.requireAuth(http.HandlerFunc(s.handleListCards)))
	mux.Handle("POST /me/payment-methods", s.requireAuth(http.HandlerFunc(s.handleAddCard)))
	mux.Handle("DELETE /me/payment-methods/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteCard)))
	mux.Handle("GET /me/payout-methods", s.requireAuth(http.HandlerFunc(s.handleListPayoutMethods)))
	mux.Handle("POST /me/payout-methods", s.requireAuth(http.HandlerFunc(s.handleAddPayoutMethod)))
	mux.Handle("POST /me/withdrawals", s.requireAuth(http.HandlerFunc(s.handleWithdraw)))
	mux.Handle("GET /transactions/{id}/receipt", s.requireAuth(http.HandlerFunc(s.handleReceipt)))
	// Webhook: no auth, signature verification only
	mux.HandleFunc("POST /webhooks/payments", s.handleWebhook)
}

func financeSvc(s *Server) *finance.Service {
	return finance.New(s.pool)
}

func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	wallet, err := financeSvc(s).Wallet(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, wallet)
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	cursor := r.URL.Query().Get("cursor")
	limit := 0
	// limit param ignored for now, but finance handles it
	txs, next, err := financeSvc(s).Transactions(r.Context(), currentUser(r).ID, typ, cursor, limit)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"transactions": txs, "next_cursor": next})
}

func (s *Server) handleListCards(w http.ResponseWriter, r *http.Request) {
	cards, err := financeSvc(s).ListCards(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"payment_methods": cards})
}

func (s *Server) handleAddCard(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProviderRef string `json:"provider_ref"`
		Last4       string `json:"last4"`
		Brand       string `json:"brand"`
		ExpMonth    int    `json:"exp_month"`
		ExpYear     int    `json:"exp_year"`
	}
	if !decode(w, r, &in) {
		return
	}
	card, err := financeSvc(s).AddCard(r.Context(), currentUser(r).ID, in.ProviderRef, in.Last4, in.Brand, in.ExpMonth, in.ExpYear)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 201, card)
}

func (s *Server) handleDeleteCard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := financeSvc(s).DeleteCard(r.Context(), currentUser(r).ID, id)
	if err != nil {
		writeErr(w, 404, "not_found", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleListPayoutMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := financeSvc(s).ListPayoutMethods(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"payout_methods": methods})
}

func (s *Server) handleAddPayoutMethod(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BankRef  string `json:"bank_ref_token"`
		BankRef2 string `json:"bank_ref"`
		Last4    string `json:"last4"`
		BankName string `json:"bank_name"`
	}
	if !decode(w, r, &in) {
		return
	}
	ref := in.BankRef
	if ref == "" {
		ref = in.BankRef2
	}
	m, err := financeSvc(s).AddPayoutMethod(r.Context(), currentUser(r).ID, ref, in.Last4, in.BankName)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 201, m)
}

func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount          int    `json:"amount"`
		PayoutMethodID  string `json:"payout_method_id"`
		IdempotencyKey  string `json:"idempotency_key"`
		IdempotencyKey2 string `json:"idempotencyKey"`
	}
	if !decode(w, r, &in) {
		return
	}
	key := in.IdempotencyKey
	if key == "" {
		key = in.IdempotencyKey2
	}
	if key == "" {
		key = r.Header.Get("Idempotency-Key")
	}
	res, err := financeSvc(s).Withdraw(r.Context(), currentUser(r).ID, in.Amount, in.PayoutMethodID, key)
	if err != nil {
		if errors.Is(err, finance.ErrVerificationRequired) || strings.Contains(err.Error(), "verification_required") {
			writeErr(w, 403, "verification_required", err.Error())
			return
		}
		if errors.Is(err, finance.ErrInsufficientFunds) || strings.Contains(err.Error(), "insufficient") {
			writeErr(w, 400, "insufficient_funds", err.Error())
			return
		}
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 201, res)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func (s *Server) handleReceipt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	receipt, err := financeSvc(s).Receipt(r.Context(), currentUser(r).ID, id)
	if err != nil {
		if errors.Is(err, finance.ErrNotFound) || strings.Contains(err.Error(), "not found") {
			writeErr(w, 404, "not_found", err.Error())
			return
		}
		if errors.Is(err, finance.ErrForbidden) || strings.Contains(err.Error(), "forbidden") {
			writeErr(w, 403, "forbidden", err.Error())
			return
		}
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, receipt)
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, 400, "bad_request", "cannot read body")
		return
	}
	sig := r.Header.Get("X-Webhook-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Signature")
	}
	if sig == "" {
		sig = r.Header.Get("Webhook-Signature")
	}
	// Also try generic
	if sig == "" {
		sig = r.Header.Get("Signature")
	}
	code, msg, _ := webhook.Handle(r.Context(), s.pool, payload, sig)
	if code == 401 {
		writeErr(w, 401, "bad_signature", msg)
		return
	}
	if code >= 400 {
		writeErr(w, code, "bad_request", msg)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": msg})
}
