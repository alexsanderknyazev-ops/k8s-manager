// Переменные для хранения данных
let clusterData = {};
let resourceChart = null;
let autoRefreshInterval = null;
let isFirstLoad = true;
let metricsData = {};

// История для графика: реальные метрики по времени (последние 60 точек ≈ 30 мин при обновлении раз в 30 сек)
const MAX_CHART_POINTS = 60;
let chartHistory = [];

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadNamespacesIntoSelector().then(() => loadDashboard());
    setupEventListeners();
    setupAutoRefresh();
    initializeChart();
});

// Настройка слушателей событий
function setupEventListeners() {
    document.addEventListener('keydown', function(e) {
        if (e.key === 'F5') {
            e.preventDefault();
            refreshDashboard();
        }
    });
    const nsSel = document.getElementById('namespace-selector');
    if (nsSel) {
        nsSel.addEventListener('change', function() { refreshDashboard(); });
    }
}

// Настройка автообновления
function setupAutoRefresh() {
    // Очищаем предыдущий интервал
    clearInterval(autoRefreshInterval);
    
    // Обновляем каждые 30 секунд
    autoRefreshInterval = setInterval(() => {
        if (document.visibilityState === 'visible') {
            loadDashboard(false); // false = не показывать loading
        }
    }, 30000);
}

// Загрузка всех данных дашборда
async function loadDashboard(showLoading = true) {
    if (showLoading) {
        showLoadingState();
    }
    
    const sel = document.getElementById('namespace-selector');
    const namespace = (sel && sel.value) ? sel.value : 'market';
    
    try {
        const toJson = (res) => res.json().catch(() => ({}));
        const [
            podsData,
            deploymentsData,
            nodesData,
            namespacesData,
            clusterInfo,
            podsMetrics,
            nodesMetrics,
            servicesData
        ] = await Promise.all([
            fetch(`/api/pods?namespace=${namespace}`).then(toJson),
            fetch(`/api/deployments?namespace=${namespace}`).then(toJson),
            fetch('/api/nodes').then(toJson),
            fetch('/api/namespaces').then(toJson),
            fetch('/api/test').then(toJson),
            fetch(`/api/metrics/pods/${namespace}`).then(toJson),
            fetch('/api/metrics/nodes').then(toJson),
            fetch(`/api/services?namespace=${namespace}`).then(toJson)
        ]);
        
        const hasError = (d) => d && d.error;
        if (hasError(podsData) || hasError(nodesData)) {
            showError(hasError(podsData) ? podsData.error : nodesData.error);
        }
        
        clusterData = {
            pods: (podsData && podsData.pods) ? podsData.pods : [],
            deployments: (deploymentsData && deploymentsData.deployments) ? deploymentsData.deployments : [],
            nodes: (nodesData && nodesData.nodes) ? nodesData.nodes : [],
            namespaces: (namespacesData && namespacesData.namespaces) ? namespacesData.namespaces : [],
            services: (servicesData && servicesData.services) ? servicesData.services : [],
            clusterInfo: clusterInfo || {}
        };
        
        metricsData = {
            pods: (podsMetrics && podsMetrics.metrics) ? podsMetrics.metrics : [],
            nodes: (nodesMetrics && nodesMetrics.nodes) ? nodesMetrics.nodes : [],
            clusterUsage: (nodesMetrics && nodesMetrics.cluster_usage) ? nodesMetrics.cluster_usage : {},
            services: (servicesData && servicesData.services) ? servicesData.services : []
        };
        
        if (namespacesData && namespacesData.namespaces) {
            populateNamespaceSelector(namespacesData.namespaces);
        }
        // Обновляем UI
        updateDashboard();
        
        // Загружаем события и дополнительную информацию
        loadEvents(namespace);
        loadTopConsumers();
        loadRecentDeployments(namespace);
        loadServices();
        updateNodeMetrics();
        
        if (isFirstLoad) {
            isFirstLoad = false;
            showToast(`Dashboard loaded successfully! (Namespace: ${namespace})`, 'success');
        }
        
    } catch (error) {
        console.error('Error loading dashboard:', error);
        showError('Failed to load dashboard data: ' + error.message);
        updateConnectionStatus(false);
    } finally {
        if (showLoading) {
            hideLoadingState();
        }
    }
}

// Обновление дашборда
function updateDashboard() {
    updateClusterStatus();
    updatePodsStatus();
    updateDeploymentsStatus();
    updateNodesStatus();
    updateResourceUsage();
    updateClusterInfo();
    updateChartData();
    updatePodsTable();
}

// Обновление статуса кластера
function updateClusterStatus() {
    const nodes = clusterData.nodes || [];
    const totalNodes = nodes.length;
    const readyNodes = nodes.filter(node => {
        const s = node.status;
        return Array.isArray(s) && s.includes('True');
    }).length;
    
    const nodePercentage = totalNodes > 0 ? Math.round((readyNodes / totalNodes) * 100) : 0;
    const clusterStatus = nodePercentage >= 80 ? 'Healthy' : 
                         nodePercentage >= 50 ? 'Degraded' : 'Unhealthy';
    const statusColor = nodePercentage >= 80 ? 'success' : 
                       nodePercentage >= 50 ? 'warning' : 'danger';
    
    document.getElementById('cluster-status').textContent = clusterStatus;
    document.getElementById('cluster-status').className = `mb-0 text-${statusColor}`;
    const nodesCountEl = document.getElementById('nodes-count');
    const nodesProgressEl = document.getElementById('nodes-progress');
    const nodesBadgeEl = document.getElementById('nodes-badge');
    if (nodesCountEl) nodesCountEl.textContent = totalNodes;
    if (nodesProgressEl) nodesProgressEl.style.width = `${nodePercentage}%`;
    if (nodesBadgeEl) nodesBadgeEl.textContent = `${totalNodes} node${totalNodes !== 1 ? 's' : ''}`;
    updateConnectionStatus(true);
}

