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

func CreateBudget(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		var req struct {
			Amount                  float64 `json:"amount"`
			PersonalFinanceCategory string  `json:"personal_finance_category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode create budget request body for user %d: %v", userID, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		budget := &models.Budget{
			UserID:                  int(userID),
			Amount:                  req.Amount,
			PersonalFinanceCategory: req.PersonalFinanceCategory,
		}
		created, err := db.CreateBudget(r.Context(), pool, budget)
		if err != nil {
			log.Printf("ERROR: Failed to create budget for user %d: %v", userID, err)
			http.Error(w, "failed to create budget", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Created budget id %d for user %d, category %s", created.ID, userID, created.PersonalFinanceCategory)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func GetBudgetByID(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		budgetIDStr := chi.URLParam(r, "budget_id")
		budgetID, err := strconv.Atoi(budgetIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid budget id param: %s", budgetIDStr)
			http.Error(w, "invalid budget id", http.StatusBadRequest)
			return
		}
		budget, err := db.GetBudgetByID(r.Context(), pool, int(userID), budgetID)
		if err != nil {
			log.Printf("ERROR: Budget id %d not found for user %d: %v", budgetID, userID, err)
			http.Error(w, "budget not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(budget)
	}
}

func GetBudgetByCategory(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		category := chi.URLParam(r, "category")
		budget, err := db.GetBudgetByCategory(r.Context(), pool, int(userID), category)
		if err != nil {
			log.Printf("ERROR: Budget not found for user %d, category %s: %v", userID, category, err)
			http.Error(w, "budget not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(budget)
	}
}

func GetAllBudgetsForUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		budgets, err := db.GetAllBudgetsForUser(r.Context(), pool, int(userID))
		if err != nil {
			log.Printf("ERROR: Failed to get budgets for user %d: %v", userID, err)
			http.Error(w, "failed to get budgets", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(budgets)
	}
}

func UpdateBudget(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		budgetIDStr := chi.URLParam(r, "budget_id")
		budgetID, err := strconv.Atoi(budgetIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid budget id param: %s", budgetIDStr)
			http.Error(w, "invalid budget id", http.StatusBadRequest)
			return
		}
		var req struct {
			Amount                  float64 `json:"amount"`
			PersonalFinanceCategory string  `json:"personal_finance_category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: Failed to decode update budget request body for user %d: %v", userID, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		budget := &models.Budget{
			ID:                      budgetID,
			UserID:                  int(userID),
			Amount:                  req.Amount,
			PersonalFinanceCategory: req.PersonalFinanceCategory,
		}
		updated, err := db.UpdateBudget(r.Context(), pool, budget)
		if err != nil {
			log.Printf("ERROR: Failed to update budget id %d for user %d: %v", budgetID, userID, err)
			http.Error(w, "failed to update budget", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Updated budget id %d for user %d", updated.ID, userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func DeleteBudget(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id").(int64)
		budgetIDStr := chi.URLParam(r, "budget_id")
		budgetID, err := strconv.Atoi(budgetIDStr)
		if err != nil {
			log.Printf("ERROR: Invalid budget id param: %s", budgetIDStr)
			http.Error(w, "invalid budget id", http.StatusBadRequest)
			return
		}
		err = db.DeleteBudget(r.Context(), pool, int(userID), budgetID)
		if err != nil {
			log.Printf("ERROR: Failed to delete budget id %d for user %d: %v", budgetID, userID, err)
			http.Error(w, "failed to delete budget", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Deleted budget id %d for user %d", budgetID, userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "budget deleted"})
	}
}
