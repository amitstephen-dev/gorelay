package sqlite

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"
    "time"
    
     _ "modernc.org/sqlite"
    "github.com/amitstephen-dev/gorelay/storage"
    "github.com/amitstephen-dev/gorelay/task"
)

type SQLiteStore struct {
    db *sql.DB
}

func New(dsn string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    
    // IMPORTANT: Configure connection pool for SQLite
    db.SetMaxOpenConns(1)  // SQLite works best with single connection
    db.SetMaxIdleConns(1)
    
    // Optimize SQLite for better concurrency
    _, err = db.Exec(`
        PRAGMA journal_mode=WAL;
        PRAGMA synchronous=NORMAL;
        PRAGMA cache_size=-65536;
        PRAGMA temp_store=MEMORY;
        PRAGMA mmap_size=268435456;
        PRAGMA busy_timeout=10000;
        PRAGMA wal_autocheckpoint=1000;
    `)
    if err != nil {
        return nil, err
    }
    
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &SQLiteStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS relay_tasks (
        id TEXT PRIMARY KEY,
        topic TEXT NOT NULL,
        payload TEXT NOT NULL,
        status TEXT NOT NULL,
        priority INTEGER NOT NULL DEFAULT 1,
        retry_count INTEGER NOT NULL DEFAULT 0,
        max_retries INTEGER NOT NULL DEFAULT 3,
        execute_at DATETIME NOT NULL,
        last_error TEXT,
        worker_id TEXT,
        claimed_at DATETIME,
        created_at DATETIME NOT NULL,
        updated_at DATETIME NOT NULL,
        completed_at DATETIME
    );
    
    CREATE INDEX IF NOT EXISTS idx_tasks_status_execute 
    ON relay_tasks(status, execute_at, priority DESC);
    
    CREATE INDEX IF NOT EXISTS idx_tasks_topic 
    ON relay_tasks(topic);
    
    CREATE INDEX IF NOT EXISTS idx_tasks_created 
    ON relay_tasks(created_at DESC);
    
    CREATE TABLE IF NOT EXISTS relay_task_history (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        task_id TEXT NOT NULL,
        event TEXT NOT NULL,
        message TEXT,
        created_at DATETIME NOT NULL,
        FOREIGN KEY (task_id) REFERENCES relay_tasks(id) ON DELETE CASCADE
    );
    
    CREATE INDEX IF NOT EXISTS idx_history_task_id 
    ON relay_task_history(task_id, created_at DESC);
    `
    
    _, err := db.Exec(schema)
    return err
}

// SaveTask saves a single task to the database
func (s *SQLiteStore) SaveTask(t *task.Task) error {
    payload, err := json.Marshal(t.Payload)
    if err != nil {
        return err
    }
    
    _, err = s.db.Exec(`
        INSERT INTO relay_tasks (
            id, topic, payload, status, priority, retry_count, 
            max_retries, execute_at, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, t.ID, t.Topic, string(payload), t.Status, t.Priority,
        t.RetryCount, t.MaxRetries, t.ExecuteAt, t.CreatedAt, t.UpdatedAt)
    
    if err != nil {
        return err
    }
    
    return s.RecordHistory(t.ID, "created", "")
}