// Обновление статуса подов
function updatePodsStatus() {
    const pods = clusterData.pods || [];
    const totalPods = pods.length;
    const runningPods = pods.filter(pod => pod.status === 'Running').length;
    const podsPercentage = totalPods > 0 ? Math.round((runningPods / totalPods) * 100) : 0;
    
    document.getElementById('pods-count').textContent = totalPods;
    document.getElementById('running-pods').textContent = runningPods;
    document.getElementById('pods-progress').style.width = `${podsPercentage}%`;
    document.getElementById('current-pods').textContent = totalPods;
    document.getElementById('current-pods-bar').style.width = `${Math.min(totalPods * 10, 100)}%`;
}

// Обновление статуса деплойментов
function updateDeploymentsStatus() {
    const deployments = clusterData.deployments || [];
    const totalDeployments = deployments.length;
    const readyDeployments = deployments.filter(deployment => {
        const ready = deployment.ready_count || 0;
        const total = deployment.total_count || 0;
        return ready === total && ready > 0;
    }).length;
    
    const deploymentsPercentage = totalDeployments > 0 ? Math.round((readyDeployments / totalDeployments) * 100) : 0;
    
    document.getElementById('deployments-count').textContent = totalDeployments;
    document.getElementById('ready-deployments').textContent = readyDeployments;
    document.getElementById('deployments-progress').style.width = `${deploymentsPercentage}%`;
}

