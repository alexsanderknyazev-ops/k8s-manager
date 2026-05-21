// Текущий выбранный namespace (default — чтобы видеть под Postgres и прочие в default)
let currentNamespace = 'default';
let currentPod = null;
let allPods = [];
let podMetrics = {};
let resourceChart = null;
let cpuChart = null;
let memoryChart = null;
let detailedCpuChart = null;
let detailedMemoryChart = null;

// Port-forward сессии
let portForwardSessions = [];
let currentPortForwardSession = null;

// Log Streams
let logWebSocket = null;
let logMessages = [];
let activeLogStreams = [];
let podsRefreshInterval = null;

// Логи в модалке пода (вкладка Logs)
let detailLogWebSocket = null;
let detailLogMessages = [];

// Exec в модалке пода (вкладка Console)
let detailPodExec = null;

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', async function() {
    if (typeof loadNamespacesIntoSelect === 'function') {
        await loadNamespacesIntoSelect('namespace-select', true);
        var nsSel = document.getElementById('namespace-select');
        if (nsSel && nsSel.options.length) {
            var hasDefault = Array.from(nsSel.options).some(function(o) { return o.value === 'default'; });
            if (hasDefault) nsSel.value = 'default';
            currentNamespace = nsSel ? nsSel.value : 'default';
            var curNsEl = document.getElementById('current-namespace');
            if (curNsEl) curNsEl.textContent = currentNamespace;
        }
    }
    loadPods();
    loadMetrics();
    loadPortForwardSessions();
    loadLogStreamSessions();
    setupEventListeners();
    initResourceChart();
    initMainCharts();
    resizeChartsOnModalOpen();
});

// Инициализация графика ресурсов (если блок удалён — не выполняем)
function initResourceChart() {
    const canvas = document.getElementById('resourceChart');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    if (resourceChart) {
        resourceChart.destroy();
    }
    
    // Устанавливаем размер canvas
    canvas.style.width = '100%';
    canvas.style.height = '100%';
    
    resourceChart = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['CPU Usage', 'Memory Usage', 'Available'],
            datasets: [{
                data: [0, 0, 100],
                backgroundColor: ['#007bff', '#28a745', '#e9ecef'],
                borderColor: '#fff',
                borderWidth: 2,
                hoverBorderWidth: 3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            aspectRatio: 1, // Квадратное соотношение для круговой диаграммы
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        boxWidth: 12,
                        padding: 10,
                        font: {
                            size: 10
                        }
                    }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            let label = context.label || '';
                            if (label) {
                                label += ': ';
                            }
                            label += context.parsed.toFixed(1) + '%';
                            return label;
                        }
                    }
                }
            },
            cutout: '65%',
            animation: {
                animateScale: true,
                animateRotate: true
            },
            layout: {
                padding: {
                    top: 10,
                    bottom: 10,
                    left: 10,
                    right: 10
                }
            }
        }
    });
}

// Инициализация основных графиков
function initMainCharts() {
    // Инициализация для cpuChart и memoryChart
    // Они будут создаваться динамически в updateCharts()
}

// Настройка слушателей событий
function setupEventListeners() {
    // Выбор namespace
    var nsSelect = document.getElementById('namespace-select');
    if (nsSelect) {
        nsSelect.addEventListener('change', function() {
            currentNamespace = this.value;
            var curNsEl = document.getElementById('current-namespace');
            if (curNsEl) curNsEl.textContent = currentNamespace;
            loadPods();
            loadMetrics();
            loadPortForwardSessions();
            loadLogStreamSessions();
        });
    }
    
    // Поиск
    const searchPodsEl = document.getElementById('search-pods');
    if (searchPodsEl) searchPodsEl.addEventListener('input', debounce(filterPods, 300));
    
    // Фильтры по статусу
    document.querySelectorAll('input[type="checkbox"]').forEach(checkbox => {
        checkbox.addEventListener('change', filterPods);
    });
    
    // Кнопка обновления
    const refreshBtn = document.getElementById('refresh-btn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', function() {
            loadPods();
            loadMetrics();
            loadPortForwardSessions();
            loadLogStreamSessions();
        });
    }

    const podsLiveEl = document.getElementById('pods-live-refresh');
    if (podsLiveEl) {
        podsLiveEl.addEventListener('change', function() {
            if (podsRefreshInterval) clearInterval(podsRefreshInterval);
            podsRefreshInterval = null;
            if (this.checked) podsRefreshInterval = setInterval(function() { loadPods(); loadMetrics(); }, 10000);
        });
    }

    // Фильтр логов
    const logFilterEl = document.getElementById('log-filter');
    if (logFilterEl) logFilterEl.addEventListener('input', debounce(applyLogFilter, 300));

    // Автоматическое обновление
    setInterval(() => {
        if (document.visibilityState === 'visible') {
            loadPortForwardSessions();
            loadLogStreamSessions();
        }
    }, 5000);
}

// ===== LOGS FUNCTIONS =====

// Показать модальное окно для логов (старое название для совместимости)
async function showLogs(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('logs-pod-name').textContent = podName;
    document.getElementById('logs-status').className = 'alert alert-info';
    document.getElementById('logs-status').innerHTML = `
        <i class="fas fa-info-circle me-2"></i>
        Ready to view logs for pod <strong>${escapeHtml(podName)}</strong> in namespace <strong>${escapeHtml(namespace)}</strong>
    `;
    
    document.getElementById('logs-content').innerHTML = '';
    document.getElementById('start-realtime-btn').style.display = 'block';
    document.getElementById('stop-realtime-btn').style.display = 'none';
    document.getElementById('log-filter').value = '';
    document.getElementById('log-count').textContent = '0 logs';
    
    logMessages = [];
    
    const modal = new bootstrap.Modal(document.getElementById('logsModal'));
    modal.show();
    
    // Автоматически запускаем логи
    setTimeout(() => startRealtimeLogs(), 500);
}

// Загрузить активные лог стримы
async function loadLogStreamSessions() {
    try {
        const response = await fetch('/api/logs/streams');
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        activeLogStreams = data.streams || [];
        
        updateLogStreamSessionsUI(activeLogStreams);
        
    } catch (error) {
        console.error('Error loading log streams:', error);
    }
}

