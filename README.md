# GoRelay - Durable Task Queue for Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/amitstephen-dev/gorelay)](https://golang.org)
[![GitHub release](https://img.shields.io/github/v/release/amitstephen-dev/gorelay)](https://github.com/amitstephen-dev/gorelay/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/amitstephen-dev/gorelay)](https://goreportcard.com/report/github.com/amitstephen-dev/gorelay)
[![Go Reference](https://pkg.go.dev/badge/github.com/amitstephen-dev/gorelay.svg)](https://pkg.go.dev/github.com/amitstephen-dev/gorelay)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub stars](https://img.shields.io/github/stars/amitstephen-dev/gorelay)](https://github.com/amitstephen-dev/gorelay/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/amitstephen-dev/gorelay)](https://github.com/amitstephen-dev/gorelay/issues)

GoRelay is a simple, durable background job library for Go. Turn any function into a crash-resistant, retryable task with zero infrastructure required.

## ✨ Features

- **Simple API** - Just `Register`, `Enqueue`, and `Schedule`
- **Durable** - Tasks survive crashes and restarts
- **Zero Infrastructure** - SQLite by default, no external dependencies
- **Dashboard** - Beautiful web UI for monitoring
- **Automatic Retries** - Exponential backoff with configurable limits
- **Scheduling** - Run tasks at any time in the future
- **Multiple Backends** - SQLite, PostgreSQL, Redis support
- **Priority Queues** - High, Normal, Low priority levels

## 📋 Requirements

- Go 1.21 or later
- C compiler (only for SQLite with mattn/go-sqlite3)

### CGO Requirements for SQLite

GoRelay uses `mattn/go-sqlite3` which requires CGO and a C compiler:

- **Windows**: Install MinGW or use WSL
- **Linux**: Install GCC (`sudo apt-get install gcc`)
- **macOS**: Install Xcode Command Line Tools (`xcode-select --install`)

## 🚀 Quick Start

### Installation

```bash
go get github.com/amitstephen-dev/gorelay
```

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/amitstephen-dev/gorelay"
)

type EmailPayload struct {
    To   string
    Body string
}

func SendEmail(payload interface{}) error {
    p := payload.(*EmailPayload)
    fmt.Printf("Sending email to %s: %s\n", p.To, p.Body)
    return nil
}

func main() {
    // Create Relay instance (creates relay.db automatically)
    r := gorelay.New(
        gorelay.WithWorkerCount(4),
        gorelay.WithDashboard(":8080"),
    )
    
    // Register your handler
    r.Register("email.send", SendEmail, &EmailPayload{})
    
    // Start workers and dashboard
    r.Start()
    
    // Enqueue a task
    r.Enqueue("email.send", &EmailPayload{
        To:   "user@example.com",
        Body: "Welcome!",
    })
    
    // Keep running
    select {}
}
```

### Environment Setup

**Windows (PowerShell):**
```powershell
$env:CGO_ENABLED=1
go run main.go
```

**Linux/macOS:**
```bash
CGO_ENABLED=1 go run main.go
```

## 📚 Core Concepts

### 1. Register Handlers

```go
type WelcomePayload struct {
    UserID int
    Email  string
}

func SendWelcome(payload interface{}) error {
    p := payload.(*WelcomePayload)
    // Your business logic here
    return nil
}

r.Register("user.welcome", SendWelcome, &WelcomePayload{})
```

### 2. Enqueue Tasks

```go
// Immediate execution
taskID, err := r.Enqueue("user.welcome", &WelcomePayload{
    UserID: 123,
    Email:  "user@example.com",
})
```

### 3. Schedule Tasks

```go
// Execute tomorrow at 9 AM
tomorrow := time.Now().Add(24 * time.Hour)
scheduledTime := time.Date(
    tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
    9, 0, 0, 0,
    time.Local,
)

taskID, err := r.Schedule(scheduledTime, "user.welcome", payload)
```

### 4. Dashboard

Once enabled, access the dashboard at `http://localhost:8080`

Features:
- Real-time task monitoring
- Task history and timeline
- Retry failed tasks
- View task payloads and errors
- Worker statistics

## 🔧 Configuration Options

```go
r := gorelay.New(
    // Worker configuration
    gorelay.WithWorkerCount(10),
    
    // Retry configuration
    gorelay.WithMaxRetries(5),
    
    // Dashboard
    gorelay.WithDashboard(":8080"),
    
    // Custom storage (SQLite default)
    gorelay.WithStorage("custom.db"),
)
```

## 💾 Storage Backends

### SQLite (Default)
```go
r := gorelay.New()  // Uses relay.db automatically
```

### PostgreSQL
```go
import _ "github.com/amitstephen-dev/gorelay/storage/postgres"

r := gorelay.New(
    gorelay.WithStorage("postgres://user:pass@localhost:5432/relay?sslmode=disable"),
)
```

### Redis
```go
import _ "github.com/amitstephen-dev/gorelay/storage/redis"

r := gorelay.New(
    gorelay.WithStorage("localhost:6379"),
)
```

## 📁 Examples

Check the `examples/` directory for complete working examples:
- `basic/` - Simple email task
- `priority/` - Priority queue demonstration
- `webapp/` - Full web application with API

## 📈 Performance

GoRelay is optimized for high performance:

- **50,000+ tasks/second** on modest hardware
- **<10MB memory** idle overhead
- **Zero-copy JSON** serialization
- **Lock-free ring buffer** for task queue
- **Batch writes** to storage

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by Resque, Sidekiq, and River
- Built with love for the Go community

---

**Made with ❤️ for Go developers**
