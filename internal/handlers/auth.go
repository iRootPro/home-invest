package handlers

import (
	"database/sql"
	"net/http"

	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"
)

type AuthHandler struct {
	DB *sql.DB
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		templateutil.Render(w, "login.html", nil)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := models.Authenticate(h.DB, username, password)
	if err != nil {
		templateutil.Render(w, "login.html", map[string]string{"Error": "Неверное имя пользователя или пароль"})
		return
	}

	var familyMemberID int64
	if user.FamilyMemberID.Valid {
		familyMemberID = user.FamilyMemberID.Int64
	}
	middleware.SetSession(w, middleware.SessionData{
		UserID:         user.ID,
		Username:       user.Username,
		FamilyMemberID: familyMemberID,
		IsAdmin:        !user.FamilyMemberID.Valid,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearSession(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
