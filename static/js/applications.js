// Конфигурация
const CONFIG = {
    refreshInterval: 30000,
    apiTimeout: 10000
};

// Состояние приложения
const AppState = {
    applications: [],
    filteredApplications: [],
    currentNamespace: 'all',
    currentView: 'grid',
    sortBy: 'name',
    sortOrder: 'asc',
    clusterMetrics: {},
    refreshIntervalId: null,
    isLoading: false
};

// Инициализация при загрузке
document.addEventListener('DOMContentLoaded', function() {
    console.log('🚀 Initializing Applications Dashboard...');
    
    initEventListeners();
    loadInitialData();
    setupAutoRefresh();
    
    console.log('✅ Dashboard initialized');
});

// Инициализация обработчиков событий
function initEventListeners() {
    // Навигация
    document.getElementById('namespace-select').addEventListener('change', handleNamespaceChange);
    document.getElementById('search-apps').addEventListener('input', debounce(handleSearch, 300));
    document.getElementById('refresh-btn').addEventListener('click', handleRefresh);
    
    // Переключение вида
    document.querySelectorAll('input[name="view-mode"]').forEach(radio => {
        radio.addEventListener('change', (e) => {
            AppState.currentView = e.target.value;
            renderApplications();
        });
    });
    
    // Сортировка
    document.querySelectorAll('.sort-option').forEach(option => {
        option.addEventListener('click', (e) => {
            e.preventDefault();
            const sortBy = e.target.dataset.sort;
            handleSort(sortBy);
        });
    });
    
    // Автообновление
    document.getElementById('refresh-interval').addEventListener('change', handleAutoRefreshChange);
    
    // Статус фильтры
    document.querySelectorAll('input[name="status-filter"]').forEach(checkbox => {
        checkbox.addEventListener('change', () => {
            filterApplications();
            renderApplications();
        });
    });
}

// Загрузка данных
async function loadInitialData() {
    if (AppState.isLoading) return;
    
    AppState.isLoading = true;
    showLoading(true);
    
    try {
        await Promise.all([
            loadApplications(),
            loadClusterMetrics()
        ]);
        
        // Загружаем метрики для приложений
        await loadApplicationsMetrics();
        
        updateDashboard();
        updateLastUpdated();
        
        showToast('Data loaded successfully', 'success');
        
    } catch (error) {
        console.error('Error loading data:', error);
        showError('Failed to load data: ' + error.message);
    } finally {
        AppState.isLoading = false;
        showLoading(false);
    }
}

