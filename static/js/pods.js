// Текущий выбранный namespace
let currentNamespace = 'market';
let currentPod = null;
let allPods = [];
let podMetrics = {};
let resourceChart = null;
let cpuChart = null;
let memoryChart = null;

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadPods();
    loadMetrics();
    setupEventListeners();
    initCharts();
});

// Инициализация графиков
function initCharts() {
    const ctx = document.getElementById('resourceChart').getContext('2d');
    resourceChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['CPU Usage', 'Memory Usage', 'Available'],
            datasets: [{
                data: [0, 0, 100],
                backgroundColor: ['#007bff', '#28a745', '#e9ecef'],
                borderWidth: 1
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        boxWidth: 12,
                        padding: 10
                    }
                }
            }
        }
    });
}

// Настройка слушателей событий
function setupEventListeners() {
    // Выбор namespace
    document.getElementById('namespace-select').addEventListener('change', function() {
        currentNamespace = this.value;
        document.getElementById('current-namespace').textContent = currentNamespace;
        loadPods();
        loadMetrics();
    });
    
    // Поиск
    document.getElementById('search-pods').addEventListener('input', debounce(filterPods, 300));
    
    // Фильтры по статусу
    document.querySelectorAll('input[type="checkbox"]').forEach(checkbox => {
        checkbox.addEventListener('change', filterPods);
    });
    
    // Кнопка обновления
    document.getElementById('refresh-btn').addEventListener('click', function() {
        loadPods();
        loadMetrics();
    });
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
        const pods = data.pods || [];
        allPods = pods;
        
        updateStats(pods);
        renderPodsTable(pods);
        updateLastUpdated();
        
        console.log(`Loaded ${pods.length} pods from ${currentNamespace}`);
        
    } catch (error) {
        console.error('Error loading pods:', error);
        showError('Failed to load pods: ' + error.message);
        showEmptyTable();
    } finally {
        showLoading(false);
    }
}

// Загрузка метрик
async function loadMetrics() {
    try {
        const response = await fetch(`/api/metrics/pods/${currentNamespace}`);
        if (!response.ok) {
            // Если метрики недоступны, показываем предупреждение
            if (response.status === 503 || response.status === 404) {
                showToast('Metrics server not available. Install Metrics Server first.', 'warning');
                return;
            }
            throw new Error(`HTTP ${response.status}`);
        }
        
        const data = await response.json();
        podMetrics = {};
        
        if (data.metrics && Array.isArray(data.metrics)) {
            data.metrics.forEach(metric => {
                podMetrics[metric.pod] = metric;
            });
            
            updateResourceChart(data.metrics);
            updateTotalMetrics(data.metrics);
            renderPodsTable(allPods); // Перерисовываем таблицу с метриками
            showMetricsCharts();
        } else {
            showToast('No metrics data available', 'info');
        }
        
    } catch (error) {
        console.error('Error loading metrics:', error);
        showToast('Failed to load metrics: ' + error.message, 'warning');
    }
}

