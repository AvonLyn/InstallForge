package main

import (
    "fmt"
    "log"
    "net/http"
    "os"

    "installforge/internal/api"
    "installforge/internal/store"
    "installforge/webembed"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    dataRoot := "data/projects"
    st := store.New(dataRoot)

    mux := http.NewServeMux()
    api.RegisterRoutes(mux, st)

    staticFS := http.FS(webembed.Content)
    mux.Handle("/", api.StaticHandler("/", staticFS))

    addr := fmt.Sprintf("127.0.0.1:%s", port)
    log.Printf("InstallForge listening on http://%s", addr)
    log.Fatal(http.ListenAndServe(addr, mux))
}