// Загрузка приложений
async function loadApplications() {
    try {
        const namespace = AppState.currentNamespace === 'all' ? '' : `?namespace=${AppState.currentNamespace}`;
        console.log('Loading applications from:', `/api/applications${namespace}`);
        
        const response = await fetchWithTimeout(`/api/applications${namespace}`, {
            timeout: CONFIG.apiTimeout
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const data = await response.json();
        console.log('Applications data:', data);
        
        AppState.applications = data.applications || [];
        
        // Добавляем начальные метрики (демо)
        AppState.applications.forEach(app => {
            if (!app.metrics) {
                app.metrics = {
                    cpu: Math.floor(Math.random() * 200) + 50,
                    memory: Math.floor(Math.random() * 512) + 128,
                    cpuPercent: Math.floor(Math.random() * 60) + 10,
                    memoryPercent: Math.floor(Math.random() * 70) + 15,
                    cpuUsage: `${Math.floor(Math.random() * 200) + 50}m`,
                    memoryUsage: `${Math.floor(Math.random() * 512) + 128}Mi`
                };
            }
        });
        
        filterApplications();
        
        console.log(`✅ Loaded ${AppState.applications.length} applications`);
        
    } catch (error) {
        console.error('Error loading applications:', error);
        // Fallback на демо-данные
        AppState.applications = getDemoApplications();
        filterApplications();
        showToast('Using demo data - API not available', 'warning');
    }
}

// Загрузка метрик для приложений
async function loadApplicationsMetrics() {
    try {
        if (AppState.currentNamespace !== 'all') {
            await loadMetricsForNamespace(AppState.currentNamespace);
        } else {
            // Для всех namespace получаем метрики из общего API
            const response = await fetchWithTimeout('/api/metrics/all-pods', {
                timeout: CONFIG.apiTimeout
            });
            
            if (response.ok) {
                const data = await response.json();
                console.log('All pods metrics:', data);
                
                // Создаем map для быстрого поиска
                const metricsMap = {};
                (data.all_metrics || []).forEach(metric => {
                    metricsMap[metric.pod] = metric;
                });
                
                // Обновляем метрики приложений
                AppState.applications.forEach(app => {
                    if (metricsMap[app.name]) {
                        const metric = metricsMap[app.name];
                        app.metrics = {
                            cpu: metric.cpu_raw || 0,
                            memory: metric.memory_raw || 0,
                            cpuPercent: metric.cpu_percent || 0,
                            memoryPercent: metric.memory_percent || 0,
                            cpuUsage: metric.cpu || '0m',
                            memoryUsage: metric.memory || '0Mi'
                        };
                    }
                });
            }
        }
    } catch (error) {
        console.warn('Could not load applications metrics:', error);
    }
}

// Загрузка метрик кластера
async function loadClusterMetrics() {
    try {
        console.log('Loading cluster metrics...');
        const response = await fetchWithTimeout('/api/metrics/nodes', {
            timeout: CONFIG.apiTimeout
        });
        
        if (response.ok) {
            const data = await response.json();
            console.log('Cluster metrics data:', data);
            AppState.clusterMetrics = data.cluster_usage || {};
        } else {
            AppState.clusterMetrics = getDemoClusterMetrics();
        }
    } catch (error) {
        console.warn('Could not load cluster metrics:', error);
        AppState.clusterMetrics = getDemoClusterMetrics();
    }
}

// Фильтрация и сортировка
function filterApplications() {
    const searchTerm = document.getElementById('search-apps').value.toLowerCase();
    
    AppState.filteredApplications = AppState.applications.filter(app => {
        // Поиск
        if (searchTerm && !app.name.toLowerCase().includes(searchTerm) && 
            !app.namespace.toLowerCase().includes(searchTerm)) {
            return false;
        }
        
        // Фильтры статуса
        const statusFilters = Array.from(document.querySelectorAll('input[name="status-filter"]:checked'))
            .map(cb => cb.value);
        
        if (statusFilters.length > 0) {
            const status = getAppStatus(app);
            if (!statusFilters.includes(status)) {
                return false;
            }
        }
        
        return true;
    });
    
    sortApplications();
}

function sortApplications() {
    AppState.filteredApplications.sort((a, b) => {
        let aVal, bVal;
        
        switch (AppState.sortBy) {
            case 'name':
                aVal = a.name.toLowerCase();
                bVal = b.name.toLowerCase();
                break;
            case 'namespace':
                aVal = a.namespace.toLowerCase();
                bVal = b.namespace.toLowerCase();
                break;
            case 'cpu':
                aVal = a.metrics?.cpu || 0;
                bVal = b.metrics?.cpu || 0;
                break;
            case 'memory':
                aVal = a.metrics?.memory || 0;
                bVal = b.metrics?.memory || 0;
                break;
            case 'pods':
                aVal = a.total_count || 0;
                bVal = b.total_count || 0;
                break;
            default:
                aVal = a.name.toLowerCase();
                bVal = b.name.toLowerCase();
        }
        
        return AppState.sortOrder === 'asc' ? 
            (aVal > bVal ? 1 : -1) : 
            (aVal < bVal ? 1 : -1);
    });
}

// Рендеринг
function renderApplications() {
    if (AppState.currentView === 'grid') {
        renderGrid();
    } else {
        renderTable();
    }
    
    updateApplicationsCount();
}

function renderGrid() {
    const container = document.getElementById('applications-grid');
    const tableView = document.getElementById('applications-table');
    
    if (!container) return;
    
    container.style.display = 'flex';
    container.classList.add('row');
    if (tableView) tableView.style.display = 'none';
    
    if (AppState.filteredApplications.length === 0) {
        container.innerHTML = createEmptyState();
        return;
    }
    
    let html = '';
    AppState.filteredApplications.forEach(app => {
        html += createAppCard(app);
    });
    
    container.innerHTML = html;
    
    // Добавляем обработчики для карточек
    setTimeout(() => {
        document.querySelectorAll('.app-card').forEach(card => {
            const appName = card.dataset.appName;
            const namespace = card.dataset.namespace;
            
            card.addEventListener('click', (e) => {
                if (!e.target.closest('.btn-action')) {
                    showAppDetails(appName, namespace);
                }
            });
        });
        
        // Кнопки действий
        document.querySelectorAll('.btn-action').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                const action = btn.dataset.action;
                const appName = btn.dataset.appName;
                const namespace = btn.dataset.namespace;
                
                if (action && appName && namespace) {
                    handleAppAction(action, appName, namespace, btn.dataset.replicas);
                }
            });
        });
    }, 100);
}

