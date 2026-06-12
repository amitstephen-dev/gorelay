package postgres

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
    
    _ "github.com/lib/pq"
    "github.com/gorelay/gorelay/storage"
    "github.com/gorelay/gorelay/task"
)

type PostgresStore struct {
    db *sql.DB
}

func New(dsn string) (*PostgresStore, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    
    // Test connection
    if err := db.Ping(); err != nil {
        return nil, err
    }
    
    // Set connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    if err := initSchema(db); err != nil {
        return nil, err
    }
    
    return &PostgresStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS relay_tasks (
        id TEXT PRIMARY KEY,
        topic TEXT NOT NULL,
        payload JSONB NOT NULL,
        status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'completed', 'failed', 'dead')),
        priority INTEGER NOT NULL DEFAULT 1 CHECK(priority IN (0, 1, 2)),
        retry_count INTEGER NOT NULL DEFAULT 0,
        max_retries INTEGER NOT NULL DEFAULT 3,
        execute_at TIMESTAMP NOT NULL,
        last_error TEXT,
        worker_id TEXT,
        claimed_at TIMESTAMP,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
        completed_at TIMESTAMP
    );
    
    CREATE INDEX IF NOT EXISTS idx_tasks_status_execute 
    ON relay_tasks(status, execute_at, priority DESC) 
    WHERE status = 'pending';
    
    CREATE INDEX IF NOT EXISTS idx_tasks_topic 
    ON relay_tasks(topic);
    
    CREATE TABLE IF NOT EXISTS relay_task_history (
        id BIGSERIAL PRIMARY KEY,
        task_id TEXT NOT NULL,
        event TEXT NOT NULL,
        message TEXT,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        FOREIGN KEY (task_id) REFERENCES relay_tasks(id) ON DELETE CASCADE
    );
    
    CREATE INDEX IF NOT EXISTS idx_history_task_id 
    ON relay_task_history(task_id, created_at DESC);
    
    -- Create function for LISTEN/NOTIFY
    CREATE OR REPLACE FUNCTION notify_task_available()
    RETURNS TRIGGER AS $$
    BEGIN
        IF NEW.status = 'pending' AND NEW.execute_at <= NOW() THEN
            PERFORM pg_notify('relay_tasks_available', NEW.id);
        END IF;
        RETURN NEW;
    END;
    $$ LANGUAGE plpgsql;
    
    -- Create trigger
    DROP TRIGGER IF EXISTS trigger_notify_task_available ON relay_tasks;
    CREATE TRIGGER trigger_notify_task_available
        AFTER INSERT OR UPDATE OF status, execute_at ON relay_tasks
        FOR EACH ROW
        EXECUTE FUNCTION notify_task_available();
    `
    
    _, err := db.Exec(schema)
    return err
}

func (s *PostgresStore) SaveTask(t *task.Task) error {
    payload, err := json.Marshal(t.Payload)
    if err != nil {
        return err
    }
    
    query := `
        INSERT INTO relay_tasks (
            id, topic, payload, status, priority, retry_count,
            max_retries, execute_at, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
    
    _, err = s.db.Exec(query, t.ID, t.Topic, payload, t.Status,
        t.Priority, t.RetryCount, t.MaxRetries, t.ExecuteAt,
        t.CreatedAt, t.UpdatedAt)
    
    if err != nil {
        return err
    }
    
    return s.RecordHistory(t.ID, "created", "")
}