// Обновить UI с активными лог стримами
function updateLogStreamSessionsUI(streams) {
    const card = document.getElementById('logstream-sessions-card');
    const tbody = document.getElementById('logstream-sessions-body');
    
    // Фильтруем только для текущего namespace
    const activeStreams = streams.filter(stream => 
        stream.namespace === currentNamespace || currentNamespace === 'all'
    );
    
    if (activeStreams.length === 0) {
        card.style.display = 'none';
        return;
    }
    
    card.style.display = 'block';
    
    let html = '';
    activeStreams.forEach(stream => {
        html += `
            <tr>
                <td>
                    <small>
                        <i class="fas fa-stream me-1 ${stream.follow ? 'text-success' : 'text-info'}"></i>
                        ${escapeHtml(stream.pod)}
                    </small>
                </td>
                <td>
                    <span class="badge ${stream.follow ? 'bg-success' : 'bg-info'} badge-sm">
                        ${stream.follow ? 'Following' : 'Static'}
                    </span>
                </td>
                <td>
                    <span class="badge bg-success badge-sm">
                        <i class="fas fa-circle status-online"></i> Active
                    </span>
                </td>
                <td>
                    <button class="btn btn-sm btn-outline-danger btn-action-sm" 
                            onclick="stopLogStreamSession('${encodeURIComponent(stream.id)}')" title="Stop">
                        <i class="fas fa-stop"></i>
                    </button>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Остановить конкретный лог стрим
async function stopLogStreamSession(streamId) {
    try {
        await apiFetch(`/api/logs/stream/${streamId}`, { method: 'DELETE' });
        
        showToast('Log stream stopped', 'info');
        setTimeout(() => loadLogStreamSessions(), 500);
        
    } catch (error) {
        showToast(`Failed to stop log stream: ${error.message}`, 'error');
    }
}

// Запустить реалтайм логи
// В pods.js в функции startRealtimeLogs добавьте:
function startRealtimeLogs() {
    if (!currentPod) {
        console.error('No current pod selected');
        showToast('No pod selected', 'error');
        return;
    }
    
    const namespace = currentPod.namespace;
    const podName = currentPod.name;
    const tailLines = document.getElementById('log-tail').value;
    const follow = document.getElementById('log-follow').checked;
    const bufferSize = document.getElementById('log-buffer').value;
    
    console.log('Starting WebSocket with params:', {
        namespace,
        podName,
        tailLines,
        follow,
        bufferSize
    });
    
    // Останавливаем предыдущее соединение если есть
    if (logWebSocket) {
        console.log('Stopping existing WebSocket');
        stopRealtimeLogs();
    }
    
    // Создаем WebSocket соединение
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/logs/stream/${namespace}/${podName}?tail=${tailLines}&follow=${follow}&buffer=${bufferSize}`;
    
    console.log('WebSocket URL:', wsUrl);
    
    logWebSocket = new WebSocket(wsUrl);
    
    // Обновляем UI
    document.getElementById('start-realtime-btn').style.display = 'none';
    document.getElementById('stop-realtime-btn').style.display = 'block';
    document.getElementById('logs-status').className = 'alert alert-warning connecting';
    document.getElementById('logs-status').innerHTML = `
        <i class="fas fa-spinner fa-spin me-2"></i>
        Connecting to pod <strong>${escapeHtml(podName)}</strong>...
    `;
    
    // Обработчики WebSocket
    logWebSocket.onopen = function() {
        console.log('WebSocket opened successfully');
        document.getElementById('logs-status').className = 'alert alert-success connected';
        document.getElementById('logs-status').innerHTML = `
            <i class="fas fa-check-circle me-2"></i>
            Connected to pod <strong>${escapeHtml(podName)}</strong>. Streaming logs...
        `;
        showToast('Live logs started', 'success');
    };
    
    logWebSocket.onmessage = function(event) {
        console.log('WebSocket message received:', event.data);
        try {
            const message = JSON.parse(event.data);
            handleLogMessage(message);
        } catch (error) {
            console.error('Error parsing log message:', error);
            addLogToDisplay('error', `Error: ${event.data}`);
        }
    };
    
    logWebSocket.onerror = function(error) {
        console.error('WebSocket error:', error);
        document.getElementById('logs-status').className = 'alert alert-danger error';
        document.getElementById('logs-status').innerHTML = `
            <i class="fas fa-exclamation-circle me-2"></i>
            Connection error. Check console for details.
        `;
        showToast('WebSocket connection error', 'error');
    };
    
    logWebSocket.onclose = function(event) {
        console.log('WebSocket closed:', event.code, event.reason);
        document.getElementById('start-realtime-btn').style.display = 'block';
        document.getElementById('stop-realtime-btn').style.display = 'none';
        document.getElementById('logs-status').className = 'alert alert-info';
        document.getElementById('logs-status').innerHTML = `
            <i class="fas fa-info-circle me-2"></i>
            Connection closed (code: ${event.code})
        `;
        logWebSocket = null;
    };
}

// Остановить реалтайм логи
function stopRealtimeLogs() {
    if (logWebSocket) {
        logWebSocket.close();
        logWebSocket = null;
    }
    
    document.getElementById('start-realtime-btn').style.display = 'block';
    document.getElementById('stop-realtime-btn').style.display = 'none';
    document.getElementById('logs-status').className = 'alert alert-info';
    document.getElementById('logs-status').innerHTML = `
        <i class="fas fa-info-circle me-2"></i>
        Live logs stopped
    `;
    
    showToast('Live logs stopped', 'info');
}

// Обработать сообщение лога
function handleLogMessage(message) {
    const { type, message: content, time } = message;
    
    // Добавляем в массив сообщений
    logMessages.push({
        type,
        content,
        time,
        timestamp: new Date(time)
    });
    
    // Ограничиваем размер буфера
    const bufferSize = parseInt(document.getElementById('log-buffer').value) || 1000;
    if (logMessages.length > bufferSize) {
        logMessages = logMessages.slice(-bufferSize);
    }
    
    // Добавляем на экран
    addLogToDisplay(type, content, time);
    
    // Обновляем счетчик
    document.getElementById('log-count').textContent = `${logMessages.length} logs`;
    
    // Автоскролл
    if (document.getElementById('autoscroll').checked) {
        const container = document.getElementById('logs-content');
        container.scrollTop = container.scrollHeight;
    }
}

// Добавить лог на экран
function addLogToDisplay(type, content, timestamp) {
    const container = document.getElementById('logs-content');
    const timestampStr = timestamp ? new Date(timestamp).toLocaleTimeString() : new Date().toLocaleTimeString();
    
    let logClass = 'log-line';
    let icon = '';
    
    switch(type) {
        case 'info':
            logClass += ' log-info';
            icon = '<i class="fas fa-info-circle me-1"></i>';
            break;
        case 'warning':
            logClass += ' log-warning';
            icon = '<i class="fas fa-exclamation-triangle me-1"></i>';
            break;
        case 'error':
            logClass += ' log-error';
            icon = '<i class="fas fa-exclamation-circle me-1"></i>';
            break;
        case 'success':
            logClass += ' log-success';
            icon = '<i class="fas fa-check-circle me-1"></i>';
            break;
        case 'log':
            logClass += ' log-system';
            icon = '<i class="fas fa-file-alt me-1"></i>';
            break;
        default:
            logClass += ' log-system';
    }
    
    const logElement = document.createElement('div');
    logElement.className = `${logClass} new-log`;
    logElement.innerHTML = `
        <span class="log-timestamp">[${timestampStr}]</span>
        ${icon}
        <span class="log-content">${escapeHtml(content)}</span>
    `;
    
    // Применяем фильтр если есть
    const filter = document.getElementById('log-filter').value.toLowerCase();
    if (filter && !content.toLowerCase().includes(filter)) {
        logElement.style.display = 'none';
    }
    
    container.appendChild(logElement);
    
    // Удаляем класс анимации через 1 секунду
    setTimeout(() => {
        logElement.classList.remove('new-log');
    }, 1000);
}

// Очистить логи
function clearLogs() {
    document.getElementById('logs-content').innerHTML = '';
    logMessages = [];
    document.getElementById('log-count').textContent = '0 logs';
}

// Скачать логи
function downloadLogs() {
    if (logMessages.length === 0) {
        showToast('No logs to download', 'warning');
        return;
    }
    
    let logText = '';
    logMessages.forEach(log => {
        const timestamp = log.timestamp.toLocaleString();
        logText += `[${timestamp}] [${log.type.toUpperCase()}] ${log.content}\n`;
    });
    
    const blob = new Blob([logText], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `logs-${currentPod.name}-${new Date().toISOString().replace(/[:.]/g, '-')}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    showToast('Logs downloaded', 'success');
}

// Фильтровать логи
function filterLogs(type) {
    const container = document.getElementById('logs-content');
    const logLines = container.querySelectorAll('.log-line');
    const buttons = document.querySelectorAll('.btn-group .btn');
    
    // Сбрасываем все кнопки
    buttons.forEach(btn => {
        btn.classList.remove('active', 'filter-active');
    });
    
    if (type === 'all') {
        // Показываем все логи
        logLines.forEach(line => {
            line.style.display = 'block';
        });
        
        // Активируем кнопку All
        const allBtn = document.querySelector('.btn-group .btn[onclick*="all"]');
        if (allBtn) {
            allBtn.classList.add('active', 'filter-active');
        }
        return;
    }
    
    // Активируем выбранную кнопку
    const activeBtn = document.querySelector(`.btn-group .btn[onclick*="${type}"]`);
    if (activeBtn) {
        activeBtn.classList.add('active', 'filter-active');
    }
    
    // Фильтруем логи по типу
    logLines.forEach(line => {
        const hasClass = line.className.includes(`log-${type}`);
        line.style.display = hasClass ? 'block' : 'none';
    });
}

// Применить фильтр из input
function applyLogFilter() {
    const filter = document.getElementById('log-filter').value.toLowerCase();
    const container = document.getElementById('logs-content');
    const logLines = container.querySelectorAll('.log-line');
    
    logLines.forEach(line => {
        const content = line.querySelector('.log-content').textContent.toLowerCase();
        line.style.display = content.includes(filter) ? 'block' : 'none';
    });
}

// Очистить фильтр
function clearLogFilter() {
    document.getElementById('log-filter').value = '';
    const container = document.getElementById('logs-content');
    const logLines = container.querySelectorAll('.log-line');
    logLines.forEach(line => {
        line.style.display = 'block';
    });
}

// ===== PORT-FORWARDING FUNCTIONS =====

// Показать модальное окно Port-forward
function showPortForwardModal(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    document.getElementById('pf-pod').value = podName;
    document.getElementById('pf-namespace').value = namespace;
    document.getElementById('pf-pod-display').value = podName;
    document.getElementById('pf-remote-port').value = '';
    document.getElementById('pf-local-port').value = '';
    document.getElementById('portForwardResult').style.display = 'none';
    document.getElementById('portForwardForm').style.display = 'block';
    
    const modal = new bootstrap.Modal(document.getElementById('portForwardModal'));
    modal.show();
}

// Установить порт
function setPort(port) {
    document.getElementById('pf-remote-port').value = port;
    document.getElementById('pf-local-port').value = port;
}

// Проверить доступность порта
async function checkPortAvailable(port) {
    try {
        const response = await fetch(`/api/portforward/check/${port}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        return data.available;
    } catch (error) {
        console.error('Error checking port:', error);
        return false;
    }
}

// Запустить port-forward
async function startPortForward() {
    const podName = document.getElementById('pf-pod').value;
    const namespace = document.getElementById('pf-namespace').value;
    const remotePort = parseInt(document.getElementById('pf-remote-port').value);
    let localPort = parseInt(document.getElementById('pf-local-port').value);
    
    // Валидация
    if (!remotePort || remotePort < 1 || remotePort > 65535) {
        showToast('Please enter a valid remote port (1-65535)', 'error');
        return;
    }
    
    // Если localPort не указан, используем тот же что и remote
    if (!localPort || localPort < 1024 || localPort > 65535) {
        localPort = remotePort;
    }
    
    // Проверяем доступность порта
    const portAvailable = await checkPortAvailable(localPort);
    if (!portAvailable) {
        showToast(`Port ${localPort} is already in use. Please choose a different port.`, 'error');
        return;
    }
    
    showLoading(true);
    
    try {
        const data = await apiFetch('/api/portforward/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                pod: podName,
                namespace: namespace,
                remotePort: remotePort,
                localPort: localPort
            })
        });
        currentPortForwardSession = data.session;
        
        // Показать результат
        document.getElementById('portForwardForm').style.display = 'none';
        document.getElementById('portForwardResult').style.display = 'block';
        
        const url = `http://localhost:${localPort}`;
        document.getElementById('forwarded-url').value = url;
        document.getElementById('forwarded-link').href = url;
        
        showToast('Port-forward started successfully!', 'success');
        
        // Обновить список сессий
        setTimeout(() => loadPortForwardSessions(), 1000);
        
    } catch (error) {
        showToast(`Failed to start port-forward: ${error.message}`, 'error');
    } finally {
        showLoading(false);
    }
}