function renderTable() {
    const container = document.getElementById('applications-grid');
    const tableView = document.getElementById('applications-table');
    
    if (!container || !tableView) return;
    
    container.style.display = 'none';
    tableView.style.display = 'block';
    
    const tbody = document.getElementById('applications-table-body');
    if (!tbody) return;
    
    if (AppState.filteredApplications.length === 0) {
        tbody.innerHTML = createEmptyTableState();
        return;
    }
    
    let html = '';
    AppState.filteredApplications.forEach(app => {
        html += createTableRow(app);
    });
    
    tbody.innerHTML = html;
    
    // Добавляем обработчики для строк таблицы
    setTimeout(() => {
        document.querySelectorAll('.app-row').forEach(row => {
            const appName = row.dataset.appName;
            const namespace = row.dataset.namespace;
            
            row.addEventListener('click', (e) => {
                if (!e.target.closest('.btn-action') && !e.target.closest('td:last-child')) {
                    showAppDetails(appName, namespace);
                }
            });
        });
        
        // Кнопки действий в таблице
        document.querySelectorAll('.table .btn-action').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                const action = btn.dataset.action;
                const appName = btn.dataset.appName;
                const namespace = btn.dataset.namespace;
                
                if (action && appName && namespace) {
                    handleAppAction(action, appName, namespace, btn.dataset.replicas);
                }
            });
        });
    }, 100);
}

// Создание карточки приложения
function createAppCard(app) {
    const status = getAppStatus(app);
    const metrics = app.metrics || {};
    const cpuUsage = metrics.cpuUsage || `${metrics.cpu || 0}m`;
    const memoryUsage = metrics.memoryUsage || formatBytes(metrics.memory || 0);
    const cpuPercent = metrics.cpuPercent || Math.min(100, (metrics.cpu || 0) / 2);
    const memoryPercent = metrics.memoryPercent || Math.min(100, (metrics.memory || 0) / (1024 * 1024 * 100));
    
    return `
        <div class="col-xl-3 col-lg-4 col-md-6 mb-4">
            <div class="card app-card ${status}" 
                 data-app-name="${app.name}" 
                 data-namespace="${app.namespace}">
                <div class="card-body">
                    <div class="d-flex align-items-start mb-3">
                        <div class="app-icon ${app.type?.toLowerCase() || 'deployment'}">
                            <i class="fas ${getAppIcon(app.type)}"></i>
                        </div>
                        <div class="flex-grow-1">
                            <div class="d-flex justify-content-between align-items-start">
                                <h5 class="card-title mb-1" title="${app.name}">
                                    ${truncateText(app.name, 20)}
                                </h5>
                                <span class="app-status status-${status}">${status}</span>
                            </div>
                            <div class="d-flex align-items-center">
                                <small class="text-muted">
                                    <i class="fas fa-layer-group me-1"></i>${app.type || 'Deployment'}
                                </small>
                                <span class="mx-2 text-muted">•</span>
                                <small class="text-muted">
                                    <i class="fas fa-cube me-1"></i>${app.namespace}
                                </small>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Pod status -->
                    <div class="mb-3">
                        <div class="d-flex justify-content-between mb-1">
                            <small class="text-muted">Pods</small>
                            <small class="text-muted">${app.ready_count || 0}/${app.total_count || 0}</small>
                        </div>
                        <div class="resource-progress">
                            <div class="progress-bar bg-${getStatusColor(status)}" 
                                 style="width: ${app.total_count ? ((app.ready_count || 0) / app.total_count * 100) : 0}%">
                            </div>
                        </div>
                    </div>
                    
                    <!-- Resource metrics -->
                    <div class="mb-3">
                        <div class="resource-metric">
                            <div class="resource-label">
                                <span>CPU</span>
                                <span>${cpuUsage}</span>
                            </div>
                            <div class="resource-progress">
                                <div class="progress-bar bg-success" style="width: ${cpuPercent}%"></div>
                            </div>
                        </div>
                        
                        <div class="resource-metric">
                            <div class="resource-label">
                                <span>Memory</span>
                                <span>${memoryUsage}</span>
                            </div>
                            <div class="resource-progress">
                                <div class="progress-bar bg-info" style="width: ${memoryPercent}%"></div>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Actions -->
                    <div class="d-flex justify-content-between">
                        <button class="btn btn-sm btn-outline-primary btn-action" 
                                data-action="metrics" 
                                data-app-name="${app.name}" 
                                data-namespace="${app.namespace}"
                                title="View metrics">
                            <i class="fas fa-chart-line"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-warning btn-action" 
                                data-action="restart" 
                                data-app-name="${app.name}" 
                                data-namespace="${app.namespace}"
                                title="Restart">
                            <i class="fas fa-redo"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-success btn-action" 
                                data-action="scale" 
                                data-app-name="${app.name}" 
                                data-namespace="${app.namespace}"
                                data-replicas="${app.total_count || 1}"
                                title="Scale">
                            <i class="fas fa-expand-alt"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger btn-action" 
                                data-action="delete" 
                                data-app-name="${app.name}" 
                                data-namespace="${app.namespace}"
                                title="Delete">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `;
}

