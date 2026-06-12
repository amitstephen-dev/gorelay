package dashboard

import (
    "embed"
    "encoding/json"
    "html/template"
    "net/http"
    "strings"
    "io/fs"
    "github.com/amitstephen-dev/gorelay/storage"
    "github.com/amitstephen-dev/gorelay/task"
)

//go:embed templates/* static/*
var assets embed.FS

type Dashboard struct {
    store   storage.Storage
    address string
    mux     *http.ServeMux
}

func New(store storage.Storage, address string) *Dashboard {
    d := &Dashboard{
        store:   store,
        address: address,
        mux:     http.NewServeMux(),
    }
    
    d.setupRoutes()
    return d
}

func (d *Dashboard) setupRoutes() {
    // Static files - use embedded filesystem
    staticFS, _ := fs.Sub(assets, "static")
    d.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
    
    // Pages
    d.mux.HandleFunc("/", d.handleIndex)
    d.mux.HandleFunc("/task/", d.handleTask)
    
    // API endpoints
    d.mux.HandleFunc("/api/stats", d.handleStats)
    d.mux.HandleFunc("/api/tasks", d.handleTasks)
    d.mux.HandleFunc("/api/task/", d.handleTaskAPI)
}

func (d *Dashboard) Start() error {
    go func() {
        http.ListenAndServe(d.address, d.mux)
    }()
    return nil
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
    tmpl, _ := template.ParseFS(assets, "templates/layout.html", "templates/index.html")
    tmpl.Execute(w, nil)
}

func (d *Dashboard) handleTask(w http.ResponseWriter, r *http.Request) {
    taskID := strings.TrimPrefix(r.URL.Path, "/task/")
    
    tmpl, _ := template.ParseFS(assets, "templates/layout.html", "templates/task.html")
    tmpl.Execute(w, map[string]interface{}{
        "TaskID": taskID,
    })
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
    stats, err := d.store.GetStats()
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleTasks(w http.ResponseWriter, r *http.Request) {
    status := r.URL.Query().Get("status")
    limit := 50
    
    filter := storage.TaskFilter{
        Status: task.StatusFromString(status),
        Limit:  limit,
    }
    
    tasks, err := d.store.GetTasks(filter)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(tasks)
}

func (d *Dashboard) handleTaskAPI(w http.ResponseWriter, r *http.Request) {
    taskID := strings.TrimPrefix(r.URL.Path, "/api/task/")
    
    switch r.Method {
    case "GET":
        task, err := d.store.GetTask(taskID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusNotFound)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(task)
        
    case "POST":
        if strings.HasSuffix(r.URL.Path, "/retry") {
            // Retry logic
            taskID = strings.TrimSuffix(taskID, "/retry")
            err := d.store.FailTask(taskID, "manual retry")
            if err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
            w.WriteHeader(http.StatusOK)
        }
        
    case "DELETE":
        err := d.store.CompleteTask(taskID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }
}
