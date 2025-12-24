package handlers

import (
	db "budgee-server/src/db/sql"
	"budgee-server/src/models"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateTransactionRule(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		var req struct {
			Name                    string          `json:"name"`
			Conditions              json.RawMessage `json:"conditions"`
			PersonalFinanceCategory string          `json:"personal_finance_category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode create transaction rule request body for user %d: %v", userID, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		rule := &models.TransactionRule{
			UserID:                  int(userID),
			Name:                    req.Name,
			Conditions:              req.Conditions,
			PersonalFinanceCategory: req.PersonalFinanceCategory,
		}
		created, err := db.CreateTransactionRule(r.Context(), pool, rule)
		if err != nil {
			log.Printf("ERROR: Failed to create transaction rule for user %d: %v", userID, err)
			http.Error(w, "failed to create transaction rule", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Created transaction rule id %d for user %d, name %s", created.ID, userID, created.Name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func GetTransactionRuleByID(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		ruleIDStr := chi.URLParam(r, "rule_id")
		ruleID, err := strconv.Atoi(ruleIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid rule id param: %s", ruleIDStr)
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		rule, err := db.GetTransactionRuleByID(r.Context(), pool, int(userID), ruleID)
		if err != nil {
			log.Printf("ERROR: Transaction rule id %d not found for user %d: %v", ruleID, userID, err)
			http.Error(w, "transaction rule not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func GetAllTransactionRules(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		rules, err := db.GetAllTransactionRules(r.Context(), pool, userID)
		if err != nil {
			log.Printf("ERROR: Failed to get transaction rules for user %d: %v", userID, err)
			http.Error(w, "failed to get transaction rules", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

func UpdateTransactionRule(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		ruleIDStr := chi.URLParam(r, "rule_id")
		ruleID, err := strconv.Atoi(ruleIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid rule id param: %s", ruleIDStr)
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		var req struct {
			Name                    string          `json:"name"`
			Conditions              json.RawMessage `json:"conditions"`
			PersonalFinanceCategory string          `json:"personal_finance_category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode update transaction rule request body for user %d: %v", userID, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		rule := &models.TransactionRule{
			ID:                      ruleID,
			UserID:                  int(userID),
			Name:                    req.Name,
			Conditions:              req.Conditions,
			PersonalFinanceCategory: req.PersonalFinanceCategory,
		}
		updated, err := db.UpdateTransactionRule(r.Context(), pool, rule)
		if err != nil {
			log.Printf("ERROR: Failed to update transaction rule id %d for user %d: %v", ruleID, userID, err)
			http.Error(w, "failed to update transaction rule", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Updated transaction rule id %d for user %d", updated.ID, userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func DeleteTransactionRule(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		ruleIDStr := chi.URLParam(r, "rule_id")
		ruleID, err := strconv.Atoi(ruleIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid rule id param: %s", ruleIDStr)
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		err = db.DeleteTransactionRule(r.Context(), pool, int(userID), ruleID)
		if err != nil {
			log.Printf("ERROR: Failed to delete transaction rule id %d for user %d: %v", ruleID, userID, err)
			http.Error(w, "failed to delete transaction rule", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Deleted transaction rule id %d for user %d", ruleID, userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "transaction rule deleted"})
	}
}

func TriggerTransactionRules(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		err := db.ApplyTransactionRulesToUser(r.Context(), pool, int64(userID))
		if err != nil {
			log.Printf("ERROR: Failed to trigger transaction rules for user %d: %v", userID, err)
			http.Error(w, "failed to trigger transaction rules", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "transaction rules triggered"})
	}
}