// Загрузка детальных метрик для конкретного пода
async function loadPodMetrics(namespace, podName) {
    try {
        const response = await fetch(`/api/metrics/pod/${namespace}/${podName}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        showDetailedMetrics(data);
        
    } catch (error) {
        console.error('Error loading pod metrics:', error);
        showToast('Failed to load pod metrics: ' + error.message, 'error');
    }
}

// Обновление статистики
function updateStats(pods) {
    const stats = {
        running: 0,
        pending: 0,
        failed: 0
    };
    
    pods.forEach(pod => {
        if (pod.status === 'Running') stats.running++;
        else if (pod.status === 'Pending') stats.pending++;
        else if (pod.status === 'Failed') stats.failed++;
    });
    
    document.getElementById('running-count').textContent = stats.running;
    document.getElementById('pending-count').textContent = stats.pending;
    document.getElementById('failed-count').textContent = stats.failed;
}

// Обновление общей статистики метрик
function updateTotalMetrics(metrics) {
    let totalCPU = 0;
    let totalMemory = 0;
    
    metrics.forEach(metric => {
        totalCPU += metric.cpu_raw || 0;
        totalMemory += metric.memory_raw || 0;
    });
    
    document.getElementById('total-cpu').textContent = `${totalCPU}m`;
    document.getElementById('total-memory').textContent = formatBytes(totalMemory);
}

// Обновление графика ресурсов
function updateResourceChart(metrics) {
    let totalCPU = 0;
    let totalMemory = 0;
    
    metrics.forEach(metric => {
        totalCPU += metric.cpu_raw || 0;
        totalMemory += metric.memory_raw || 0;
    });
    
    // Для графика используем примерные лимиты (можно заменить реальными)
    const cpuLimit = Math.max(totalCPU * 2, 1000); // Примерный лимит
    const memoryLimit = Math.max(totalMemory * 2, 100 * 1024 * 1024); // 100MB примерный лимит
    
    const cpuPercent = Math.min((totalCPU / cpuLimit) * 100, 100);
    const memoryPercent = Math.min((totalMemory / memoryLimit) * 100, 100);
    
    resourceChart.data.datasets[0].data = [
        cpuPercent,
        memoryPercent,
        100 - Math.max(cpuPercent, memoryPercent)
    ];
    resourceChart.update();
    
    document.getElementById('resource-info').innerHTML = `
        CPU: ${cpuPercent.toFixed(1)}%<br>
        Memory: ${memoryPercent.toFixed(1)}%
    `;
}

// Показать графики метрик
function showMetricsCharts() {
    document.getElementById('metrics-charts').style.display = 'flex';
    updateCharts();
}

// Обновление графиков CPU и Memory
function updateCharts() {
    const metrics = Object.values(podMetrics);
    
    if (metrics.length === 0) return;
    
    // Сортируем по использованию CPU
    metrics.sort((a, b) => (b.cpu_raw || 0) - (a.cpu_raw || 0));
    
    const topPods = metrics.slice(0, 8); // Берем топ-8 подов
    
    // Обновляем график CPU
    const cpuCtx = document.getElementById('cpuChart').getContext('2d');
    if (cpuChart) cpuChart.destroy();
    
    cpuChart = new Chart(cpuCtx, {
        type: 'bar',
        data: {
            labels: topPods.map(m => m.pod.substring(0, 20)),
            datasets: [{
                label: 'CPU Usage (m)',
                data: topPods.map(m => m.cpu_raw || 0),
                backgroundColor: '#007bff'
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: { display: false }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'mCPU'
                    }
                }
            }
        }
    });
    
    // Обновляем график Memory
    const memoryCtx = document.getElementById('memoryChart').getContext('2d');
    if (memoryChart) memoryChart.destroy();
    
    memoryChart = new Chart(memoryCtx, {
        type: 'bar',
        data: {
            labels: topPods.map(m => m.pod.substring(0, 20)),
            datasets: [{
                label: 'Memory Usage',
                data: topPods.map(m => (m.memory_raw || 0) / (1024 * 1024)), // Конвертируем в MB
                backgroundColor: '#28a745'
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: { display: false }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'MB'
                    }
                }
            }
        }
    });
}

// Рендер таблицы подов с метриками
function renderPodsTable(pods) {
    const tbody = document.getElementById('pods-table-body');
    
    if (!Array.isArray(pods) || pods.length === 0) {
        showEmptyTable();
        return;
    }
    
    const searchTerm = document.getElementById('search-pods').value.toLowerCase();
    const activeStatuses = Array.from(document.querySelectorAll('input[type="checkbox"]:checked'))
        .map(cb => cb.value);
    
    let filteredPods = pods.filter(pod => {
        if (!pod || typeof pod !== 'object') return false;
        
        if (searchTerm && (!pod.name || !pod.name.toLowerCase().includes(searchTerm))) {
            return false;
        }
        if (activeStatuses.length > 0 && (!pod.status || !activeStatuses.includes(pod.status))) {
            return false;
        }
        return true;
    });
    
    if (filteredPods.length === 0) {
        showEmptyTable('No pods match the current filters');
        return;
    }
    
    let html = '';
    filteredPods.forEach((pod, index) => {
        const podName = pod.name || `pod-${index}`;
        const namespace = pod.namespace || currentNamespace;
        const status = pod.status || 'Unknown';
        const ready = pod.ready || '0/0';
        const restarts = pod.restarts || 0;
        
        const metric = podMetrics[podName];
        const cpuUsage = metric ? metric.cpu_usage : 'N/A';
        const memoryUsage = metric ? metric.memory_usage : 'N/A';
        const cpuPercent = metric ? metric.cpu_percent : 0;
        const memoryPercent = metric ? metric.memory_percent : 0;
        const cpuRaw = metric ? metric.cpu_raw : 0;
        const memoryRaw = metric ? metric.memory_raw : 0;
        
        const statusClass = getStatusClass(status);
        
        html += `
            <tr data-pod-name="${podName}" data-namespace="${namespace}">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-cube me-2 text-primary"></i>
                        <strong>${highlightSearch(podName, searchTerm)}</strong>
                        ${pod.ip ? `<small class="text-muted ms-2">(${pod.ip})</small>` : ''}
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
                    <div class="d-flex align-items-center">
                        <span class="me-2">${cpuUsage}</span>
                        ${cpuPercent > 0 ? `
                            <div class="progress flex-grow-1" style="height: 6px;">
                                <div class="progress-bar bg-primary" style="width: ${Math.min(cpuPercent, 100)}%"></div>
                            </div>
                        ` : ''}
                    </div>
                </td>
                <td>
                    <div class="d-flex align-items-center">
                        <span class="me-2">${memoryUsage}</span>
                        ${memoryPercent > 0 ? `
                            <div class="progress flex-grow-1" style="height: 6px;">
                                <div class="progress-bar bg-success" style="width: ${Math.min(memoryPercent, 100)}%"></div>
                            </div>
                        ` : ''}
                    </div>
                </td>
                <td>
                    <span class="badge ${cpuPercent > 80 ? 'bg-danger' : cpuPercent > 50 ? 'bg-warning' : 'bg-info'}">
                        ${cpuPercent}%
                    </span>
                </td>
                <td>
                    <span class="badge ${memoryPercent > 80 ? 'bg-danger' : memoryPercent > 50 ? 'bg-warning' : 'bg-info'}">
                        ${memoryPercent}%
                    </span>
                </td>
                <td>
                    <div class="btn-group" role="group">
                        <button class="btn btn-action btn-outline-info btn-sm" 
                                onclick="showPodMetrics('${namespace}', '${podName}')"
                                title="Metrics">
                            <i class="fas fa-chart-line"></i>
                        </button>
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

// Показать пустую таблицу
function showEmptyTable(message = 'No pods found') {
    const tbody = document.getElementById('pods-table-body');
    tbody.innerHTML = `
        <tr>
            <td colspan="10" class="text-center py-5">
                <i class="fas fa-search fa-2x text-muted mb-3"></i>
                <p class="text-muted">${message}</p>
                ${message.includes('No pods') ? '<small class="text-muted">Try selecting a different namespace</small>' : ''}
            </td>
        </tr>
    `;
}

// Показать детальные метрики
function showPodMetrics(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('metrics-pod-name').textContent = podName;
    loadPodMetrics(namespace, podName);
    
    const modal = new bootstrap.Modal(document.getElementById('metricsModal'));
    modal.show();
}

// Показать детали метрик
function showDetailedMetrics(data) {
    // Обновляем CPU детали
    const cpuDetails = document.getElementById('cpu-details');
    cpuDetails.innerHTML = `
        <p><strong>CPU Usage:</strong> ${data.total_cpu}</p>
        <p><strong>Containers:</strong> ${data.containers ? data.containers.length : 0}</p>
        <p><strong>Last Updated:</strong> ${data.timestamp ? new Date(data.timestamp).toLocaleString() : 'N/A'}</p>
    `;
    
    // Обновляем Memory детали
    const memoryDetails = document.getElementById('memory-details');
    memoryDetails.innerHTML = `
        <p><strong>Memory Usage:</strong> ${data.total_memory}</p>
        <p><strong>Containers:</strong> ${data.containers ? data.containers.length : 0}</p>
        <p><strong>Last Updated:</strong> ${data.timestamp ? new Date(data.timestamp).toLocaleString() : 'N/A'}</p>
    `;
    
    // Обновляем таблицу контейнеров
    const containerTable = document.getElementById('container-metrics-table').querySelector('tbody');
    containerTable.innerHTML = '';
    
    if (data.containers && Array.isArray(data.containers)) {
        data.containers.forEach(container => {
            const row = containerTable.insertRow();
            row.innerHTML = `
                <td>${container.name}</td>
                <td>${container.cpu_usage}</td>
                <td>${container.cpu_limit}</td>
                <td><span class="badge ${container.cpu_percent > 80 ? 'bg-danger' : container.cpu_percent > 50 ? 'bg-warning' : 'bg-info'}">${container.cpu_percent}%</span></td>
                <td>${container.memory_usage}</td>
                <td>${container.memory_limit}</td>
                <td><span class="badge ${container.memory_percent > 80 ? 'bg-danger' : container.memory_percent > 50 ? 'bg-warning' : 'bg-info'}">${container.memory_percent}%</span></td>
            `;
        });
    }
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
    if (data.containers && Array.isArray(data.containers)) {
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
    }
    
    // Условия
    const conditionsTable = document.getElementById('conditions-table').querySelector('tbody');
    conditionsTable.innerHTML = '';
    if (data.conditions && Array.isArray(data.conditions)) {
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
    console.error(message);
    showToast(message, 'error');
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

// Форматирование байтов
function formatBytes(bytes) {
    if (bytes === 0 || isNaN(bytes)) return '0 B';
    
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    
    if (i === 0) return bytes + ' ' + sizes[i];
    return (bytes / Math.pow(1024, i)).toFixed(2) + ' ' + sizes[i];
}

// Экспорт в CSV
function exportToCSV() {
    const rows = allPods.map(pod => {
        const metric = podMetrics[pod.name] || {};
        return {
            Name: pod.name,
            Namespace: pod.namespace,
            Status: pod.status,
            Ready: pod.ready,
            Restarts: pod.restarts,
            'CPU Usage': metric.cpu_usage || 'N/A',
            'Memory Usage': metric.memory_usage || 'N/A',
            'CPU %': metric.cpu_percent || 0,
            'Memory %': metric.memory_percent || 0,
            Age: pod.age || '',
            Node: pod.node || '',
            IP: pod.ip || ''
        };
    });
    
    if (rows.length === 0) {
        showToast('No data to export', 'warning');
        return;
    }
    
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
    
    const failedPods = allPods.filter(pod => pod.status === 'Failed');
    if (failedPods.length === 0) {
        showToast('No failed pods to delete', 'info');
        return;
    }
    
    failedPods.forEach(pod => {
        deletePod(pod.namespace, pod.name);
    });
}

function restartAllPods() {
    if (!confirm('Restart all pods? This will trigger redeployment.')) return;
    showToast('Restart all pods functionality would be implemented here', 'info');
}

function toggleViewMode() {
    showToast('Grid view would be implemented here', 'info');
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

// Автообновление каждые 30 секунд
setInterval(() => {
    if (document.visibilityState === 'visible') {
        loadPods();
        loadMetrics();
    }
}, 30000);