// Остановить port-forward (из модального окна)
async function stopPortForward() {
    if (!currentPortForwardSession) return;
    
    try {
        await apiFetch(`/api/portforward/stop/${currentPortForwardSession.id}`, { method: 'POST' });
        
        showToast('Port-forward stopped', 'info');
        
        // Закрыть модальное окно
        bootstrap.Modal.getInstance(document.getElementById('portForwardModal')).hide();
        
        // Обновить список сессий
        setTimeout(() => loadPortForwardSessions(), 1000);
        
    } catch (error) {
        showToast(`Failed to stop port-forward: ${error.message}`, 'error');
    }
}

// Загрузить активные port-forward сессии
async function loadPortForwardSessions() {
    try {
        const response = await fetch('/api/portforward/sessions');
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        portForwardSessions = data.sessions || [];
        
        updatePortForwardSessionsUI(portForwardSessions);
        
    } catch (error) {
        console.error('Error loading port-forward sessions:', error);
    }
}

// Обновить UI с активными сессиями
function updatePortForwardSessionsUI(sessions) {
    const card = document.getElementById('portforward-sessions-card');
    const tbody = document.getElementById('portforward-sessions-body');
    if (!card || !tbody) return;

    if (sessions.length === 0) {
        card.style.display = 'none';
        return;
    }
    
    card.style.display = 'block';
    
    // Фильтруем только активные сессии для текущего namespace
    const activeSessions = sessions.filter(session => 
        session.status === 'running' && 
        (session.namespace === currentNamespace || currentNamespace === 'all')
    );
    
    if (activeSessions.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="4" class="text-center py-2">
                    <small class="text-muted">No active port-forwards</small>
                </td>
            </tr>
        `;
        return;
    }
    
    let html = '';
    activeSessions.forEach(session => {
        const statusClass = session.status === 'running' ? 'bg-success' : 
                          session.status === 'starting' ? 'bg-warning' : 'bg-danger';
        
        html += `
            <tr>
                <td>
                    <small>${escapeHtml(session.pod)}</small>
                </td>
                <td>
                    <small><code>${session.localPort}→${session.remotePort}</code></small>
                </td>
                <td>
                    <span class="badge ${statusClass} badge-sm">${session.status}</span>
                </td>
                <td>
                    <button class="btn btn-sm btn-outline-danger btn-action-sm" 
                            onclick="stopPortForwardSession('${encodeURIComponent(session.id)}')" title="Stop">
                        <i class="fas fa-stop"></i>
                    </button>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Остановить конкретную port-forward сессию (из боковой панели)
async function stopPortForwardSession(sessionId) {
    try {
        await apiFetch(`/api/portforward/stop/${sessionId}`, { method: 'POST' });
        
        showToast('Port-forward session stopped', 'info');
        
        // Обновить список сессий
        setTimeout(() => loadPortForwardSessions(), 500);
        
    } catch (error) {
        showToast(`Failed to stop session: ${error.message}`, 'error');
    }
}

// Копировать forwarded URL
function copyForwardedURL() {
    const url = document.getElementById('forwarded-url').value;
    navigator.clipboard.writeText(url).then(() => {
        showToast('URL copied to clipboard!', 'success');
    }).catch(err => {
        showToast('Failed to copy URL', 'error');
    });
}

// ===== ОСНОВНЫЕ ФУНКЦИИ PODS =====

// Загрузка списка подов
async function loadPods() {
    showLoading(true);
    
    try {
        let url = `/api/pods?namespace=${currentNamespace}`;
        const labelsEl = document.getElementById('pods-label-filter');
        if (labelsEl && labelsEl.value.trim()) url += '&labels=' + encodeURIComponent(labelsEl.value.trim());
        const response = await fetch(url);
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
            renderPodsTable(allPods);
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

// Обновление графика ресурсов (если блок удалён — не выполняем)
function updateResourceChart(metrics) {
    if (!resourceChart) return;
    let totalCPU = 0;
    let totalMemory = 0;

    metrics.forEach(metric => {
        totalCPU += metric.cpu_raw || 0;
        totalMemory += metric.memory_raw || 0;
    });
    
    const cpuLimit = Math.max(totalCPU * 2, 1000);
    const memoryLimit = Math.max(totalMemory * 2, 100 * 1024 * 1024);
    
    const cpuPercent = Math.min((totalCPU / cpuLimit) * 100, 100);
    const memoryPercent = Math.min((totalMemory / memoryLimit) * 100, 100);
    
    resourceChart.data.datasets[0].data = [
        cpuPercent,
        memoryPercent,
        Math.max(0, 100 - cpuPercent - memoryPercent)
    ];
    
    resourceChart.data.datasets[0].backgroundColor = [
        cpuPercent > 80 ? '#dc3545' : cpuPercent > 50 ? '#ffc107' : '#007bff',
        memoryPercent > 80 ? '#dc3545' : memoryPercent > 50 ? '#ffc107' : '#28a745',
        '#e9ecef'
    ];
    
    resourceChart.update('none');

    const resourceInfo = document.getElementById('resource-info');
    if (resourceInfo) resourceInfo.innerHTML = `
        <div class="d-flex justify-content-around">
            <div>
                <strong class="text-primary">CPU:</strong><br>
                ${cpuPercent.toFixed(1)}%
            </div>
            <div>
                <strong class="text-success">Memory:</strong><br>
                ${memoryPercent.toFixed(1)}%
            </div>
        </div>
    `;
}

// Функция обновления только графика ресурсов (кнопка может быть удалена)
function refreshResourceChart() {
    if (Object.keys(podMetrics).length === 0) {
        loadMetrics();
    } else {
        const metrics = Object.values(podMetrics);
        updateResourceChart(metrics);
        showToast('Resource chart refreshed', 'info');
    }
}

// Показать графики метрик (блок может быть удалён)
function showMetricsCharts() {
    const el = document.getElementById('metrics-charts');
    if (!el) return;
    el.style.display = 'flex';
    updateCharts();
}

// Градиенты для графиков (создаём один раз на canvas)
function createChartGradient(ctx, chartArea, colorStops) {
    if (!chartArea) return colorStops[0];
    const g = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top);
    colorStops.forEach((s, i) => g.addColorStop(i / (colorStops.length - 1), s));
    return g;
}

// Обновление графиков CPU и Memory (блоки могут быть удалены)
function updateCharts() {
    const cpuCanvas = document.getElementById('cpuChart');
    if (!cpuCanvas) return;

    const metrics = Object.values(podMetrics);
    if (metrics.length === 0) return;

    metrics.sort((a, b) => (b.cpu_raw || 0) - (a.cpu_raw || 0));
    const topPods = metrics.slice(0, 8);

    const fontFamily = "'Inter', -apple-system, BlinkMacSystemFont, sans-serif";
    const gridColor = 'rgba(0, 0, 0, 0.06)';
    const tickColor = '#6c757d';

    // Обновляем график CPU
    const cpuCtx = cpuCanvas.getContext('2d');
    
    if (cpuChart) cpuChart.destroy();
    
    cpuChart = new Chart(cpuCtx, {
        type: 'bar',
        data: {
            labels: topPods.map(m => m.pod.length > 18 ? m.pod.substring(0, 18) + '…' : m.pod),
            datasets: [{
                label: 'CPU (m)',
                data: topPods.map(m => m.cpu_raw || 0),
                backgroundColor: function(context) {
                    const chart = context.chart;
                    const ctx = chart.ctx;
                    const chartArea = chart.chartArea;
                    if (!chartArea) return '#64748b';
                    const g = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top);
                    g.addColorStop(0, '#94a3b8');
                    g.addColorStop(0.5, '#64748b');
                    g.addColorStop(1, '#475569');
                    return g;
                },
                borderRadius: 6,
                borderSkipped: false
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            aspectRatio: 2.2,
            plugins: {
                legend: { display: false },
                tooltip: {
                    backgroundColor: 'rgba(33, 37, 41, 0.9)',
                    padding: 10,
                    cornerRadius: 8,
                    titleFont: { family: fontFamily, size: 12 },
                    bodyFont: { family: fontFamily, size: 13 }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'mCPU',
                        font: { family: fontFamily, size: 11 },
                        color: tickColor
                    },
                    grid: { color: gridColor, drawBorder: false },
                    ticks: { font: { family: fontFamily, size: 10 }, color: tickColor }
                },
                x: {
                    grid: { display: false },
                    ticks: {
                        font: { family: fontFamily, size: 10 },
                        color: tickColor,
                        maxRotation: 40,
                        minRotation: 35
                    }
                }
            },
            layout: {
                padding: { left: 8, right: 8, top: 12, bottom: 8 }
            }
        }
    });
    
    // Обновляем график Memory
    const memoryCanvas = document.getElementById('memoryChart');
    const memoryCtx = memoryCanvas.getContext('2d');
    
    if (memoryChart) memoryChart.destroy();
    
    memoryChart = new Chart(memoryCtx, {
        type: 'bar',
        data: {
            labels: topPods.map(m => m.pod.length > 18 ? m.pod.substring(0, 18) + '…' : m.pod),
            datasets: [{
                label: 'Memory (MB)',
                data: topPods.map(m => Math.round((m.memory_raw || 0) / (1024 * 1024) * 10) / 10),
                backgroundColor: function(context) {
                    const chart = context.chart;
                    const ctx = chart.ctx;
                    const chartArea = chart.chartArea;
                    if (!chartArea) return '#64748b';
                    const g = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top);
                    g.addColorStop(0, '#cbd5e1');
                    g.addColorStop(0.5, '#94a3b8');
                    g.addColorStop(1, '#64748b');
                    return g;
                },
                borderRadius: 6,
                borderSkipped: false
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            aspectRatio: 2.2,
            plugins: {
                legend: { display: false },
                tooltip: {
                    backgroundColor: 'rgba(33, 37, 41, 0.9)',
                    padding: 10,
                    cornerRadius: 8,
                    titleFont: { family: fontFamily, size: 12 },
                    bodyFont: { family: fontFamily, size: 13 }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'MB',
                        font: { family: fontFamily, size: 11 },
                        color: tickColor
                    },
                    grid: { color: gridColor, drawBorder: false },
                    ticks: { font: { family: fontFamily, size: 10 }, color: tickColor }
                },
                x: {
                    grid: { display: false },
                    ticks: {
                        font: { family: fontFamily, size: 10 },
                        color: tickColor,
                        maxRotation: 40,
                        minRotation: 35
                    }
                }
            },
            layout: {
                padding: { left: 8, right: 8, top: 12, bottom: 8 }
            }
        }
    });
}
// Рендер таблицы подов
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
    
    document.getElementById('stats-count').textContent = filteredPods.length;
    
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
        const cpuPercent = metric && metric.cpu_percent !== undefined ? metric.cpu_percent : 0;
        const memoryPercent = metric && metric.memory_percent !== undefined ? metric.memory_percent : 0;
        
        const statusClass = getStatusClass(status);
        
        // Проверяем, есть ли активный port-forward для этого пода
        const hasActivePortForward = portForwardSessions.some(session => 
            session.pod === podName && session.namespace === namespace && session.status === 'running'
        );
        
        // Проверяем, есть ли активный лог стрим для этого пода
        const hasActiveLogStream = activeLogStreams.some(stream => 
            stream.pod === podName && stream.namespace === namespace
        );
        
        html += `
            <tr data-pod-name="${podName}" data-namespace="${namespace}">
                <td class="pods-td-name">
                    <div class="d-flex align-items-center pods-name-cell">
                        <i class="fas fa-cube text-primary pods-name-icon"></i>
                        <span class="pods-name-text">${highlightSearch(podName, searchTerm)}</span>
                        ${pod.ip ? `<small class="text-muted pods-name-ip">(${pod.ip})</small>` : ''}
                        ${hasActivePortForward ? `<i class="fas fa-exchange-alt text-success ms-1" title="Active Port-forward"></i>` : ''}
                        ${hasActiveLogStream ? `<i class="fas fa-stream text-warning ms-1" title="Active Log Stream"></i>` : ''}
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
                <td class="pods-td-actions">
                    <button type="button" class="btn btn-action btn-outline-danger btn-sm" 
                            onclick="event.stopPropagation(); deletePod('${namespace}', '${podName}')"
                            title="Delete">
                        <i class="fas fa-trash"></i>
                    </button>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
    
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
    const cpuVal = (data.total_cpu || 'N/A').toString().trim();
    const memVal = (data.total_memory || 'N/A').toString().trim();
    const containerCount = data.containers && Array.isArray(data.containers) ? data.containers.length : 0;
    const updatedAt = data.timestamp ? new Date(data.timestamp).toLocaleString() : '—';

    const cpuDetails = document.getElementById('cpu-details');
    cpuDetails.innerHTML = `
        <div class="metrics-summary">
            <div class="metrics-summary-value">${escapeHtml(cpuVal)}</div>
            <div class="metrics-summary-meta">
                <span>${containerCount} container${containerCount !== 1 ? 's' : ''}</span>
                <span class="metrics-summary-sep">·</span>
                <span>${escapeHtml(updatedAt)}</span>
            </div>
        </div>
    `;

    const memoryDetails = document.getElementById('memory-details');
    memoryDetails.innerHTML = `
        <div class="metrics-summary">
            <div class="metrics-summary-value">${escapeHtml(memVal)}</div>
            <div class="metrics-summary-meta">
                <span>${containerCount} container${containerCount !== 1 ? 's' : ''}</span>
                <span class="metrics-summary-sep">·</span>
                <span>${escapeHtml(updatedAt)}</span>
            </div>
        </div>
    `;

    createDetailedCharts(data);

    const containerTable = document.getElementById('container-metrics-table').querySelector('tbody');
    containerTable.innerHTML = '';

    if (data.containers && Array.isArray(data.containers)) {
        data.containers.forEach(container => {
            const cpuPct = Number(container.cpu_percent) || 0;
            const memPct = Number(container.memory_percent) || 0;
            const cpuBadge = cpuPct > 80 ? 'bg-danger' : cpuPct > 50 ? 'bg-warning' : 'bg-info';
            const memBadge = memPct > 80 ? 'bg-danger' : memPct > 50 ? 'bg-warning' : 'bg-info';
            const row = containerTable.insertRow();
            row.innerHTML = `
                <td><span class="fw-medium">${escapeHtml(container.name)}</span></td>
                <td class="text-end font-monospace">${escapeHtml(container.cpu_usage || '—')}</td>
                <td class="text-end font-monospace">${escapeHtml(container.cpu_limit || '—')}</td>
                <td class="text-end">
                    <div class="metrics-pct-cell">
                        <div class="progress metrics-pct-bar" title="${cpuPct}%">
                            <div class="progress-bar ${cpuBadge}" style="width:${Math.min(cpuPct, 100)}%"></div>
                        </div>
                        <span class="badge ${cpuBadge} metrics-pct-badge">${cpuPct}%</span>
                    </div>
                </td>
                <td class="text-end font-monospace">${escapeHtml(container.memory_usage || '—')}</td>
                <td class="text-end font-monospace">${escapeHtml(container.memory_limit || '—')}</td>
                <td class="text-end">
                    <div class="metrics-pct-cell">
                        <div class="progress metrics-pct-bar" title="${memPct}%">
                            <div class="progress-bar ${memBadge}" style="width:${Math.min(memPct, 100)}%"></div>
                        </div>
                        <span class="badge ${memBadge} metrics-pct-badge">${memPct}%</span>
                    </div>
                </td>
            `;
        });
    }
}

// Создание подробных графиков (остается без изменений)
function createDetailedCharts(data) {
    // Получаем контексты Canvas
    const cpuCanvas = document.getElementById('detailedCpuChart');
    const memoryCanvas = document.getElementById('detailedMemoryChart');
    
    const cpuCtx = cpuCanvas.getContext('2d');
    const memoryCtx = memoryCanvas.getContext('2d');
    
    // Уничтожаем старые графики если они есть
    if (detailedCpuChart) detailedCpuChart.destroy();
    if (detailedMemoryChart) detailedMemoryChart.destroy();
    
    // Подготовка данных
    if (!data.containers || !Array.isArray(data.containers) || data.containers.length === 0) {
        // График для отсутствия данных
        detailedCpuChart = new Chart(cpuCtx, {
            type: 'bar',
            data: {
                labels: ['No container data'],
                datasets: [{
                    label: 'CPU Usage',
                    data: [0],
                    backgroundColor: '#007bff',
                    borderColor: '#0056b3',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                aspectRatio: 2, // Устанавливаем соотношение сторон
                plugins: {
                    legend: {
                        display: true
                    },
                    title: {
                        display: true,
                        text: 'CPU Usage per Container',
                        font: {
                            size: 14
                        }
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'mCPU'
                        }
                    },
                    x: {
                        grid: {
                            display: false
                        }
                    }
                },
                layout: {
                    padding: {
                        left: 10,
                        right: 10,
                        top: 10,
                        bottom: 10
                    }
                }
            }
        });
        
        detailedMemoryChart = new Chart(memoryCtx, {
            type: 'bar',
            data: {
                labels: ['No container data'],
                datasets: [{
                    label: 'Memory Usage',
                    data: [0],
                    backgroundColor: '#28a745',
                    borderColor: '#1e7e34',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                aspectRatio: 2,
                plugins: {
                    legend: {
                        display: true
                    },
                    title: {
                        display: true,
                        text: 'Memory Usage per Container',
                        font: {
                            size: 14
                        }
                    }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'MB'
                        }
                    },
                    x: {
                        grid: {
                            display: false
                        }
                    }
                },
                layout: {
                    padding: {
                        left: 10,
                        right: 10,
                        top: 10,
                        bottom: 10
                    }
                }
            }
        });
        return;
    }
    
    const containerNames = data.containers.map(c => c.name);
    
    const cpuData = data.containers.map(c => {
        if (c.cpu_usage && c.cpu_usage.includes('m')) {
            const value = parseFloat(c.cpu_usage.replace('m', ''));
            return isNaN(value) ? 0 : value;
        }
        return 0;
    });
    
    const memoryData = data.containers.map(c => {
        if (c.memory_usage) {
            const match = c.memory_usage.match(/(\d+(?:\.\d+)?)\s*(Ki|Mi|Gi|Ti|B)?/i);
            if (match) {
                let value = parseFloat(match[1]);
                const unit = match[2] || '';
                
                if (isNaN(value)) return 0;
                
                switch(unit.toLowerCase()) {
                    case 'ki': return value / 1024;
                    case 'mi': return value;
                    case 'gi': return value * 1024;
                    case 'ti': return value * 1024 * 1024;
                    default: return value / (1024 * 1024);
                }
            }
        }
        return 0;
    });
    
    const cpuPercentData = data.containers.map(c => c.cpu_percent || 0);
    const memoryPercentData = data.containers.map(c => c.memory_percent || 0);
    
    // Создаем график CPU
    detailedCpuChart = new Chart(cpuCtx, {
        type: 'bar',
        data: {
            labels: containerNames,
            datasets: [
                {
                    label: 'CPU Usage (m)',
                    data: cpuData,
                    backgroundColor: '#007bff',
                    borderColor: '#0056b3',
                    borderWidth: 1,
                    yAxisID: 'y'
                },
                {
                    label: 'CPU %',
                    data: cpuPercentData,
                    type: 'line',
                    borderColor: '#ff6b6b',
                    backgroundColor: 'rgba(255, 107, 107, 0.1)',
                    borderWidth: 2,
                    fill: true,
                    tension: 0.4,
                    pointBackgroundColor: '#ff6b6b',
                    pointBorderColor: '#fff',
                    pointBorderWidth: 1,
                    pointRadius: 4,
                    pointHoverRadius: 6,
                    yAxisID: 'y1'
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            aspectRatio: 2,
            plugins: {
                legend: {
                    position: 'top',
                    labels: {
                        boxWidth: 12,
                        padding: 10
                    }
                },
                title: {
                    display: true,
                    text: 'CPU Usage per Container',
                    font: {
                        size: 14
                    }
                },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.datasetIndex === 0) {
                                label += context.parsed.y.toFixed(1) + ' mCPU';
                            } else {
                                label += context.parsed.y.toFixed(1) + '%';
                            }
                            return label;
                        }
                    }
                }
            },
            scales: {
                x: {
                    grid: {
                        display: false
                    },
                    ticks: {
                        maxRotation: 45,
                        minRotation: 45
                    }
                },
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'mCPU'
                    },
                    position: 'left',
                    grid: {
                        drawBorder: true
                    }
                },
                y1: {
                    beginAtZero: true,
                    max: 100,
                    title: {
                        display: true,
                        text: 'Percentage %'
                    },
                    position: 'right',
                    grid: {
                        drawOnChartArea: false,
                        drawBorder: true
                    }
                }
            },
            interaction: {
                intersect: false,
                mode: 'index'
            },
            layout: {
                padding: {
                    left: 10,
                    right: 10,
                    top: 10,
                    bottom: 10
                }
            }
        }
    });
    
    // Создаем график Memory
    detailedMemoryChart = new Chart(memoryCtx, {
        type: 'bar',
        data: {
            labels: containerNames,
            datasets: [
                {
                    label: 'Memory Usage (MB)',
                    data: memoryData,
                    backgroundColor: '#28a745',
                    borderColor: '#1e7e34',
                    borderWidth: 1,
                    yAxisID: 'y'
                },
                {
                    label: 'Memory %',
                    data: memoryPercentData,
                    type: 'line',
                    borderColor: '#4ecdc4',
                    backgroundColor: 'rgba(78, 205, 196, 0.1)',
                    borderWidth: 2,
                    fill: true,
                    tension: 0.4,
                    pointBackgroundColor: '#4ecdc4',
                    pointBorderColor: '#fff',
                    pointBorderWidth: 1,
                    pointRadius: 4,
                    pointHoverRadius: 6,
                    yAxisID: 'y1'
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: true,
            aspectRatio: 2,
            plugins: {
                legend: {
                    position: 'top',
                    labels: {
                        boxWidth: 12,
                        padding: 10
                    }
                },
                title: {
                    display: true,
                    text: 'Memory Usage per Container',
                    font: {
                        size: 14
                    }
                },
                tooltip: {
                    mode: 'index',
                    intersect: false,
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.datasetIndex === 0) {
                                label += context.parsed.y.toFixed(2) + ' MB';
                            } else {
                                label += context.parsed.y.toFixed(1) + '%';
                            }
                            return label;
                        }
                    }
                }
            },
            scales: {
                x: {
                    grid: {
                        display: false
                    },
                    ticks: {
                        maxRotation: 45,
                        minRotation: 45
                    }
                },
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Memory (MB)'
                    },
                    position: 'left',
                    grid: {
                        drawBorder: true
                    }
                },
                y1: {
                    beginAtZero: true,
                    max: 100,
                    title: {
                        display: true,
                        text: 'Percentage %'
                    },
                    position: 'right',
                    grid: {
                        drawOnChartArea: false,
                        drawBorder: true
                    }
                }
            },
            interaction: {
                intersect: false,
                mode: 'index'
            },
            layout: {
                padding: {
                    left: 10,
                    right: 10,
                    top: 10,
                    bottom: 10
                }
            }
        }
    });
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
        await apiFetch(`/api/pod/yaml/${namespace}/${podName}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml })
        });
        
        showToast('Pod configuration updated successfully!', 'success');
        document.getElementById('yaml-content').textContent = yaml;
        document.getElementById('yaml-content').style.display = 'block';
        document.getElementById('yaml-editor').style.display = 'none';
        document.getElementById('save-yaml-btn').style.display = 'none';
        
        loadPods();
        
    } catch (error) {
        showToast(`Failed to update: ${error.message}`, 'error');
    }
}

