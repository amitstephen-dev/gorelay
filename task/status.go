package task

type Status string

const (
    StatusPending   Status = "pending"
    StatusRunning   Status = "running"
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusDead      Status = "dead"
)

func (s Status) String() string {
    return string(s)
}

func (s Status) IsTerminal() bool {
    return s == StatusCompleted || s == StatusDead
}

func (s Status) IsActive() bool {
    return s == StatusPending || s == StatusRunning
}

func StatusFromString(s string) Status {
    switch s {
    case "pending":
        return StatusPending
    case "running":
        return StatusRunning
    case "completed":
        return StatusCompleted
    case "failed":
        return StatusFailed
    case "dead":
        return StatusDead
    default:
        return StatusPending
    }
}