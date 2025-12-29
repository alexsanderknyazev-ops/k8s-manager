// Текущий выбранный namespace и режим просмотра
let currentNamespace = 'all';
let currentViewMode = 'grid';
let allApplications = [];
let currentApp = null;

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadApplications();
    setupEventListeners();
    initializeCharts();
});

// Настройка слушателей событий
function setupEventListeners() {
    // Выбор namespace
    document.getElementById('namespace-select').addEventListener('change', function() {
        currentNamespace = this.value;
        document.getElementById('current-namespace').textContent = 
            currentNamespace === 'all' ? 'all' : currentNamespace;
        loadApplications();
    });
    
    // Поиск
    document.getElementById('search-apps').addEventListener('input', debounce(filterApplications, 300));
    
    // Фильтры по типу
    document.querySelectorAll('#type-deployment, #type-statefulset, #type-daemonset').forEach(checkbox => {
        checkbox.addEventListener('change', filterApplications);
    });
    
    // Фильтры по статусу
    document.querySelectorAll('#status-healthy, #status-unhealthy, #status-progressing').forEach(checkbox => {
        checkbox.addEventListener('change', filterApplications);
    });
    
    // Кнопка обновления
    document.getElementById('refresh-btn').addEventListener('click', loadApplications);
    
    // Обработчик для слайдера масштабирования
    document.getElementById('scale-app-slider').addEventListener('input', function() {
        document.getElementById('scale-app-input').value = this.value;
        document.getElementById('scale-app-value').textContent = this.value;
    });
    
    document.getElementById('scale-app-input').addEventListener('input', function() {
        const value = Math.min(10, Math.max(0, parseInt(this.value) || 0));
        document.getElementById('scale-app-slider').value = value;
        document.getElementById('scale-app-value').textContent = value;
    });
}

// Загрузка списка приложений
async function loadApplications() {
    showLoading(true);
    
    try {
        const url = currentNamespace === 'all' 
            ? '/api/applications' 
            : `/api/applications?namespace=${currentNamespace}`;
        
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        
        const data = await response.json();
        
        // Проверяем структуру ответа
        if (!data) {
            throw new Error('Empty response from server');
        }
        
        // data.applications может быть undefined, если нет приложений
        const applications = data.applications || [];
        const namespace = data.namespace || currentNamespace;
        const count = data.count || applications.length;
        
        allApplications = applications;
        
        updateStats(applications);
        renderApplications(applications);
        updateLastUpdated();
        
        console.log(`Loaded ${applications.length} applications from ${namespace}`);
        
    } catch (error) {
        console.error('Error loading applications:', error);
        showError('Failed to load applications: ' + error.message);
        
        // Показываем сообщение об ошибке
        const grid = document.getElementById('applications-grid');
        grid.innerHTML = `
            <div class="col-12 text-center py-5">
                <i class="fas fa-exclamation-triangle fa-2x text-danger mb-3"></i>
                <p class="text-danger">Failed to load applications</p>
                <p class="text-muted small">${error.message}</p>
                <button class="btn btn-sm btn-outline-primary mt-2" onclick="testConnection()">
                    Test Connection
                </button>
            </div>
        `;
    } finally {
        showLoading(false);
    }
}

// Обновление статистики
function updateStats(applications) {
    const stats = {
        healthy: 0,
        warning: 0,
        error: 0,
        totalPods: 0
    };
    
    applications.forEach(app => {
        const ready = app.ready_count || 0;
        const total = app.total_count || 0;
        
        if (ready === total && ready > 0) {
            stats.healthy++;
        } else if (ready > 0 && ready < total) {
            stats.warning++;
        } else {
            stats.error++;
        }
        
        stats.totalPods += total;
    });
    
    document.getElementById('healthy-count').textContent = stats.healthy;
    document.getElementById('warning-count').textContent = stats.warning;
    document.getElementById('error-count').textContent = stats.error;
    document.getElementById('pods-count').textContent = stats.totalPods;
    document.getElementById('stats-count').textContent = applications.length;
    document.getElementById('total-apps').textContent = applications.length;
    document.getElementById('healthy-apps').textContent = `${stats.healthy} healthy`;
    
    // Обновляем прогресс-бары ресурсов (заглушка)
    const cpuUsage = Math.min(100, Math.floor((stats.totalPods * 10) + 20));
    const memoryUsage = Math.min(100, Math.floor((stats.totalPods * 15) + 25));
    
    document.getElementById('cpu-usage').style.width = `${cpuUsage}%`;
    document.getElementById('cpu-usage').textContent = `${cpuUsage}%`;
    document.getElementById('memory-usage').style.width = `${memoryUsage}%`;
    document.getElementById('memory-usage').textContent = `${memoryUsage}%`;
}

