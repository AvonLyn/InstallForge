package render

import (
    "bytes"
    "encoding/json"
    "fmt"
    "text/template"
    "time"

    "installforge/internal/recipe"
)

// RenderResponse holds rendered artifacts.
type RenderResponse struct {
    InstallSh       string          `json:"installSh"`
    Readme          string          `json:"readme"`
    RecipePretty    string          `json:"recipeJsonPretty"`
    Issues          []recipe.Issue  `json:"issues"`
}

// Render generates preview artifacts.
func Render(r recipe.Recipe) (RenderResponse, error) {
    issues := recipe.Validate(r)
    install, err := renderInstall(r)
    if err != nil {
        return RenderResponse{}, err
    }
    readme := renderReadme(r)
    recipePretty, err := pretty(r)
    if err != nil {
        return RenderResponse{}, err
    }
    return RenderResponse{InstallSh: install, Readme: readme, RecipePretty: recipePretty, Issues: issues}, nil
}

func pretty(r recipe.Recipe) (string, error) {
    buf, err := json.MarshalIndent(r, "", "  ")
    if err != nil {
        return "", err
    }
    return string(buf), nil
}

func renderInstall(r recipe.Recipe) (string, error) {
    tmpl := template.Must(template.New("install").Parse(installTemplate))
    var buf bytes.Buffer
    data := map[string]interface{}{
        "Recipe":      r,
        "GeneratedAt": time.Now().Format(time.RFC3339),
        "Preflight":   gatherPreflight(r),
    }
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

func renderReadme(r recipe.Recipe) string {
    return fmt.Sprintf(`InstallForge bundle\n===================\n\nProject: %s\nTargets: %v\n\nUsage:\n  chmod +x install.sh\n  sudo ./install.sh\n\nLogs are written under {{LOG_DIR}} (default /var/log/asg).\n`, r.Project.Name, r.Project.Target)
}

func gatherPreflight(r recipe.Recipe) []string {
    checks := map[string]bool{}
    for _, s := range r.Steps {
        switch s.Type {
        case "extract_zip":
            checks["unzip"] = true
        case "extract_tar_gz":
            checks["tar"] = true
        case "rpm_install":
            checks["rpm"] = true
        case "append_lines", "delete_lines", "replace":
            checks["sed"] = true
            checks["grep"] = true
        case "service_systemd", "auto_service":
            checks["systemctl"] = true
        case "service_sysv":
            checks["chkconfig"] = true
        }
    }
    var deps []string
    for cmd := range checks {
        deps = append(deps, cmd)
    }
    return deps
}

const installTemplate = `#!/bin/bash
set -eu

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ASSET_DIR="$SCRIPT_DIR/assets"
LOG_DIR="${LOG_DIR:-{{ index .Recipe.Vars "LOG_DIR"}}}"
LOG_FILE="$LOG_DIR/install-$(date +%Y%m%d-%H%M%S).log"

mkdir -p "$LOG_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "[InstallForge] Generated at {{.GeneratedAt}}"

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root (sudo ./install.sh)" >&2
  exit 1
fi

# Preflight checks
missing=()
{{- range .Preflight }}
if ! command -v {{.}} >/dev/null 2>&1; then
  missing+=("{{.}}")
fi
{{- end }}
if [ ${#missing[@]} -ne 0 ]; then
  echo "Missing required commands: ${missing[*]}" >&2
  exit 1
fi

step_idx=0
total={{len .Recipe.Steps}}

run_step() {
  local id="$1"
  local type="$2"
  local desc="$3"
  echo "[$((step_idx+1))/$total] step=$id type=$type name=$desc"
}

{{range .Recipe.Steps}}
run_step "{{.ID}}" "{{.Type}}" "{{.Name}}"
case "{{.Type}}" in
  mkdir)
    mkdir -p "{{index .Config "path"}}"
    ;;
  copy)
    {{if index .Config "overwrite"}}cp -f{{else}}cp -n{{end}} "{{index .Config "src"}}" "{{index .Config "dest"}}"
    {{if index .Config "mode"}}chmod {{index .Config "mode"}} "{{index .Config "dest"}}"{{end}}
    ;;
  chmod)
    chmod {{index .Config "mode"}} "{{index .Config "path"}}"
    ;;
  chown)
    chown {{index .Config "owner"}}{{if index .Config "group"}}:{{index .Config "group"}}{{end}} "{{index .Config "path"}}"
    ;;
  extract_tar_gz)
    if [ -n "{{index .Config "creates"}}" ] && [ -e "{{index .Config "creates"}}" ]; then
      echo "skip extract_tar_gz because creates exists"
    else
      mkdir -p "{{index .Config "dest"}}"
      tar -xzf "{{index .Config "src"}}" -C "{{index .Config "dest"}}"
    fi
    ;;
  extract_zip)
    if [ -n "{{index .Config "creates"}}" ] && [ -e "{{index .Config "creates"}}" ]; then
      echo "skip extract_zip because creates exists"
    else
      mkdir -p "{{index .Config "dest"}}"
      unzip -o "{{index .Config "src"}}" -d "{{index .Config "dest"}}"
    fi
    ;;
  rpm_install)
    for rpm in {{range $i, $v := index .Config "rpms"}} "{{$v}}"{{end}}; do
      if [ "{{index .Config "mode"}}" = "upgrade" ]; then
        rpm -Uvh $rpm {{if index .Config "nodeps"}}--nodeps{{end}}
      else
        rpm -ivh $rpm {{if index .Config "nodeps"}}--nodeps{{end}}
      fi
    done
    ;;
  append_lines)
    target="{{index .Config "file"}}"
    cp -a "$target" "$target.bak.$(date +%s)"
    while IFS= read -r line; do
      if {{if index .Config "unique"}}! grep -Fqx "$line" "$target"; then{{else}}true; then{{end}}
        echo "$line" >> "$target"
      fi
    done <<'LINES'
{{range index .Config "lines"}}{{.}}
{{end}}LINES
    ;;
  delete_lines)
    target="{{index .Config "file"}}"
    cp -a "$target" "$target.bak.$(date +%s)"
    if [ "{{index .Config "mode"}}" = "regex" ]; then
      grep -Ev "{{index .Config "match"}}" "$target" > "$target.tmp" && mv "$target.tmp" "$target"
    else
      grep -Fv "{{index .Config "match"}}" "$target" > "$target.tmp" && mv "$target.tmp" "$target"
    fi
    ;;
  replace)
    target="{{index .Config "file"}}"
    cp -a "$target" "$target.bak.$(date +%s)"
    if [ "{{index .Config "mode"}}" = "regex" ]; then
      sed -r 's/{{index .Config "pattern"}}/{{index .Config "replacement"}}/g' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
    else
      sed 's/{{index .Config "pattern"}}/{{index .Config "replacement"}}/g' "$target" > "$target.tmp" && mv "$target.tmp" "$target"
    fi
    ;;
  run_cmd)
    {{if index .Config "cwd"}}(cd "{{index .Config "cwd"}}" && {{index .Config "cmd"}}){{else}}{{index .Config "cmd"}}{{end}}
    ;;
  service_sysv)
    cp "{{index .Config "src"}}" "/etc/init.d/{{index .Config "name"}}"
    chmod +x "/etc/init.d/{{index .Config "name"}}"
    if command -v chkconfig >/dev/null 2>&1; then
      chkconfig --add {{index .Config "name"}}
      chkconfig {{index .Config "name"}} on
    else
      echo "chkconfig not found; ensure service enabled manually" >&2
    fi
    {{if or (not (index .Config "start")) (eq (index .Config "start") false)}}true{{else}}service {{index .Config "name"}} start{{end}}
    ;;
  service_systemd)
    cp "{{index .Config "src"}}" "/etc/systemd/system/{{index .Config "name"}}.service"
    systemctl daemon-reload
    {{if or (not (index .Config "start")) (eq (index .Config "start") false)}}true{{else}}systemctl enable --now {{index .Config "name"}}{{end}}
    ;;
  auto_service)
    if command -v systemctl >/dev/null 2>&1; then
      cp "{{index .Config "systemd_src"}}" "/etc/systemd/system/{{index .Config "name"}}.service"
      systemctl daemon-reload
      {{if or (not (index .Config "start")) (eq (index .Config "start") false)}}true{{else}}systemctl enable --now {{index .Config "name"}}{{end}}
    else
      cp "{{index .Config "sysv_src"}}" "/etc/init.d/{{index .Config "name"}}"
      chmod +x "/etc/init.d/{{index .Config "name"}}"
      if command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add {{index .Config "name"}}
        chkconfig {{index .Config "name"}} on
      else
        echo "chkconfig not found; ensure service enabled manually" >&2
      fi
      {{if or (not (index .Config "start")) (eq (index .Config "start") false)}}true{{else}}service {{index .Config "name"}} start{{end}}
    fi
    ;;
  *)
    echo "Unknown step type {{.Type}}" >&2
    exit 1
    ;;
esac
step_idx=$((step_idx+1))
done

echo "Completed $total steps"
`
