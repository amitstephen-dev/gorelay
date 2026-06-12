package task

type Priority int

const (
    PriorityHigh   Priority = 0
    PriorityNormal Priority = 1
    PriorityLow    Priority = 2
)

func (p Priority) String() string {
    switch p {
    case PriorityHigh:
        return "high"
    case PriorityNormal:
        return "normal"
    case PriorityLow:
        return "low"
    default:
        return "unknown"
    }
}

func (p Priority) Int() int {
    return int(p)
}

func PriorityFromString(s string) Priority {
    switch s {
    case "high":
        return PriorityHigh
    case "normal":
        return PriorityNormal
    case "low":
        return PriorityLow
    default:
        return PriorityNormal
    }
}

func PriorityFromInt(i int) Priority {
    switch i {
    case 0:
        return PriorityHigh
    case 1:
        return PriorityNormal
    case 2:
        return PriorityLow
    default:
        return PriorityNormal
    }
}
