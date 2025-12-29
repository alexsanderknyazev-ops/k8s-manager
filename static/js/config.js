// Текущее состояние
let currentSection = 'configmaps';
let currentNamespace = 'market';
let configData = {
    configmaps: [],
    secrets: [],
    namespaces: [],
    services: [],
    nodes: []
};

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadNamespaces();
    loadCurrentSection();
    setupEventListeners();
});

// Настройка слушателей событий
function setupEventListeners() {
    // Переключение видимости значений секретов
    document.getElementById('show-secret-values').addEventListener('change', function() {
        const inputs = document.querySelectorAll('#secret-data-entries input[type="password"]');
        inputs.forEach(input => {
            input.type = this.checked ? 'text' : 'password';
        });
    });
}

// Загрузка namespaces для выпадающего списка
async function loadNamespaces() {
    try {
        const response = await fetch('/api/namespaces');
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const namespaces = data.namespaces || [];
        
        // Обновляем выпадающий список
        const select = document.getElementById('namespace-select');
        const cmSelect = document.getElementById('cm-namespace');
        const secretSelect = document.getElementById('secret-namespace');
        
        let html = '<option value="all">All Namespaces</option>';
        namespaces.forEach(ns => {
            const selected = ns.name === currentNamespace ? 'selected' : '';
            html += `<option value="${ns.name}" ${selected}>${ns.name}</option>`;
        });
        
        select.innerHTML = html;
        cmSelect.innerHTML = html.replace('all', 'default');
        secretSelect.innerHTML = html.replace('all', 'default');
        
        // Загружаем статистику по namespaces
        updateNamespaceStats();
        
    } catch (error) {
        console.error('Error loading namespaces:', error);
        showError('Failed to load namespaces');
    }
}

// Обновление статистики namespace
async function updateNamespaceStats() {
    try {
        // Загружаем данные для текущего namespace
        const [configmaps, secrets, services] = await Promise.all([
            fetch(`/api/configmaps/${currentNamespace}`).then(res => res.json()),
            fetch(`/api/secrets/${currentNamespace}`).then(res => res.json()),
            fetch(`/api/services?namespace=${currentNamespace}`).then(res => res.json())
        ]);
        
        document.getElementById('selected-namespace').textContent = currentNamespace;
        document.getElementById('namespace-cm-count').textContent = configmaps.count || 0;
        document.getElementById('namespace-secrets-count').textContent = secrets.count || 0;
        document.getElementById('namespace-services-count').textContent = services.count || 0;
        
    } catch (error) {
        console.error('Error updating namespace stats:', error);
    }
}

// Переключение секций
function showSection(section) {
    // Скрываем все секции
    document.querySelectorAll('.config-section').forEach(el => {
        el.style.display = 'none';
    });
    
    // Убираем активный класс у всех кнопок навигации
    document.querySelectorAll('#config-nav .list-group-item').forEach(el => {
        el.classList.remove('active');
    });
    
    // Показываем выбранную секцию
    document.getElementById(`${section}-section`).style.display = 'block';
    
    // Активируем соответствующую кнопку навигации
    document.querySelector(`#config-nav button[onclick="showSection('${section}')"]`).classList.add('active');
    
    // Обновляем заголовок и описание
    updateSectionHeader(section);
    
    // Загружаем данные для секции
    currentSection = section;
    loadCurrentSection();
}

// Обновление заголовка секции
function updateSectionHeader(section) {
    const titles = {
        configmaps: { title: 'ConfigMaps', desc: 'Manage configuration data as key-value pairs' },
        secrets: { title: 'Secrets', desc: 'Manage sensitive information like passwords and tokens' },
        namespaces: { title: 'Namespaces', desc: 'Manage cluster namespaces and resources' },
        services: { title: 'Services', desc: 'Manage network services and load balancers' },
        ingress: { title: 'Ingress', desc: 'Manage external access to services' },
        pvcs: { title: 'Persistent Volumes', desc: 'Manage storage volumes and claims' },
        rbac: { title: 'RBAC', desc: 'Manage Role-Based Access Control' },
        'cluster-info': { title: 'Cluster Information', desc: 'View cluster configuration and status' }
    };
    
    const info = titles[section] || { title: 'Configuration', desc: 'Manage cluster configuration' };
    
    document.getElementById('section-title').innerHTML = 
        `<i class="fas ${getSectionIcon(section)} me-2"></i>${info.title}`;
    document.getElementById('section-description').textContent = info.desc;
}