// Обновление таблицы подов
function updatePodsTable() {
    const podsTableBody = document.getElementById('pods-table');
    if (!podsTableBody) return;
    
    const sel = document.getElementById('namespace-selector');
    const namespace = (sel && sel.value) ? sel.value : 'market';
    const pods = clusterData.pods || [];
    const filteredPods = namespace === 'all' ? pods : pods.filter(pod => pod.namespace === namespace);
    
    const currentNsEl = document.getElementById('current-namespace');
    const podsBadgeEl = document.getElementById('pods-badge');
    if (currentNsEl) currentNsEl.textContent = namespace === 'all' ? 'all' : namespace;
    if (podsBadgeEl) podsBadgeEl.textContent = `${filteredPods.length} pod${filteredPods.length !== 1 ? 's' : ''}`;
    
    let html = '';
    filteredPods.forEach(pod => {
        
        // Находим метрики для этого пода
        const podMetrics = metricsData.pods.find(p => p.pod === pod.name);
        
        // Считаем готовые контейнеры
        const ready = pod.ready ? pod.ready.split('/')[0] : '0';
        const total = pod.ready ? pod.ready.split('/')[1] : '0';
        const readyPercent = total > 0 ? Math.round((ready / total) * 100) : 0;
        
        // Определяем цвет статуса
        let statusColor = 'secondary';
        if (pod.status === 'Running') statusColor = 'success';
        else if (pod.status === 'Pending') statusColor = 'warning';
        else if (pod.status === 'Failed') statusColor = 'danger';
        else if (pod.status === 'Unknown') statusColor = 'dark';
        
        html += `
            <tr>
                <td>
                    <strong>${pod.name}</strong>
                    <div class="small text-muted">${pod.namespace}</div>
                </td>
                <td>
                    <span class="badge bg-${statusColor}">${pod.status}</span>
                </td>
                <td>
                    <div class="d-flex align-items-center">
                        <div class="me-2" style="min-width: 60px;">
                            <small>${pod.ready || '0/0'}</small>
                        </div>
                        <div class="progress flex-grow-1" style="height: 6px;">
                            <div class="progress-bar bg-success" style="width: ${readyPercent}%"></div>
                        </div>
                    </div>
                </td>
                <td>${pod.restarts || 0}</td>
                <td>
                    <div class="small">
                        <div>CPU: ${podMetrics ? podMetrics.cpu_usage : 'N/A'}</div>
                        <div>Mem: ${podMetrics ? podMetrics.memory_usage : 'N/A'}</div>
                    </div>
                </td>
                <td>${pod.node || 'N/A'}</td>
                <td>${pod.age || 'N/A'}</td>
                <td>
                    <div class="btn-group btn-group-sm">
                        <button class="btn btn-outline-info" onclick="viewPodLogs('${pod.namespace}', '${pod.name}')">
                            <i class="fas fa-file-alt"></i>
                        </button>
                        <button class="btn btn-outline-primary" onclick="viewPodYAML('${pod.namespace}', '${pod.name}')">
                            <i class="fas fa-code"></i>
                        </button>
                        <button class="btn btn-outline-danger" onclick="deletePod('${pod.namespace}', '${pod.name}')">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    podsTableBody.innerHTML = html || `
        <tr>
            <td colspan="8" class="text-center py-4">
                <i class="fas fa-cube fa-2x text-muted mb-2"></i>
                <p class="text-muted">No pods found in ${namespace} namespace</p>
            </td>
        </tr>
    `;
}

// Обновление статуса нод с метриками
function updateNodesStatus() {
    const nodesList = document.getElementById('nodes-list');
    
    if (!clusterData.nodes || clusterData.nodes.length === 0) {
        nodesList.innerHTML = `
            <div class="text-center py-4">
                <i class="fas fa-server fa-2x text-muted mb-2"></i>
                <p class="text-muted small">No nodes found</p>
            </div>
        `;
        return;
    }
    
    let html = '';
    (clusterData.nodes || []).forEach(node => {
        const s = node.status;
        const isReady = Array.isArray(s) && s.includes('True');
        const nodeClass = isReady ? 'ready' : 'not-ready';
        const statusColor = isReady ? 'success' : 'danger';
        const statusText = isReady ? 'Ready' : 'Not Ready';
        
        // Находим метрики для этой ноды
        const nodeMetrics = metricsData.nodes.find(n => n.name === node.name);
        
        html += `
            <div class="node-card mb-3 p-3 border rounded">
                <div class="d-flex justify-content-between align-items-center">
                    <div>
                        <div class="d-flex align-items-center mb-2">
                            <span class="node-status ${nodeClass} me-2"></span>
                            <strong>${node.name}</strong>
                        </div>
                        <div class="small text-muted">
                            <div>OS: ${node.os || 'Unknown'}</div>
                            <div>Version: ${node.version || 'Unknown'}</div>
                        </div>
                    </div>
                    <div class="text-end">
                        <span class="badge bg-${statusColor}">${statusText}</span>
                        <div class="small text-muted mt-1">${node.age || 'Unknown'}</div>
                    </div>
                </div>
                ${nodeMetrics ? `
                <div class="node-metrics mt-3">
                    <div class="row g-2">
                        <div class="col-6">
                            <small class="text-muted">CPU Usage</small>
                            <div class="fw-bold">${nodeMetrics.cpu_usage || 'N/A'}</div>
                            <div class="progress" style="height: 4px;">
                                <div class="progress-bar bg-danger" style="width: ${nodeMetrics.cpu_percent || 0}%"></div>
                            </div>
                        </div>
                        <div class="col-6">
                            <small class="text-muted">Memory Usage</small>
                            <div class="fw-bold">${nodeMetrics.memory_usage || 'N/A'}</div>
                            <div class="progress" style="height: 4px;">
                                <div class="progress-bar bg-info" style="width: ${nodeMetrics.memory_percent || 0}%"></div>
                            </div>
                        </div>
                    </div>
                </div>
                ` : ''}
            </div>
        `;
    });
    
    nodesList.innerHTML = html;
}

// Обновление метрик нод
function updateNodeMetrics() {
    if (metricsData.clusterUsage && metricsData.clusterUsage.cpu_percent) {
        const cpuPercent = metricsData.clusterUsage.cpu_percent;
        const memoryPercent = metricsData.clusterUsage.memory_percent;
        
        document.getElementById('resource-usage').textContent = `${Math.round((cpuPercent + memoryPercent) / 2)}%`;
        document.getElementById('cpu-usage').textContent = `${cpuPercent}%`;
        document.getElementById('cpu-progress').style.width = `${cpuPercent}%`;
        
        document.getElementById('current-cpu').textContent = `${cpuPercent}%`;
        document.getElementById('current-cpu-bar').style.width = `${cpuPercent}%`;
        document.getElementById('current-memory').textContent = `${memoryPercent}%`;
        document.getElementById('current-memory-bar').style.width = `${memoryPercent}%`;
        
        // Обновляем информацию об общих ресурсах
        document.getElementById('total-cpu').textContent = metricsData.clusterUsage.total_cpu_allocatable || '--';
        document.getElementById('total-memory').textContent = metricsData.clusterUsage.total_memory_allocatable || '--';
    }
}

// Обновление использования ресурсов
function updateResourceUsage() {
    // Используем реальные метрики, если они есть
    if (metricsData.clusterUsage && metricsData.clusterUsage.cpu_percent) {
        const cpuPercent = metricsData.clusterUsage.cpu_percent;
        const memoryPercent = metricsData.clusterUsage.memory_percent;
        const totalUsage = Math.round((cpuPercent + memoryPercent) / 2);
        
        document.getElementById('resource-usage').textContent = `${totalUsage}%`;
        document.getElementById('cpu-usage').textContent = `${cpuPercent}%`;
        document.getElementById('cpu-progress').style.width = `${cpuPercent}%`;
        
        document.getElementById('current-cpu').textContent = `${cpuPercent}%`;
        document.getElementById('current-cpu-bar').style.width = `${cpuPercent}%`;
        document.getElementById('current-memory').textContent = `${memoryPercent}%`;
        document.getElementById('current-memory-bar').style.width = `${memoryPercent}%`;
    } else {
        const totalPods = (clusterData.pods || []).length;
        const totalDeployments = (clusterData.deployments || []).length;
        
        const cpuUsage = Math.min(100, Math.floor(totalPods * 8 + totalDeployments * 5));
        const memoryUsage = Math.min(100, Math.floor(totalPods * 12 + totalDeployments * 8));
        const totalUsage = Math.round((cpuUsage + memoryUsage) / 2);
        
        document.getElementById('resource-usage').textContent = `${totalUsage}%`;
        document.getElementById('cpu-usage').textContent = `${cpuUsage}%`;
        document.getElementById('cpu-progress').style.width = `${cpuUsage}%`;
        
        document.getElementById('current-cpu').textContent = `${cpuUsage}%`;
        document.getElementById('current-cpu-bar').style.width = `${cpuUsage}%`;
        document.getElementById('current-memory').textContent = `${memoryUsage}%`;
        document.getElementById('current-memory-bar').style.width = `${memoryUsage}%`;
    }
}

// Обновление информации о кластере
function updateClusterInfo() {
    const nodes = clusterData.nodes || [];
    const setText = (id, text) => { const el = document.getElementById(id); if (el) el.textContent = text; };
    if (nodes.length > 0) {
        const firstNode = nodes[0];
        setText('k8s-version', firstNode.version || '—');
        setText('container-runtime', firstNode.containerd || '—');
        setText('os-info', firstNode.os || '—');
        setText('kernel-version', firstNode.kernel || '—');
    } else {
        setText('k8s-version', '—');
        setText('container-runtime', '—');
        setText('os-info', '—');
        setText('kernel-version', '—');
    }
    
    const namespacesList = document.getElementById('namespaces-list');
    if (namespacesList && clusterData.namespaces && clusterData.namespaces.length > 0) {
        let html = '';
        const nsVal = (document.getElementById('namespace-selector') || {}).value || '';
        clusterData.namespaces.forEach(ns => {
            const isActive = ns.status === 'Active';
            const statusColor = isActive ? 'success' : 'warning';
            const badgeClass = nsVal === (ns.name || ns) ? 'border border-primary' : '';
            
            const name = ns.name || ns;
            html += `<span class="badge bg-${statusColor} namespace-badge ${badgeClass} me-1 mb-1" onclick="filterByNamespace('${name}')" style="cursor: pointer;">${name}${isActive ? '' : ' (inactive)'}</span>`;
        });
        namespacesList.innerHTML = html;
    }
    
    const totalCpuEl = document.getElementById('total-cpu');
    const totalMemEl = document.getElementById('total-memory');
    if (totalCpuEl) totalCpuEl.textContent = (metricsData.clusterUsage && metricsData.clusterUsage.total_cpu_allocatable) ? metricsData.clusterUsage.total_cpu_allocatable : '—';
    if (totalMemEl) totalMemEl.textContent = (metricsData.clusterUsage && metricsData.clusterUsage.total_memory_allocatable) ? metricsData.clusterUsage.total_memory_allocatable : '—';
}

// Загрузка списка namespace в селектор (при первой загрузке страницы)
async function loadNamespacesIntoSelector() {
    try {
        const res = await fetch('/api/namespaces');
        const data = await res.json();
        if (data.namespaces && data.namespaces.length) {
            populateNamespaceSelector(data.namespaces);
        }
    } catch (e) {
        console.warn('Could not load namespaces for selector:', e);
    }
}

// Заполнение селектора namespace (сохраняем текущее значение)
function populateNamespaceSelector(namespaces) {
    const sel = document.getElementById('namespace-selector');
    if (!sel) return;
    const current = sel.value;
    sel.innerHTML = '';
    const option = (val, label) => {
        const o = document.createElement('option');
        o.value = val;
        o.textContent = label || val;
        sel.appendChild(o);
    };
    option('market', 'market');
    option('default', 'default');
    option('kube-system', 'kube-system');
    namespaces.forEach(ns => {
        const name = ns.name || ns;
        if (!['market', 'default', 'kube-system'].includes(name)) {
            option(name, name);
        }
    });
    option('all', 'All Namespaces');
    if (['market', 'default', 'kube-system', 'all'].includes(current) || namespaces.some(n => (n.name || n) === current)) {
        sel.value = current;
    }
}

// Загрузка событий из API (namespace опционален — берётся из селектора)
async function loadEvents(namespace) {
    const tbody = document.getElementById('events-table');
    if (!tbody) return;
    const ns = namespace || ((document.getElementById('namespace-selector') || {}).value) || 'market';
    try {
        const res = await fetch(`/api/events?namespace=${encodeURIComponent(ns)}`);
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
        const events = data.events || [];
        let html = '';
        events.forEach(event => {
            const typeClass = event.type === 'Normal' ? 'event-normal' : event.type === 'Warning' ? 'event-warning' : 'event-error';
            const typeColor = event.type === 'Normal' ? 'success' : event.type === 'Warning' ? 'warning' : 'danger';
            const obj = event.object || event.reason || '—';
            const msg = (event.message || event.reason || '').substring(0, 80);
            const time = event.time || '—';
            html += `
                <tr class="${typeClass}">
                    <td><span class="badge bg-${typeColor}">${escapeHtml(event.type || 'Normal')}</span></td>
                    <td><small>${escapeHtml(obj)}</small></td>
                    <td><small>${escapeHtml(msg)}</small></td>
                    <td><small class="text-muted">${escapeHtml(time)}</small></td>
                </tr>`;
        });
        tbody.innerHTML = html || '<tr><td colspan="4" class="text-center text-muted py-3">No events</td></tr>';
    } catch (error) {
        console.error('Error loading events:', error);
        tbody.innerHTML = '<tr><td colspan="4" class="text-center text-muted py-3">Events unavailable</td></tr>';
    }
}

function escapeHtml(s) {
    if (!s) return '';
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
}

// Загрузка топ потребителей ресурсов с реальными метриками
async function loadTopConsumers() {
    try {
        const namespace = document.getElementById('namespace-selector').value || 'market';
        
        if (metricsData.pods && metricsData.pods.length > 0) {
            updateTopConsumersWithRealMetrics(namespace);
        } else {
            simulateTopConsumers();
        }
        
    } catch (error) {
        console.error('Error loading top consumers:', error);
    }
}

// Обновление топ потребителей с реальными метриками
function updateTopConsumersWithRealMetrics(namespace) {
    // Сортируем по использованию CPU
    const cpuConsumers = [...metricsData.pods]
        .filter(pod => pod.namespace === namespace)
        .sort((a, b) => (b.cpu_raw || 0) - (a.cpu_raw || 0))
        .slice(0, 5);
    
    // Сортируем по использованию памяти
    const memoryConsumers = [...metricsData.pods]
        .filter(pod => pod.namespace === namespace)
        .sort((a, b) => (b.memory_raw || 0) - (a.memory_raw || 0))
        .slice(0, 5);
    
    // Обновляем топ CPU
    const cpuList = document.getElementById('top-cpu-consumers');
    if (cpuList) {
        let html = '';
        cpuConsumers.forEach((pod, index) => {
            const cpuUsage = pod.cpu_usage || '0m';
            const cpuPercent = pod.cpu_percent || 0;
            
            html += `
                <div class="list-group-item">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <div class="fw-bold">${pod.pod}</div>
                            <small class="text-muted">CPU: ${pod.cpu_limit || 'No limit'}</small>
                        </div>
                        <div class="text-end">
                            <span class="badge bg-danger">${cpuUsage}</span>
                            <div class="progress mt-1" style="height: 3px; width: 60px;">
                                <div class="progress-bar bg-danger" style="width: ${Math.min(cpuPercent, 100)}%"></div>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        });
        cpuList.innerHTML = html || `
            <div class="list-group-item">
                <div class="text-center text-muted py-3">
                    No CPU metrics available
                </div>
            </div>
        `;
    }
    
    // Обновляем топ памяти
    const memoryList = document.getElementById('top-memory-consumers');
    if (memoryList) {
        let html = '';
        memoryConsumers.forEach((pod, index) => {
            const memoryUsage = pod.memory_usage || '0Mi';
            const memoryPercent = pod.memory_percent || 0;
            
            html += `
                <div class="list-group-item">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <div class="fw-bold">${pod.pod}</div>
                            <small class="text-muted">Mem: ${pod.memory_limit || 'No limit'}</small>
                        </div>
                        <div class="text-end">
                            <span class="badge bg-info">${memoryUsage}</span>
                            <div class="progress mt-1" style="height: 3px; width: 60px;">
                                <div class="progress-bar bg-info" style="width: ${Math.min(memoryPercent, 100)}%"></div>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        });
        memoryList.innerHTML = html || `
            <div class="list-group-item">
                <div class="text-center text-muted py-3">
                    No memory metrics available
                </div>
            </div>
        `;
    }
}

// Пустое состояние, когда Metrics Server недоступен (без фейковых данных)
function simulateTopConsumers() {
    var emptyMsg = '<div class="list-group-item"><div class="text-center text-muted py-3 small">Метрики недоступны. Установите <a href="https://github.com/kubernetes-sigs/metrics-server" target="_blank" rel="noopener">Metrics Server</a>.</div></div>';
    var cpuList = document.getElementById('top-cpu-consumers');
    var memoryList = document.getElementById('top-memory-consumers');
    if (cpuList) cpuList.innerHTML = emptyMsg;
    if (memoryList) memoryList.innerHTML = emptyMsg;
}

// Загрузка последних деплойментов
async function loadRecentDeployments(namespace) {
    try {
        const deployments = clusterData.deployments || [];
        const recentDeployments = deployments
            .sort((a, b) => new Date(b.created || 0) - new Date(a.created || 0))
            .slice(0, 5);
        
        const deploymentsList = document.getElementById('recent-deployments');
        if (deploymentsList) {
            let html = '';
            recentDeployments.forEach(deployment => {
                const readyCount = deployment.ready_count || 0;
                const totalCount = deployment.total_count || 0;
                const statusColor = readyCount === totalCount ? 'success' : 'warning';
                
                html += `
                    <div class="list-group-item">
                        <div class="d-flex justify-content-between align-items-center">
                            <div>
                                <div class="fw-bold">${deployment.name}</div>
                                <small class="text-muted">Replicas: ${deployment.replicas || 1}</small>
                            </div>
                            <div class="text-end">
                                <span class="badge bg-${statusColor}">${readyCount}/${totalCount}</span>
                                <div class="small text-muted mt-1">${deployment.age}</div>
                            </div>
                        </div>
                    </div>
                `;
            });
            
            deploymentsList.innerHTML = html || `
                <div class="list-group-item">
                    <div class="text-center text-muted py-3">
                        No deployments found in ${namespace}
                    </div>
                </div>
            `;
        }
        
    } catch (error) {
        console.error('Error loading recent deployments:', error);
    }
}

// Загрузка сервисов
async function loadServices() {
    try {
        const services = metricsData.services || [];
        const servicesList = document.getElementById('services-list');
        
        if (servicesList) {
            let html = '';
            services.forEach(service => {
                html += `
                    <div class="list-group-item">
                        <div class="d-flex justify-content-between align-items-center">
                            <div>
                                <div class="fw-bold">${service.name}</div>
                                <small class="text-muted">Type: ${service.type}</small>
                            </div>
                            <div class="text-end">
                                <span class="badge bg-info">${service.clusterIP}</span>
                                <div class="small text-muted mt-1">Ports: ${service.ports?.join(', ') || 'None'}</div>
                            </div>
                        </div>
                    </div>
                `;
            });
            
            servicesList.innerHTML = html || `
                <div class="list-group-item">
                    <div class="text-center text-muted py-3">
                        No services found
                    </div>
                </div>
            `;
        }
    } catch (error) {
        console.error('Error loading services:', error);
    }
}

// Инициализация графика (данные подставятся при первом updateChartData)
function initializeChart() {
    const ctx = document.getElementById('resourceChart').getContext('2d');
    resourceChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [{
                label: 'CPU (%)',
                data: [],
                borderColor: '#dc3545',
                backgroundColor: 'rgba(220, 53, 69, 0.1)',
                tension: 0.4,
                fill: true
            }, {
                label: 'Memory (%)',
                data: [],
                borderColor: '#17a2b8',
                backgroundColor: 'rgba(23, 162, 184, 0.1)',
                tension: 0.4,
                fill: true
            }, {
                label: 'Pods',
                data: [],
                borderColor: '#28a745',
                backgroundColor: 'rgba(40, 167, 69, 0.1)',
                tension: 0.4,
                fill: false,
                yAxisID: 'yPods'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: { display: true, position: 'top' },
                tooltip: { mode: 'index', intersect: false }
            },
            scales: {
                y: {
                    type: 'linear',
                    display: true,
                    position: 'left',
                    beginAtZero: true,
                    max: 100,
                    ticks: { callback: function(v) { return v + '%'; } }
                },
                yPods: {
                    type: 'linear',
                    display: false,
                    position: 'right',
                    beginAtZero: true,
                    grid: { drawOnChartArea: false }
                }
            }
        }
    });
}

// Добавляет текущие метрики в историю графика (реальные данные с API или оценка по подам)
function pushChartHistory() {
    var cpu = null, memory = null;
    if (metricsData.clusterUsage && typeof metricsData.clusterUsage.cpu_percent === 'number')
        cpu = Math.round(metricsData.clusterUsage.cpu_percent);
    if (metricsData.clusterUsage && typeof metricsData.clusterUsage.memory_percent === 'number')
        memory = Math.round(metricsData.clusterUsage.memory_percent);
    var pods = (clusterData.pods && clusterData.pods.length) || 0;
    var deployments = (clusterData.deployments && clusterData.deployments.length) || 0;
    if (cpu === null) cpu = Math.min(100, Math.floor(pods * 8 + deployments * 5));
    if (memory === null) memory = Math.min(100, Math.floor(pods * 12 + deployments * 8));
    chartHistory.push({ t: Date.now(), cpu: cpu, memory: memory, pods: pods });
    if (chartHistory.length > MAX_CHART_POINTS) chartHistory.shift();
}

// Обновление данных графика из накопленной истории (актуальные метрики кластера)
function updateChartData() {
    if (!resourceChart) return;
    pushChartHistory();
    const metric = document.getElementById('metric-select').value;
    const labels = chartHistory.map(function(h) {
        var d = new Date(h.t);
        return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    });
    resourceChart.data.labels = labels;
    resourceChart.data.datasets[0].data = chartHistory.map(function(h) { return h.cpu; });
    resourceChart.data.datasets[1].data = chartHistory.map(function(h) { return h.memory; });
    resourceChart.data.datasets[2].data = chartHistory.map(function(h) { return h.pods; });
    resourceChart.data.datasets[0].hidden = metric !== 'cpu' && metric !== 'both';
    resourceChart.data.datasets[1].hidden = metric !== 'memory' && metric !== 'both';
    resourceChart.data.datasets[2].hidden = metric !== 'pods';
    var yAxis = resourceChart.options.scales.y;
    if (metric === 'pods') {
        var maxPods = Math.max(1, Math.max.apply(null, resourceChart.data.datasets[2].data));
        resourceChart.options.scales.yPods.display = true;
        yAxis.display = false;
        resourceChart.options.scales.yPods.max = Math.ceil(maxPods * 1.2);
    } else {
        resourceChart.options.scales.yPods.display = false;
        yAxis.display = true;
        yAxis.max = 100;
        yAxis.ticks.callback = function(v) { return v + '%'; };
    }
    resourceChart.update();
}

// Обновление графика при смене метрики
function updateChart() {
    updateChartData();
}


// Обновление статуса подключения
function updateConnectionStatus(connected) {
    const statusElement = document.getElementById('connection-status');
    if (!statusElement) return;
    if (connected) {
        statusElement.className = 'badge bg-success me-2';
        statusElement.innerHTML = '<i class="fas fa-plug"></i> Connected';
    } else {
        statusElement.className = 'badge bg-danger me-2';
        statusElement.innerHTML = '<i class="fas fa-plug"></i> Disconnected';
    }
}

// Обновление дашборда
function refreshDashboard() {
    loadDashboard();
    showToast('Dashboard refreshed', 'info');
}

// Экспорт дашборда с метриками
function exportDashboard() {
    const data = {
        timestamp: new Date().toISOString(),
        namespace: document.getElementById('namespace-selector').value || 'market',
        clusterData: {
            pods: clusterData.pods?.length || 0,
            deployments: clusterData.deployments?.length || 0,
            nodes: clusterData.nodes?.length || 0,
            namespaces: clusterData.namespaces?.length || 0,
            services: clusterData.services?.length || 0
        },
        metrics: {
            totalPods: metricsData.pods?.length || 0,
            clusterUsage: metricsData.clusterUsage || {},
            topCPUConsumers: metricsData.pods?.slice(0, 5).map(p => ({pod: p.pod, cpu: p.cpu_usage})) || [],
            topMemoryConsumers: metricsData.pods?.slice(0, 5).map(p => ({pod: p.pod, memory: p.memory_usage})) || []
        },
        status: {
            cluster: document.getElementById('cluster-status').textContent,
            pods: document.getElementById('pods-count').textContent,
            deployments: document.getElementById('deployments-count').textContent,
            cpuUsage: document.getElementById('cpu-usage').textContent,
            memoryUsage: document.getElementById('current-memory').textContent
        }
    };
    
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `dashboard-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    showToast('Dashboard data exported with metrics', 'success');
}

// Экспорт метрик в CSV
function exportMetricsToCSV() {
    if (!metricsData.pods || metricsData.pods.length === 0) {
        showToast('No metrics data to export', 'warning');
        return;
    }
    
    const headers = ['Pod', 'Namespace', 'CPU Usage', 'Memory Usage', 'CPU %', 'Memory %', 'CPU Limit', 'Memory Limit'];
    const rows = metricsData.pods.map(pod => [
        pod.pod,
        pod.namespace,
        pod.cpu_usage || 'N/A',
        pod.memory_usage || 'N/A',
        pod.cpu_percent || 0,
        pod.memory_percent || 0,
        pod.cpu_limit || 'N/A',
        pod.memory_limit || 'N/A'
    ]);
    
    const csvContent = [
        headers.join(','),
        ...rows.map(row => row.join(','))
    ].join('\n');
    
    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `k8s-metrics-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    showToast('Metrics exported to CSV', 'success');
}

// Фильтрация по namespace
function filterByNamespace(namespace) {
    document.getElementById('namespace-selector').value = namespace;
    refreshDashboard();
}

// Показать детали события
function showEventDetails(objectName) {
    showToast(`Showing details for: ${objectName}`, 'info');
    // В реальном приложении здесь будет открытие модального окна с деталями
}

// Функции для работы с подами
function viewPodLogs(namespace, podName) {
    window.open(`/ui/pods?logs=${namespace}/${podName}`, '_blank');
}

function viewPodYAML(namespace, podName) {
    window.open(`/api/pod/yaml/${namespace}/${podName}`, '_blank');
}

async function deletePod(namespace, podName) {
    if (!confirm(`Are you sure you want to delete pod ${podName} in namespace ${namespace}?`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/pod/${namespace}/${podName}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            showToast(`Pod ${podName} deleted successfully`, 'success');
            setTimeout(() => refreshDashboard(), 2000);
        } else {
            const error = await response.json();
            showToast(`Failed to delete pod: ${error.error}`, 'error');
        }
    } catch (error) {
        showToast(`Failed to delete pod: ${error.message}`, 'error');
    }
}

// Быстрое развертывание приложения
function deployQuickApp() {
    const modal = new bootstrap.Modal(document.getElementById('quickDeployModal'));
    modal.show();
}

async function confirmQuickDeploy() {
    const appType = document.getElementById('quick-app-type').value;
    const namespace = document.getElementById('quick-namespace').value || 'market';
    const replicas = parseInt(document.getElementById('quick-replicas').value) || 2;
    
    const apps = {
        nginx: { name: 'nginx-web', image: 'nginx:latest', port: 80 },
        redis: { name: 'redis-cache', image: 'redis:alpine', port: 6379 },
        busybox: { name: 'busybox-test', image: 'busybox:latest', port: 80 }
    };
    
    const app = apps[appType];
    if (!app) return;
    
    const deploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${app.name}
  namespace: ${namespace}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${app.name}
  template:
    metadata:
      labels:
        app: ${app.name}
    spec:
      containers:
      - name: ${app.name}
        image: ${app.image}
        ports:
        - containerPort: ${app.port}`;
    
    try {
        showToast(`Deploying ${app.name} to ${namespace}...`, 'info');
        bootstrap.Modal.getInstance(document.getElementById('quickDeployModal')).hide();
        
        const res = await fetch('/api/deployment', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml: deploymentYAML })
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || res.statusText);
        
        showToast(`${app.name} created in ${namespace}`, 'success');
        refreshDashboard();
    } catch (error) {
        showToast(`Deploy failed: ${error.message}`, 'error');
    }
}

