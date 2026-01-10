package models

type RegisterResponse struct {
	ID         int    `json:"id"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	SuperAdmin bool   `json:"super_admin"`
}