// Открыть веб-терминал (exec) для пода
function openPodExec(namespace, podName) {
    const params = new URLSearchParams({ namespace, pod: podName });
    window.location.href = `/ui/exec?${params.toString()}`;
}

// Показать детали пода (модалка с вкладками: Details, Logs, Port Forward, YAML, Console)
async function showPodDetails(namespace, podName) {
    currentPod = { namespace, name: podName };
    
    const el = document.getElementById('detailsModal');
    if (!el) return;
    document.getElementById('details-pod-name').textContent = podName;
    if (el.parentNode !== document.body) document.body.appendChild(el);
    const modal = new bootstrap.Modal(el);
    
    // При открытии модалки — автозапуск лог-стрима и консоли; при закрытии — остановка
    if (!el._detailModalLifecycleInited) {
        el._detailModalLifecycleInited = true;
        el.addEventListener('shown.bs.modal', function() {
            if (!currentPod) return;
            startDetailLogs();
            const logsTab = document.getElementById('tab-logs-btn');
            if (logsTab && typeof bootstrap !== 'undefined' && bootstrap.Tab) {
                try { bootstrap.Tab.getOrCreateInstance(logsTab).show(); } catch (_) {}
            }
        });
        el.addEventListener('hidden.bs.modal', function() {
            stopDetailLogs();
            stopDetailConsole();
            if (typeof loadLogStreamSessions === 'function') loadLogStreamSessions();
        });
    }
    
    modal.show();
    await loadPodDetails();
    
    // При переключении вкладки — инициализация контента (вкладки: Details, Logs, Port Forward, YAML, Console)
    const tabList = document.getElementById('podDetailTabs');
    if (!tabList) {
        if (typeof showToast === 'function') showToast('Обновите страницу (Ctrl+F5) и перезапустите сервер — нужна новая версия страницы', 'warning');
        return;
    }
    if (!tabList._detailTabInited) {
        tabList._detailTabInited = true;
        tabList.addEventListener('shown.bs.tab', function(e) {
            const targetId = (e.target.getAttribute && e.target.getAttribute('data-bs-target')) || '';
            if (targetId === '#tab-logs') initDetailLogsTab();
            else if (targetId === '#tab-portforward') initDetailPortForwardTab();
            else if (targetId === '#tab-yaml') loadDetailYaml();
            else if (targetId === '#tab-console') initDetailConsoleTab();
        });
    }
}

