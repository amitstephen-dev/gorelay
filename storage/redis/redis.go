package redis

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
    "github.com/gorelay/gorelay/storage"
    "github.com/gorelay/gorelay/task"
)

type RedisStore struct {
    client *redis.Client
    ctx    context.Context
}

func New(addr string) (*RedisStore, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         addr,
        Password:     "",
        DB:           0,
        PoolSize:     10,
        MinIdleConns: 5,
    })
    
    ctx := context.Background()
    
    // Test connection
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, err
    }
    
    return &RedisStore{
        client: client,
        ctx:    ctx,
    }, nil
}

func (s *RedisStore) SaveTask(t *task.Task) error {
    data, err := json.Marshal(t)
    if err != nil {
        return err
    }
    
    // Store task hash
    key := fmt.Sprintf("relay:task:%s", t.ID)
    if err := s.client.Set(s.ctx, key, data, 7*24*time.Hour).Err(); err != nil {
        return err
    }
    
    // Add to priority queue
    queueKey := fmt.Sprintf("relay:queue:%d", t.Priority)
    score := float64(t.ExecuteAt.Unix())
    if err := s.client.ZAdd(s.ctx, queueKey, redis.Z{Score: score, Member: t.ID}).Err(); err != nil {
        return err
    }
    
    return s.RecordHistory(t.ID, "created", "")
}

func (s *RedisStore) ClaimTask(workerID string) (*task.Task, error) {
    // Try each priority queue in order
    priorities := []int{0, 1, 2} // High, Normal, Low
    
    for _, priority := range priorities {
        queueKey := fmt.Sprintf("relay:queue:%d", priority)
        
        // Get earliest task
        result, err := s.client.ZRangeByScore(s.ctx, queueKey, &redis.ZRangeBy{
            Min:    "-inf",
            Max:    fmt.Sprintf("%d", time.Now().Unix()),
            Offset: 0,
            Count:  1,
        }).Result()
        
        if err != nil || len(result) == 0 {
            continue
        }
        
        taskID := result[0]
        
        // Remove from queue
        removed, err := s.client.ZRem(s.ctx, queueKey, taskID).Result()
        if err != nil || removed == 0 {
            continue
        }
        
        // Get task data
        taskKey := fmt.Sprintf("relay:task:%s", taskID)
        data, err := s.client.Get(s.ctx, taskKey).Bytes()
        if err != nil {
            // Put back if error
            s.client.ZAdd(s.ctx, queueKey, redis.Z{Score: float64(time.Now().Unix()), Member: taskID})
            continue
        }
        
        var t task.Task
        if err := json.Unmarshal(data, &t); err != nil {
            continue
        }
        
        // Mark as running with TTL
        runningKey := fmt.Sprintf("relay:running:%s", taskID)
        s.client.Set(s.ctx, runningKey, workerID, 30*time.Second)
        
        t.Status = task.StatusRunning
        t.WorkerID = workerID
        t.ClaimedAt = time.Now()
        
        s.RecordHistory(t.ID, "claimed", "")
        
        return &t, nil
    }
    
    return nil, nil
}

func (s *RedisStore) BatchClaimTasks(workerID string, limit int) ([]*task.Task, error) {
    var tasks []*task.Task
    
    for i := 0; i < limit; i++ {
        task, err := s.ClaimTask(workerID)
        if err != nil || task == nil {
            break
        }
        tasks = append(tasks, task)
    }
    
    return tasks, nil
}

func (s *RedisStore) CompleteTask(id string) error {
    taskKey := fmt.Sprintf("relay:task:%s", id)
    runningKey := fmt.Sprintf("relay:running:%s", id)
    
    // Get task
    data, err := s.client.Get(s.ctx, taskKey).Bytes()
    if err != nil {
        return err
    }
    
    var t task.Task
    if err := json.Unmarshal(data, &t); err != nil {
        return err
    }
    
    // Update status
    t.Status = task.StatusCompleted
    t.CompletedAt = time.Now()
    
    newData, err := json.Marshal(t)
    if err != nil {
        return err
    }
    
    // Save updated task
    if err := s.client.Set(s.ctx, taskKey, newData, 7*24*time.Hour).Err(); err != nil {
        return err
    }
    
    // Remove from running
    s.client.Del(s.ctx, runningKey)
    
    // Add to completed set (for dashboard)
    completedKey := fmt.Sprintf("relay:completed:%s", time.Now().Format("2006-01-02"))
    s.client.SAdd(s.ctx, completedKey, id)
    s.client.Expire(s.ctx, completedKey, 24*time.Hour)
    
    return s.RecordHistory(id, "completed", "")
}

