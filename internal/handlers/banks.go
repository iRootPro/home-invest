package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"banki/internal/models"
	"banki/internal/templateutil"

	"github.com/go-chi/chi/v5"
)

type BanksHandler struct {
	DB         *sql.DB
	UploadsDir string
}

// ASVMemberRow is a per-member row within a bank's ASV cell.
type ASVMemberRow struct {
	Name    string
	HasASV  bool
	Total   float64
	Percent float64
}

// ASVBankCell is the pre-computed data for one bank's ASV column cell.
type ASVBankCell struct {
	Count      int
	Total      int
	Free       string // comma-separated names of members without deposits
	MaxPercent float64
	Members    []ASVMemberRow
}

func (h *BanksHandler) Index(w http.ResponseWriter, r *http.Request) {
	banks, err := models.ListBanks(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	asvEntries, _ := models.ASVCheck(h.DB)
	asvByBankHolder := make(map[int64]map[int64]models.ASVEntry)
	for _, e := range asvEntries {
		if asvByBankHolder[e.BankID] == nil {
			asvByBankHolder[e.BankID] = make(map[int64]models.ASVEntry)
		}
		asvByBankHolder[e.BankID][e.HolderID] = e
	}
	members, _ := models.ListFamilyMembers(h.DB)

	asvCells := make(map[int64]ASVBankCell)
	for _, bank := range banks {
		holderMap := asvByBankHolder[bank.ID]
		cell := ASVBankCell{Total: len(members)}
		var freeNames []string
		for _, m := range members {
			if e, ok := holderMap[m.ID]; ok {
				cell.Count++
				cell.Members = append(cell.Members, ASVMemberRow{
					Name:    m.Name,
					HasASV:  true,
					Total:   e.Total,
					Percent: e.Percent,
				})
			} else {
				freeNames = append(freeNames, m.Name)
				cell.Members = append(cell.Members, ASVMemberRow{
					Name: m.Name,
				})
			}
		}
		cell.Free = strings.Join(freeNames, ", ")
		for _, m := range cell.Members {
			if m.HasASV && m.Percent > cell.MaxPercent {
				cell.MaxPercent = m.Percent
			}
		}
		asvCells[bank.ID] = cell
	}

	templateutil.Render(w, "banks/index.html", map[string]any{
		"Banks":    banks,
		"IsAdmin":  true,
		"ASVCells": asvCells,
		"ASVLimit": models.ASVLimit,
	})
}

func (h *BanksHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(1 << 20) // 1MB
	name := strings.TrimSpace(r.FormValue("name"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	if name == "" {
		http.Error(w, "Название банка обязательно", http.StatusBadRequest)
		return
	}
	id, err := models.CreateBank(h.DB, name, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if filename, err := h.saveLogo(r, id); err == nil && filename != "" {
		models.UpdateBankLogo(h.DB, id, filename)
	}
	http.Redirect(w, r, "/banks", http.StatusSeeOther)
}

func (h *BanksHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	bank, err := models.GetBank(h.DB, id)
	if err != nil {
		http.Error(w, "Банк не найден", http.StatusNotFound)
		return
	}
	templateutil.Render(w, "banks/form.html", map[string]any{
		"Bank":    bank,
		"IsAdmin": true,
	})
}

func (h *BanksHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(1 << 20)
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	name := strings.TrimSpace(r.FormValue("name"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	if name == "" {
		http.Error(w, "Название банка обязательно", http.StatusBadRequest)
		return
	}
	if err := models.UpdateBank(h.DB, id, name, notes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if filename, err := h.saveLogo(r, id); err == nil && filename != "" {
		models.UpdateBankLogo(h.DB, id, filename)
	}
	http.Redirect(w, r, "/banks", http.StatusSeeOther)
}

var allowedLogoExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".svg": true,
}

func (h *BanksHandler) saveLogo(r *http.Request, bankID int64) (string, error) {
	file, header, err := r.FormFile("logo")
	if err != nil {
		return "", err // no file uploaded
	}
	defer file.Close()

	if header.Size > 1<<20 {
		return "", fmt.Errorf("file too large")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedLogoExts[ext] {
		return "", fmt.Errorf("unsupported format")
	}

	filename := fmt.Sprintf("bank_%d%s", bankID, ext)
	dst, err := os.Create(filepath.Join(h.UploadsDir, filename))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return filename, nil
}

func (h *BanksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := models.DeleteBank(h.DB, id); err != nil {
		// RESTRICT error - has related deposits
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/banks", http.StatusSeeOther)
}
