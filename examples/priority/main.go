package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/amitstephen-dev/gorelay"
    // Remove this line: "github.com/amitstephen-dev/gorelay/task"
)

type JobPayload struct {
    JobID   string
    Data    string
}

func ProcessJob(payload interface{}) error {
    p := payload.(*JobPayload)
    log.Printf("Processing job %s: %s", p.JobID, p.Data)
    
    // Simulate work based on priority
    switch {
    case p.JobID[:4] == "HIGH":
        log.Printf("High priority job - processing immediately")
        time.Sleep(100 * time.Millisecond)
    case p.JobID[:4] == "NORM":
        log.Printf("Normal priority job - standard processing")
        time.Sleep(200 * time.Millisecond)
    case p.JobID[:4] == "LOW":
        log.Printf("Low priority job - will be processed when system is idle")
        time.Sleep(500 * time.Millisecond)
    }
    
    return nil
}

func main() {
    r := gorelay.New(
        gorelay.WithWorkerCount(2),
        gorelay.WithDashboard(":8080"),
    )
    
    r.Register("job.process", ProcessJob, &JobPayload{})
    r.Start()
    
    // Enqueue high priority jobs
    for i := 1; i <= 5; i++ {
        taskID, _ := r.Enqueue("job.process", &JobPayload{
            JobID: fmt.Sprintf("HIGH_%d", i),
            Data:  "Urgent task",
        })
        log.Printf("Enqueued high priority task: %s", taskID)
    }
    
    // Enqueue normal priority jobs
    for i := 1; i <= 10; i++ {
        taskID, _ := r.Enqueue("job.process", &JobPayload{
            JobID: fmt.Sprintf("NORM_%d", i),
            Data:  "Regular task",
        })
        log.Printf("Enqueued normal priority task: %s", taskID)
    }
    
    // Enqueue low priority jobs
    for i := 1; i <= 20; i++ {
        taskID, _ := r.Enqueue("job.process", &JobPayload{
            JobID: fmt.Sprintf("LOW_%d", i),
            Data:  "Background task",
        })
        log.Printf("Enqueued low priority task: %s", taskID)
    }
    
    // Schedule a high priority job for later
    r.Schedule(time.Now().Add(10*time.Second), "job.process", &JobPayload{
        JobID: "HIGH_SCHEDULED",
        Data:  "Scheduled urgent task",
    })
    
    log.Println("All jobs enqueued. Dashboard: http://localhost:8080")
    log.Println("Observe how high priority jobs are processed first")
    
    select {}
}