// Масштабирование ресурсов
function scaleResources() {
    const modal = new bootstrap.Modal(document.getElementById('scaleModal'));
    modal.show();
}

async function confirmScaleAll() {
    const factor = parseFloat(document.getElementById('scale-factor').value);
    const scaleAll = document.getElementById('scale-all-namespaces').checked;
    const scaleDefault = document.getElementById('scale-default').checked;
    const scaleMarket = document.getElementById('scale-market').checked;
    
    const namespaces = [];
    if (scaleAll) {
        namespaces.push('all');
    } else {
        if (scaleDefault) namespaces.push('default');
        if (scaleMarket) namespaces.push('market');
    }
    
    if (namespaces.length === 0) {
        showToast('Please select at least one namespace', 'warning');
        return;
    }
    
    bootstrap.Modal.getInstance(document.getElementById('scaleModal')).hide();
    
    try {
        let deployments = [];
        if (namespaces.includes('all')) {
            const res = await fetch('/api/deployments?namespace=all');
            const data = await res.json();
            deployments = data.deployments || [];
        } else {
            for (const ns of namespaces) {
                const res = await fetch(`/api/deployments?namespace=${encodeURIComponent(ns)}`);
                const data = await res.json();
                deployments = deployments.concat(data.deployments || []);
            }
        }
        
        let done = 0, failed = 0;
        for (const d of deployments) {
            const ns = d.namespace, name = d.name;
            const current = d.replicas != null ? d.replicas : (d.total_count || 1);
            const newReplicas = Math.max(0, Math.round(current * factor));
            const res = await fetch(`/api/scale/${encodeURIComponent(ns)}/${encodeURIComponent(name)}?replicas=${newReplicas}`, { method: 'POST' });
            if (res.ok) done++; else failed++;
        }
        showToast(`Scaled: ${done} ok, ${failed} failed`, failed ? 'warning' : 'success');
        refreshDashboard();
    } catch (error) {
        showToast(`Scale failed: ${error.message}`, 'error');
    }
}