// Рендер приложений в зависимости от режима просмотра
function renderApplications(applications) {
    if (currentViewMode === 'grid') {
        renderApplicationsGrid(applications);
    } else if (currentViewMode === 'list') {
        renderApplicationsList(applications);
    } else if (currentViewMode === 'compact') {
        renderApplicationsCompact(applications);
    }
}

// Рендер в виде сетки (карточек)
function renderApplicationsGrid(applications) {
    const grid = document.getElementById('applications-grid');
    const list = document.getElementById('applications-list');
    
    // Показываем сетку, скрываем таблицу
    grid.style.display = 'block';
    list.style.display = 'none';
    
    // Фильтруем приложения
    const filteredApps = filterApplicationsList(applications);
    
    if (filteredApps.length === 0) {
        grid.innerHTML = `
            <div class="col-12 text-center py-5">
                <i class="fas fa-search fa-2x text-muted mb-3"></i>
                <p class="text-muted">No applications found</p>
                ${applications.length > 0 ? '<small class="text-muted">Try changing your filters</small>' : ''}
                <div class="mt-3">
                    <button class="btn btn-sm btn-outline-primary" onclick="showDeployModal()">
                        <i class="fas fa-plus me-1"></i>Deploy First Application
                    </button>
                </div>
            </div>
        `;
        return;
    }
    
    let html = '';
    filteredApps.forEach((app, index) => {
        // Дефолтные значения
        const name = app.name || `app-${index}`;
        const namespace = app.namespace || currentNamespace;
        const type = app.type || 'Deployment';
        const ready = app.ready || '0/0';
        const readyCount = app.ready_count || 0;
        const totalCount = app.total_count || 0;
        const instances = app.instances || totalCount;
        const age = app.age || 'unknown';
        const labels = app.labels || {};
        
        // Определяем статус и цвет
        let status = 'unhealthy';
        let statusClass = 'unhealthy';
        let statusColor = 'danger';
        
        if (readyCount === totalCount && readyCount > 0) {
            status = 'healthy';
            statusClass = 'healthy';
            statusColor = 'success';
        } else if (readyCount > 0 && readyCount < totalCount) {
            status = 'progressing';
            statusClass = 'progressing';
            statusColor = 'warning';
        } else if (readyCount === 0 && totalCount > 0) {
            status = 'warning';
            statusClass = 'warning';
            statusColor = 'warning';
        }
        
        const readyPercentage = totalCount > 0 ? Math.round((readyCount / totalCount) * 100) : 0;
        
        // Иконка в зависимости от типа
        let iconClass = 'deployment';
        let iconColor = '#007bff';
        if (type.includes('StatefulSet')) {
            iconClass = 'statefulset';
            iconColor = '#17a2b8';
        } else if (type.includes('DaemonSet')) {
            iconClass = 'daemonset';
            iconColor = '#ffc107';
        }
        
        // Генерация меток
        const labelsHtml = Object.entries(labels)
            .slice(0, 3)
            .map(([key, value]) => 
                `<span class="app-label">${key}: ${value}</span>`
            ).join('');
        
        html += `
            <div class="col-xl-3 col-lg-4 col-md-6 mb-4">
                <div class="card app-card ${statusClass} fade-in" 
                     onclick="showAppDetails('${namespace}', '${name}', '${type}')">
                    <div class="card-body">
                        <div class="d-flex align-items-start mb-3">
                            <div class="app-icon ${iconClass}">
                                <i class="fas ${getAppIcon(type)}"></i>
                            </div>
                            <div class="flex-grow-1">
                                <div class="d-flex justify-content-between align-items-start">
                                    <h5 class="card-title mb-1">${name}</h5>
                                    <span class="app-status status-${statusClass}">${status}</span>
                                </div>
                                <p class="card-text small text-muted mb-1">
                                    <i class="fas fa-layer-group me-1"></i>${type}
                                    <span class="ms-2"><i class="fas fa-cube me-1"></i>${namespace}</span>
                                </p>
                            </div>
                        </div>
                        
                        <div class="mb-3">
                            <div class="d-flex justify-content-between mb-1">
                                <small class="text-muted">Pods: ${ready}</small>
                                <small class="text-muted">${readyPercentage}%</small>
                            </div>
                            <div class="resource-progress">
                                <div class="progress-bar bg-${statusColor}" style="width: ${readyPercentage}%"></div>
                            </div>
                        </div>
                        
                        <div class="row text-center">
                            <div class="col-4">
                                <div class="small text-muted">Instances</div>
                                <div class="fw-bold">${instances}</div>
                            </div>
                            <div class="col-4">
                                <div class="small text-muted">Age</div>
                                <div class="fw-bold">${age}</div>
                            </div>
                            <div class="col-4">
                                <div class="small text-muted">Status</div>
                                <div>
                                    <span class="badge bg-${statusColor}">${status}</span>
                                </div>
                            </div>
                        </div>
                        
                        ${labelsHtml ? `
                        <div class="mt-3">
                            <div class="app-labels">
                                ${labelsHtml}
                                ${Object.keys(labels).length > 3 ? 
                                    `<span class="app-label">+${Object.keys(labels).length - 3} more</span>` : ''}
                            </div>
                        </div>
                        ` : ''}
                        
                        <div class="mt-3 pt-3 border-top">
                            <div class="btn-group w-100">
                                <button class="btn btn-sm btn-outline-primary" 
                                        onclick="event.stopPropagation(); scaleApp('${namespace}', '${name}', ${totalCount})">
                                    <i class="fas fa-expand-alt"></i>
                                </button>
                                <button class="btn btn-sm btn-outline-warning" 
                                        onclick="event.stopPropagation(); restartApp('${namespace}', '${name}')">
                                    <i class="fas fa-redo"></i>
                                </button>
                                <button class="btn btn-sm btn-outline-info" 
                                        onclick="event.stopPropagation(); showAppYAML('${namespace}', '${name}', '${type}')">
                                    <i class="fas fa-code"></i>
                                </button>
                                <button class="btn btn-sm btn-outline-danger" 
                                        onclick="event.stopPropagation(); deleteApp('${namespace}', '${name}', '${type}')">
                                    <i class="fas fa-trash"></i>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    });
    
    grid.innerHTML = html;
}

// Рендер в виде списка (таблицы)
function renderApplicationsList(applications) {
    const grid = document.getElementById('applications-grid');
    const list = document.getElementById('applications-list');
    const tbody = document.getElementById('applications-table-body');
    
    // Показываем таблицу, скрываем сетку
    grid.style.display = 'none';
    list.style.display = 'block';
    
    // Фильтруем приложения
    const filteredApps = filterApplicationsList(applications);
    
    if (filteredApps.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="text-center py-5">
                    <i class="fas fa-search fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No applications found</p>
                </td>
            </tr>
        `;
        return;
    }
    
    let html = '';
    filteredApps.forEach((app, index) => {
        const name = app.name || `app-${index}`;
        const namespace = app.namespace || currentNamespace;
        const type = app.type || 'Deployment';
        const ready = app.ready || '0/0';
        const readyCount = app.ready_count || 0;
        const totalCount = app.total_count || 0;
        const age = app.age || 'unknown';
        
        // Определяем статус
        let status = 'unhealthy';
        let statusClass = 'danger';
        if (readyCount === totalCount && readyCount > 0) {
            status = 'healthy';
            statusClass = 'success';
        } else if (readyCount > 0 && readyCount < totalCount) {
            status = 'progressing';
            statusClass = 'warning';
        }
        
        // Симуляция использования ресурсов
        const cpuUsage = `${Math.floor(Math.random() * 200) + 50}m`;
        const memoryUsage = `${Math.floor(Math.random() * 256) + 128}Mi`;
        
        html += `
            <tr onclick="showAppDetails('${namespace}', '${name}', '${type}')" style="cursor: pointer;">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas ${getAppIcon(type)} me-2 text-primary"></i>
                        <strong>${name}</strong>
                    </div>
                </td>
                <td><span class="badge bg-secondary">${namespace}</span></td>
                <td><span class="badge bg-info">${type}</span></td>
                <td>
                    <span class="badge bg-${statusClass}">
                        ${status}
                    </span>
                </td>
                <td>
                    <span class="badge ${readyCount === totalCount ? 'bg-success' : 'bg-warning'}">
                        ${ready}
                    </span>
                </td>
                <td><small>${cpuUsage}</small></td>
                <td><small>${memoryUsage}</small></td>
                <td><small class="text-muted">${age}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary" 
                                onclick="event.stopPropagation(); scaleApp('${namespace}', '${name}', ${totalCount})">
                            <i class="fas fa-expand-alt"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-warning" 
                                onclick="event.stopPropagation(); restartApp('${namespace}', '${name}')">
                            <i class="fas fa-redo"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" 
                                onclick="event.stopPropagation(); deleteApp('${namespace}', '${name}', '${type}')">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Рендер в компактном виде
function renderApplicationsCompact(applications) {
    const grid = document.getElementById('applications-grid');
    const list = document.getElementById('applications-list');
    
    grid.style.display = 'block';
    list.style.display = 'none';
    
    // Фильтруем приложения
    const filteredApps = filterApplicationsList(applications);
    
    if (filteredApps.length === 0) {
        grid.innerHTML = `
            <div class="col-12 text-center py-5">
                <i class="fas fa-search fa-2x text-muted mb-3"></i>
                <p class="text-muted">No applications found</p>
            </div>
        `;
        return;
    }
    
    let html = '';
    filteredApps.forEach((app, index) => {
        const name = app.name || `app-${index}`;
        const namespace = app.namespace || currentNamespace;
        const type = app.type || 'Deployment';
        const ready = app.ready || '0/0';
        const readyCount = app.ready_count || 0;
        const totalCount = app.total_count || 0;
        
        let statusClass = 'danger';
        if (readyCount === totalCount && readyCount > 0) {
            statusClass = 'success';
        } else if (readyCount > 0 && readyCount < totalCount) {
            statusClass = 'warning';
        }
        
        html += `
            <div class="col-xl-2 col-lg-3 col-md-4 col-sm-6 mb-3">
                <div class="card compact-card app-card" 
                     onclick="showAppDetails('${namespace}', '${name}', '${type}')">
                    <div class="card-body p-3">
                        <div class="d-flex align-items-center mb-2">
                            <div class="app-icon ${type.toLowerCase()}">
                                <i class="fas ${getAppIcon(type)}"></i>
                            </div>
                            <div class="flex-grow-1 ms-2">
                                <h6 class="mb-0">${name}</h6>
                                <small class="text-muted">${namespace}</small>
                            </div>
                            <span class="badge bg-${statusClass}">${ready}</span>
                        </div>
                        <div class="d-flex justify-content-between small">
                            <span>${type}</span>
                            <span class="text-muted">${readyCount}/${totalCount}</span>
                        </div>
                    </div>
                </div>
            </div>
        `;
    });
    
    grid.innerHTML = html;
}

// Фильтрация списка приложений
function filterApplicationsList(applications) {
    const searchTerm = document.getElementById('search-apps').value.toLowerCase();
    
    // Фильтры по типу
    const activeTypes = Array.from(document.querySelectorAll(
        '#type-deployment, #type-statefulset, #type-daemonset'
    ))
    .filter(cb => cb.checked)
    .map(cb => cb.value);
    
    // Фильтры по статусу
    const activeStatuses = Array.from(document.querySelectorAll(
        '#status-healthy, #status-unhealthy, #status-progressing'
    ))
    .filter(cb => cb.checked)
    .map(cb => cb.value);
    
    return applications.filter(app => {
        // Проверяем структуру объекта app
        if (!app || typeof app !== 'object') return false;
        
        // Поиск по имени
        if (searchTerm && (!app.name || !app.name.toLowerCase().includes(searchTerm))) {
            return false;
        }
        
        // Фильтр по типу
        if (activeTypes.length > 0) {
            const appType = app.type || 'Deployment';
            if (!activeTypes.some(type => appType.includes(type))) {
                return false;
            }
        }
        
        // Фильтр по статусу
        if (activeStatuses.length > 0) {
            const readyCount = app.ready_count || 0;
            const totalCount = app.total_count || 0;
            let status = '';
            
            if (readyCount === totalCount && readyCount > 0) {
                status = 'healthy';
            } else if (readyCount > 0 && readyCount < totalCount) {
                status = 'progressing';
            } else {
                status = 'unhealthy';
            }
            
            if (!activeStatuses.includes(status)) {
                return false;
            }
        }
        
        return true;
    });
}

// Функция фильтрации (вызывается при изменении фильтров)
function filterApplications() {
    renderApplications(allApplications);
}

// Изменение режима просмотра
function setViewMode(mode) {
    currentViewMode = mode;
    renderApplications(allApplications);
}

function toggleViewMode() {
    // Переключаем между grid и list
    if (currentViewMode === 'grid') {
        setViewMode('list');
    } else {
        setViewMode('grid');
    }
}

// Показать детали приложения
async function showAppDetails(namespace, name, type) {
    currentApp = { namespace, name, type };
    
    document.getElementById('app-details-name').textContent = name;
    const modal = new bootstrap.Modal(document.getElementById('appDetailsModal'));
    modal.show();
    
    await loadAppDetails(namespace, name, type);
}

// Загрузить детали приложения
async function loadAppDetails(namespace, name, type) {
    try {
        // Загружаем базовую информацию
        document.getElementById('app-name').textContent = name;
        document.getElementById('app-namespace').textContent = namespace;
        document.getElementById('app-type').textContent = type;
        document.getElementById('app-created').textContent = new Date().toLocaleString();
        
        // Загружаем деплоймент для получения деталей
        let url = '';
        if (type.includes('Deployment')) {
            url = `/api/deployment/yaml/${namespace}/${name}`;
        } else {
            // Для других типов можно добавить аналогичные эндпоинты
            url = `/api/deployments?namespace=${namespace}`;
        }
        
        const response = await fetch(url);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        
        // Обновляем статус
        const readyCount = data.status?.readyReplicas || 0;
        const totalCount = data.spec?.replicas || 0;
        const status = readyCount === totalCount ? 'Healthy' : 'Unhealthy';
        
        document.getElementById('app-status').innerHTML = `
            <span class="badge ${readyCount === totalCount ? 'bg-success' : 'bg-warning'}">
                ${status}
            </span>
        `;
        
        document.getElementById('app-pods-ready').textContent = readyCount;
        document.getElementById('app-pods-total').textContent = totalCount;
        
        const podsPercentage = totalCount > 0 ? Math.round((readyCount / totalCount) * 100) : 0;
        document.getElementById('app-pods-progress').style.width = `${podsPercentage}%`;
        
        // Симуляция использования ресурсов
        const cpuUsage = Math.min(100, Math.floor(Math.random() * 60) + 20);
        const memoryUsage = Math.min(100, Math.floor(Math.random() * 70) + 15);
        
        document.getElementById('app-cpu-progress').style.width = `${cpuUsage}%`;
        document.getElementById('app-memory-progress').style.width = `${memoryUsage}%`;
        
        // Лейблы
        const labelsContainer = document.getElementById('app-labels');
        labelsContainer.innerHTML = '';
        if (data.metadata?.labels) {
            Object.entries(data.metadata.labels).forEach(([key, value]) => {
                const badge = document.createElement('span');
                badge.className = 'badge bg-secondary me-1 mb-1';
                badge.textContent = `${key}: ${value}`;
                labelsContainer.appendChild(badge);
            });
        }
        
        // YAML конфигурация
        if (data.yaml) {
            document.getElementById('app-yaml').textContent = data.yaml;
        } else if (data) {
            document.getElementById('app-yaml').textContent = JSON.stringify(data, null, 2);
        }
        
        // Загружаем поды
        await loadAppPods(namespace, name);
        
        // Загружаем сервисы
        await loadAppServices(namespace, name);
        
        // Обновляем графики
        updateAppCharts();
        
    } catch (error) {
        console.error('Error loading app details:', error);
        showError('Failed to load application details: ' + error.message);
    }
}

// Загрузить поды приложения
async function loadAppPods(namespace, appName) {
    try {
        const response = await fetch(`/api/pods?namespace=${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const pods = data.pods || [];
        
        // Фильтруем поды по имени приложения
        const appPods = pods.filter(pod => pod.name.includes(appName));
        
        const tbody = document.getElementById('app-pods-list');
        let html = '';
        
        appPods.forEach(pod => {
            const statusClass = pod.status === 'Running' ? 'success' : 
                               pod.status === 'Pending' ? 'warning' : 'danger';
            
            html += `
                <tr>
                    <td><small>${pod.name}</small></td>
                    <td>
                        <span class="badge bg-${statusClass}">${pod.status}</span>
                    </td>
                    <td>
                        <span class="badge ${pod.ready === pod.ready ? 'bg-success' : 'bg-warning'}">
                            ${pod.ready}
                        </span>
                    </td>
                    <td>
                        <span class="badge ${pod.restarts > 0 ? 'bg-warning' : 'bg-secondary'}">
                            ${pod.restarts}
                        </span>
                    </td>
                    <td><small>${pod.age}</small></td>
                    <td><small>${pod.node || '-'}</small></td>
                    <td>
                        <button class="btn btn-xs btn-outline-primary" 
                                onclick="showPodLogs('${pod.namespace}', '${pod.name}')">
                            <i class="fas fa-file-alt"></i>
                        </button>
                    </td>
                </tr>
            `;
        });
        
        tbody.innerHTML = html || '<tr><td colspan="7" class="text-center">No pods found</td></tr>';
        
    } catch (error) {
        console.error('Error loading app pods:', error);
    }
}

// Загрузить сервисы приложения
async function loadAppServices(namespace, appName) {
    try {
        const response = await fetch(`/api/services?namespace=${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const services = data.services || [];
        
        // Фильтруем сервисы по имени приложения
        const appServices = services.filter(svc => 
            svc.name.includes(appName) || svc.name === appName
        );
        
        const tbody = document.getElementById('app-services-list');
        let html = '';
        
        appServices.forEach(svc => {
            html += `
                <tr>
                    <td><small>${svc.name}</small></td>
                    <td><span class="badge bg-info">${svc.type}</span></td>
                    <td><small>${svc.clusterIP}</small></td>
                    <td><small>${Array.isArray(svc.ports) ? svc.ports.join(', ') : svc.ports}</small></td>
                    <td><small>${svc.age}</small></td>
                </tr>
            `;
        });
        
        tbody.innerHTML = html || '<tr><td colspan="5" class="text-center">No services found</td></tr>';
        
    } catch (error) {
        console.error('Error loading app services:', error);
    }
}

// Показать логи пода
async function showPodLogs(namespace, podName) {
    try {
        const response = await fetch(`/api/logs/${namespace}/${podName}?tail=50`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        
        // Показываем логи в новом окне
        const logWindow = window.open('', '_blank');
        logWindow.document.write(`
            <html>
            <head><title>Logs: ${podName}</title>
            <style>
                body { font-family: monospace; background: #1e1e1e; color: #d4d4d4; padding: 20px; }
                pre { white-space: pre-wrap; }
            </style>
            </head>
            <body>
                <h2>Logs: ${podName}</h2>
                <pre>${data.logs}</pre>
            </body>
            </html>
        `);
        
    } catch (error) {
        showToast(`Failed to load logs: ${error.message}`, 'error');
    }
}

// Масштабирование приложения
function scaleApp(namespace, name, currentReplicas) {
    currentApp = { namespace, name };
    
    document.getElementById('scale-app-name').textContent = name;
    document.getElementById('scale-current-replicas').textContent = currentReplicas;
    document.getElementById('scale-app-slider').value = currentReplicas;
    document.getElementById('scale-app-input').value = currentReplicas;
    document.getElementById('scale-app-value').textContent = currentReplicas;
    
    const modal = new bootstrap.Modal(document.getElementById('scaleAppModal'));
    modal.show();
}

async function confirmScaleApp() {
    const replicas = parseInt(document.getElementById('scale-app-input').value);
    const namespace = currentApp.namespace;
    const name = currentApp.name;
    
    if (isNaN(replicas) || replicas < 0) {
        showToast('Please enter a valid number of replicas', 'error');
        return;
    }
    
    try {
        const response = await fetch(`/api/scale/${namespace}/${name}?replicas=${replicas}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Scaled ${name} to ${replicas} replicas`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('scaleAppModal')).hide();
        
        // Обновляем список приложений
        setTimeout(() => loadApplications(), 1000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    }
}

// Рестарт приложения
async function restartApp(namespace, name) {
    if (!confirm(`Restart application "${name}"?`)) return;
    
    try {
        const response = await fetch(`/api/restart/${namespace}/${name}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Application "${name}" restarted`, 'success');
        
        // Обновляем список приложений
        setTimeout(() => loadApplications(), 1000);
        
    } catch (error) {
        showToast(`Failed to restart: ${error.message}`, 'error');
    }
}

// Рестарт из модального окна деталей
async function restartApplication() {
    if (currentApp) {
        await restartApp(currentApp.namespace, currentApp.name);
        bootstrap.Modal.getInstance(document.getElementById('appDetailsModal')).hide();
    }
}

// Показать YAML приложения
async function showAppYAML(namespace, name, type) {
    try {
        let url = '';
        if (type.includes('Deployment')) {
            url = `/api/deployment/yaml/${namespace}/${name}`;
        } else {
            url = `/api/deployments?namespace=${namespace}`;
        }
        
        const response = await fetch(url);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const yaml = data.yaml || JSON.stringify(data, null, 2);
        
        // Показываем YAML в новом окне
        const yamlWindow = window.open('', '_blank');
        yamlWindow.document.write(`
            <html>
            <head><title>YAML: ${name}</title>
            <style>
                body { font-family: monospace; background: #f5f5f5; padding: 20px; }
                pre { white-space: pre-wrap; }
            </style>
            </head>
            <body>
                <h2>YAML: ${name}</h2>
                <pre>${yaml}</pre>
            </body>
            </html>
        `);
        
    } catch (error) {
        showToast(`Failed to load YAML: ${error.message}`, 'error');
    }
}

// Скопировать YAML приложения
async function copyAppYAML() {
    const yaml = document.getElementById('app-yaml').textContent;
    try {
        await navigator.clipboard.writeText(yaml);
        showToast('YAML copied to clipboard!', 'success');
    } catch (err) {
        showToast('Failed to copy YAML', 'error');
    }
}

// Скачать YAML приложения
function downloadAppYAML() {
    const name = currentApp?.name || 'application';
    const yaml = document.getElementById('app-yaml').textContent;
    
    const blob = new Blob([yaml], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${name}.yaml`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// Удаление приложения
async function deleteApp(namespace, name, type) {
    if (!confirm(`Delete application "${name}"? This action cannot be undone.`)) return;
    
    try {
        let url = '';
        let method = 'DELETE';
        
        if (type.includes('Deployment')) {
            url = `/api/deployment/${namespace}/${name}`;
        } else {
            showToast(`Deletion for ${type} not implemented yet`, 'warning');
            return;
        }
        
        const response = await fetch(url, { method });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Application "${name}" deleted`, 'success');
        
        // Обновляем список приложений
        setTimeout(() => loadApplications(), 1000);
        
    } catch (error) {
        showToast(`Failed to delete: ${error.message}`, 'error');
    }
}

// Развертывание приложения
function showDeployModal() {
    const modal = new bootstrap.Modal(document.getElementById('deployModal'));
    modal.show();
}

function selectTemplate(template) {
    // Убираем выделение со всех шаблонов
    document.querySelectorAll('.template-card').forEach(card => {
        card.classList.remove('selected');
    });
    
    // Выделяем выбранный шаблон
    event.currentTarget.classList.add('selected');
    
    // Заполняем форму в зависимости от шаблона
    const forms = {
        nginx: {
            name: 'nginx-web',
            image: 'nginx:latest',
            port: 80,
            replicas: 2
        },
        redis: {
            name: 'redis-cache',
            image: 'redis:alpine',
            port: 6379,
            replicas: 1
        },
        postgres: {
            name: 'postgres-db',
            image: 'postgres:13',
            port: 5432,
            replicas: 1
        }
    };
    
    const config = forms[template];
    if (config) {
        document.getElementById('app-name-input').value = config.name;
        document.getElementById('app-image-input').value = config.image;
        document.getElementById('app-port-input').value = config.port;
        document.getElementById('app-replicas-input').value = config.replicas;
        document.getElementById('app-type-input').value = 'Deployment';
    }
}

async function deployApplication() {
    const name = document.getElementById('app-name-input').value.trim();
    const namespace = document.getElementById('app-namespace-input').value;
    const image = document.getElementById('app-image-input').value.trim();
    const replicas = parseInt(document.getElementById('app-replicas-input').value) || 2;
    const port = parseInt(document.getElementById('app-port-input').value) || 80;
    const type = document.getElementById('app-type-input').value;
    const envText = document.getElementById('app-env-input').value;
    
    // Валидация
    if (!name || !image) {
        showToast('Please fill in all required fields', 'error');
        return;
    }
    
    // Парсинг переменных окружения
    const envVars = {};
    envText.split('\n').forEach(line => {
        const parts = line.trim().split('=');
        if (parts.length === 2) {
            envVars[parts[0].trim()] = parts[1].trim();
        }
    });
    
    // Создаем YAML для развертывания
    const deploymentYAML = `apiVersion: apps/v1
kind: ${type}
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: ${name}
    managed-by: k8s-manager
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${name}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
      - name: ${name}
        image: ${image}
        ports:
        - containerPort: ${port}
        ${Object.keys(envVars).length > 0 ? `
        env:
${Object.entries(envVars).map(([key, value]) => `        - name: ${key}
          value: "${value}"`).join('\n')}
        ` : ''}
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"`;
    
    try {
        // В реальном приложении здесь будет вызов API для создания деплоймента
        console.log('Deploying application:', { name, namespace, image, replicas, port, type });
        
        // Показываем успешное сообщение
        showToast(`Application "${name}" deployed successfully!`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('deployModal')).hide();
        
        // Сбрасываем форму
        document.getElementById('deploy-form').reset();
        
        // Обновляем список приложений
        setTimeout(() => loadApplications(), 2000);
        
    } catch (error) {
        showToast(`Failed to deploy: ${error.message}`, 'error');
    }
}

// Развертывание sample приложения
async function deploySampleApp() {
    const name = `sample-app-${Date.now().toString().slice(-6)}`;
    const namespace = 'market';
    const image = 'nginx:latest';
    
    try {
        // Используем API для создания деплоймента
        const response = await fetch('/api/deployments', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name,
                namespace,
                image,
                replicas: 2,
                port: 80,
                type: 'Deployment'
            })
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        
        showToast(`Sample application "${name}" deployed!`, 'success');
        
        // Обновляем список приложений
        setTimeout(() => loadApplications(), 2000);
        
    } catch (error) {
        // Если API не реализован, показываем информационное сообщение
        showToast('Deployment API not implemented. Check console for details.', 'info');
        console.log('Would deploy:', { name, namespace, image });
    }
}

// Экспорт приложений
function exportApplications() {
    const rows = allApplications.map(app => ({
        Name: app.name,
        Namespace: app.namespace,
        Type: app.type,
        Status: app.ready_count === app.total_count ? 'Healthy' : 'Unhealthy',
        Pods: app.ready,
        Instances: app.instances || app.total_count,
        Age: app.age
    }));
    
    const csvContent = [
        Object.keys(rows[0]).join(','),
        ...rows.map(row => Object.values(row).join(','))
    ].join('\n');
    
    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `applications-${currentNamespace}-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// Инициализация графиков
function initializeCharts() {
    // Инициализируем графики для вкладки метрик
    window.cpuChart = new Chart(document.getElementById('cpuChart'), {
        type: 'line',
        data: {
            labels: ['1m', '2m', '3m', '4m', '5m', '6m'],
            datasets: [{
                label: 'CPU Usage',
                data: [25, 30, 28, 35, 40, 38],
                borderColor: '#28a745',
                backgroundColor: 'rgba(40, 167, 69, 0.1)',
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: { display: false }
            }
        }
    });
    
    window.memoryChart = new Chart(document.getElementById('memoryChart'), {
        type: 'line',
        data: {
            labels: ['1m', '2m', '3m', '4m', '5m', '6m'],
            datasets: [{
                label: 'Memory Usage',
                data: [45, 50, 48, 55, 60, 58],
                borderColor: '#17a2b8',
                backgroundColor: 'rgba(23, 162, 184, 0.1)',
                tension: 0.4
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: { display: false }
            }
        }
    });
    
    window.networkChart = new Chart(document.getElementById('networkChart'), {
        type: 'bar',
        data: {
            labels: ['In', 'Out'],
            datasets: [{
                label: 'Network Traffic (MB)',
                data: [120, 85],
                backgroundColor: ['#007bff', '#6f42c1']
            }]
        },
        options: {
            responsive: true,
            plugins: {
                legend: { display: false }
            }
        }
    });
}

// Обновление графиков приложения
function updateAppCharts() {
    if (window.cpuChart && window.memoryChart && window.networkChart) {
        // Обновляем данные графиков случайными значениями
        const newCpuData = Array(6).fill(0).map(() => Math.floor(Math.random() * 40) + 20);
        const newMemoryData = Array(6).fill(0).map(() => Math.floor(Math.random() * 50) + 30);
        const newNetworkData = [Math.floor(Math.random() * 150) + 50, Math.floor(Math.random() * 100) + 30];
        
        window.cpuChart.data.datasets[0].data = newCpuData;
        window.cpuChart.update();
        
        window.memoryChart.data.datasets[0].data = newMemoryData;
        window.memoryChart.update();
        
        window.networkChart.data.datasets[0].data = newNetworkData;
        window.networkChart.update();
    }
}

// Показать метрики кластера
function showClusterMetrics() {
    showToast('Cluster metrics would open in a new tab', 'info');
    // В реальном приложении здесь будет переход на страницу метрик
}

// Вспомогательные функции
function getAppIcon(type) {
    if (type.includes('StatefulSet')) return 'fa-database';
    if (type.includes('DaemonSet')) return 'fa-server';
    return 'fa-layer-group';
}

function showLoading(show) {
    // Можно добавить спиннер, если нужно
    if (show) {
        // Показываем loading state
    }
}

function updateLastUpdated() {
    const now = new Date();
    // Можно добавить элемент для отображения времени обновления
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

// Тест подключения
async function testConnection() {
    try {
        const response = await fetch('/api/test');
        const data = await response.json();
        
        if (data.connected) {
            showToast('Connected to Kubernetes API!', 'success');
            loadApplications();
        } else {
            showToast(`Not connected: ${data.error}`, 'error');
        }
    } catch (error) {
        showToast('Connection test failed: ' + error.message, 'error');
    }
}