// Получение иконки для секции
function getSectionIcon(section) {
    const icons = {
        configmaps: 'fa-map',
        secrets: 'fa-key',
        namespaces: 'fa-layer-group',
        services: 'fa-network-wired',
        ingress: 'fa-globe',
        pvcs: 'fa-database',
        rbac: 'fa-user-shield',
        'cluster-info': 'fa-info-circle'
    };
    return icons[section] || 'fa-cogs';
}

// Загрузка текущей секции
async function loadCurrentSection() {
    showLoading(true);
    
    try {
        switch(currentSection) {
            case 'configmaps':
                await loadConfigMaps();
                break;
            case 'secrets':
                await loadSecrets();
                break;
            case 'namespaces':
                await loadAllNamespaces();
                break;
            case 'services':
                await loadServices();
                break;
            case 'ingress':
                await loadIngress();
                break;
            case 'pvcs':
                await loadPVCs();
                break;
            case 'rbac':
                await loadRBAC();
                break;
            case 'cluster-info':
                await loadClusterInfo();
                break;
        }
        
        // Обновляем информацию о namespace
        if (currentSection !== 'namespaces' && currentSection !== 'cluster-info') {
            updateNamespaceStats();
        }
        
    } catch (error) {
        console.error(`Error loading ${currentSection}:`, error);
        showError(`Failed to load ${currentSection}`);
    } finally {
        showLoading(false);
    }
}

