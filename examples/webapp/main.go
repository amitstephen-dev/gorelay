package main

import (
    "encoding/json"
    "log"
    "net/http"
    "time"
    
    "github.com/gorelay/gorelay"
)

type User struct {
    ID    int    `json:"id"`
    Email string `json:"email"`
    Name  string `json:"name"`
}

type EmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

type ReportPayload struct {
    UserID int    `json:"user_id"`
    Date   string `json:"date"`
    Format string `json:"format"`
}

func SendEmail(payload interface{}) error {
    p := payload.(*EmailPayload)
    log.Printf("Sending email to %s: %s", p.To, p.Subject)
    // Simulate email sending
    time.Sleep(100 * time.Millisecond)
    return nil
}

func GenerateReport(payload interface{}) error {
    p := payload.(*ReportPayload)
    log.Printf("Generating %s report for user %d on %s", p.Format, p.UserID, p.Date)
    // Simulate report generation
    time.Sleep(2 * time.Second)
    return nil
}

func main() {
    // Initialize Relay
    r := gorelay.New(
        gorelay.WithWorkerCount(4),
        gorelay.WithMaxRetries(3),
        gorelay.WithDashboard(":8080"),
    )
    
    // Register handlers
    r.Register("email.send", SendEmail, &EmailPayload{})
    r.Register("report.generate", GenerateReport, &ReportPayload{})
    
    // Start Relay
    r.Start()
    
    // Setup HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/api/signup", handleSignup(r))
    mux.HandleFunc("/api/report", handleReport(r))
    
    log.Println("Server starting on :3000")
    log.Println("Dashboard: http://localhost:8080")
    
    if err := http.ListenAndServe(":3000", mux); err != nil {
        log.Fatal(err)
    }
}

func handleSignup(r *gorelay.Relay) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        var user User
        if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        // Enqueue welcome email
        taskID, err := r.Enqueue("email.send", &EmailPayload{
            To:      user.Email,
            Subject: "Welcome to GoRelay!",
            Body:    "Thanks for signing up!",
        })
        
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message": "User created successfully",
            "task_id": taskID,
        })
    }
}

func handleReport(r *gorelay.Relay) http.HandlerFunc {
    return func(w http.ResponseWriter, req *http.Request) {
        var payload ReportPayload
        if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        taskID, err := r.Enqueue("report.generate", &payload)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "task_id": taskID,
            "status":  "queued",
        })
    }
}