// Создание строки таблицы
function createTableRow(app) {
    const status = getAppStatus(app);
    const metrics = app.metrics || {};
    const cpuUsage = metrics.cpuUsage || `${metrics.cpu || 0}m`;
    const memoryUsage = metrics.memoryUsage || formatBytes(metrics.memory || 0);
    
    return `
        <tr class="app-row" data-app-name="${app.name}" data-namespace="${app.namespace}">
            <td>
                <div class="d-flex align-items-center">
                    <span class="status-indicator ${status}"></span>
                    <div>
                        <strong>${app.name}</strong>
                        <small class="text-muted d-block">${app.namespace}</small>
                    </div>
                </div>
            </td>
            <td>
                <span class="badge bg-secondary">${app.type || 'Deployment'}</span>
            </td>
            <td>
                <span class="badge bg-${getStatusColor(status)}">${status}</span>
            </td>
            <td>
                <div class="d-flex align-items-center">
                    <div class="progress flex-grow-1 me-2" style="height: 6px;">
                        <div class="progress-bar bg-${getStatusColor(status)}" 
                             style="width: ${app.total_count ? ((app.ready_count || 0) / app.total_count * 100) : 0}%">
                        </div>
                    </div>
                    <small>${app.ready_count || 0}/${app.total_count || 0}</small>
                </div>
            </td>
            <td>
                <div class="metrics-bar">
                    <div class="metric-bar">
                        <div class="metric-bar-fill cpu" 
                             style="width: ${Math.min(100, (metrics.cpu || 0) / 2)}%"></div>
                    </div>
                    <span class="metric-value">${cpuUsage}</span>
                </div>
            </td>
            <td>
                <div class="metrics-bar">
                    <div class="metric-bar">
                        <div class="metric-bar-fill memory" 
                             style="width: ${Math.min(100, (metrics.memory || 0) / (1024 * 1024 * 2))}%"></div>
                    </div>
                    <span class="metric-value">${memoryUsage}</span>
                </div>
            </td>
            <td>
                <small class="text-muted">${formatAge(app.age)}</small>
            </td>
            <td>
                <div class="btn-group btn-group-sm">
                    <button class="btn btn-outline-primary btn-action" 
                            data-action="metrics" 
                            data-app-name="${app.name}" 
                            data-namespace="${app.namespace}"
                            title="Metrics">
                        <i class="fas fa-chart-line"></i>
                    </button>
                    <button class="btn btn-outline-success btn-action" 
                            data-action="scale" 
                            data-app-name="${app.name}" 
                            data-namespace="${app.namespace}"
                            data-replicas="${app.total_count || 1}"
                            title="Scale">
                        <i class="fas fa-expand-alt"></i>
                    </button>
                    <button class="btn btn-outline-danger btn-action" 
                            data-action="delete" 
                            data-app-name="${app.name}" 
                            data-namespace="${app.namespace}"
                            title="Delete">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </td>
        </tr>
    `;
}

// Обновление дашборда
function updateDashboard() {
    updateMetricsDashboard();
    renderApplications();
    updateClusterStats();
}

function updateMetricsDashboard() {
    const metrics = AppState.clusterMetrics;
    
    console.log('Updating metrics dashboard with:', metrics);
    
    // CPU
    const cpuPercent = metrics.cpu_percent || 0;
    document.getElementById('cpu-usage-value').textContent = `${cpuPercent}%`;
    document.getElementById('cpu-progress').style.width = `${cpuPercent}%`;
    document.getElementById('cpu-usage-detail').textContent = 
        `${metrics.total_cpu_used || '0m'} / ${metrics.total_cpu_allocatable || '0m'}`;
    
    // Memory
    const memoryPercent = metrics.memory_percent || 0;
    document.getElementById('memory-usage-value').textContent = `${memoryPercent}%`;
    document.getElementById('memory-progress').style.width = `${memoryPercent}%`;
    document.getElementById('memory-usage-detail').textContent = 
        `${metrics.total_memory_used || '0Gi'} / ${metrics.total_memory_allocatable || '0Gi'}`;
    
    // Pods
    const totalPods = AppState.applications.reduce((sum, app) => sum + (app.total_count || 0), 0);
    const readyPods = AppState.applications.reduce((sum, app) => sum + (app.ready_count || 0), 0);
    
    document.getElementById('pod-status-value').textContent = `${readyPods}/${totalPods}`;
    document.getElementById('ready-pods').textContent = readyPods;
    document.getElementById('pending-pods').textContent = Math.max(0, totalPods - readyPods - Math.floor(totalPods * 0.1));
    document.getElementById('failed-pods').textContent = Math.max(0, totalPods - readyPods - Math.floor(totalPods * 0.2));
    
    // Applications count
    document.getElementById('apps-count').textContent = AppState.applications.length;
    
    // Update mini charts
    updateMiniCharts();
}

function updateClusterStats() {
    const metrics = AppState.clusterMetrics;
    
    document.getElementById('stats-count').textContent = AppState.applications.length;
    document.getElementById('stats-pods').textContent = 
        AppState.applications.reduce((sum, app) => sum + (app.total_count || 0), 0);
    document.getElementById('stats-nodes').textContent = metrics.node_count || '?';
}

