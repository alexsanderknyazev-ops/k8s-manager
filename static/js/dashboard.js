// Переменные для хранения данных
let clusterData = {};
let resourceChart = null;
let autoRefreshInterval = null;
let isFirstLoad = true;
let metricsData = {};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadDashboard();
    setupEventListeners();
    setupAutoRefresh();
    initializeChart();
});

// Настройка слушателей событий
function setupEventListeners() {
    // Обработчик для обновления по F5
    document.addEventListener('keydown', function(e) {
        if (e.key === 'F5') {
            e.preventDefault();
            refreshDashboard();
        }
    });
    
    // Обработчик для смены namespace
    document.getElementById('namespace-selector').addEventListener('change', function() {
        refreshDashboard();
    });
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
    
    try {
        const namespace = document.getElementById('namespace-selector').value || 'market';
        
        // Загружаем все данные параллельно
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
            fetch(`/api/pods?namespace=${namespace}`).then(res => res.json()),
            fetch(`/api/deployments?namespace=${namespace}`).then(res => res.json()),
            fetch('/api/nodes').then(res => res.json()),
            fetch('/api/namespaces').then(res => res.json()),
            fetch('/api/test').then(res => res.json()),
            fetch(`/api/metrics/pods/${namespace}`).then(res => res.json()),
            fetch('/api/metrics/nodes').then(res => res.json()),
            fetch(`/api/services?namespace=${namespace}`).then(res => res.json())
        ]);
        
        // Сохраняем данные
        clusterData = {
            pods: podsData.pods || [],
            deployments: deploymentsData.deployments || [],
            nodes: nodesData.nodes || [],
            namespaces: namespacesData.namespaces || [],
            services: servicesData.services || [],
            clusterInfo: clusterInfo
        };
        
        metricsData = {
            pods: podsMetrics.metrics || [],
            nodes: nodesMetrics.nodes || [],
            clusterUsage: nodesMetrics.cluster_usage || {},
            services: servicesData.services || []
        };
        
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
    const totalNodes = clusterData.nodes.length;
    const readyNodes = clusterData.nodes.filter(node => 
        node.status && node.status.includes('Ready')
    ).length;
    
    const nodePercentage = totalNodes > 0 ? Math.round((readyNodes / totalNodes) * 100) : 0;
    const clusterStatus = nodePercentage >= 80 ? 'Healthy' : 
                         nodePercentage >= 50 ? 'Degraded' : 'Unhealthy';
    const statusColor = nodePercentage >= 80 ? 'success' : 
                       nodePercentage >= 50 ? 'warning' : 'danger';
    
    document.getElementById('cluster-status').textContent = clusterStatus;
    document.getElementById('cluster-status').className = `mb-0 text-${statusColor}`;
    document.getElementById('nodes-count').textContent = totalNodes;
    document.getElementById('nodes-progress').style.width = `${nodePercentage}%`;
    document.getElementById('nodes-badge').textContent = `${totalNodes} node${totalNodes !== 1 ? 's' : ''}`;
    
    // Обновляем статус подключения
    updateConnectionStatus(true);
}

// Обновление статуса подов
function updatePodsStatus() {
    const totalPods = clusterData.pods.length;
    const runningPods = clusterData.pods.filter(pod => pod.status === 'Running').length;
    const podsPercentage = totalPods > 0 ? Math.round((runningPods / totalPods) * 100) : 0;
    
    document.getElementById('pods-count').textContent = totalPods;
    document.getElementById('running-pods').textContent = runningPods;
    document.getElementById('pods-progress').style.width = `${podsPercentage}%`;
    document.getElementById('current-pods').textContent = totalPods;
    document.getElementById('current-pods-bar').style.width = `${Math.min(totalPods * 10, 100)}%`;
}