function initDetailLogsTab() {
    document.getElementById('detail-logs-status').className = 'alert alert-info py-2 small';
    document.getElementById('detail-logs-status').textContent = 'Click Start to load logs';
    document.getElementById('detail-log-count').textContent = '0 logs';
}

function initDetailPortForwardTab() {
    if (!currentPod) return;
    document.getElementById('detail-pf-pod').value = currentPod.name;
    document.getElementById('detail-pf-namespace').value = currentPod.namespace;
    document.getElementById('detail-pf-remote-port').value = '';
    document.getElementById('detail-pf-local-port').value = '';
    document.getElementById('detail-portforward-result').style.display = 'none';
}

function initDetailConsoleTab() {
    if (!currentPod || typeof PodExec === 'undefined') return;
    const label = document.getElementById('detail-exec-pod-label');
    if (label) label.textContent = `${currentPod.namespace}/${currentPod.name}`;
    if (!detailPodExec) {
        detailPodExec = PodExec.create({
            ids: {
                namespace: 'detail-exec-namespace-hidden',
                pod: 'detail-exec-pod-hidden',
                container: 'detail-exec-container-hidden',
                terminal: 'detail-exec-terminal',
                terminalContainer: 'detail-exec-terminal-container',
                placeholder: 'detail-exec-placeholder',
                status: 'detail-exec-status',
                connectBtn: 'detail-exec-connect-btn',
                disconnectBtn: 'detail-exec-disconnect-btn',
                podLabel: 'detail-exec-pod-label'
            },
            getParams: function() {
                if (!currentPod) return { namespace: '', pod: '', container: '' };
                return {
                    namespace: currentPod.namespace,
                    pod: currentPod.name,
                    container: ''
                };
            }
        });
    }
    detailPodExec.connect();
}