function updateMiniCharts() {
    // CPU sparkline
    updateSparkline('cpu-chart-small', AppState.clusterMetrics.cpu_percent || 0, '#28a745');
    
    // Memory sparkline
    updateSparkline('memory-chart-small', AppState.clusterMetrics.memory_percent || 0, '#17a2b8');
    
    // Pod status donut
    updatePodDonut();
}

function updateSparkline(elementId, value, color) {
    const container = document.getElementById(elementId);
    if (!container) return;
    
    const canvas = document.createElement('canvas');
    canvas.width = 60;
    canvas.height = 30;
    const ctx = canvas.getContext('2d');
    
    // Простой график
    ctx.beginPath();
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.moveTo(5, 20);
    ctx.lineTo(25, 15);
    ctx.lineTo(45, 10);
    ctx.lineTo(55, 5);
    ctx.stroke();
    
    container.innerHTML = '';
    container.appendChild(canvas);
}

function updatePodDonut() {
    const container = document.getElementById('pod-chart-small');
    if (!container) return;
    
    const totalPods = AppState.applications.reduce((sum, app) => sum + (app.total_count || 0), 0);
    const readyPods = AppState.applications.reduce((sum, app) => sum + (app.ready_count || 0), 0);
    const readyPercent = totalPods ? (readyPods / totalPods * 100) : 0;
    
    const canvas = document.createElement('canvas');
    canvas.width = 50;
    canvas.height = 50;
    const ctx = canvas.getContext('2d');
    
    // Фон
    ctx.beginPath();
    ctx.arc(25, 25, 20, 0, Math.PI * 2);
    ctx.fillStyle = '#e9ecef';
    ctx.fill();
    
    // Прогресс
    const startAngle = -Math.PI / 2;
    const endAngle = startAngle + (Math.PI * 2 * readyPercent / 100);
    
    ctx.beginPath();
    ctx.arc(25, 25, 20, startAngle, endAngle);
    ctx.lineTo(25, 25);
    ctx.closePath();
    ctx.fillStyle = readyPercent >= 90 ? '#28a745' : 
                    readyPercent >= 70 ? '#ffc107' : '#dc3545';
    ctx.fill();
    
    // Текст
    ctx.fillStyle = '#495057';
    ctx.font = 'bold 10px Arial';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(`${Math.round(readyPercent)}%`, 25, 25);
    
    container.innerHTML = '';
    container.appendChild(canvas);
}

// Обработчики событий
function handleNamespaceChange(e) {
    AppState.currentNamespace = e.target.value;
    document.getElementById('current-namespace').textContent = 
        AppState.currentNamespace === 'all' ? 'all' : AppState.currentNamespace;
    loadInitialData();
}

function handleSearch() {
    filterApplications();
    renderApplications();
}

function handleRefresh() {
    loadInitialData();
}

function handleSort(sortBy) {
    if (AppState.sortBy === sortBy) {
        AppState.sortOrder = AppState.sortOrder === 'asc' ? 'desc' : 'asc';
    } else {
        AppState.sortBy = sortBy;
        AppState.sortOrder = 'asc';
    }
    
    sortApplications();
    renderApplications();
    updateSortIndicator();
}

function handleAutoRefreshChange(e) {
    const interval = parseInt(e.target.value) * 1000;
    setupAutoRefresh(interval);
    
    const statusElement = document.getElementById('auto-refresh-status');
    if (interval > 0) {
        statusElement.innerHTML = `<i class="fas fa-sync-alt"></i> Auto: ${e.target.value}s`;
        statusElement.className = 'badge bg-success ms-2';
    } else {
        statusElement.innerHTML = `<i class="fas fa-sync-alt"></i> Auto: Off`;
        statusElement.className = 'badge bg-secondary ms-2';
    }
}

function handleAppAction(action, appName, namespace, replicas) {
    switch (action) {
        case 'metrics':
            showAppMetrics(appName, namespace);
            break;
        case 'scale':
            showScaleModal(appName, namespace, replicas);
            break;
        case 'restart':
            restartApp(appName, namespace);
            break;
        case 'delete':
            deleteApp(appName, namespace);
            break;
    }
}