// Обновление статуса деплойментов
function updateDeploymentsStatus() {
    const totalDeployments = clusterData.deployments.length;
    const readyDeployments = clusterData.deployments.filter(deployment => {
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
    
    let html = '';
    const namespace = document.getElementById('namespace-selector').value || 'market';
    
    clusterData.pods.forEach(pod => {
        if (pod.namespace !== namespace) return;
        
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
    clusterData.nodes.forEach(node => {
        const isReady = node.status && node.status.includes('Ready');
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
        // Симуляция использования ресурсов (на случай если метрики недоступны)
        const totalPods = clusterData.pods.length;
        const totalDeployments = clusterData.deployments.length;
        
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
    // Информация о кластере
    if (clusterData.nodes && clusterData.nodes.length > 0) {
        const firstNode = clusterData.nodes[0];
        document.getElementById('k8s-version').textContent = firstNode.version || 'Unknown';
        document.getElementById('container-runtime').textContent = firstNode.containerd || 'Unknown';
        document.getElementById('os-info').textContent = firstNode.os || 'Unknown';
        document.getElementById('kernel-version').textContent = firstNode.kernel || 'Unknown';
    }
    
    // Namespaces
    const namespacesList = document.getElementById('namespaces-list');
    if (clusterData.namespaces && clusterData.namespaces.length > 0) {
        let html = '';
        clusterData.namespaces.forEach(ns => {
            const isActive = ns.status === 'Active';
            const statusColor = isActive ? 'success' : 'warning';
            const badgeClass = document.getElementById('namespace-selector').value === ns.name ? 'border border-primary' : '';
            
            html += `
                <span class="badge bg-${statusColor} namespace-badge ${badgeClass} me-1 mb-1" 
                      onclick="filterByNamespace('${ns.name}')" style="cursor: pointer;">
                    ${ns.name}
                    ${isActive ? '' : ' (inactive)'}
                </span>
            `;
        });
        namespacesList.innerHTML = html;
    }
    
    // Общие ресурсы
    if (metricsData.clusterUsage) {
        document.getElementById('total-cpu').textContent = metricsData.clusterUsage.total_cpu_allocatable || '--';
        document.getElementById('total-memory').textContent = metricsData.clusterUsage.total_memory_allocatable || '--';
    } else {
        document.getElementById('total-cpu').textContent = '4 Cores';
        document.getElementById('total-memory').textContent = '8 GB';
    }
}

// Загрузка событий
async function loadEvents(namespace) {
    try {
        // В реальном приложении здесь будет вызов API для событий
        // Пока используем симуляцию
        simulateEvents(namespace);
        
    } catch (error) {
        console.error('Error loading events:', error);
    }
}

// Симуляция событий (временная реализация)
function simulateEvents(namespace) {
    const events = [
        { type: 'Normal', object: 'market-app', message: 'Successfully assigned pod to node', time: '2m ago' },
        { type: 'Normal', object: 'postgres', message: 'Container image already present', time: '5m ago' },
        { type: 'Warning', object: 'user-service', message: 'Back-off restarting failed container', time: '10m ago' },
        { type: 'Normal', object: 'nginx', message: 'Created container nginx', time: '15m ago' },
        { type: 'Normal', object: 'redis', message: 'Started container redis', time: '20m ago' }
    ];
    
    const tbody = document.getElementById('events-table');
    let html = '';
    
    events.forEach(event => {
        const typeClass = event.type === 'Normal' ? 'event-normal' : 
                         event.type === 'Warning' ? 'event-warning' : 'event-error';
        const typeColor = event.type === 'Normal' ? 'success' : 
                         event.type === 'Warning' ? 'warning' : 'danger';
        
        html += `
            <tr class="${typeClass}" onclick="showEventDetails('${event.object}')" style="cursor: pointer;">
                <td>
                    <span class="badge bg-${typeColor}">${event.type}</span>
                </td>
                <td><small>${event.object}</small></td>
                <td><small>${event.message}</small></td>
                <td><small class="text-muted">${event.time}</small></td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
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

// Симуляция топ потребителей (если метрики недоступны)
function simulateTopConsumers() {
    // Топ CPU
    const cpuConsumers = [
        { name: 'market-app', namespace: 'market', usage: '320m', percent: 65 },
        { name: 'postgres', namespace: 'market', usage: '280m', percent: 56 },
        { name: 'user-service', namespace: 'market', usage: '210m', percent: 42 },
        { name: 'redis', namespace: 'default', usage: '150m', percent: 30 },
        { name: 'nginx', namespace: 'default', usage: '80m', percent: 16 }
    ];
    
    // Топ памяти
    const memoryConsumers = [
        { name: 'postgres', namespace: 'market', usage: '512Mi', percent: 64 },
        { name: 'market-app', namespace: 'market', usage: '256Mi', percent: 32 },
        { name: 'user-service', namespace: 'market', usage: '128Mi', percent: 16 },
        { name: 'redis', namespace: 'default', usage: '64Mi', percent: 8 },
        { name: 'nginx', namespace: 'default', usage: '32Mi', percent: 4 }
    ];
    
    // Обновляем топ CPU
    const cpuList = document.getElementById('top-cpu-consumers');
    if (cpuList) {
        let html = '';
        cpuConsumers.forEach((pod, index) => {
            html += `
                <div class="list-group-item">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <div class="fw-bold">${pod.name}</div>
                            <small class="text-muted">Namespace: ${pod.namespace}</small>
                        </div>
                        <div class="text-end">
                            <span class="badge bg-danger">${pod.usage}</span>
                            <div class="progress mt-1" style="height: 3px; width: 60px;">
                                <div class="progress-bar bg-danger" style="width: ${pod.percent}%"></div>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        });
        cpuList.innerHTML = html;
    }
    
    // Обновляем топ памяти
    const memoryList = document.getElementById('top-memory-consumers');
    if (memoryList) {
        let html = '';
        memoryConsumers.forEach((pod, index) => {
            html += `
                <div class="list-group-item">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <div class="fw-bold">${pod.name}</div>
                            <small class="text-muted">Namespace: ${pod.namespace}</small>
                        </div>
                        <div class="text-end">
                            <span class="badge bg-info">${pod.usage}</span>
                            <div class="progress mt-1" style="height: 3px; width: 60px;">
                                <div class="progress-bar bg-info" style="width: ${pod.percent}%"></div>
                            </div>
                        </div>
                    </div>
                </div>
            `;
        });
        memoryList.innerHTML = html;
    }
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

// Инициализация графика
function initializeChart() {
    const ctx = document.getElementById('resourceChart').getContext('2d');
    
    resourceChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: generateTimeLabels(),
            datasets: [{
                label: 'CPU Usage (%)',
                data: generateRandomData(50, 80),
                borderColor: '#dc3545',
                backgroundColor: 'rgba(220, 53, 69, 0.1)',
                tension: 0.4,
                fill: true
            }, {
                label: 'Memory Usage (%)',
                data: generateRandomData(40, 70),
                borderColor: '#17a2b8',
                backgroundColor: 'rgba(23, 162, 184, 0.1)',
                tension: 0.4,
                fill: true
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: true,
                    position: 'top'
                },
                tooltip: {
                    mode: 'index',
                    intersect: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100,
                    ticks: {
                        callback: function(value) {
                            return value + '%';
                        }
                    }
                }
            }
        }
    });
}

// Обновление данных графика
function updateChartData() {
    if (!resourceChart) return;
    
    const metric = document.getElementById('metric-select').value;
    
    // Обновляем только первый датасет
    if (metric === 'cpu') {
        resourceChart.data.datasets[0].label = 'CPU Usage (%)';
        resourceChart.data.datasets[0].borderColor = '#dc3545';
        resourceChart.data.datasets[0].backgroundColor = 'rgba(220, 53, 69, 0.1)';
        resourceChart.data.datasets[0].data = generateRandomData(50, 80);
        resourceChart.data.datasets[1].hidden = true;
    } else if (metric === 'memory') {
        resourceChart.data.datasets[0].label = 'Memory Usage (%)';
        resourceChart.data.datasets[0].borderColor = '#17a2b8';
        resourceChart.data.datasets[0].backgroundColor = 'rgba(23, 162, 184, 0.1)';
        resourceChart.data.datasets[0].data = generateRandomData(40, 70);
        resourceChart.data.datasets[1].hidden = true;
    } else if (metric === 'pods') {
        resourceChart.data.datasets[0].label = 'Pods Count';
        resourceChart.data.datasets[0].borderColor = '#28a745';
        resourceChart.data.datasets[0].backgroundColor = 'rgba(40, 167, 69, 0.1)';
        resourceChart.data.datasets[0].data = generateRandomData(5, 20, false);
        resourceChart.data.datasets[1].hidden = true;
        
        resourceChart.options.scales.y.max = Math.max(...resourceChart.data.datasets[0].data) * 1.2;
        resourceChart.options.scales.y.ticks.callback = function(value) {
            return Math.round(value);
        };
    } else {
        // Показываем оба графика
        resourceChart.data.datasets[0].label = 'CPU Usage (%)';
        resourceChart.data.datasets[0].borderColor = '#dc3545';
        resourceChart.data.datasets[0].backgroundColor = 'rgba(220, 53, 69, 0.1)';
        resourceChart.data.datasets[0].data = generateRandomData(50, 80);
        resourceChart.data.datasets[0].hidden = false;
        
        resourceChart.data.datasets[1].label = 'Memory Usage (%)';
        resourceChart.data.datasets[1].borderColor = '#17a2b8';
        resourceChart.data.datasets[1].backgroundColor = 'rgba(23, 162, 184, 0.1)';
        resourceChart.data.datasets[1].data = generateRandomData(40, 70);
        resourceChart.data.datasets[1].hidden = false;
        
        resourceChart.options.scales.y.max = 100;
        resourceChart.options.scales.y.ticks.callback = function(value) {
            return value + '%';
        };
    }
    
    resourceChart.update();
}

// Обновление графика при смене метрики
function updateChart() {
    updateChartData();
}

// Генерация меток времени
function generateTimeLabels() {
    const labels = [];
    const now = new Date();
    
    for (let i = 23; i >= 0; i--) {
        const time = new Date(now.getTime() - i * 60 * 60 * 1000);
        labels.push(time.getHours().toString().padStart(2, '0') + ':00');
    }
    
    return labels;
}

// Генерация случайных данных
function generateRandomData(min, max, isPercentage = true) {
    const data = [];
    let lastValue = (min + max) / 2;
    
    for (let i = 0; i < 24; i++) {
        // Добавляем некоторую случайность, но сохраняем плавность
        const change = (Math.random() - 0.5) * (max - min) * 0.1;
        lastValue = Math.max(min, Math.min(max, lastValue + change));
        data.push(isPercentage ? Math.round(lastValue) : Math.round(lastValue));
    }
    
    return data;
}

// Обновление статуса подключения
function updateConnectionStatus(connected) {
    const statusElement = document.getElementById('connection-status');
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
    
    try {
        // Создаем простой deployment YAML
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
        
        // В реальном приложении здесь будет вызов API для создания deployment
        console.log('Creating deployment:', deploymentYAML);
        
        showToast(`Deploying ${app.name} to ${namespace}...`, 'info');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('quickDeployModal')).hide();
        
        // Обновляем дашборд через 3 секунды
        setTimeout(() => refreshDashboard(), 3000);
        
    } catch (error) {
        showToast(`Failed to deploy: ${error.message}`, 'error');
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
    
    try {
        showToast(`Scaling deployments in ${namespaces.join(', ')} by ${factor}x`, 'info');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('scaleModal')).hide();
        
        // Обновляем дашборд
        setTimeout(() => refreshDashboard(), 2000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    }
}

// Рестарт всех приложений
async function restartAll() {
    if (!confirm('Restart all deployments? This will cause temporary downtime.')) {
        return;
    }
    
    try {
        showToast('Restarting all deployments...', 'info');
        
        // Пока просто обновляем дашборд
        setTimeout(() => refreshDashboard(), 3000);
        
    } catch (error) {
        showToast(`Failed to restart: ${error.message}`, 'error');
    }
}

// Очистка кластера
async function cleanupCluster() {
    if (!confirm('Clean up failed pods and completed jobs? This action cannot be undone.')) {
        return;
    }
    
    try {
        showToast('Cleaning up cluster resources...', 'info');
        
        // Пока просто обновляем дашборд
        setTimeout(() => refreshDashboard(), 2000);
        
    } catch (error) {
        showToast(`Failed to cleanup: ${error.message}`, 'error');
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

function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast-alert alert alert-${type} alert-dismissible fade show`;
    toast.style.cssText = `
        position: fixed;
        top: 20px;
        right: 20px;
        z-index: 9999;
        min-width: 300px;
        animation: slideIn 0.3s ease-out;
    `;
    toast.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(toast);
    
    // Добавляем CSS для анимации
    const style = document.createElement('style');
    style.textContent = `
        @keyframes slideIn {
            from {
                transform: translateX(100%);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }
    `;
    document.head.appendChild(style);
    
    setTimeout(() => {
        toast.remove();
        style.remove();
    }, 3000);
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