// ClaimTask claims a single task for processing
func (s *SQLiteStore) ClaimTask(workerID string) (*task.Task, error) {
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    var t task.Task
    var payload string
    var lastError sql.NullString  // Change this
    
    query := `
        SELECT id, topic, payload, status, priority, retry_count, max_retries, 
               execute_at, last_error, created_at, updated_at
        FROM relay_tasks
        WHERE status = 'pending' AND execute_at <= ?
        ORDER BY priority ASC, execute_at ASC
        LIMIT 1
    `
    
    err = tx.QueryRow(query, time.Now()).Scan(
        &t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
        &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &lastError,
        &t.CreatedAt, &t.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    
    // Convert NullString to string
    if lastError.Valid {
        t.LastError = lastError.String
    } else {
        t.LastError = ""
    }
    
    // Update task status
    _, err = tx.Exec(`
        UPDATE relay_tasks 
        SET status = 'running', worker_id = ?, claimed_at = ?, updated_at = ?
        WHERE id = ?
    `, workerID, time.Now(), time.Now(), t.ID)
    if err != nil {
        return nil, err
    }
    
    t.Status = task.StatusRunning
    t.WorkerID = workerID
    t.ClaimedAt = time.Now()
    
    if err := json.Unmarshal([]byte(payload), &t.Payload); err != nil {
        return nil, err
    }
    
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    s.RecordHistory(t.ID, "claimed", "")
    
    return &t, nil
}

// BatchClaimTasks claims multiple tasks at once
func (s *SQLiteStore) BatchClaimTasks(workerID string, limit int) ([]*task.Task, error) {
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    query := `
        SELECT id, topic, payload, status, priority, retry_count, max_retries,
               execute_at, last_error, created_at, updated_at
        FROM relay_tasks
        WHERE status = 'pending' AND execute_at <= ?
        ORDER BY priority ASC, execute_at ASC
        LIMIT ?
    `
    
    rows, err := tx.Query(query, time.Now(), limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var tasks []*task.Task
    var taskIDs []string
    
    for rows.Next() {
        var t task.Task
        var payload string
        var lastError sql.NullString  // Change this
        
        err := rows.Scan(&t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
            &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &lastError,
            &t.CreatedAt, &t.UpdatedAt)
        if err != nil {
            return nil, err
        }
        
        // Convert NullString to string
        if lastError.Valid {
            t.LastError = lastError.String
        } else {
            t.LastError = ""
        }
        
        if err := json.Unmarshal([]byte(payload), &t.Payload); err != nil {
            return nil, err
        }
        
        t.Status = task.StatusRunning
        t.WorkerID = workerID
        t.ClaimedAt = time.Now()
        
        tasks = append(tasks, &t)
        taskIDs = append(taskIDs, t.ID)
    }
    
    if len(taskIDs) > 0 {
        // Update status to running for all claimed tasks
        placeholders := strings.Repeat("?,", len(taskIDs))
        placeholders = placeholders[:len(placeholders)-1]
        
        updateQuery := fmt.Sprintf(`
            UPDATE relay_tasks 
            SET status = 'running', worker_id = ?, claimed_at = ?, updated_at = ?
            WHERE id IN (%s)
        `, placeholders)
        
        args := []interface{}{workerID, time.Now(), time.Now()}
        for _, id := range taskIDs {
            args = append(args, id)
        }
        
        _, err := tx.Exec(updateQuery, args...)
        if err != nil {
            return nil, err
        }
    }
    
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    // Record history for claimed tasks
    for _, t := range tasks {
        s.RecordHistory(t.ID, "claimed", "")
    }
    
    return tasks, nil
}

// CompleteTask marks a task as completed
func (s *SQLiteStore) CompleteTask(id string) error {
    result, err := s.db.Exec(`
        UPDATE relay_tasks 
        SET status = 'completed', completed_at = ?, updated_at = ?
        WHERE id = ?
    `, time.Now(), time.Now(), id)
    
    if err != nil {
        return err
    }
    
    rows, _ := result.RowsAffected()
    if rows > 0 {
        s.RecordHistory(id, "completed", "")
    }
    
    return nil
}

// FailTask marks a task as failed and handles retries
func (s *SQLiteStore) FailTask(id string, errMsg string) error {
    var retryCount, maxRetries int
    
    err := s.db.QueryRow(`
        SELECT retry_count, max_retries FROM relay_tasks WHERE id = ?
    `, id).Scan(&retryCount, &maxRetries)
    
    if err != nil {
        return err
    }
    
    retryCount++
    
    if retryCount < maxRetries {
        // Retry later with exponential backoff
        backoff := time.Duration(retryCount*retryCount) * time.Second
        
        _, err = s.db.Exec(`
            UPDATE relay_tasks 
            SET status = 'pending', retry_count = ?, last_error = ?, 
                execute_at = ?, updated_at = ?
            WHERE id = ?
        `, retryCount, errMsg, time.Now().Add(backoff), time.Now(), id)
        
        if err == nil {
            s.RecordHistory(id, "retried", errMsg)
        }
    } else {
        // Move to dead letter
        _, err = s.db.Exec(`
            UPDATE relay_tasks 
            SET status = 'dead', last_error = ?, updated_at = ?
            WHERE id = ?
        `, errMsg, time.Now(), id)
        
        if err == nil {
            s.RecordHistory(id, "dead", errMsg)
        }
    }
    
    return err
}

// GetTask retrieves a single task by ID
func (s *SQLiteStore) GetTask(id string) (*task.Task, error) {
    var t task.Task
    var payload string
    var lastError sql.NullString  // Change this
    var workerID sql.NullString
    var claimedAt sql.NullTime
    var completedAt sql.NullTime
    
    query := `
        SELECT id, topic, payload, status, priority, retry_count, max_retries,
               execute_at, last_error, worker_id, claimed_at, created_at, updated_at, completed_at
        FROM relay_tasks WHERE id = ?
    `
    
    err := s.db.QueryRow(query, id).Scan(
        &t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
        &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &lastError,
        &workerID, &claimedAt, &t.CreatedAt, &t.UpdatedAt, &completedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    // Convert Null types to zero values
    if lastError.Valid {
        t.LastError = lastError.String
    }
    if workerID.Valid {
        t.WorkerID = workerID.String
    }
    if claimedAt.Valid {
        t.ClaimedAt = claimedAt.Time
    }
    if completedAt.Valid {
        t.CompletedAt = completedAt.Time
    }
    
    if err := json.Unmarshal([]byte(payload), &t.Payload); err != nil {
        return nil, err
    }
    
    return &t, nil
}

// GetTasks retrieves tasks with filtering
func (s *SQLiteStore) GetTasks(filter storage.TaskFilter) ([]*task.Task, error) {
    query := `SELECT id, topic, payload, status, priority, retry_count, 
                     max_retries, execute_at, created_at FROM relay_tasks WHERE 1=1`
    
    args := []interface{}{}
    argCount := 1
    
    if filter.Topic != "" {
        query += fmt.Sprintf(" AND topic = $%d", argCount)
        args = append(args, filter.Topic)
        argCount++
    }
    
    if filter.Status != "" {
        query += fmt.Sprintf(" AND status = $%d", argCount)
        args = append(args, filter.Status)
        argCount++
    }
    
    if filter.OrderBy != "" {
        query += fmt.Sprintf(" ORDER BY %s", filter.OrderBy)
    } else {
        query += " ORDER BY created_at DESC"
    }
    
    if filter.Limit > 0 {
        query += fmt.Sprintf(" LIMIT $%d", argCount)
        args = append(args, filter.Limit)
        argCount++
    }
    
    if filter.Offset > 0 {
        query += fmt.Sprintf(" OFFSET $%d", argCount)
        args = append(args, filter.Offset)
    }
    
    rows, err := s.db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var tasks []*task.Task
    for rows.Next() {
        var t task.Task
        var payload string
        
        err := rows.Scan(&t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
            &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &t.CreatedAt)
        if err != nil {
            return nil, err
        }
        
        if err := json.Unmarshal([]byte(payload), &t.Payload); err != nil {
            return nil, err
        }
        
        tasks = append(tasks, &t)
    }
    
    return tasks, nil
}

// GetStats returns statistics about tasks
func (s *SQLiteStore) GetStats() (*storage.Stats, error) {
    query := `
        SELECT 
            COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending,
            COUNT(CASE WHEN status = 'running' THEN 1 END) as running,
            COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
            COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed,
            COUNT(CASE WHEN status = 'dead' THEN 1 END) as dead,
            COUNT(*) as total
        FROM relay_tasks
    `
    
    var stats storage.Stats
    err := s.db.QueryRow(query).Scan(
        &stats.Pending, &stats.Running, &stats.Completed,
        &stats.Failed, &stats.Dead, &stats.Total,
    )
    
    return &stats, err
}

// GetTaskHistory retrieves the history of a task
func (s *SQLiteStore) GetTaskHistory(taskID string) ([]*storage.HistoryEntry, error) {
    rows, err := s.db.Query(`
        SELECT id, task_id, event, message, created_at
        FROM relay_task_history
        WHERE task_id = ?
        ORDER BY created_at ASC
    `, taskID)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var history []*storage.HistoryEntry
    for rows.Next() {
        var h storage.HistoryEntry
        err := rows.Scan(&h.ID, &h.TaskID, &h.Event, &h.Message, &h.CreatedAt)
        if err != nil {
            return nil, err
        }
        history = append(history, &h)
    }
    
    return history, nil
}

// RecordHistory records an event in task history
func (s *SQLiteStore) RecordHistory(taskID, event, message string) error {
    _, err := s.db.Exec(`
        INSERT INTO relay_task_history (task_id, event, message, created_at)
        VALUES (?, ?, ?, ?)
    `, taskID, event, message, time.Now())
    
    return err
}

// BatchSaveTasks saves multiple tasks in a single transaction
func (s *SQLiteStore) BatchSaveTasks(tasks []*task.Task) error {
    tx, err := s.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    for _, t := range tasks {
        payload, err := json.Marshal(t.Payload)
        if err != nil {
            return err
        }
        
        _, err = tx.Exec(`
            INSERT INTO relay_tasks (
                id, topic, payload, status, priority, retry_count,
                max_retries, execute_at, created_at, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `, t.ID, t.Topic, string(payload), t.Status, t.Priority,
            t.RetryCount, t.MaxRetries, t.ExecuteAt, t.CreatedAt, t.UpdatedAt)
        
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// Cleanup removes old completed and failed tasks
func (s *SQLiteStore) Cleanup(completedRetention, failedRetention time.Duration) error {
    // Delete completed tasks older than retention period
    _, err := s.db.Exec(`
        DELETE FROM relay_tasks 
        WHERE status = 'completed' AND completed_at < ?
    `, time.Now().Add(-completedRetention))
    
    if err != nil {
        return err
    }
    
    // Delete failed tasks older than retention period
    _, err = s.db.Exec(`
        DELETE FROM relay_tasks 
        WHERE status = 'failed' AND updated_at < ?
    `, time.Now().Add(-failedRetention))
    
    return err
}

// Ping checks the database connection
func (s *SQLiteStore) Ping() error {
    return s.db.Ping()
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
    return s.db.Close()
}