function stopDetailConsole() {
    if (detailPodExec) {
        detailPodExec.disconnect();
    }
}

function startDetailLogs() {
    if (!currentPod) return;
    if (detailLogWebSocket) stopDetailLogs();
    const tail = document.getElementById('detail-log-tail').value || '100';
    const follow = document.getElementById('detail-log-follow').checked;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/logs/stream/${currentPod.namespace}/${currentPod.name}?tail=${tail}&follow=${follow}&buffer=1000`;
    detailLogMessages = [];
    document.getElementById('detail-logs-content').innerHTML = '';
    document.getElementById('detail-logs-status').className = 'alert alert-warning py-2 small';
    document.getElementById('detail-logs-status').innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i> Connecting...';
    detailLogWebSocket = new WebSocket(wsUrl);
    detailLogWebSocket.onopen = function() {
        document.getElementById('detail-logs-status').className = 'alert alert-success py-2 small';
        document.getElementById('detail-logs-status').textContent = 'Connected. Streaming logs...';
    };
    detailLogWebSocket.onmessage = function(event) {
        try {
            const msg = JSON.parse(event.data);
            detailLogMessages.push(msg);
            if (detailLogMessages.length > 1000) detailLogMessages = detailLogMessages.slice(-1000);
            const pre = document.getElementById('detail-logs-content');
            const line = document.createElement('div');
            line.textContent = (msg.time ? new Date(msg.time).toLocaleTimeString() + ' ' : '') + (msg.message || msg.content || '');
            pre.appendChild(line);
            pre.scrollTop = pre.scrollHeight;
            document.getElementById('detail-log-count').textContent = detailLogMessages.length + ' logs';
        } catch (_) {}
    };
    detailLogWebSocket.onerror = function() {
        document.getElementById('detail-logs-status').className = 'alert alert-danger py-2 small';
        document.getElementById('detail-logs-status').textContent = 'Connection error';
    };
    detailLogWebSocket.onclose = function() {
        document.getElementById('detail-logs-status').className = 'alert alert-info py-2 small';
        document.getElementById('detail-logs-status').textContent = 'Disconnected';
        detailLogWebSocket = null;
    };
}

function stopDetailLogs() {
    if (detailLogWebSocket) {
        detailLogWebSocket.close();
        detailLogWebSocket = null;
    }
    const statusEl = document.getElementById('detail-logs-status');
    if (statusEl) {
        statusEl.className = 'alert alert-info py-2 small';
        statusEl.textContent = 'Stopped';
    }
}

function clearDetailLogs() {
    stopDetailLogs();
    detailLogMessages = [];
    document.getElementById('detail-logs-content').innerHTML = '';
    document.getElementById('detail-log-count').textContent = '0 logs';
}

function setDetailPort(port) {
    document.getElementById('detail-pf-remote-port').value = port;
    document.getElementById('detail-pf-local-port').value = port || '';
}

async function startPortForwardFromDetail() {
    if (!currentPod) return;
    const remotePort = parseInt(document.getElementById('detail-pf-remote-port').value);
    let localPort = parseInt(document.getElementById('detail-pf-local-port').value);
    if (!remotePort || remotePort < 1 || remotePort > 65535) {
        showToast('Enter valid remote port (1-65535)', 'error');
        return;
    }
    if (!localPort || localPort < 1024) localPort = remotePort;
    try {
        const data = await apiFetch('/api/portforward/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ pod: currentPod.name, namespace: currentPod.namespace, remotePort, localPort })
        });
        currentPortForwardSession = data.session;
        const url = `http://localhost:${localPort}`;
        document.getElementById('detail-forwarded-link').href = url;
        document.getElementById('detail-forwarded-link').textContent = url;
        document.getElementById('detail-portforward-result').style.display = 'block';
        showToast('Port forward started', 'success');
    } catch (e) {
        showToast('Port forward failed: ' + e.message, 'error');
    }
}

