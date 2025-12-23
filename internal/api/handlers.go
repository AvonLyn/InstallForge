package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strings"

    "installforge/internal/recipe"
    "installforge/internal/render"
    "installforge/internal/store"
)

// RegisterRoutes attaches handlers to mux.
func RegisterRoutes(mux *http.ServeMux, st *store.Store) {
    mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodGet:
            listProjects(st)(w, r)
        case http.MethodPost:
            createProject(st)(w, r)
        default:
            w.WriteHeader(http.StatusMethodNotAllowed)
        }
    })

    mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
        rest := strings.TrimPrefix(r.URL.Path, "/api/projects/")
        parts := strings.Split(rest, "/")
        if len(parts) == 0 || parts[0] == "" {
            w.WriteHeader(http.StatusNotFound)
            return
        }
        id := parts[0]
        if len(parts) == 1 {
            switch r.Method {
            case http.MethodGet:
                getProject(st, id)(w, r)
            case http.MethodPut:
                saveProject(st, id)(w, r)
            default:
                w.WriteHeader(http.StatusMethodNotAllowed)
            }
            return
        }
        if len(parts) >= 2 {
            switch parts[1] {
            case "assets":
                if r.Method == http.MethodGet {
                    listAssets(st, id)(w, r)
                } else if r.Method == http.MethodPost {
                    uploadAssets(st, id)(w, r)
                } else {
                    w.WriteHeader(http.StatusMethodNotAllowed)
                }
            case "generate":
                if r.Method == http.MethodPost {
                    generatePreview(st, id)(w, r)
                } else {
                    w.WriteHeader(http.StatusMethodNotAllowed)
                }
            case "export":
                if r.Method == http.MethodPost {
                    exportBundle(st, id)(w, r)
                } else {
                    w.WriteHeader(http.StatusMethodNotAllowed)
                }
            default:
                w.WriteHeader(http.StatusNotFound)
            }
        }
    })
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}

func listProjects(st *store.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        projects, err := st.ListProjects()
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        writeJSON(w, http.StatusOK, projects)
    }
}

func createProject(st *store.Store) http.HandlerFunc {
    type req struct {
        Name        string   `json:"name"`
        Description string   `json:"description"`
        Target      []string `json:"target"`
    }
    return func(w http.ResponseWriter, r *http.Request) {
        var body req
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
            return
        }
        rec, err := st.CreateProject(body.Name, body.Description, body.Target)
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        writeJSON(w, http.StatusOK, rec)
    }
}

func getProject(st *store.Store, id string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rec, err := st.LoadRecipe(id)
        if err != nil {
            writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
            return
        }
        writeJSON(w, http.StatusOK, rec)
    }
}

func saveProject(st *store.Store, id string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var rec recipe.Recipe
        if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
            return
        }
        rec.Project.ID = id
        if err := st.SaveRecipe(rec); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        issues := recipe.Validate(rec)
        writeJSON(w, http.StatusOK, map[string]interface{}{"recipe": rec, "issues": issues})
    }
}

func listAssets(st *store.Store, id string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        entries, err := st.AssetList(id)
        if err != nil {
            writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
            return
        }
        var res []map[string]interface{}
        for _, e := range entries {
            info, _ := e.Info()
            res = append(res, map[string]interface{}{
                "filename": e.Name(),
                "size":     info.Size(),
            })
        }
        writeJSON(w, http.StatusOK, res)
    }
}

func uploadAssets(st *store.Store, id string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := r.ParseMultipartForm(32 << 20); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid form"})
            return
        }
        files := r.MultipartForm.File["files"]
        for _, fh := range files {
            f, err := fh.Open()
            if err != nil {
                writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
                return
            }
            defer f.Close()
            if err := st.SaveAsset(id, fh.Filename, f); err != nil {
                writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
                return
            }
        }
        writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
    }
}

func generatePreview(st *store.Store, id string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rec, err := st.LoadRecipe(id)
        if err != nil {
            writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
            return
        }
        renderRes, err := render.Render(rec)
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        writeJSON(w, http.StatusOK, renderRes)
    }
}

func exportBundle(st *store.Store, id string) http.HandlerFunc {
    type req struct {
        Format string `json:"format"`
    }
    return func(w http.ResponseWriter, r *http.Request) {
        var body req
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
            return
        }
        rec, err := st.LoadRecipe(id)
        if err != nil {
            writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
            return
        }
        issues := recipe.Validate(rec)
        hasError := false
        for _, is := range issues {
            if is.Level == "error" {
                hasError = true
            }
        }
        if hasError {
            writeJSON(w, http.StatusBadRequest, map[string]interface{}{"issues": issues})
            return
        }
        target := filepath.Join(os.TempDir(), fmt.Sprintf("bundle_%s", rec.Project.Name))
        if err := os.RemoveAll(target); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        if err := st.WriteBundle(rec, target); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        renderRes, err := render.Render(rec)
        if err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        if err := os.WriteFile(filepath.Join(target, "install.sh"), []byte(renderRes.InstallSh), 0o755); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        if err := os.WriteFile(filepath.Join(target, "README.txt"), []byte(renderRes.Readme), 0o644); err != nil {
            writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
            return
        }
        writeJSON(w, http.StatusOK, map[string]string{"path": target})
    }
}

// StaticHandler serves embedded files.
func StaticHandler(prefix string, fs http.FileSystem) http.Handler {
    fileServer := http.FileServer(fs)
    return http.StripPrefix(prefix, fileServer)
}