// Рестарт всех приложений
async function restartAll() {
    if (!confirm('Restart all deployments in current namespace? This will cause temporary downtime.')) {
        return;
    }
    const namespace = document.getElementById('namespace-selector').value || 'market';
    
    try {
        showToast('Restarting deployments...', 'info');
        const res = await fetch(`/api/deployments?namespace=${encodeURIComponent(namespace)}`);
        const data = await res.json();
        const deployments = data.deployments || [];
        let done = 0, failed = 0;
        for (const d of deployments) {
            const r = await fetch(`/api/restart/${encodeURIComponent(d.namespace)}/${encodeURIComponent(d.name)}`, { method: 'POST' });
            if (r.ok) done++; else failed++;
        }
        showToast(`Restart: ${done} ok, ${failed} failed`, failed ? 'warning' : 'success');
        refreshDashboard();
    } catch (error) {
        showToast(`Restart failed: ${error.message}`, 'error');
    }
}

// Очистка кластера (удаление failed/evicted подов в текущем namespace)
async function cleanupCluster() {
    if (!confirm('Delete failed/evicted pods in current namespace? This cannot be undone.')) {
        return;
    }
    const namespace = document.getElementById('namespace-selector').value || 'market';
    
    try {
        showToast('Cleaning up failed pods...', 'info');
        const res = await fetch(`/api/pods?namespace=${encodeURIComponent(namespace)}`);
        const data = await res.json();
        const pods = (data.pods || []).filter(p => (p.status === 'Failed' || p.status === 'Evicted' || p.status === 'Unknown'));
        let done = 0, failed = 0;
        for (const p of pods) {
            const r = await fetch(`/api/pod/${encodeURIComponent(p.namespace)}/${encodeURIComponent(p.name)}`, { method: 'DELETE' });
            if (r.ok) done++; else failed++;
        }
        showToast(`Deleted ${done} failed pod(s)`, done ? 'success' : 'info');
        refreshDashboard();
    } catch (error) {
        showToast(`Cleanup failed: ${error.message}`, 'error');
    }
}