async function stopPortForwardFromDetail() {
    if (!currentPortForwardSession) return;
    try {
        await apiFetch(`/api/portforward/stop/${currentPortForwardSession.id}`, { method: 'POST' });
        currentPortForwardSession = null;
        document.getElementById('detail-portforward-result').style.display = 'none';
        showToast('Port forward stopped', 'info');
    } catch (e) {
        showToast('Stop failed: ' + e.message, 'error');
    }
}

async function loadDetailYaml() {
    if (!currentPod) return;
    try {
        const res = await fetch(`/api/pod/yaml/${currentPod.namespace}/${currentPod.name}`);
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        document.getElementById('detail-yaml-content').textContent = data.yaml;
        document.getElementById('detail-yaml-editor').value = data.yaml;
        document.getElementById('detail-yaml-content').classList.remove('d-none');
        document.getElementById('detail-yaml-editor').classList.add('d-none');
        document.getElementById('detail-yaml-save-btn').classList.add('d-none');
    } catch (e) {
        document.getElementById('detail-yaml-content').textContent = 'Error: ' + e.message;
    }
}

function copyDetailYaml() {
    const text = document.getElementById('detail-yaml-content').textContent;
    navigator.clipboard.writeText(text).then(() => showToast('YAML copied', 'success')).catch(() => showToast('Copy failed', 'error'));
}