// Действия с приложениями
async function showAppMetrics(appName, namespace) {
    try {
        console.log(`Loading metrics for ${namespace}/${appName}`);
        const response = await fetch(`/api/metrics/pod/${namespace}/${appName}`);
        if (response.ok) {
            const data = await response.json();
            console.log('App metrics data:', data);
            showAppMetricsModal(appName, namespace, data);
        } else {
            // Пробуем загрузить из общего списка
            const allResponse = await fetch('/api/metrics/all-pods');
            if (allResponse.ok) {
                const allData = await allResponse.json();
                const appMetric = allData.all_metrics?.find(m => m.pod === appName && m.namespace === namespace);
                if (appMetric) {
                    showAppMetricsModal(appName, namespace, {
                        total_cpu: appMetric.cpu,
                        total_memory: appMetric.memory,
                        containers: []
                    });
                } else {
                    throw new Error('Metrics not found');
                }
            } else {
                throw new Error('Failed to load metrics');
            }
        }
    } catch (error) {
        console.error('Error loading app metrics:', error);
        showError('Could not load application metrics. Using demo data.');
        
        // Показываем демо-данные
        showAppMetricsModal(appName, namespace, {
            total_cpu: `${Math.floor(Math.random() * 200) + 50}m`,
            total_memory: `${Math.floor(Math.random() * 512) + 128}Mi`,
            containers: [
                {
                    name: 'main',
                    cpu_usage: `${Math.floor(Math.random() * 100) + 20}m`,
                    cpu_limit: '200m',
                    cpu_percent: Math.floor(Math.random() * 60) + 20,
                    memory_usage: `${Math.floor(Math.random() * 256) + 64}Mi`,
                    memory_limit: '512Mi',
                    memory_percent: Math.floor(Math.random() * 70) + 15
                }
            ]
        });
    }
}

function showAppMetricsModal(appName, namespace, metrics) {
    const modalElement = document.getElementById('appMetricsModal');
    if (!modalElement) return;
    
    // Заполняем данные
    document.getElementById('metrics-app-name').textContent = appName;
    document.getElementById('metrics-namespace').textContent = namespace;
    
    if (metrics) {
        document.getElementById('metrics-cpu').textContent = metrics.total_cpu || '0m';
        document.getElementById('metrics-memory').textContent = metrics.total_memory || '0Mi';
        
        // Контейнеры
        const containersList = document.getElementById('metrics-containers');
        if (containersList) {
            if (metrics.containers && metrics.containers.length > 0) {
                containersList.innerHTML = metrics.containers.map(container => `
                    <div class="list-group-item">
                        <div class="d-flex justify-content-between align-items-center">
                            <div>
                                <strong>${container.name}</strong>
                                <div class="small text-muted">
                                    CPU: ${container.cpu_usage || '0m'} / ${container.cpu_limit || 'N/A'}
                                </div>
                                <div class="small text-muted">
                                    Memory: ${container.memory_usage || '0Mi'} / ${container.memory_limit || 'N/A'}
                                </div>
                            </div>
                            <div class="text-end">
                                <div class="progress mb-1" style="width: 60px; height: 4px;">
                                    <div class="progress-bar bg-success" style="width: ${container.cpu_percent || 0}%"></div>
                                </div>
                                <div class="progress" style="width: 60px; height: 4px;">
                                    <div class="progress-bar bg-info" style="width: ${container.memory_percent || 0}%"></div>
                                </div>
                            </div>
                        </div>
                    </div>
                `).join('');
            } else {
                containersList.innerHTML = `
                    <div class="list-group-item text-muted text-center">
                        No detailed container metrics available
                    </div>
                `;
            }
        }
    }
    
    const modal = new bootstrap.Modal(modalElement);
    modal.show();
}

function showScaleModal(appName, namespace, currentReplicas) {
    document.getElementById('scale-app-name').textContent = appName;
    document.getElementById('scale-current-replicas').textContent = currentReplicas || 1;
    
    const slider = document.getElementById('scale-replicas-slider');
    const input = document.getElementById('scale-replicas-input');
    const display = document.getElementById('scale-replicas-display');
    
    if (slider && input && display) {
        const replicas = parseInt(currentReplicas) || 1;
        slider.value = replicas;
        input.value = replicas;
        display.textContent = replicas;
        
        // Обработчики
        slider.oninput = function() {
            input.value = this.value;
            display.textContent = this.value;
        };
        
        input.oninput = function() {
            let value = parseInt(this.value) || 1;
            value = Math.max(1, Math.min(20, value));
            slider.value = value;
            display.textContent = value;
        };
    }
    
    const modal = new bootstrap.Modal(document.getElementById('scaleAppModal'));
    modal.show();
}

