// Auto-refresh dashboard
let refreshInterval = null;

document.addEventListener('DOMContentLoaded', function() {
    // Set up auto-refresh
    refreshInterval = setInterval(() => {
        refreshData();
    }, 3000);
});

function refreshData() {
    // Refresh stats
    fetch('/api/stats')
        .then(response => response.json())
        .then(stats => {
            document.getElementById('stat-pending').textContent = stats.pending;
            document.getElementById('stat-running').textContent = stats.running;
            document.getElementById('stat-completed').textContent = stats.completed;
            document.getElementById('stat-failed').textContent = stats.failed;
            document.getElementById('stat-dead').textContent = stats.dead;
            document.getElementById('stat-total').textContent = stats.total;
        })
        .catch(err => console.error('Failed to load stats:', err));
    
    // Refresh tasks
    const status = getCurrentStatus();
    if (status) {
        loadTasksByStatus(status);
    } else {
        loadRecentTasks();
    }
}

function getCurrentStatus() {
    const path = window.location.pathname;
    if (path.includes('/tasks/pending')) return 'pending';
    if (path.includes('/tasks/running')) return 'running';
    if (path.includes('/tasks/completed')) return 'completed';
    if (path.includes('/tasks/failed')) return 'failed';
    if (path.includes('/tasks/dead')) return 'dead';
    return null;
}

function loadRecentTasks() {
    fetch('/api/tasks?limit=50')
        .then(response => response.json())
        .then(tasks => renderTasks(tasks))
        .catch(err => console.error('Failed to load tasks:', err));
}

function loadTasksByStatus(status) {
    fetch(`/api/tasks?status=${status}&limit=100`)
        .then(response => response.json())
        .then(tasks => renderTasks(tasks))
        .catch(err => console.error('Failed to load tasks:', err));
}

function renderTasks(tasks) {
    const tbody = document.getElementById('tasks-list');
    if (!tbody) return;
    
    if (tasks.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7">No tasks found</td></tr>';
        return;
    }
    
    tbody.innerHTML = '';
    for (const task of tasks) {
        const row = tbody.insertRow();
        row.insertCell(0).innerHTML = `<a href="/task/${task.id}">${task.id}</a>`;
        row.insertCell(1).textContent = task.topic;
        row.insertCell(2).innerHTML = `<span class="status-${task.status}">${task.status}</span>`;
        row.insertCell(3).textContent = task.priority;
        row.insertCell(4).textContent = `${task.retry_count}/${task.max_retries}`;
        row.insertCell(5).textContent = new Date(task.created_at).toLocaleString();
        
        const actions = row.insertCell(6);
        if (task.status === 'failed' || task.status === 'dead') {
            const retryBtn = document.createElement('button');
            retryBtn.textContent = 'Retry';
            retryBtn.onclick = () => retryTask(task.id);
            actions.appendChild(retryBtn);
        }
    }
}

function retryTask(taskId) {
    fetch(`/api/task/${taskId}/retry`, { method: 'POST' })
        .then(() => {
            refreshData();
        })
        .catch(err => console.error('Failed to retry task:', err));
}