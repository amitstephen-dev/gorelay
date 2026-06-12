<<<<<<< HEAD
# GoRelay - Durable Task Queue for Go

[![Go Report Card](https://goreportcard.com/badge/github.com/gorelay/gorelay)](https://goreportcard.com/report/github.com/gorelay/gorelay)
[![Go Reference](https://pkg.go.dev/badge/github.com/gorelay/gorelay.svg)](https://pkg.go.dev/github.com/gorelay/gorelay)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

GoRelay is a simple, durable background job library for Go. It turns any function into a crash-resistant, retryable task with zero infrastructure required.

## Features

- **Simple API** - Just `Register`, `Enqueue`, and `Schedule`
- **Durable** - Tasks survive crashes and restarts
- **Zero Infrastructure** - SQLite by default, no external dependencies
- **Dashboard** - Beautiful web UI for monitoring
- **Retries** - Automatic with exponential backoff
- **Scheduling** - Run tasks at any time in the future
- **Multiple Backends** - SQLite, PostgreSQL, Redis support

## Quick Start

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
    r := gorelay.New()
    
    r.Register("email.send", SendEmail, &EmailPayload{})
    r.EnableDashboard(":8080")
    r.Start()
    
    r.Enqueue("email.send", &EmailPayload{
        To:   "user@example.com",
        Body: "Hello!",
    })
    
    select {}
}package main

import "fmt"

// hasDuplicate returns true and a slice of duplicates if any values repeat.
func hasDuplicate(nums []int) (bool, []int) {
    seen := make(map[int]struct{})
    // Map to track duplicates uniquely so we don't report the same duplicate twice
    duplicatesMap := make(map[int]struct{})
    
    for _, num := range nums {
        if _, exists := seen[num]; exists {
            duplicatesMap[num] = struct{}{}
        } else {
            seen[num] = struct{}{}
        }
    }
    
    // If no duplicates were found, return early
    if len(duplicatesMap) == 0 {
        return false, []int{}
    }
    
    // Convert the duplicates map into a slice
    var duplicates []int
    for num := range duplicatesMap {
        duplicates = append(duplicates, num)
    }
    
    return true, duplicates
}

func main() {
    nums := []int{1, 2, 3, 1, 2}
    hasDup, values := hasDuplicate(nums)
    
    fmt.Println("Contains duplicates:", hasDup)
    fmt.Println("Duplicate values:", values)
}