func (s *PostgresStore) ClaimTask(workerID string) (*task.Task, error) {
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    var t task.Task
    var payload []byte
    
    query := `
        SELECT id, topic, payload, status, priority, retry_count, max_retries,
               execute_at, last_error, created_at, updated_at
        FROM relay_tasks
        WHERE status = 'pending' AND execute_at <= NOW()
        ORDER BY priority ASC, execute_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    `
    
    err = tx.QueryRow(query).Scan(
        &t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
        &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &t.LastError,
        &t.CreatedAt, &t.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    
    // Update task status
    _, err = tx.Exec(`
        UPDATE relay_tasks 
        SET status = 'running', worker_id = $1, claimed_at = NOW(), updated_at = NOW()
        WHERE id = $2
    `, workerID, t.ID)
    if err != nil {
        return nil, err
    }
    
    if err := json.Unmarshal(payload, &t.Payload); err != nil {
        return nil, err
    }
    
    t.Status = task.StatusRunning
    t.WorkerID = workerID
    t.ClaimedAt = time.Now()
    
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    s.RecordHistory(t.ID, "claimed", "")
    
    return &t, nil
}

func (s *PostgresStore) BatchClaimTasks(workerID string, limit int) ([]*task.Task, error) {
    tx, err := s.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    query := `
        WITH claimed AS (
            SELECT id FROM relay_tasks
            WHERE status = 'pending' AND execute_at <= NOW()
            ORDER BY priority ASC, execute_at ASC
            LIMIT $1
            FOR UPDATE SKIP LOCKED
        )
        UPDATE relay_tasks 
        SET status = 'running', worker_id = $2, claimed_at = NOW(), updated_at = NOW()
        WHERE id IN (SELECT id FROM claimed)
        RETURNING id, topic, payload, priority, retry_count, max_retries, execute_at
    `
    
    rows, err := tx.Query(query, limit, workerID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var tasks []*task.Task
    for rows.Next() {
        var t task.Task
        var payload []byte
        
        err := rows.Scan(&t.ID, &t.Topic, &payload, &t.Priority,
            &t.RetryCount, &t.MaxRetries, &t.ExecuteAt)
        if err != nil {
            return nil, err
        }
        
        if err := json.Unmarshal(payload, &t.Payload); err != nil {
            return nil, err
        }
        
        t.Status = task.StatusRunning
        t.WorkerID = workerID
        t.ClaimedAt = time.Now()
        
        tasks = append(tasks, &t)
    }
    
    if err := tx.Commit(); err != nil {
        return nil, err
    }
    
    for _, t := range tasks {
        s.RecordHistory(t.ID, "claimed", "")
    }
    
    return tasks, nil
}

func (s *PostgresStore) CompleteTask(id string) error {
    result, err := s.db.Exec(`
        UPDATE relay_tasks 
        SET status = 'completed', completed_at = NOW(), updated_at = NOW()
        WHERE id = $1 AND status = 'running'
    `, id)
    
    if err != nil {
        return err
    }
    
    rows, _ := result.RowsAffected()
    if rows > 0 {
        s.RecordHistory(id, "completed", "")
    }
    
    return nil
}

func (s *PostgresStore) FailTask(id string, errMsg string) error {
    var retryCount, maxRetries int
    
    err := s.db.QueryRow(`
        SELECT retry_count, max_retries FROM relay_tasks WHERE id = $1
    `, id).Scan(&retryCount, &maxRetries)
    
    if err != nil {
        return err
    }
    
    retryCount++
    
    if retryCount < maxRetries {
        backoff := time.Duration(retryCount*retryCount) * time.Second
        
        _, err = s.db.Exec(`
            UPDATE relay_tasks 
            SET status = 'pending', retry_count = $1, last_error = $2,
                execute_at = NOW() + $3, updated_at = NOW()
            WHERE id = $4
        `, retryCount, errMsg, backoff, id)
        
        if err == nil {
            s.RecordHistory(id, "retried", errMsg)
        }
    } else {
        _, err = s.db.Exec(`
            UPDATE relay_tasks 
            SET status = 'dead', last_error = $1, updated_at = NOW()
            WHERE id = $2
        `, errMsg, id)
        
        if err == nil {
            s.RecordHistory(id, "dead", errMsg)
        }
    }
    
    return err
}

func (s *PostgresStore) GetTask(id string) (*task.Task, error) {
    var t task.Task
    var payload []byte
    
    query := `
        SELECT id, topic, payload, status, priority, retry_count, max_retries,
               execute_at, last_error, worker_id, claimed_at, created_at, updated_at, completed_at
        FROM relay_tasks WHERE id = $1
    `
    
    err := s.db.QueryRow(query, id).Scan(
        &t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
        &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &t.LastError,
        &t.WorkerID, &t.ClaimedAt, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
    )
    
    if err != nil {
        return nil, err
    }
    
    if err := json.Unmarshal(payload, &t.Payload); err != nil {
        return nil, err
    }
    
    return &t, nil
}

func (s *PostgresStore) GetTasks(filter storage.TaskFilter) ([]*task.Task, error) {
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
        var payload []byte
        
        err := rows.Scan(&t.ID, &t.Topic, &payload, &t.Status, &t.Priority,
            &t.RetryCount, &t.MaxRetries, &t.ExecuteAt, &t.CreatedAt)
        if err != nil {
            return nil, err
        }
        
        if err := json.Unmarshal(payload, &t.Payload); err != nil {
            return nil, err
        }
        
        tasks = append(tasks, &t)
    }
    
    return tasks, nil
}

func (s *PostgresStore) GetStats() (*storage.Stats, error) {
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

func (s *PostgresStore) GetTaskHistory(taskID string) ([]*storage.HistoryEntry, error) {
    rows, err := s.db.Query(`
        SELECT id, task_id, event, message, created_at
        FROM relay_task_history
        WHERE task_id = $1
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

func (s *PostgresStore) RecordHistory(taskID, event, message string) error {
    _, err := s.db.Exec(`
        INSERT INTO relay_task_history (task_id, event, message)
        VALUES ($1, $2, $3)
    `, taskID, event, message)
    
    return err
}

func (s *PostgresStore) BatchSaveTasks(tasks []*task.Task) error {
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
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        `, t.ID, t.Topic, payload, t.Status, t.Priority,
            t.RetryCount, t.MaxRetries, t.ExecuteAt, t.CreatedAt, t.UpdatedAt)
        
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

func (s *PostgresStore) Cleanup(completedRetention, failedRetention time.Duration) error {
    // Delete completed tasks older than retention period
    _, err := s.db.Exec(`
        DELETE FROM relay_tasks 
        WHERE status = 'completed' AND completed_at < NOW() - $1
    `, completedRetention)
    
    if err != nil {
        return err
    }
    
    // Delete failed tasks older than retention period
    _, err = s.db.Exec(`
        DELETE FROM relay_tasks 
        WHERE status = 'failed' AND updated_at < NOW() - $1
    `, failedRetention)
    
    return err
}

func (s *PostgresStore) Ping() error {
    return s.db.Ping()
}

func (s *PostgresStore) Close() error {
    return s.db.Close()
}