// Вспомогательные функции
function showLoadingState() {
    // Показываем overlay с спиннером
    let overlay = document.getElementById('loading-overlay');
    if (!overlay) {
        overlay = document.createElement('div');
        overlay.id = 'loading-overlay';
        overlay.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            display: flex;
            justify-content: center;
            align-items: center;
            z-index: 99999;
        `;
        overlay.innerHTML = `
            <div class="spinner-border text-light" role="status">
                <span class="visually-hidden">Loading...</span>
            </div>
        `;
        document.body.appendChild(overlay);
    }
    overlay.style.display = 'flex';
}

function hideLoadingState() {
    const overlay = document.getElementById('loading-overlay');
    if (overlay) {
        overlay.style.display = 'none';
    }
}

function showError(message) {
    console.error(message);
    showToast(message, 'error');
}

// Очистка интервала при закрытии страницы
window.addEventListener('beforeunload', function() {
    clearInterval(autoRefreshInterval);
});

// Функция для отображения детальной информации о метриках
function showMetricsDetails() {
    if (metricsData.clusterUsage) {
        const metricsText = `
Cluster Metrics:
- CPU Usage: ${metricsData.clusterUsage.cpu_percent}%
- Memory Usage: ${metricsData.clusterUsage.memory_percent}%
- Total CPU Allocatable: ${metricsData.clusterUsage.total_cpu_allocatable}
- Total Memory Allocatable: ${metricsData.clusterUsage.total_memory_allocatable}
- Total CPU Used: ${metricsData.clusterUsage.total_cpu_used}
- Total Memory Used: ${metricsData.clusterUsage.total_memory_used}
`;
        alert(metricsText);
    } else {
        alert('Metrics data not available');
    }
}