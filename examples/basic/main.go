package main

import (
    "fmt"
    "time"
    
    "github.com/gorelay/gorelay"
)

type EmailPayload struct {
    To      string
    Subject string
    Body    string
}

func SendEmail(payload interface{}) error {
    p := payload.(*EmailPayload)
    fmt.Printf("Sending email to: %s\n", p.To)
    fmt.Printf("Subject: %s\n", p.Subject)
    fmt.Printf("Body: %s\n", p.Body)
    return nil
}

func main() {
    r := gorelay.New(
        gorelay.WithWorkerCount(1),
        gorelay.WithMaxRetries(3),
        gorelay.WithDashboard(":8080"),
    )
    
    r.Register("email.send", SendEmail, &EmailPayload{})
    r.Start()
    
    // Enqueue immediate task
    r.Enqueue("email.send", &EmailPayload{
        To:      "user@example.com",
        Subject: "Welcome!",
        Body:    "Thanks for signing up",
    })
    
    // Schedule task for tomorrow
    r.Schedule(time.Now().Add(24*time.Hour), "email.send", &EmailPayload{
        To:      "reminder@example.com",
        Subject: "Daily Report",
        Body:    "Here's your report",
    })
    
    select {}
}