async function confirmScale() {
    const appName = document.getElementById('scale-app-name').textContent;
    const namespace = AppState.currentNamespace === 'all' ? 'default' : AppState.currentNamespace;
    const replicas = parseInt(document.getElementById('scale-replicas-input').value) || 1;
    
    try {
        console.log(`Scaling ${appName} in ${namespace} to ${replicas} replicas`);
        
        const response = await fetch(`/api/scale/${namespace}/${appName}?replicas=${replicas}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        
        if (response.ok) {
            const result = await response.json();
            showToast(result.message || `Scaled ${appName} to ${replicas} replicas`, 'success');
        } else {
            // Для демо
            showToast(`Application ${appName} would be scaled to ${replicas} replicas (demo)`, 'info');
            
            // Обновляем локальное состояние для демо
            const appIndex = AppState.applications.findIndex(a => 
                a.name === appName && a.namespace === namespace);
            if (appIndex !== -1) {
                AppState.applications[appIndex].total_count = replicas;
                AppState.applications[appIndex].ready_count = replicas;
                filterApplications();
                renderApplications();
            }
        }
        
        // Закрываем модальное окно
        const modal = bootstrap.Modal.getInstance(document.getElementById('scaleAppModal'));
        if (modal) modal.hide();
        
        // Обновляем данные через секунду
        setTimeout(() => loadInitialData(), 1000);
        
    } catch (error) {
        console.error('Scale error:', error);
        showError(`Failed to scale: ${error.message}`);
    }
}

async function restartApp(appName, namespace) {
    if (!confirm(`Are you sure you want to restart "${appName}"?`)) return;
    
    try {
        const response = await fetch(`/api/restart/${namespace}/${appName}`, {
            method: 'POST'
        });
        
        if (response.ok) {
            showToast(`Application ${appName} restarted`, 'success');
            setTimeout(loadInitialData, 1000);
        } else {
            // Для демо
            showToast(`Application ${appName} restarted (demo)`, 'success');
            setTimeout(loadInitialData, 2000);
        }
        
    } catch (error) {
        console.error('Restart error:', error);
        showError('Failed to restart application');
    }
}

async function deleteApp(appName, namespace) {
    if (!confirm(`Are you sure you want to delete "${appName}"?\nThis action cannot be undone.`)) return;
    
    try {
        const response = await fetch(`/api/deployment/${namespace}/${appName}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            showToast(`Application ${appName} deleted`, 'success');
            setTimeout(loadInitialData, 1000);
        } else {
            // Для демо - удаляем из локального состояния
            AppState.applications = AppState.applications.filter(app => 
                !(app.name === appName && app.namespace === namespace));
            filterApplications();
            renderApplications();
            showToast(`Application ${appName} removed from view (demo)`, 'warning');
        }
        
    } catch (error) {
        console.error('Delete error:', error);
        showError('Failed to delete application');
    }
}

// Утилиты
function getAppStatus(app) {
    const ready = app.ready_count || 0;
    const total = app.total_count || 0;
    
    if (total === 0) return 'warning';
    if (ready === total) return 'healthy';
    if (ready > 0 && ready < total) return 'progressing';
    return 'danger';
}

function getStatusColor(status) {
    switch (status) {
        case 'healthy': return 'success';
        case 'progressing': return 'warning';
        case 'danger': return 'danger';
        default: return 'secondary';
    }
}

function getAppIcon(type) {
    if (!type) return 'fa-cube';
    if (type.includes('StatefulSet')) return 'fa-database';
    if (type.includes('DaemonSet')) return 'fa-server';
    return 'fa-layer-group';
}

function truncateText(text, maxLength) {
    if (!text || text.length <= maxLength) return text || '';
    return text.substring(0, maxLength) + '...';
}

