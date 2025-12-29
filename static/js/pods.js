// Текущий выбранный namespace
let currentNamespace = 'market';
let currentPod = null;
let allPods = [];

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadPods();
    setupEventListeners();
});

// Настройка слушателей событий
function setupEventListeners() {
    // Выбор namespace
    document.getElementById('namespace-select').addEventListener('change', function() {
        currentNamespace = this.value;
        document.getElementById('current-namespace').textContent = currentNamespace;
        loadPods();
    });
    
    // Поиск
    document.getElementById('search-pods').addEventListener('input', debounce(filterPods, 300));
    
    // Фильтры по статусу
    document.querySelectorAll('input[type="checkbox"]').forEach(checkbox => {
        checkbox.addEventListener('change', filterPods);
    });
    
    // Кнопка обновления
    document.getElementById('refresh-btn').addEventListener('click', loadPods);
}

// Загрузка списка подов
async function loadPods() {
    showLoading(true);
    
    try {
        const response = await fetch(`/api/pods?namespace=${currentNamespace}`);
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`HTTP ${response.status}: ${errorText}`);
        }
        
        const data = await response.json();
        
        // Проверяем структуру ответа
        if (!data) {
            throw new Error('Empty response from server');
        }
        
        // data.pods может быть undefined, если нет подов
        const pods = data.pods || [];
        const namespace = data.namespace || currentNamespace;
        const count = data.count || pods.length;
        
        allPods = pods;
        
        updateStats(pods);
        renderPodsTable(pods);
        updateLastUpdated();
        
        console.log(`Loaded ${pods.length} pods from ${namespace}`);
        
    } catch (error) {
        console.error('Error loading pods:', error);
        showError('Failed to load pods: ' + error.message);
        
        // Показываем сообщение об ошибке в таблице
        const tbody = document.getElementById('pods-table-body');
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="text-center py-5">
                    <i class="fas fa-exclamation-triangle fa-2x text-danger mb-3"></i>
                    <p class="text-danger">Failed to load pods</p>
                    <p class="text-muted small">${error.message}</p>
                    <button class="btn btn-sm btn-outline-primary mt-2" onclick="testConnection()">
                        Test Connection
                    </button>
                </td>
            </tr>
        `;
    } finally {
        showLoading(false);
    }
}

async function testConnection() {
    try {
        const response = await fetch('/api/test');
        const data = await response.json();
        
        if (data.connected) {
            showToast('Connected to Kubernetes API!', 'success');
            loadPods();
        } else {
            showToast(`Not connected: ${data.error}`, 'error');
        }
    } catch (error) {
        showToast('Connection test failed: ' + error.message, 'error');
    }
}

// Обновление статистики
function updateStats(pods) {
    const stats = {
        running: 0,
        pending: 0,
        failed: 0,
        memory: 0
    };
    
    pods.forEach(pod => {
        if (pod.status === 'Running') stats.running++;
        else if (pod.status === 'Pending') stats.pending++;
        else if (pod.status === 'Failed') stats.failed++;
        
        // Симуляция использования памяти (в реальном приложении нужно получать метрики)
        stats.memory += Math.floor(Math.random() * 100) + 10;
    });
    
    document.getElementById('running-count').textContent = stats.running;
    document.getElementById('pending-count').textContent = stats.pending;
    document.getElementById('failed-count').textContent = stats.failed;
    document.getElementById('memory-usage').textContent = stats.memory;
    document.getElementById('stats-count').textContent = pods.length;
}

// Рендер таблицы подов
function renderPodsTable(pods) {
    const tbody = document.getElementById('pods-table-body');
    
    // Проверяем, что pods - массив
    if (!Array.isArray(pods)) {
        console.error('pods is not an array:', pods);
        pods = [];
    }
    
    const searchTerm = document.getElementById('search-pods').value.toLowerCase();
    
    // Фильтрация по статусу
    const activeStatuses = Array.from(document.querySelectorAll('input[type="checkbox"]:checked'))
        .map(cb => cb.value);
    
    let filteredPods = pods.filter(pod => {
        // Проверяем структуру объекта pod
        if (!pod || typeof pod !== 'object') return false;
        
        // Поиск по имени
        if (searchTerm && (!pod.name || !pod.name.toLowerCase().includes(searchTerm))) {
            return false;
        }
        // Фильтр по статусу
        if (activeStatuses.length > 0 && (!pod.status || !activeStatuses.includes(pod.status))) {
            return false;
        }
        return true;
    });
    
    if (filteredPods.length === 0) {
        let message = 'No pods found';
        if (pods.length === 0) {
            message = 'No pods in this namespace';
        } else if (searchTerm || activeStatuses.length > 0) {
            message = 'No pods match the current filters';
        }
        
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="text-center py-5">
                    <i class="fas fa-search fa-2x text-muted mb-3"></i>
                    <p class="text-muted">${message}</p>
                    ${pods.length > 0 ? '<small class="text-muted">Try changing your filters</small>' : ''}
                </td>
            </tr>
        `;
        return;
    }
    
    let html = '';
    filteredPods.forEach((pod, index) => {
        // Дефолтные значения на случай отсутствия полей
        const podName = pod.name || `pod-${index}`;
        const namespace = pod.namespace || currentNamespace;
        const status = pod.status || 'Unknown';
        const ready = pod.ready || '0/0';
        const restarts = pod.restarts || 0;
        const age = pod.age || 'unknown';
        const node = pod.node || '-';
        const ip = pod.ip || '-';
        
        const statusClass = getStatusClass(status);
        const memoryUsage = Math.floor(Math.random() * 100) + 10; // Симуляция
        
        html += `
            <tr data-pod-name="${podName}" data-namespace="${namespace}">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-cube me-2 text-primary"></i>
                        <strong>${highlightSearch(podName, searchTerm)}</strong>
                        ${ip ? `<small class="text-muted ms-2">(${ip})</small>` : ''}
                    </div>
                </td>
                <td><span class="badge bg-secondary">${namespace}</span></td>
                <td>
                    <span class="badge-status ${statusClass}">
                        <i class="fas ${getStatusIcon(status)} me-1"></i>
                        ${status}
                    </span>
                </td>
                <td>
                    <span class="badge ${ready.includes('/') ? 
                        (ready.split('/')[0] === ready.split('/')[1] ? 'bg-success' : 'bg-warning') : 
                        'bg-secondary'}">
                        ${ready}
                    </span>
                </td>
                <td>
                    <span class="badge ${restarts > 0 ? 'bg-warning' : 'bg-secondary'}">
                        ${restarts}
                    </span>
                </td>
                <td>
                    <div>
                        <small>${memoryUsage} MB</small>
                        <div class="memory-bar">
                            <div class="memory-fill" style="width: ${Math.min(memoryUsage / 2, 100)}%"></div>
                        </div>
                    </div>
                </td>
                <td><small class="text-muted">${age}</small></td>
                <td><small>${node}</small></td>
                <td>
                    <div class="btn-group" role="group">
                        <button class="btn btn-action btn-outline-primary btn-sm" 
                                onclick="showPodDetails('${namespace}', '${podName}')"
                                title="Details">
                            <i class="fas fa-info"></i>
                        </button>
                        <button class="btn btn-action btn-outline-success btn-sm" 
                                onclick="showLogs('${namespace}', '${podName}')"
                                title="Logs">
                            <i class="fas fa-file-alt"></i>
                        </button>
                        <button class="btn btn-action btn-outline-warning btn-sm" 
                                onclick="showConfig('${namespace}', '${podName}')"
                                title="YAML">
                            <i class="fas fa-code"></i>
                        </button>
                        <button class="btn btn-action btn-outline-danger btn-sm" 
                                onclick="deletePod('${namespace}', '${podName}')"
                                title="Delete">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
    
    // Добавляем обработчики кликов на строки
    tbody.querySelectorAll('tr[data-pod-name]').forEach(row => {
        row.addEventListener('click', (e) => {
            if (!e.target.closest('button')) {
                const namespace = row.dataset.namespace;
                const podName = row.dataset.podName;
                showPodDetails(namespace, podName);
            }
        });
    });
}

// Показать логи пода
async function showLogs(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('logs-pod-name').textContent = podName;
    const modal = new bootstrap.Modal(document.getElementById('logsModal'));
    modal.show();
    
    await loadLogs();
}

// Загрузить логи
async function loadLogs() {
    const lines = document.getElementById('log-lines').value;
    const podName = currentPod.name;
    const namespace = currentPod.namespace;
    
    try {
        const response = await fetch(`/api/logs/${namespace}/${podName}?tail=${lines}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        document.getElementById('logs-content').textContent = data.logs;
        document.getElementById('logs-info').textContent = 
            `Showing last ${lines} lines from pod ${podName}`;
        
        // Автопрокрутка вниз
        const logsContainer = document.getElementById('logs-content');
        logsContainer.scrollTop = logsContainer.scrollHeight;
        
    } catch (error) {
        document.getElementById('logs-content').textContent = 
            `Error loading logs: ${error.message}`;
    }
}

