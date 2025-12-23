package store

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "installforge/internal/recipe"
)

// Store handles local file storage.
type Store struct {
    Root string
}

// New creates a new store rooted at path.
func New(root string) *Store {
    return &Store{Root: root}
}

// EnsureProjectDir makes project directory.
func (s *Store) EnsureProjectDir(id string) (string, error) {
    path := filepath.Join(s.Root, id)
    if err := os.MkdirAll(filepath.Join(path, "assets"), 0o755); err != nil {
        return "", err
    }
    return path, nil
}

// ListProjects lists existing projects.
func (s *Store) ListProjects() ([]recipe.ProjectMeta, error) {
    entries, err := os.ReadDir(s.Root)
    if err != nil {
        return nil, err
    }
    var res []recipe.ProjectMeta
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        recPath := filepath.Join(s.Root, e.Name(), "recipe.json")
        data, err := os.ReadFile(recPath)
        if err != nil {
            continue
        }
        var r recipe.Recipe
        if err := json.Unmarshal(data, &r); err != nil {
            continue
        }
        res = append(res, r.Project)
    }
    return res, nil
}

// LoadRecipe reads recipe.
func (s *Store) LoadRecipe(id string) (recipe.Recipe, error) {
    recPath := filepath.Join(s.Root, id, "recipe.json")
    data, err := os.ReadFile(recPath)
    if err != nil {
        return recipe.Recipe{}, err
    }
    var r recipe.Recipe
    if err := json.Unmarshal(data, &r); err != nil {
        return recipe.Recipe{}, err
    }
    return r, nil
}

// SaveRecipe writes recipe.
func (s *Store) SaveRecipe(r recipe.Recipe) error {
    dir, err := s.EnsureProjectDir(r.Project.ID)
    if err != nil {
        return err
    }
    r.UpdatedAt = time.Now()
    data, err := json.MarshalIndent(r, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, "recipe.json"), data, 0o644)
}

// CreateProject creates new project with default recipe.
func (s *Store) CreateProject(name, description string, targets []string) (recipe.Recipe, error) {
    id := randomID()
    r := recipe.NewEmptyRecipe(id, name)
    r.Project.Description = description
    if len(targets) > 0 {
        r.Project.Target = targets
    }
    if err := s.SaveRecipe(r); err != nil {
        return recipe.Recipe{}, err
    }
    return r, nil
}

// AssetList lists assets for project.
func (s *Store) AssetList(id string) ([]os.DirEntry, error) {
    return os.ReadDir(filepath.Join(s.Root, id, "assets"))
}

// SaveAsset saves uploaded asset.
func (s *Store) SaveAsset(id, filename string, src io.Reader) error {
    if strings.Contains(filename, "..") {
        return fmt.Errorf("invalid filename")
    }
    dir, err := s.EnsureProjectDir(id)
    if err != nil {
        return err
    }
    f, err := os.Create(filepath.Join(dir, "assets", filename))
    if err != nil {
        return err
    }
    defer f.Close()
    _, err = io.Copy(f, src)
    return err
}

// WriteBundle exports recipe and assets into target dir.
func (s *Store) WriteBundle(r recipe.Recipe, targetDir string) error {
    if err := os.MkdirAll(filepath.Join(targetDir, "assets"), 0o755); err != nil {
        return err
    }
    recipeData, err := json.MarshalIndent(r, "", "  ")
    if err != nil {
        return err
    }
    if err := os.WriteFile(filepath.Join(targetDir, "recipe.json"), recipeData, 0o644); err != nil {
        return err
    }
    // copy assets
    assetsDir := filepath.Join(s.Root, r.Project.ID, "assets")
    entries, _ := os.ReadDir(assetsDir)
    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        src := filepath.Join(assetsDir, e.Name())
        dest := filepath.Join(targetDir, "assets", e.Name())
        if err := copyFile(src, dest); err != nil {
            return err
        }
    }
    return nil
}

func copyFile(src, dest string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dest)
    if err != nil {
        return err
    }
    defer out.Close()
    if _, err := io.Copy(out, in); err != nil {
        return err
    }
    return out.Close()
}

// ExportHandler is helper to send file download.
func ExportHandler(w http.ResponseWriter, path string) {
    w.Header().Set("Content-Type", "application/octet-stream")
    http.ServeFile(w, &http.Request{}, path)
}

func randomID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        ts := time.Now().UnixNano()
        return fmt.Sprintf("id-%d", ts)
    }
    return hex.EncodeToString(b)
}
