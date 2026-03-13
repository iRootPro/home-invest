package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"banki/internal/models"
	"banki/internal/templateutil"

	"github.com/go-chi/chi/v5"
)

type FamilyMembersHandler struct {
	DB *sql.DB
}

func (h *FamilyMembersHandler) Index(w http.ResponseWriter, r *http.Request) {
	members, err := models.ListFamilyMembers(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templateutil.Render(w, "family_members/index.html", map[string]any{
		"Members": members,
		"IsAdmin": true,
	})
}

func (h *FamilyMembersHandler) Create(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Имя обязательно", http.StatusBadRequest)
		return
	}
	memberID, err := models.CreateFamilyMember(h.DB, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If username and password provided, create a login
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if username != "" && password != "" {
		if err := models.CreateUserForMember(h.DB, memberID, username, password); err != nil {
			// Member created but login failed — not critical, show the page
			_ = err
		}
	}

	http.Redirect(w, r, "/members", http.StatusSeeOther)
}

func (h *FamilyMembersHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	member, err := models.GetFamilyMember(h.DB, id)
	if err != nil {
		http.Error(w, "Член семьи не найден", http.StatusNotFound)
		return
	}
	user, _ := models.GetUserByMemberID(h.DB, id)
	templateutil.Render(w, "family_members/form.html", map[string]any{
		"Member":  member,
		"User":    user,
		"IsAdmin": true,
	})
}

func (h *FamilyMembersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "Имя обязательно", http.StatusBadRequest)
		return
	}
	if err := models.UpdateFamilyMember(h.DB, id, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Handle login changes
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	removeLogin := r.FormValue("remove_login") == "on"

	existingUser, _ := models.GetUserByMemberID(h.DB, id)

	if removeLogin && existingUser != nil {
		models.DeleteUserForMember(h.DB, id)
	} else if username != "" {
		if existingUser == nil && password != "" {
			// Create new login
			models.CreateUserForMember(h.DB, id, username, password)
		} else if existingUser != nil && password != "" {
			// Update password
			models.UpdateUserPassword(h.DB, id, password)
		}
	}

	http.Redirect(w, r, "/members", http.StatusSeeOther)
}

func (h *FamilyMembersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	// Delete user login first if exists
	models.DeleteUserForMember(h.DB, id)
	if err := models.DeleteFamilyMember(h.DB, id); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/members", http.StatusSeeOther)
}