// Загрузка ConfigMaps
async function loadConfigMaps() {
    const namespace = currentNamespace === 'all' ? '' : `/${currentNamespace}`;
    
    try {
        const response = await fetch(`/api/configmaps${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        configData.configmaps = data.configmaps || [];
        
        renderConfigMaps(configData.configmaps);
        
    } catch (error) {
        console.error('Error loading ConfigMaps:', error);
        showError('Failed to load ConfigMaps');
    }
}

// Рендер ConfigMaps
function renderConfigMaps(configmaps) {
    const tbody = document.getElementById('configmaps-table');
    
    if (!configmaps || configmaps.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" class="text-center py-4">
                    <i class="fas fa-map fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No ConfigMaps found</p>
                    <button class="btn btn-sm btn-outline-primary mt-2" onclick="createConfigMap()">
                        <i class="fas fa-plus me-1"></i>Create ConfigMap
                    </button>
                </td>
            </tr>
        `;
        return;
    }
    
    // Фильтрация по поиску
    const searchTerm = document.getElementById('config-search').value.toLowerCase();
    const filtered = configmaps.filter(cm => 
        !searchTerm || 
        cm.name.toLowerCase().includes(searchTerm) ||
        cm.namespace.toLowerCase().includes(searchTerm)
    );
    
    let html = '';
    filtered.forEach(cm => {
        const dataCount = cm.data || 0;
        
        html += `
            <tr onclick="viewConfigMap('${cm.namespace}', '${cm.name}')" style="cursor: pointer;">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-map me-2 text-primary"></i>
                        <strong>${cm.name}</strong>
                    </div>
                </td>
                <td><span class="badge bg-secondary">${cm.namespace}</span></td>
                <td>
                    <span class="badge bg-info">${dataCount} key${dataCount !== 1 ? 's' : ''}</span>
                </td>
                <td><small class="text-muted">${cm.age}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary" 
                                onclick="event.stopPropagation(); viewConfigMap('${cm.namespace}', '${cm.name}')">
                            <i class="fas fa-eye"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" 
                                onclick="event.stopPropagation(); deleteConfigMapConfirm('${cm.namespace}', '${cm.name}')">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Загрузка Secrets
async function loadSecrets() {
    const namespace = currentNamespace === 'all' ? '' : `/${currentNamespace}`;
    
    try {
        const response = await fetch(`/api/secrets${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        configData.secrets = data.secrets || [];
        
        renderSecrets(configData.secrets);
        
    } catch (error) {
        console.error('Error loading Secrets:', error);
        showError('Failed to load Secrets');
    }
}

// Рендер Secrets
function renderSecrets(secrets) {
    const tbody = document.getElementById('secrets-table');
    
    if (!secrets || secrets.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="6" class="text-center py-4">
                    <i class="fas fa-key fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No Secrets found</p>
                    <button class="btn btn-sm btn-outline-success mt-2" onclick="createSecret()">
                        <i class="fas fa-plus me-1"></i>Create Secret
                    </button>
                </td>
            </tr>
        `;
        return;
    }
    
    // Фильтрация по поиску
    const searchTerm = document.getElementById('config-search').value.toLowerCase();
    const filtered = secrets.filter(secret => 
        !searchTerm || 
        secret.name.toLowerCase().includes(searchTerm) ||
        secret.namespace.toLowerCase().includes(searchTerm)
    );
    
    let html = '';
    filtered.forEach(secret => {
        const type = secret.type || 'Opaque';
        const dataCount = secret.data || 0;
        
        html += `
            <tr onclick="viewSecret('${secret.namespace}', '${secret.name}')" style="cursor: pointer;">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-key me-2 text-success"></i>
                        <strong>${secret.name}</strong>
                    </div>
                </td>
                <td><span class="badge bg-secondary">${secret.namespace}</span></td>
                <td><span class="badge bg-warning">${type}</span></td>
                <td>
                    <span class="badge bg-info">${dataCount} key${dataCount !== 1 ? 's' : ''}</span>
                </td>
                <td><small class="text-muted">${secret.age}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-success" 
                                onclick="event.stopPropagation(); viewSecret('${secret.namespace}', '${secret.name}')">
                            <i class="fas fa-eye"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" 
                                onclick="event.stopPropagation(); deleteSecretConfirm('${secret.namespace}', '${secret.name}')">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Загрузка всех Namespaces
async function loadAllNamespaces() {
    try {
        const response = await fetch('/api/namespaces');
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        configData.namespaces = data.namespaces || [];
        
        renderNamespaces(configData.namespaces);
        
    } catch (error) {
        console.error('Error loading Namespaces:', error);
        showError('Failed to load Namespaces');
    }
}

// Рендер Namespaces
function renderNamespaces(namespaces) {
    const tbody = document.getElementById('namespaces-table');
    
    if (!namespaces || namespaces.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" class="text-center py-4">
                    <i class="fas fa-layer-group fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No Namespaces found</p>
                </td>
            </tr>
        `;
        return;
    }
    
    // Фильтрация по поиску
    const searchTerm = document.getElementById('config-search').value.toLowerCase();
    const filtered = namespaces.filter(ns => 
        !searchTerm || ns.name.toLowerCase().includes(searchTerm)
    );
    
    let html = '';
    filtered.forEach(ns => {
        const statusClass = ns.status === 'Active' ? 'success' : 'danger';
        
        html += `
            <tr onclick="viewNamespace('${ns.name}')" style="cursor: pointer;">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-layer-group me-2 text-info"></i>
                        <strong>${ns.name}</strong>
                    </div>
                </td>
                <td>
                    <span class="badge bg-${statusClass}">${ns.status}</span>
                </td>
                <td><span class="badge bg-primary">--</span></td>
                <td><span class="badge bg-info">--</span></td>
                <td><span class="badge bg-secondary">--</span></td>
                <td><small class="text-muted">${ns.age}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary" 
                                onclick="event.stopPropagation(); setDefaultNamespace('${ns.name}')">
                            <i class="fas fa-check"></i>
                        </button>
                        <button class="btn btn-sm btn-outline-danger" 
                                onclick="event.stopPropagation(); deleteNamespaceConfirm('${ns.name}')">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Загрузка Services
async function loadServices() {
    const namespace = currentNamespace === 'all' ? '' : `?namespace=${currentNamespace}`;
    
    try {
        const response = await fetch(`/api/services${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        configData.services = data.services || [];
        
        renderServices(configData.services);
        
    } catch (error) {
        console.error('Error loading Services:', error);
        showError('Failed to load Services');
    }
}

// Рендер Services
function renderServices(services) {
    const tbody = document.getElementById('services-table');
    
    if (!services || services.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" class="text-center py-4">
                    <i class="fas fa-network-wired fa-2x text-muted mb-3"></i>
                    <p class="text-muted">No Services found</p>
                </td>
            </tr>
        `;
        return;
    }
    
    // Фильтрация по поиску
    const searchTerm = document.getElementById('config-search').value.toLowerCase();
    const filtered = services.filter(svc => 
        !searchTerm || 
        svc.name.toLowerCase().includes(searchTerm) ||
        svc.namespace.toLowerCase().includes(searchTerm)
    );
    
    let html = '';
    filtered.forEach(svc => {
        const type = svc.type || 'ClusterIP';
        const ports = Array.isArray(svc.ports) ? svc.ports.join(', ') : svc.ports;
        
        html += `
            <tr onclick="viewService('${svc.namespace}', '${svc.name}')" style="cursor: pointer;">
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-network-wired me-2 text-purple"></i>
                        <strong>${svc.name}</strong>
                    </div>
                </td>
                <td><span class="badge bg-secondary">${svc.namespace}</span></td>
                <td><span class="badge bg-info">${type}</span></td>
                <td><small>${svc.clusterIP || 'None'}</small></td>
                <td><small>${ports}</small></td>
                <td><small class="text-muted">${svc.age}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-sm btn-outline-primary" 
                                onclick="event.stopPropagation(); viewServiceYAML('${svc.namespace}', '${svc.name}')">
                            <i class="fas fa-code"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Загрузка Ingress (заглушка)
async function loadIngress() {
    const content = document.getElementById('ingress-content');
    
    // В реальном приложении здесь будет API для Ingress
    content.innerHTML = `
        <div class="alert alert-warning">
            <i class="fas fa-exclamation-triangle me-2"></i>
            Ingress API not implemented in this version. Check back later!
        </div>
        <div class="text-center py-4">
            <i class="fas fa-globe fa-2x text-muted mb-3"></i>
            <p class="text-muted">Ingress management coming soon</p>
        </div>
    `;
}

// Загрузка PVC (заглушка)
async function loadPVCs() {
    const content = document.getElementById('pvcs-content');
    
    // В реальном приложении здесь будет API для PVC
    content.innerHTML = `
        <div class="alert alert-warning">
            <i class="fas fa-exclamation-triangle me-2"></i>
            Persistent Volume Claims API not implemented in this version.
        </div>
        <div class="text-center py-4">
            <i class="fas fa-database fa-2x text-muted mb-3"></i>
            <p class="text-muted">PVC management coming soon</p>
        </div>
    `;
}

// Загрузка RBAC (заглушка)
async function loadRBAC() {
    // В реальном приложении здесь будет API для RBAC
    document.getElementById('service-accounts').innerHTML = `
        <div class="text-center py-3">
            <i class="fas fa-users fa-2x text-muted mb-2"></i>
            <p class="text-muted small">Service Accounts API not implemented</p>
        </div>
    `;
    
    document.getElementById('roles-list').innerHTML = `
        <div class="text-center py-3">
            <i class="fas fa-user-tag fa-2x text-muted mb-2"></i>
            <p class="text-muted small">Roles API not implemented</p>
        </div>
    `;
    
    document.getElementById('cluster-roles').innerHTML = `
        <div class="text-center py-3">
            <i class="fas fa-user-shield fa-2x text-muted mb-2"></i>
            <p class="text-muted small">Cluster Roles API not implemented</p>
        </div>
    `;
}

// Загрузка информации о кластере
async function loadClusterInfo() {
    try {
        // Загружаем информацию о нодах
        const nodesResponse = await fetch('/api/nodes');
        const nodesData = await nodesResponse.json();
        
        // Загружаем информацию о namespaces
        const nsResponse = await fetch('/api/namespaces');
        const nsData = await nsResponse.json();
        
        // Загружаем информацию о подах
        const podsResponse = await fetch('/api/pods?namespace=all');
        const podsData = await podsResponse.json();
        
        // Обновляем информацию о кластере
        if (nodesData.nodes && nodesData.nodes.length > 0) {
            const firstNode = nodesData.nodes[0];
            document.getElementById('cluster-version').textContent = firstNode.version || 'Unknown';
            document.getElementById('cluster-runtime').textContent = firstNode.containerd || 'Unknown';
            document.getElementById('cluster-platform').textContent = firstNode.os || 'Unknown';
        }
        
        document.getElementById('cluster-api').textContent = window.location.host;
        document.getElementById('cluster-nodes').textContent = nodesData.count || 0;
        document.getElementById('cluster-pods').textContent = podsData.count || 0;
        document.getElementById('cluster-namespaces').textContent = nsData.count || 0;
        
        // Определяем здоровье кластера
        const readyNodes = nodesData.nodes?.filter(n => 
            n.status && n.status.includes('Ready')
        ).length || 0;
        const totalNodes = nodesData.count || 0;
        const health = totalNodes > 0 && readyNodes === totalNodes ? 
            '<span class="badge bg-success">Healthy</span>' : 
            '<span class="badge bg-warning">Degraded</span>';
        
        document.getElementById('cluster-health').innerHTML = health;
        
        // Показываем конфигурацию кластера
        document.getElementById('cluster-config').textContent = JSON.stringify({
            kubernetesVersion: document.getElementById('cluster-version').textContent,
            containerRuntime: document.getElementById('cluster-runtime').textContent,
            totalNodes: totalNodes,
            readyNodes: readyNodes,
            totalNamespaces: nsData.count || 0,
            totalPods: podsData.count || 0,
            apiServer: window.location.host
        }, null, 2);
        
    } catch (error) {
        console.error('Error loading cluster info:', error);
        showError('Failed to load cluster information');
    }
}

// Смена namespace
function changeNamespace() {
    currentNamespace = document.getElementById('namespace-select').value;
    loadCurrentSection();
}

// Фильтрация конфигурации
function filterConfig() {
    switch(currentSection) {
        case 'configmaps':
            renderConfigMaps(configData.configmaps);
            break;
        case 'secrets':
            renderSecrets(configData.secrets);
            break;
        case 'namespaces':
            renderNamespaces(configData.namespaces);
            break;
        case 'services':
            renderServices(configData.services);
            break;
    }
}

// Фильтрация по типу
function filterByType(type) {
    // В реальном приложении здесь будет более сложная фильтрация
    document.getElementById('config-search').value = type === 'all' ? '' : type;
    filterConfig();
}

// Обновление секции
function refreshSection() {
    loadCurrentSection();
    showToast(`${currentSection} refreshed`, 'info');
}

// Экспорт конфигурации
function exportConfig() {
    let data = {};
    
    switch(currentSection) {
        case 'configmaps':
            data = configData.configmaps;
            break;
        case 'secrets':
            data = configData.secrets;
            break;
        case 'namespaces':
            data = configData.namespaces;
            break;
        case 'services':
            data = configData.services;
            break;
        default:
            data = { message: 'No data to export' };
    }
    
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${currentSection}-${currentNamespace}-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    showToast(`${currentSection} exported`, 'success');
}

// Создание ConfigMap
function createConfigMap() {
    const modal = new bootstrap.Modal(document.getElementById('createConfigMapModal'));
    modal.show();
}

// Добавление поля данных в ConfigMap
function addDataEntry() {
    const container = document.getElementById('cm-data-entries');
    const entry = document.createElement('div');
    entry.className = 'row mb-2 data-entry-row';
    entry.innerHTML = `
        <div class="col-5">
            <input type="text" class="form-control" placeholder="key" required>
        </div>
        <div class="col-6">
            <input type="text" class="form-control" placeholder="value" required>
        </div>
        <div class="col-1">
            <button type="button" class="btn btn-sm btn-danger" onclick="removeDataEntry(this)">
                <i class="fas fa-times"></i>
            </button>
        </div>
    `;
    container.appendChild(entry);
}

// Добавление поля данных в Secret
function addSecretEntry() {
    const container = document.getElementById('secret-data-entries');
    const entry = document.createElement('div');
    entry.className = 'row mb-2 data-entry-row';
    entry.innerHTML = `
        <div class="col-5">
            <input type="text" class="form-control" placeholder="key" required>
        </div>
        <div class="col-6">
            <input type="password" class="form-control" placeholder="value" required>
        </div>
        <div class="col-1">
            <button type="button" class="btn btn-sm btn-danger" onclick="removeDataEntry(this)">
                <i class="fas fa-times"></i>
            </button>
        </div>
    `;
    container.appendChild(entry);
}

// Удаление поля данных
function removeDataEntry(button) {
    button.closest('.row').remove();
}

// Сохранение ConfigMap
async function saveConfigMap() {
    const name = document.getElementById('cm-name').value.trim();
    const namespace = document.getElementById('cm-namespace').value;
    const labelsText = document.getElementById('cm-labels').value;
    
    if (!name) {
        showToast('ConfigMap name is required', 'error');
        return;
    }
    
    // Собираем данные
    const dataEntries = document.querySelectorAll('#cm-data-entries .row');
    const data = {};
    
    dataEntries.forEach(row => {
        const keyInput = row.querySelector('input[placeholder="key"]');
        const valueInput = row.querySelector('input[placeholder="value"]');
        if (keyInput && valueInput && keyInput.value && valueInput.value) {
            data[keyInput.value] = valueInput.value;
        }
    });
    
    if (Object.keys(data).length === 0) {
        showToast('At least one key-value pair is required', 'error');
        return;
    }
    
    // Парсинг лейблов
    const labels = {};
    labelsText.split('\n').forEach(line => {
        const parts = line.trim().split('=');
        if (parts.length === 2) {
            labels[parts[0].trim()] = parts[1].trim();
        }
    });
    
    try {
        // В реальном приложении здесь будет вызов API
        console.log('Creating ConfigMap:', { name, namespace, data, labels });
        
        showToast(`ConfigMap "${name}" created`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('createConfigMapModal')).hide();
        
        // Сбрасываем форму
        document.getElementById('create-configmap-form').reset();
        
        // Обновляем список ConfigMaps
        setTimeout(() => loadConfigMaps(), 1000);
        
    } catch (error) {
        showToast(`Failed to create ConfigMap: ${error.message}`, 'error');
    }
}

// Создание Secret
function createSecret() {
    const modal = new bootstrap.Modal(document.getElementById('createSecretModal'));
    modal.show();
}

// Сохранение Secret
async function saveSecret() {
    const name = document.getElementById('secret-name').value.trim();
    const namespace = document.getElementById('secret-namespace').value;
    const type = document.getElementById('secret-type').value;
    
    if (!name) {
        showToast('Secret name is required', 'error');
        return;
    }
    
    // Собираем данные
    const dataEntries = document.querySelectorAll('#secret-data-entries .row');
    const data = {};
    
    dataEntries.forEach(row => {
        const keyInput = row.querySelector('input[placeholder="key"]');
        const valueInput = row.querySelector('input[type="password"], input[type="text"]');
        if (keyInput && valueInput && keyInput.value && valueInput.value) {
            // В реальном приложении здесь будет base64 кодирование
            data[keyInput.value] = btoa(valueInput.value);
        }
    });
    
    if (Object.keys(data).length === 0) {
        showToast('At least one key-value pair is required', 'error');
        return;
    }
    
    try {
        // В реальном приложении здесь будет вызов API
        console.log('Creating Secret:', { name, namespace, type, data });
        
        showToast(`Secret "${name}" created`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('createSecretModal')).hide();
        
        // Сбрасываем форму
        document.getElementById('create-secret-form').reset();
        
        // Обновляем список Secrets
        setTimeout(() => loadSecrets(), 1000);
        
    } catch (error) {
        showToast(`Failed to create Secret: ${error.message}`, 'error');
    }
}

// Создание Namespace
function createNamespace() {
    const modal = new bootstrap.Modal(document.getElementById('createNamespaceModal'));
    modal.show();
}

// Сохранение Namespace
async function saveNamespace() {
    const name = document.getElementById('namespace-name').value.trim();
    const labelsText = document.getElementById('namespace-labels').value;
    const annotationsText = document.getElementById('namespace-annotations').value;
    
    if (!name) {
        showToast('Namespace name is required', 'error');
        return;
    }
    
    // Парсинг лейблов и аннотаций
    const labels = {};
    labelsText.split('\n').forEach(line => {
        const parts = line.trim().split('=');
        if (parts.length === 2) {
            labels[parts[0].trim()] = parts[1].trim();
        }
    });
    
    const annotations = {};
    annotationsText.split('\n').forEach(line => {
        const parts = line.trim().split('=');
        if (parts.length === 2) {
            annotations[parts[0].trim()] = parts[1].trim();
        }
    });
    
    try {
        // В реальном приложении здесь будет вызов API
        console.log('Creating Namespace:', { name, labels, annotations });
        
        showToast(`Namespace "${name}" created`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('createNamespaceModal')).hide();
        
        // Сбрасываем форму
        document.getElementById('create-namespace-form').reset();
        
        // Обновляем список namespaces
        setTimeout(() => {
            loadNamespaces();
            if (currentSection === 'namespaces') {
                loadAllNamespaces();
            }
        }, 1000);
        
    } catch (error) {
        showToast(`Failed to create Namespace: ${error.message}`, 'error');
    }
}

// Просмотр ConfigMap
async function viewConfigMap(namespace, name) {
    try {
        const response = await fetch(`/api/configmap/yaml/${namespace}/${name}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        
        document.getElementById('view-cm-name').textContent = name;
        document.getElementById('view-cm-namespace').textContent = namespace;
        document.getElementById('view-cm-created').textContent = new Date().toLocaleString();
        document.getElementById('view-cm-yaml').textContent = data.yaml || 'No YAML available';
        
        // Отображаем данные ConfigMap
        const dataContainer = document.getElementById('view-cm-data');
        if (data.data) {
            let html = '';
            Object.entries(data.data).forEach(([key, value]) => {
                html += `
                    <div class="config-data-item">
                        <div class="config-data-key">${key}</div>
                        <div class="config-data-value">${value}</div>
                    </div>
                `;
            });
            dataContainer.innerHTML = html;
        } else {
            dataContainer.innerHTML = '<p class="text-muted">No data</p>';
        }
        
        // Отображаем лейблы
        const labelsContainer = document.getElementById('view-cm-labels');
        if (data.labels) {
            labelsContainer.innerHTML = Object.entries(data.labels)
                .map(([k, v]) => `<span class="label-item">${k}: ${v}</span>`)
                .join('');
        } else {
            labelsContainer.textContent = 'None';
        }
        
        const modal = new bootstrap.Modal(document.getElementById('viewConfigMapModal'));
        modal.show();
        
    } catch (error) {
        console.error('Error viewing ConfigMap:', error);
        showToast(`Failed to view ConfigMap: ${error.message}`, 'error');
    }
}

// Просмотр Secret
async function viewSecret(namespace, name) {
    try {
        const response = await fetch(`/api/secrets/${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const secret = data.secrets?.find(s => s.name === name);
        
        if (!secret) {
            showToast('Secret not found', 'error');
            return;
        }
        
        document.getElementById('view-secret-name').textContent = name;
        document.getElementById('view-secret-namespace').textContent = namespace;
        document.getElementById('view-secret-type').textContent = secret.type || 'Opaque';
        document.getElementById('view-secret-created').textContent = new Date().toLocaleString();
        
        // В реальном приложении здесь будет загрузка полного YAML
        document.getElementById('view-secret-yaml').textContent = 
            `# Secret YAML would be loaded from API\nname: ${name}\ntype: ${secret.type}\nnamespace: ${namespace}`;
        
        // Отображаем данные Secret (в реальном приложении будет загрузка данных)
        const dataContainer = document.getElementById('view-secret-data');
        dataContainer.innerHTML = `
            <div class="alert alert-info">
                <i class="fas fa-info-circle me-2"></i>
                Secret data requires additional API call. In production, this would show actual secret values.
            </div>
            <p class="text-muted small">Secret contains ${secret.data || 0} key${secret.data !== 1 ? 's' : ''}</p>
        `;
        
        const modal = new bootstrap.Modal(document.getElementById('viewSecretModal'));
        modal.show();
        
    } catch (error) {
        console.error('Error viewing Secret:', error);
        showToast(`Failed to view Secret: ${error.message}`, 'error');
    }
}

// Просмотр Namespace
function viewNamespace(name) {
    showToast(`Viewing namespace: ${name}`, 'info');
    // В реальном приложении здесь будет открытие детальной страницы namespace
    currentNamespace = name;
    document.getElementById('namespace-select').value = name;
    changeNamespace();
}

// Установка namespace по умолчанию
function setDefaultNamespace(name) {
    currentNamespace = name;
    document.getElementById('namespace-select').value = name;
    changeNamespace();
    showToast(`Default namespace set to: ${name}`, 'success');
}

// Просмотр Service
function viewService(namespace, name) {
    showToast(`Viewing service: ${name}`, 'info');
    // В реальном приложении здесь будет открытие детальной страницы service
}

// Просмотр YAML Service
async function viewServiceYAML(namespace, name) {
    try {
        const response = await fetch(`/api/service/yaml/${namespace}/${name}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        
        // Показываем YAML в новом окне
        const yamlWindow = window.open('', '_blank');
        yamlWindow.document.write(`
            <html>
            <head><title>Service YAML: ${name}</title>
            <style>
                body { font-family: monospace; background: #f5f5f5; padding: 20px; }
                pre { white-space: pre-wrap; }
            </style>
            </head>
            <body>
                <h2>Service YAML: ${name}</h2>
                <pre>${data.yaml || 'No YAML available'}</pre>
            </body>
            </html>
        `);
        
    } catch (error) {
        showToast(`Failed to load service YAML: ${error.message}`, 'error');
    }
}

// Удаление ConfigMap
function deleteConfigMapConfirm(namespace, name) {
    if (confirm(`Delete ConfigMap "${name}" in namespace "${namespace}"?`)) {
        deleteConfigMap(namespace, name);
    }
}

async function deleteConfigMap(namespace, name) {
    try {
        // В реальном приложении здесь будет вызов API DELETE
        console.log('Deleting ConfigMap:', { namespace, name });
        
        showToast(`ConfigMap "${name}" deleted`, 'success');
        
        // Обновляем список ConfigMaps
        setTimeout(() => loadConfigMaps(), 1000);
        
    } catch (error) {
        showToast(`Failed to delete ConfigMap: ${error.message}`, 'error');
    }
}

// Удаление Secret
function deleteSecretConfirm(namespace, name) {
    if (confirm(`Delete Secret "${name}" in namespace "${namespace}"?`)) {
        deleteSecret(namespace, name);
    }
}

async function deleteSecret(namespace, name) {
    try {
        // В реальном приложении здесь будет вызов API DELETE
        console.log('Deleting Secret:', { namespace, name });
        
        showToast(`Secret "${name}" deleted`, 'success');
        
        // Обновляем список Secrets
        setTimeout(() => loadSecrets(), 1000);
        
    } catch (error) {
        showToast(`Failed to delete Secret: ${error.message}`, 'error');
    }
}

// Удаление Namespace
function deleteNamespaceConfirm(name) {
    if (name === 'default' || name === 'kube-system' || name === 'kube-public') {
        showToast('Cannot delete system namespaces', 'error');
        return;
    }
    
    if (confirm(`Delete Namespace "${name}"? This will delete ALL resources in this namespace!`)) {
        deleteNamespace(name);
    }
}

async function deleteNamespace(name) {
    try {
        // В реальном приложении здесь будет вызов API DELETE
        console.log('Deleting Namespace:', { name });
        
        showToast(`Namespace "${name}" deleted`, 'success');
        
        // Обновляем список namespaces
        setTimeout(() => {
            loadNamespaces();
            if (currentSection === 'namespaces') {
                loadAllNamespaces();
            }
        }, 1000);
        
    } catch (error) {
        showToast(`Failed to delete Namespace: ${error.message}`, 'error');
    }
}

// Редактирование ConfigMap
function editConfigMap() {
    showToast('Edit ConfigMap feature coming soon', 'info');
}

// Редактирование Secret
function editSecret() {
    showToast('Edit Secret feature coming soon', 'info');
}

// Удаление из модального окна
function deleteConfigMap() {
    const name = document.getElementById('view-cm-name').textContent;
    const namespace = document.getElementById('view-cm-namespace').textContent;
    
    bootstrap.Modal.getInstance(document.getElementById('viewConfigMapModal')).hide();
    deleteConfigMapConfirm(namespace, name);
}

function deleteSecret() {
    const name = document.getElementById('view-secret-name').textContent;
    const namespace = document.getElementById('view-secret-namespace').textContent;
    
    bootstrap.Modal.getInstance(document.getElementById('viewSecretModal')).hide();
    deleteSecretConfirm(namespace, name);
}

// Вспомогательные функции
function showLoading(show) {
    // Можно добавить спиннер, если нужно
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