function downloadDetailYaml() {
    const text = document.getElementById('detail-yaml-content').textContent;
    const a = document.createElement('a');
    a.href = 'data:text/yaml;charset=utf-8,' + encodeURIComponent(text);
    a.download = (currentPod && currentPod.name ? currentPod.name : 'pod') + '.yaml';
    a.click();
}

function toggleDetailYamlEdit() {
    document.getElementById('detail-yaml-content').classList.toggle('d-none');
    document.getElementById('detail-yaml-editor').classList.toggle('d-none');
    document.getElementById('detail-yaml-save-btn').classList.toggle('d-none');
}

async function saveDetailYaml() {
    if (!currentPod) return;
    const yaml = document.getElementById('detail-yaml-editor').value;
    try {
        await apiFetch(`/api/pod/yaml/${currentPod.namespace}/${currentPod.name}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml })
        });
        document.getElementById('detail-yaml-content').textContent = yaml;
        toggleDetailYamlEdit();
        showToast('YAML saved', 'success');
        loadPods();
    } catch (e) {
        showToast('Save failed: ' + e.message, 'error');
    }
}

function deletePodFromDetails() {
    if (!currentPod) return;
    if (!confirm(`Delete pod "${currentPod.name}"?`)) return;
    const modal = bootstrap.Modal.getInstance(document.getElementById('detailsModal'));
    if (modal) modal.hide();
    deletePod(currentPod.namespace, currentPod.name);
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
    document.getElementById('details-namespace').textContent = data.namespace;
    document.getElementById('details-status').innerHTML = 
        `<span class="badge-status ${getStatusClass(data.status.phase)}">${escapeHtml(data.status.phase)}</span>`;
    document.getElementById('details-node').textContent = data.nodeName || '-';
    document.getElementById('details-pod-ip').textContent = data.podIP || '-';
    document.getElementById('details-host-ip').textContent = data.hostIP || '-';
    document.getElementById('details-created').textContent = 
        new Date(data.metadata.creationTimestamp).toLocaleString();
    document.getElementById('details-start-time').textContent = 
        data.startTime ? new Date(data.startTime).toLocaleString() : '-';
    
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
    
    const containersList = document.getElementById('containers-list');
    containersList.innerHTML = '';
    if (data.containers && Array.isArray(data.containers)) {
        data.containers.forEach(container => {
            const containerDiv = document.createElement('div');
            containerDiv.className = 'card mb-2';
            containerDiv.innerHTML = `
                <div class="card-header">
                    <strong>${escapeHtml(container.name)}</strong> - ${escapeHtml(container.image)}
                </div>
                <div class="card-body">
                    <p><strong>Resources:</strong> ${escapeHtml(JSON.stringify(container.resources || {}))}</p>
                    <p><strong>Ports:</strong> ${escapeHtml(JSON.stringify(container.ports || []))}</p>
                </div>
            `;
            containersList.appendChild(containerDiv);
        });
    }
    
    const conditionsTable = document.getElementById('conditions-table').querySelector('tbody');
    conditionsTable.innerHTML = '';
    if (data.conditions && Array.isArray(data.conditions)) {
        data.conditions.forEach(condition => {
            const row = conditionsTable.insertRow();
            row.innerHTML = `
                <td>${escapeHtml(condition.type)}</td>
                <td>${escapeHtml(condition.status)}</td>
                <td>${escapeHtml(condition.lastProbeTime || '-')}</td>
                <td>${escapeHtml(condition.lastTransitionTime || '-')}</td>
                <td>${escapeHtml(condition.reason || '-')}</td>
                <td>${escapeHtml(condition.message || '-')}</td>
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
        await apiFetch(`/api/pod/${namespace}/${podName}`, { method: 'DELETE' });
        
        showToast(`Pod "${podName}" deleted successfully!`, 'success');
        loadPods();
        
    } catch (error) {
        showToast(`Failed to delete pod: ${error.message}`, 'error');
    }
}

// ===== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ =====

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
    const safeText = escapeHtml(String(text || ''));
    if (!search) return safeText;
    const escapedSearch = escapeRegExp(String(search));
    if (!escapedSearch) return safeText;
    const regex = new RegExp(`(${escapedSearch})`, 'gi');
    return safeText.replace(regex, '<span class="highlight">$1</span>');
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
    if (typeof window.showToast === 'function' && window.showToast !== showToast) {
        window.showToast(message, type);
        return;
    }
    const toast = document.createElement('div');
    toast.className = `toast-alert alert alert-${type} alert-dismissible fade show`;
    toast.style.cssText = 'position: fixed; top: 20px; right: 20px; z-index: 9999; min-width: 300px;';
    toast.innerHTML = `
        ${escapeHtml(String(message || ''))}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
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

// Экранирование HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function escapeRegExp(text) {
    return String(text || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
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
// Добавьте эту функцию в конце файла pods.js
function resizeChartsOnModalOpen() {
    // Ресайз графиков при открытии модального окна метрик
    $('#metricsModal').on('shown.bs.modal', function () {
        if (detailedCpuChart) {
            detailedCpuChart.resize();
        }
        if (detailedMemoryChart) {
            detailedMemoryChart.resize();
        }
    });
    
    // Ресайз графиков при открытии модального окна с основными графиками
    $('#logsModal').on('shown.bs.modal', function () {
        // Можете добавить ресайз других графиков если нужно
    });
}

// Автообновление каждые 30 секунд
setInterval(() => {
    if (document.visibilityState === 'visible') {
        loadPods();
        loadMetrics();
    }
}, 30000);