// Скачать логи
function downloadLogs() {
    const podName = currentPod.name;
    const namespace = currentPod.namespace;
    const lines = document.getElementById('log-lines').value;
    
    window.location.href = `/api/logs/download/${namespace}/${podName}?tail=${lines}`;
}

// Показать конфигурацию пода
async function showConfig(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('config-pod-name').textContent = podName;
    const modal = new bootstrap.Modal(document.getElementById('configModal'));
    modal.show();
    
    await loadYAML();
}

// Загрузить YAML конфигурацию
async function loadYAML() {
    const podName = currentPod.name;
    const namespace = currentPod.namespace;
    
    try {
        const response = await fetch(`/api/pod/yaml/${namespace}/${podName}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        document.getElementById('yaml-content').textContent = data.yaml;
        document.getElementById('yaml-editor').value = data.yaml;
        
    } catch (error) {
        document.getElementById('yaml-content').textContent = 
            `Error loading YAML: ${error.message}`;
    }
}

// Скопировать YAML в буфер обмена
async function copyYAML() {
    const yaml = document.getElementById('yaml-content').textContent;
    try {
        await navigator.clipboard.writeText(yaml);
        showToast('YAML copied to clipboard!', 'success');
    } catch (err) {
        showToast('Failed to copy YAML', 'error');
    }
}

// Скачать YAML
function downloadYAML() {
    const podName = currentPod.name;
    const yaml = document.getElementById('yaml-content').textContent;
    
    const blob = new Blob([yaml], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${podName}-config.yaml`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// Редактировать YAML
function editYAML() {
    document.getElementById('yaml-content').style.display = 'none';
    document.getElementById('yaml-editor').style.display = 'block';
    document.getElementById('save-yaml-btn').style.display = 'block';
}

// Сохранить изменения YAML
async function saveYAML() {
    const yaml = document.getElementById('yaml-editor').value;
    const podName = currentPod.name;
    const namespace = currentPod.namespace;
    
    try {
        const response = await fetch(`/api/pod/yaml/${namespace}/${podName}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml })
        });
        
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        showToast('Pod configuration updated successfully!', 'success');
        document.getElementById('yaml-content').textContent = yaml;
        document.getElementById('yaml-content').style.display = 'block';
        document.getElementById('yaml-editor').style.display = 'none';
        document.getElementById('save-yaml-btn').style.display = 'none';
        
        // Перезагружаем список подов
        loadPods();
        
    } catch (error) {
        showToast(`Failed to update: ${error.message}`, 'error');
    }
}

// Показать детали пода
async function showPodDetails(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('details-pod-name').textContent = podName;
    const modal = new bootstrap.Modal(document.getElementById('detailsModal'));
    modal.show();
    
    await loadPodDetails();
}

// Загрузить детали пода
async function loadPodDetails() {
    const podName = currentPod.name;
    const namespace = currentPod.namespace;
    
    try {
        const response = await fetch(`/api/pod/details/${namespace}/${podName}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        populatePodDetails(data);
        
    } catch (error) {
        showError('Failed to load pod details: ' + error.message);
    }
}

// Заполнить детали пода
function populatePodDetails(data) {
    // Основная информация
    document.getElementById('details-namespace').textContent = data.namespace;
    document.getElementById('details-status').innerHTML = 
        `<span class="badge-status ${getStatusClass(data.status.phase)}">${data.status.phase}</span>`;
    document.getElementById('details-node').textContent = data.nodeName || '-';
    document.getElementById('details-pod-ip').textContent = data.podIP || '-';
    document.getElementById('details-host-ip').textContent = data.hostIP || '-';
    document.getElementById('details-created').textContent = 
        new Date(data.metadata.creationTimestamp).toLocaleString();
    document.getElementById('details-start-time').textContent = 
        data.startTime ? new Date(data.startTime).toLocaleString() : '-';
    
    // Лейблы
    const labelsContainer = document.getElementById('details-labels');
    labelsContainer.innerHTML = '';
    if (data.metadata.labels) {
        Object.entries(data.metadata.labels).forEach(([key, value]) => {
            const badge = document.createElement('span');
            badge.className = 'label-badge me-1 mb-1';
            badge.textContent = `${key}: ${value}`;
            labelsContainer.appendChild(badge);
        });
    }
    
    // Контейнеры
    const containersList = document.getElementById('containers-list');
    containersList.innerHTML = '';
    data.containers.forEach(container => {
        const containerDiv = document.createElement('div');
        containerDiv.className = 'card mb-2';
        containerDiv.innerHTML = `
            <div class="card-header">
                <strong>${container.name}</strong> - ${container.image}
            </div>
            <div class="card-body">
                <p><strong>Resources:</strong> ${JSON.stringify(container.resources || {})}</p>
                <p><strong>Ports:</strong> ${JSON.stringify(container.ports || [])}</p>
            </div>
        `;
        containersList.appendChild(containerDiv);
    });
    
    // Условия
    const conditionsTable = document.getElementById('conditions-table').querySelector('tbody');
    conditionsTable.innerHTML = '';
    if (data.conditions) {
        data.conditions.forEach(condition => {
            const row = conditionsTable.insertRow();
            row.innerHTML = `
                <td>${condition.type}</td>
                <td>${condition.status}</td>
                <td>${condition.lastProbeTime || '-'}</td>
                <td>${condition.lastTransitionTime || '-'}</td>
                <td>${condition.reason || '-'}</td>
                <td>${condition.message || '-'}</td>
            `;
        });
    }
}

// Удалить под
async function deletePod(namespace, podName) {
    if (!confirm(`Are you sure you want to delete pod "${podName}"?`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/pod/${namespace}/${podName}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        showToast(`Pod "${podName}" deleted successfully!`, 'success');
        loadPods();
        
    } catch (error) {
        showToast(`Failed to delete pod: ${error.message}`, 'error');
    }
}

// Вспомогательные функции
function getStatusClass(status) {
    switch(status) {
        case 'Running': return 'badge-running';
        case 'Pending': return 'badge-pending';
        case 'Failed': return 'badge-failed';
        case 'Succeeded': return 'badge-succeeded';
        default: return 'badge-secondary';
    }
}

function getStatusIcon(status) {
    switch(status) {
        case 'Running': return 'fa-play-circle';
        case 'Pending': return 'fa-clock';
        case 'Failed': return 'fa-exclamation-circle';
        case 'Succeeded': return 'fa-check-circle';
        default: return 'fa-question-circle';
    }
}

function filterPods() {
    renderPodsTable(allPods);
}

function highlightSearch(text, search) {
    if (!search) return text;
    const regex = new RegExp(`(${search})`, 'gi');
    return text.replace(regex, '<span class="highlight">$1</span>');
}

function updateLastUpdated() {
    const now = new Date();
    document.getElementById('last-updated').textContent = 
        `Last updated: ${now.toLocaleTimeString()}`;
}

function showLoading(show) {
    const spinner = document.getElementById('loading-spinner');
    spinner.style.display = show ? 'inline-block' : 'none';
}

function showError(message) {
    // В реальном приложении можно использовать toast или alert
    console.error(message);
    alert(message);
}

function showToast(message, type = 'info') {
    // Простая реализация toast уведомления
    const toast = document.createElement('div');
    toast.className = `toast-alert alert alert-${type} alert-dismissible fade show`;
    toast.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        z-index: 9999;
        min-width: 300px;
    `;
    toast.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.remove();
    }, 3000);
}

function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Экспорт в CSV
function exportToCSV() {
    const rows = allPods.map(pod => ({
        Name: pod.name,
        Namespace: pod.namespace,
        Status: pod.status,
        Ready: pod.ready,
        Restarts: pod.restarts,
        Age: pod.age,
        Node: pod.node || '',
        IP: pod.ip || ''
    }));
    
    const csvContent = [
        Object.keys(rows[0]).join(','),
        ...rows.map(row => Object.values(row).join(','))
    ].join('\n');
    
    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `pods-${currentNamespace}-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// Дополнительные функции
function deleteAllFailedPods() {
    if (!confirm('Delete all failed pods?')) return;
    
    allPods.filter(pod => pod.status === 'Failed').forEach(pod => {
        deletePod(pod.namespace, pod.name);
    });
}

function restartAllPods() {
    if (!confirm('Restart all pods? This will trigger redeployment.')) return;
    alert('Restart all pods functionality would be implemented here');
}

function toggleViewMode() {
    alert('Grid view would be implemented here');
}

// Автообновление каждые 30 секунд
setInterval(loadPods, 30000);