func (s *RedisStore) FailTask(id string, errMsg string) error {
    taskKey := fmt.Sprintf("relay:task:%s", id)
    runningKey := fmt.Sprintf("relay:running:%s", id)
    
    // Get task
    data, err := s.client.Get(s.ctx, taskKey).Bytes()
    if err != nil {
        return err
    }
    
    var t task.Task
    if err := json.Unmarshal(data, &t); err != nil {
        return err
    }
    
    t.RetryCount++
    t.LastError = errMsg
    
    if t.RetryCount < t.MaxRetries {
        // Retry with backoff
        backoff := time.Duration(t.RetryCount*t.RetryCount) * time.Second
        t.Status = task.StatusPending
        t.ExecuteAt = time.Now().Add(backoff)
        
        newData, err := json.Marshal(t)
        if err != nil {
            return err
        }
        
        // Save updated task
        if err := s.client.Set(s.ctx, taskKey, newData, 7*24*time.Hour).Err(); err != nil {
            return err
        }
        
        // Re-add to queue
        queueKey := fmt.Sprintf("relay:queue:%d", t.Priority)
        s.client.ZAdd(s.ctx, queueKey, redis.Z{Score: float64(t.ExecuteAt.Unix()), Member: t.ID})
        
        s.RecordHistory(id, "retried", errMsg)
    } else {
        // Move to dead letter
        t.Status = task.StatusDead
        
        newData, err := json.Marshal(t)
        if err != nil {
            return err
        }
        
        // Save as dead
        if err := s.client.Set(s.ctx, taskKey, newData, 30*24*time.Hour).Err(); err != nil {
            return err
        }
        
        // Add to dead set
        s.client.SAdd(s.ctx, "relay:dead", id)
        
        s.RecordHistory(id, "dead", errMsg)
    }
    
    // Remove from running
    s.client.Del(s.ctx, runningKey)
    
    return nil
}

func (s *RedisStore) GetTask(id string) (*task.Task, error) {
    taskKey := fmt.Sprintf("relay:task:%s", id)
    data, err := s.client.Get(s.ctx, taskKey).Bytes()
    if err != nil {
        return nil, err
    }
    
    var t task.Task
    if err := json.Unmarshal(data, &t); err != nil {
        return nil, err
    }
    
    return &t, nil
}

func (s *RedisStore) GetTasks(filter storage.TaskFilter) ([]*task.Task, error) {
    // For Redis, we'll get from different sources based on status
    var taskIDs []string
    var err error
    
    switch filter.Status {
    case task.StatusPending:
        // Get from all priority queues
        for priority := 0; priority <= 2; priority++ {
            queueKey := fmt.Sprintf("relay:queue:%d", priority)
            ids, err := s.client.ZRange(s.ctx, queueKey, 0, int64(filter.Limit-1)).Result()
            if err == nil {
                taskIDs = append(taskIDs, ids...)
            }
        }
    case task.StatusCompleted:
        completedKey := fmt.Sprintf("relay:completed:%s", time.Now().Format("2006-01-02"))
        taskIDs, err = s.client.SMembers(s.ctx, completedKey).Result()
    case task.StatusDead:
        taskIDs, err = s.client.SMembers(s.ctx, "relay:dead").Result()
    default:
        // For other statuses, we need to scan (expensive)
        return nil, fmt.Errorf("filter by status %s not efficient in Redis", filter.Status)
    }
    
    if err != nil {
        return nil, err
    }
    
    var tasks []*task.Task
    for i, id := range taskIDs {
        if filter.Limit > 0 && i >= filter.Limit {
            break
        }
        t, err := s.GetTask(id)
        if err == nil {
            tasks = append(tasks, t)
        }
    }
    
    return tasks, nil
}

func (s *RedisStore) GetStats() (*storage.Stats, error) {
    stats := &storage.Stats{}
    
    // Count pending from all queues
    for priority := 0; priority <= 2; priority++ {
        queueKey := fmt.Sprintf("relay:queue:%d", priority)
        count, _ := s.client.ZCard(s.ctx, queueKey).Result()
        stats.Pending += count
    }
    
    // Count running (approximate via keys pattern)
    keys, _ := s.client.Keys(s.ctx, "relay:running:*").Result()
    stats.Running = int64(len(keys))
    
    // Count dead
    stats.Dead, _ = s.client.SCard(s.ctx, "relay:dead").Result()
    
    // Count completed (today)
    completedKey := fmt.Sprintf("relay:completed:%s", time.Now().Format("2006-01-02"))
    stats.Completed, _ = s.client.SCard(s.ctx, completedKey).Result()
    
    return stats, nil
}

func (s *RedisStore) GetTaskHistory(taskID string) ([]*storage.HistoryEntry, error) {
    // Redis doesn't store history by default
    // We could store in a list, but for simplicity return empty
    return []*storage.HistoryEntry{}, nil
}

func (s *RedisStore) RecordHistory(taskID, event, message string) error {
    // Store history in a list with TTL
    historyKey := fmt.Sprintf("relay:history:%s", taskID)
    entry := storage.HistoryEntry{
        TaskID:    taskID,
        Event:     event,
        Message:   message,
        CreatedAt: time.Now(),
    }
    
    data, err := json.Marshal(entry)
    if err != nil {
        return err
    }
    
    s.client.RPush(s.ctx, historyKey, data)
    s.client.Expire(s.ctx, historyKey, 7*24*time.Hour)
    
    return nil
}

func (s *RedisStore) BatchSaveTasks(tasks []*task.Task) error {
    for _, t := range tasks {
        if err := s.SaveTask(t); err != nil {
            return err
        }
    }
    return nil
}

func (s *RedisStore) Cleanup(completedRetention, failedRetention time.Duration) error {
    // Redis handles TTL automatically
    return nil
}

func (s *RedisStore) Ping() error {
    return s.client.Ping(s.ctx).Err()
}

func (s *RedisStore) Close() error {
    return s.client.Close()
}