function formatBytes(bytes) {
    if (bytes === 0 || !bytes) return '0 B';
    
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatAge(age) {
    if (!age) return '-';
    return age;
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

async function fetchWithTimeout(resource, options = {}) {
    const { timeout = 10000 } = options;
    
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeout);
    
    try {
        const response = await fetch(resource, {
            ...options,
            signal: controller.signal
        });
        clearTimeout(id);
        return response;
    } catch (error) {
        clearTimeout(id);
        throw error;
    }
}

function updateApplicationsCount() {
    const totalApps = AppState.applications.length;
    const filteredApps = AppState.filteredApplications.length;
    
    const statsElement = document.getElementById('stats-count');
    if (statsElement) {
        statsElement.textContent = totalApps === filteredApps ? 
            totalApps : `${filteredApps}/${totalApps}`;
    }
}

function updateLastUpdated() {
    const now = new Date();
    const timeStr = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const element = document.getElementById('last-updated');
    if (element) {
        element.textContent = `Last updated: ${timeStr}`;
    }
}

function setupAutoRefresh(interval = CONFIG.refreshInterval) {
    if (AppState.refreshIntervalId) {
        clearInterval(AppState.refreshIntervalId);
    }
    
    if (interval > 0) {
        AppState.refreshIntervalId = setInterval(() => {
            if (!document.hidden) {
                loadInitialData();
            }
        }, interval);
    }
}

function showLoading(show) {
    const indicator = document.getElementById('loading-indicator');
    const refreshBtn = document.getElementById('refresh-btn');
    
    if (show) {
        if (indicator) indicator.style.display = 'inline-block';
        if (refreshBtn) {
            refreshBtn.disabled = true;
            refreshBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Loading';
        }
    } else {
        if (indicator) indicator.style.display = 'none';
        if (refreshBtn) {
            refreshBtn.disabled = false;
            refreshBtn.innerHTML = '<i class="fas fa-sync-alt me-2"></i>Refresh';
        }
    }
}

function showToast(message, type = 'info') {
    let container = document.querySelector('.toast-container');
    if (!container) {
        container = document.createElement('div');
        container.className = 'toast-container position-fixed top-0 end-0 p-3';
        document.body.appendChild(container);
    }
    
    const toastId = 'toast-' + Date.now();
    const toastHtml = `
        <div id="${toastId}" class="toast align-items-center text-bg-${type}" role="alert">
            <div class="d-flex">
                <div class="toast-body">
                    <i class="fas ${getToastIcon(type)} me-2"></i>
                    ${message}
                </div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
            </div>
        </div>
    `;
    
    container.insertAdjacentHTML('beforeend', toastHtml);
    
    const toastElement = document.getElementById(toastId);
    const toast = new bootstrap.Toast(toastElement, { delay: 3000 });
    toast.show();
    
    toastElement.addEventListener('hidden.bs.toast', () => {
        toastElement.remove();
    });
}

function showError(message) {
    showToast(message, 'danger');
}

function getToastIcon(type) {
    switch (type) {
        case 'success': return 'fa-check-circle';
        case 'danger': return 'fa-exclamation-circle';
        case 'warning': return 'fa-exclamation-triangle';
        default: return 'fa-info-circle';
    }
}

function createEmptyState() {
    return `
        <div class="col-12">
            <div class="empty-state">
                <i class="fas fa-search fa-3x text-muted mb-3"></i>
                <h5 class="text-muted">No applications found</h5>
                <p class="text-muted mb-4">Try changing your search or filters</p>
                <button class="btn btn-primary" onclick="clearSearch()">
                    <i class="fas fa-times me-2"></i>Clear Search
                </button>
            </div>
        </div>
    `;
}

function createEmptyTableState() {
    return `
        <tr>
            <td colspan="8" class="text-center py-5">
                <div class="empty-state">
                    <i class="fas fa-search fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No applications found</p>
                </div>
            </td>
        </tr>
    `;
}

function clearSearch() {
    const searchInput = document.getElementById('search-apps');
    if (searchInput) {
        searchInput.value = '';
        handleSearch();
    }
}

function updateSortIndicator() {
    document.querySelectorAll('.sort-indicator').forEach(el => el.remove());
    
    const currentSort = document.querySelector(`[data-sort="${AppState.sortBy}"]`);
    if (currentSort) {
        const icon = AppState.sortOrder === 'asc' ? 'fa-sort-up' : 'fa-sort-down';
        currentSort.innerHTML += ` <i class="fas ${icon} sort-indicator"></i>`;
    }
}

// Демо-данные
function getDemoApplications() {
    return [
        {
            name: 'api-gateway',
            namespace: 'market',
            type: 'Deployment',
            ready_count: 3,
            total_count: 3,
            age: '2d',
            metrics: {
                cpu: 150,
                memory: 256 * 1024 * 1024,
                cpuPercent: 25,
                memoryPercent: 30,
                cpuUsage: '150m',
                memoryUsage: '256Mi'
            }
        },
        {
            name: 'redis-cache',
            namespace: 'market',
            type: 'StatefulSet',
            ready_count: 1,
            total_count: 1,
            age: '5d',
            metrics: {
                cpu: 80,
                memory: 512 * 1024 * 1024,
                cpuPercent: 15,
                memoryPercent: 50,
                cpuUsage: '80m',
                memoryUsage: '512Mi'
            }
        },
        {
            name: 'postgres-db',
            namespace: 'default',
            type: 'StatefulSet',
            ready_count: 1,
            total_count: 1,
            age: '7d',
            metrics: {
                cpu: 120,
                memory: 1024 * 1024 * 1024,
                cpuPercent: 20,
                memoryPercent: 60,
                cpuUsage: '120m',
                memoryUsage: '1Gi'
            }
        }
    ];
}

function getDemoClusterMetrics() {
    return {
        cpu_percent: 45,
        memory_percent: 65,
        total_cpu_used: '1250m',
        total_memory_used: '5.2Gi',
        total_cpu_allocatable: '2000m',
        total_memory_allocatable: '8Gi',
        node_count: 3
    };
}

// Глобальные функции для HTML
window.clearSearch = clearSearch;
window.confirmScale = confirmScale;
window.testConnection = async function() {
    try {
        const response = await fetch('/api/health');
        const data = await response.json();
        
        if (data.k8s) {
            showToast('Connected to Kubernetes API', 'success');
            loadInitialData();
        } else {
            showToast('Kubernetes API not available', 'warning');
        }
    } catch (error) {
        showToast('Connection test failed: ' + error.message, 'danger');
    }
};