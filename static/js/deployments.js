// Текущий выбранный namespace
let currentNamespace = 'market';
let currentDeployment = null;
let allDeployments = [];
let autoRefreshInterval = null;
let selectedDeployments = new Set();
let currentPods = [];

// Инициализация при загрузке страницы
document.addEventListener('DOMContentLoaded', function() {
    loadDeployments();
    setupEventListeners();
});

// Настройка слушателей событий
function setupEventListeners() {
    // Выбор namespace
    document.getElementById('namespace-select').addEventListener('change', function() {
        currentNamespace = this.value;
        document.getElementById('current-namespace').textContent = currentNamespace;
        loadDeployments();
    });
    
    // Поиск
    document.getElementById('search-deployments').addEventListener('input', debounce(filterDeployments, 300));
    
    // Фильтры по статусу
    document.querySelectorAll('input[type="checkbox"]').forEach(checkbox => {
        checkbox.addEventListener('change', filterDeployments);
    });
    
    // Кнопка обновления
    document.getElementById('refresh-btn').addEventListener('click', loadDeployments);
    
    // Slider синхронизация
    document.getElementById('replicas-slider').addEventListener('input', function() {
        document.getElementById('replicas-input').value = this.value;
        document.getElementById('replicas-value').textContent = this.value;
    });
    
    document.getElementById('replicas-input').addEventListener('input', function() {
        const value = Math.min(20, Math.max(0, parseInt(this.value) || 0));
        document.getElementById('replicas-slider').value = value;
        document.getElementById('replicas-value').textContent = value;
    });
}

// Загрузка списка деплойментов
async function loadDeployments() {
    showLoading(true);
    
    try {
        const url = currentNamespace === 'all' 
            ? '/api/deployments' 
            : `/api/deployments?namespace=${currentNamespace}`;
        
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        
        const data = await response.json();
        
        // Проверяем структуру ответа
        if (!data) {
            throw new Error('Empty response from server');
        }
        
        // data.deployments может быть undefined, если нет деплойментов
        const deployments = data.deployments || [];
        const namespace = data.namespace || currentNamespace;
        const count = data.count || deployments.length;
        
        allDeployments = deployments;
        
        updateStats(deployments);
        renderDeploymentsTable(deployments);
        updateLastUpdated();
        
        console.log(`Loaded ${deployments.length} deployments from ${namespace}`);
        
    } catch (error) {
        console.error('Error loading deployments:', error);
        showError('Failed to load deployments: ' + error.message);
        
        // Показываем сообщение об ошибке в таблице
        const tbody = document.getElementById('deployments-table-body');
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="text-center py-5">
                    <i class="fas fa-exclamation-triangle fa-2x text-danger mb-3"></i>
                    <p class="text-danger">Failed to load deployments</p>
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

// Обновление статистики
function updateStats(deployments) {
    const stats = {
        ready: 0,
        notReady: 0,
        updating: 0,
        totalReplicas: 0,
        readyReplicas: 0,
        available: 0
    };
    
    deployments.forEach(deployment => {
        const readyCount = deployment.ready_count || 0;
        const totalCount = deployment.total_count || 0;
        const replicas = deployment.replicas || 0;
        const available = deployment.available_replicas || 0;
        
        if (readyCount === totalCount && readyCount > 0) {
            stats.ready++;
        } else if (readyCount < totalCount && readyCount > 0) {
            stats.updating++;
        } else {
            stats.notReady++;
        }
        
        stats.totalReplicas += replicas;
        stats.readyReplicas += readyCount;
        stats.available += available;
    });
    
    document.getElementById('ready-count').textContent = stats.ready;
    document.getElementById('replicas-count').textContent = stats.totalReplicas;
    document.getElementById('updating-count').textContent = stats.updating;
    document.getElementById('available-count').textContent = stats.available;
    document.getElementById('stats-count').textContent = deployments.length;
}

// Рендер таблицы деплойментов
function renderDeploymentsTable(deployments) {
    const tbody = document.getElementById('deployments-table-body');
    
    // Проверяем, что deployments - массив
    if (!Array.isArray(deployments)) {
        console.error('deployments is not an array:', deployments);
        deployments = [];
    }
    
    const searchTerm = document.getElementById('search-deployments').value.toLowerCase();
    
    // Фильтрация по статусу
    const activeFilters = Array.from(document.querySelectorAll('input[type="checkbox"]:checked'))
        .map(cb => cb.value);
    
    let filteredDeployments = deployments.filter(deployment => {
        // Проверяем структуру объекта deployment
        if (!deployment || typeof deployment !== 'object') return false;
        
        // Поиск по имени
        if (searchTerm && (!deployment.name || !deployment.name.toLowerCase().includes(searchTerm))) {
            return false;
        }
        
        // Фильтр по статусу
        if (activeFilters.length > 0) {
            const readyCount = deployment.ready_count || 0;
            const totalCount = deployment.total_count || 0;
            let status = '';
            
            if (readyCount === totalCount && readyCount > 0) {
                status = 'ready';
            } else if (readyCount < totalCount && readyCount > 0) {
                status = 'progressing';
            } else {
                status = 'not-ready';
            }
            
            if (!activeFilters.includes(status)) {
                return false;
            }
        }
        
        return true;
    });
    
    if (filteredDeployments.length === 0) {
        let message = 'No deployments found';
        if (deployments.length === 0) {
            message = 'No deployments in this namespace';
        } else if (searchTerm || activeFilters.length > 0) {
            message = 'No deployments match the current filters';
        }
        
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="text-center py-5">
                    <i class="fas fa-search fa-2x text-muted mb-3"></i>
                    <p class="text-muted">${message}</p>
                    ${deployments.length > 0 ? '<small class="text-muted">Try changing your filters</small>' : ''}
                </td>
            </tr>
        `;
        return;
    }
    
    let html = '';
    filteredDeployments.forEach((deployment, index) => {
        // Дефолтные значения на случай отсутствия полей
        const name = deployment.name || `deployment-${index}`;
        const namespace = deployment.namespace || currentNamespace;
        const ready = deployment.ready || '0/0';
        const readyCount = deployment.ready_count || 0;
        const totalCount = deployment.total_count || 0;
        const replicas = deployment.replicas || 0;
        const strategy = deployment.strategy || 'RollingUpdate';
        const age = deployment.age || 'unknown';
        const labels = deployment.labels || {};
        
        // Определяем статус
        let status = 'not-ready';
        let statusClass = 'badge-not-ready';
        if (readyCount === totalCount && readyCount > 0) {
            status = 'ready';
            statusClass = 'badge-ready';
        } else if (readyCount < totalCount && readyCount > 0) {
            status = 'progressing';
            statusClass = 'badge-progressing';
        }
        
        const readyPercentage = totalCount > 0 ? Math.round((readyCount / totalCount) * 100) : 0;
        const isSelected = selectedDeployments.has(`${namespace}/${name}`);
        
        html += `
            <tr data-deployment-name="${name}" data-namespace="${namespace}" 
                class="${isSelected ? 'selected' : ''}">
                <td>
                    <input type="checkbox" class="deployment-checkbox" 
                           data-namespace="${namespace}" data-name="${name}"
                           ${isSelected ? 'checked' : ''}
                           onclick="toggleDeploymentSelection('${namespace}', '${name}')">
                </td>
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-layer-group me-2 text-primary"></i>
                        <strong>${highlightSearch(name, searchTerm)}</strong>
                    </div>
                </td>
                <td><span class="badge bg-secondary">${namespace}</span></td>
                <td>
                    <span class="badge-status ${statusClass}">
                        <i class="fas ${getStatusIcon(status)} me-1"></i>
                        ${status.replace('-', ' ').toUpperCase()}
                    </span>
                </td>
                <td>
                    <div>
                        <span class="badge ${readyCount === totalCount ? 'bg-success' : 'bg-warning'}">
                            ${ready}
                        </span>
                        <div class="replica-progress">
                            <div class="progress-bar bg-${readyCount === totalCount ? 'success' : 'warning'}" 
                                 style="width: ${readyPercentage}%"></div>
                        </div>
                    </div>
                </td>
                <td>
                    <div class="d-flex align-items-center">
                        <i class="fas fa-server me-2 text-muted"></i>
                        <span class="fw-bold">${replicas}</span>
                        <div class="btn-group ms-2">
                            <button class="btn btn-xs btn-outline-success" 
                                    onclick="addPodToDeployment('${namespace}', '${name}', ${replicas})"
                                    title="Add Pod">
                                <i class="fas fa-plus"></i>
                            </button>
                            <button class="btn btn-xs btn-outline-warning" 
                                    onclick="removePodFromDeployment('${namespace}', '${name}', ${replicas})"
                                    title="Remove Pod">
                                <i class="fas fa-minus"></i>
                            </button>
                        </div>
                    </div>
                </td>
                <td>
                    <span class="badge ${strategy === 'RollingUpdate' ? 'bg-info' : 'bg-secondary'}">
                        ${strategy}
                    </span>
                </td>
                <td><small class="text-muted">${age}</small></td>
                <td>
                    <div class="btn-group" role="group">
                        <button class="btn btn-action btn-outline-primary btn-sm" 
                                onclick="showManagePods('${namespace}', '${name}', ${replicas})"
                                title="Manage Pods">
                            <i class="fas fa-cubes"></i>
                        </button>
                        <button class="btn btn-action btn-outline-success btn-sm" 
                                onclick="showScaleModal('${namespace}', '${name}', ${replicas})"
                                title="Scale">
                            <i class="fas fa-expand-alt"></i>
                        </button>
                        <button class="btn btn-action btn-outline-warning btn-sm" 
                                onclick="showRestartModal('${namespace}', '${name}')"
                                title="Restart">
                            <i class="fas fa-redo"></i>
                        </button>
                        <button class="btn btn-action btn-outline-info btn-sm" 
                                onclick="showConfig('${namespace}', '${name}')"
                                title="YAML">
                            <i class="fas fa-code"></i>
                        </button>
                        <button class="btn btn-action btn-outline-danger btn-sm" 
                                onclick="showDeleteModal('${namespace}', '${name}')"
                                title="Delete">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
    
    // Обновляем чекбокс "Выбрать все"
    updateSelectAllCheckbox();
}

// Управление выделением деплойментов
function toggleDeploymentSelection(namespace, name) {
    const key = `${namespace}/${name}`;
    if (selectedDeployments.has(key)) {
        selectedDeployments.delete(key);
    } else {
        selectedDeployments.add(key);
    }
    
    // Обновляем стиль строки
    const rows = document.querySelectorAll(`tr[data-namespace="${namespace}"][data-deployment-name="${name}"]`);
    rows.forEach(row => {
        if (selectedDeployments.has(key)) {
            row.classList.add('selected');
        } else {
            row.classList.remove('selected');
        }
    });
    
    // Обновляем чекбокс
    const checkbox = document.querySelector(`.deployment-checkbox[data-namespace="${namespace}"][data-name="${name}"]`);
    if (checkbox) {
        checkbox.checked = selectedDeployments.has(key);
    }
    
    updateSelectAllCheckbox();
}

function toggleSelectAll() {
    const selectAll = document.getElementById('select-all').checked;
    const checkboxes = document.querySelectorAll('.deployment-checkbox');
    
    selectedDeployments.clear();
    
    checkboxes.forEach(checkbox => {
        checkbox.checked = selectAll;
        const namespace = checkbox.dataset.namespace;
        const name = checkbox.dataset.name;
        
        if (selectAll) {
            selectedDeployments.add(`${namespace}/${name}`);
            const row = checkbox.closest('tr');
            if (row) row.classList.add('selected');
        } else {
            const row = checkbox.closest('tr');
            if (row) row.classList.remove('selected');
        }
    });
}

function updateSelectAllCheckbox() {
    const checkboxes = document.querySelectorAll('.deployment-checkbox');
    const selectAll = document.getElementById('select-all');
    
    if (checkboxes.length === 0) {
        selectAll.checked = false;
        selectAll.indeterminate = false;
        return;
    }
    
    const checkedCount = Array.from(checkboxes).filter(cb => cb.checked).length;
    
    if (checkedCount === 0) {
        selectAll.checked = false;
        selectAll.indeterminate = false;
    } else if (checkedCount === checkboxes.length) {
        selectAll.checked = true;
        selectAll.indeterminate = false;
    } else {
        selectAll.checked = false;
        selectAll.indeterminate = true;
    }
}

// Функции для добавления/удаления подов
async function addPodToDeployment(namespace, name, currentReplicas) {
    const newReplicas = currentReplicas + 1;
    
    try {
        const response = await fetch(`/api/scale/${namespace}/${name}?replicas=${newReplicas}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Added pod to ${name}. New total: ${newReplicas}`, 'success');
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to add pod: ${error.message}`, 'error');
    }
}

async function removePodFromDeployment(namespace, name, currentReplicas) {
    if (currentReplicas <= 1) {
        showToast('Cannot have less than 1 pod', 'warning');
        return;
    }
    
    const newReplicas = currentReplicas - 1;
    
    try {
        const response = await fetch(`/api/scale/${namespace}/${name}?replicas=${newReplicas}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Removed pod from ${name}. New total: ${newReplicas}`, 'success');
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to remove pod: ${error.message}`, 'error');
    }
}

// Массовое масштабирование
async function scaleSelectedDeployments() {
    if (selectedDeployments.size === 0) {
        showToast('Please select deployments first', 'warning');
        return;
    }
    
    // Заполняем список выбранных деплойментов
    const listContainer = document.getElementById('bulk-scale-list');
    listContainer.innerHTML = '';
    
    selectedDeployments.forEach(key => {
        const [namespace, name] = key.split('/');
        const deployment = allDeployments.find(d => 
            d.namespace === namespace && d.name === name
        );
        
        if (deployment) {
            const div = document.createElement('div');
            div.className = 'd-flex justify-content-between align-items-center mb-2';
            div.innerHTML = `
                <span>${name} <small class="text-muted">(${namespace})</small></span>
                <span class="badge bg-info">${deployment.replicas || 1} → 
                <span class="bulk-target-${namespace}-${name.replace(/[^a-zA-Z0-9]/g, '-')}">2</span></span>
            `;
            listContainer.appendChild(div);
        }
    });
    
    document.getElementById('selected-count').textContent = selectedDeployments.size;
    const modal = new bootstrap.Modal(document.getElementById('bulkScaleModal'));
    modal.show();
}

async function applyBulkScale() {
    const targetReplicas = parseInt(document.getElementById('bulk-replicas').value);
    const proportional = document.getElementById('scale-proportionally').checked;
    
    if (isNaN(targetReplicas) || targetReplicas < 0) {
        showToast('Please enter a valid number', 'error');
        return;
    }
    
    const promises = [];
    
    selectedDeployments.forEach(key => {
        const [namespace, name] = key.split('/');
        const deployment = allDeployments.find(d => 
            d.namespace === namespace && d.name === name
        );
        
        if (deployment) {
            let newReplicas = targetReplicas;
            if (proportional) {
                // Пропорциональное масштабирование на основе текущего количества реплик
                const current = deployment.replicas || 1;
                newReplicas = Math.max(1, Math.round(targetReplicas * (current / 2)));
            }
            
            promises.push(
                fetch(`/api/scale/${namespace}/${name}?replicas=${newReplicas}`, {
                    method: 'POST'
                }).then(response => {
                    if (!response.ok) throw new Error(`HTTP ${response.status}`);
                    return { namespace, name, success: true };
                }).catch(error => {
                    return { namespace, name, success: false, error: error.message };
                })
            );
        }
    });
    
    showLoading(true);
    
    try {
        const results = await Promise.all(promises);
        const successful = results.filter(r => r.success).length;
        const failed = results.filter(r => !r.success);
        
        if (failed.length > 0) {
            showToast(`Scaled ${successful} deployments, ${failed.length} failed`, 'warning');
            console.error('Failed deployments:', failed);
        } else {
            showToast(`Successfully scaled ${successful} deployments`, 'success');
        }
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('bulkScaleModal')).hide();
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Error: ${error.message}`, 'error');
    } finally {
        showLoading(false);
    }
}

// Быстрое масштабирование
async function quickScaleSelected() {
    if (selectedDeployments.size === 0) {
        showToast('Please select deployments first', 'warning');
        return;
    }
    
    const targetReplicas = parseInt(document.getElementById('quick-scale-value').value);
    
    if (isNaN(targetReplicas) || targetReplicas < 0) {
        showToast('Please enter a valid number', 'error');
        return;
    }
    
    const confirmScale = confirm(`Scale ${selectedDeployments.size} selected deployments to ${targetReplicas} replicas each?`);
    if (!confirmScale) return;
    
    showLoading(true);
    
    try {
        const promises = Array.from(selectedDeployments).map(key => {
            const [namespace, name] = key.split('/');
            return fetch(`/api/scale/${namespace}/${name}?replicas=${targetReplicas}`, {
                method: 'POST'
            });
        });
        
        await Promise.all(promises);
        showToast(`Scaled ${selectedDeployments.size} deployments to ${targetReplicas} replicas`, 'success');
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    } finally {
        showLoading(false);
    }
}

async function scaleAllUp() {
    const confirmScale = confirm('Add 1 pod to all deployments?');
    if (!confirmScale) return;
    
    showLoading(true);
    
    try {
        const promises = allDeployments.map(deployment => {
            const current = deployment.replicas || 1;
            return fetch(`/api/scale/${deployment.namespace}/${deployment.name}?replicas=${current + 1}`, {
                method: 'POST'
            });
        });
        
        await Promise.all(promises);
        showToast(`Added pods to all ${allDeployments.length} deployments`, 'success');
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    } finally {
        showLoading(false);
    }
}

async function scaleAllDown() {
    const confirmScale = confirm('Remove 1 pod from all deployments (minimum 1 pod)?');
    if (!confirmScale) return;
    
    showLoading(true);
    
    try {
        const promises = allDeployments.map(deployment => {
            const current = deployment.replicas || 1;
            const newReplicas = Math.max(1, current - 1);
            return fetch(`/api/scale/${deployment.namespace}/${deployment.name}?replicas=${newReplicas}`, {
                method: 'POST'
            });
        });
        
        await Promise.all(promises);
        showToast(`Removed pods from all ${allDeployments.length} deployments`, 'success');
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    } finally {
        showLoading(false);
    }
}

// Управление подами (полное управление)
async function showManagePods(namespace, name, currentReplicas) {
    currentDeployment = { namespace, name };
    
    document.getElementById('manage-pods-deployment').textContent = name;
    document.getElementById('current-replica-count').textContent = currentReplicas;
    document.getElementById('new-replica-count').value = currentReplicas;
    
    const modal = new bootstrap.Modal(document.getElementById('managePodsModal'));
    modal.show();
    
    await loadDeploymentPodsDetailed(namespace, name);
}

async function loadDeploymentPodsDetailed(namespace, deploymentName) {
    try {
        // Получаем поды этого деплоймента
        const response = await fetch(`/api/pods?namespace=${namespace}`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        const allPods = data.pods || [];
        
        // Фильтруем поды, принадлежащие этому деплойменту
        currentPods = allPods.filter(pod => {
            // Ищем поды, имя которых содержит имя деплоймента
            return pod.name.includes(deploymentName);
        });
        
        updatePodStatistics(currentPods);
        renderPodsManagementTable(currentPods);
        
    } catch (error) {
        console.error('Error loading deployment pods:', error);
        showError('Failed to load pods: ' + error.message);
    }
}

function updatePodStatistics(pods) {
    const total = pods.length;
    const running = pods.filter(p => p.status === 'Running').length;
    const pending = pods.filter(p => p.status === 'Pending').length;
    const failed = pods.filter(p => p.status === 'Failed').length;
    
    document.getElementById('total-pods').textContent = total;
    document.getElementById('showing-pods').textContent = total;
    
    // Обновляем прогресс-бары
    if (total > 0) {
        document.getElementById('running-progress').style.width = `${(running / total) * 100}%`;
        document.getElementById('pending-progress').style.width = `${(pending / total) * 100}%`;
        document.getElementById('failed-progress').style.width = `${(failed / total) * 100}%`;
    }
}

function renderPodsManagementTable(pods) {
    const tbody = document.getElementById('pods-management-body');
    
    if (!pods || pods.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" class="text-center py-4">
                    <i class="fas fa-cube fa-2x text-muted mb-2"></i>
                    <p class="text-muted">No pods found for this deployment</p>
                </td>
            </tr>
        `;
        return;
    }
    
    let html = '';
    pods.forEach((pod, index) => {
        const statusClass = getPodStatusClass(pod.status);
        const statusIcon = getPodStatusIcon(pod.status);
        
        html += `
            <tr class="pod-row ${pod.status.toLowerCase()}">
                <td>
                    <div class="d-flex align-items-center">
                        <span class="pod-status-icon ${pod.status.toLowerCase()}"></span>
                        <span class="font-monospace small">${pod.name}</span>
                    </div>
                </td>
                <td>
                    <span class="badge ${statusClass === 'running' ? 'bg-success' : 
                                       statusClass === 'pending' ? 'bg-warning' : 
                                       statusClass === 'failed' ? 'bg-danger' : 'bg-secondary'}">
                        ${pod.status}
                    </span>
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
                <td><small class="text-muted">${pod.age}</small></td>
                <td><small>${pod.node || '-'}</small></td>
                <td>
                    <div class="btn-group">
                        <button class="btn btn-xs btn-outline-primary" 
                                onclick="showPodLogs('${pod.namespace}', '${pod.name}')"
                                title="Logs">
                            <i class="fas fa-file-alt"></i>
                        </button>
                        <button class="btn btn-xs btn-outline-danger" 
                                onclick="deletePod('${pod.namespace}', '${pod.name}')"
                                title="Delete">
                            <i class="fas fa-trash"></i>
                        </button>
                    </div>
                </td>
            </tr>
        `;
    });
    
    tbody.innerHTML = html;
}

// Функции для управления репликами из модального окна
function addPod() {
    const input = document.getElementById('new-replica-count');
    const current = parseInt(input.value) || 0;
    input.value = current + 1;
    applyReplicaChange();
}

function removePod() {
    const input = document.getElementById('new-replica-count');
    const current = parseInt(input.value) || 0;
    if (current > 1) {
        input.value = current - 1;
        applyReplicaChange();
    } else {
        showToast('Cannot have less than 1 pod', 'warning');
    }
}

async function applyReplicaChange() {
    const newReplicas = parseInt(document.getElementById('new-replica-count').value);
    const namespace = currentDeployment.namespace;
    const name = currentDeployment.name;
    
    if (isNaN(newReplicas) || newReplicas < 0) {
        showToast('Please enter a valid number', 'error');
        return;
    }
    
    try {
        const response = await fetch(`/api/scale/${namespace}/${name}?replicas=${newReplicas}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Scaled ${name} to ${newReplicas} replicas`, 'success');
        document.getElementById('current-replica-count').textContent = newReplicas;
        
        // Обновляем список подов через 2 секунды
        setTimeout(() => loadDeploymentPodsDetailed(namespace, name), 2000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    }
}

async function refreshPodsList() {
    if (currentDeployment) {
        await loadDeploymentPodsDetailed(currentDeployment.namespace, currentDeployment.name);
        showToast('Pods list refreshed', 'info');
    }
}

// Показать логи пода
async function showPodLogs(namespace, podName) {
    try {
        const response = await fetch(`/api/logs/${namespace}/${podName}?tail=50`);
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        const data = await response.json();
        
        // Показываем логи в alert или можно создать отдельное модальное окно
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

// Удалить под
async function deletePod(namespace, podName) {
    if (!confirm(`Delete pod "${podName}"?`)) return;
    
    try {
        const response = await fetch(`/api/pod/${namespace}/${podName}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        showToast(`Pod "${podName}" deleted`, 'success');
        
        // Обновляем список подов
        setTimeout(() => {
            if (currentDeployment) {
                loadDeploymentPodsDetailed(currentDeployment.namespace, currentDeployment.name);
            }
        }, 1000);
        
    } catch (error) {
        showToast(`Failed to delete pod: ${error.message}`, 'error');
    }
}

// Остальные функции (scaleModal, restartModal и т.д.) остаются как были
// Показать модальное окно для масштабирования
function showScaleModal(namespace, name, currentReplicas) {
    currentDeployment = { namespace, name };
    
    document.getElementById('scale-deployment-name').textContent = name;
    document.getElementById('scale-namespace').textContent = namespace;
    document.getElementById('scale-current').textContent = currentReplicas;
    document.getElementById('replicas-slider').value = currentReplicas;
    document.getElementById('replicas-input').value = currentReplicas;
    document.getElementById('replicas-value').textContent = currentReplicas;
    
    // Инициализируем слайдер
    $('#replicas-slider').slider({
        tooltip: 'always',
        tooltip_position: 'bottom'
    });
    
    const modal = new bootstrap.Modal(document.getElementById('scaleModal'));
    modal.show();
}

// Масштабирование деплоймента
async function scaleDeployment() {
    const replicas = parseInt(document.getElementById('replicas-input').value);
    const namespace = currentDeployment.namespace;
    const name = currentDeployment.name;
    
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
        
        const data = await response.json();
        
        showToast(data.message || `Scaled to ${replicas} replicas`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('scaleModal')).hide();
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to scale: ${error.message}`, 'error');
    }
}

function adjustReplicas(delta) {
    const input = document.getElementById('replicas-input');
    const current = parseInt(input.value) || 0;
    const newValue = Math.max(0, current + delta);
    input.value = newValue;
    document.getElementById('replicas-slider').value = newValue;
    document.getElementById('replicas-value').textContent = newValue;
}

// Показать модальное окно для рестарта
function showRestartModal(namespace, name) {
    currentDeployment = { namespace, name };
    
    document.getElementById('restart-deployment-name').textContent = name;
    const modal = new bootstrap.Modal(document.getElementById('restartModal'));
    modal.show();
}

// Подтверждение рестарта
async function confirmRestart() {
    const namespace = currentDeployment.namespace;
    const name = currentDeployment.name;
    
    try {
        const response = await fetch(`/api/restart/${namespace}/${name}`, {
            method: 'POST'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        const data = await response.json();
        
        showToast(data.message || 'Deployment restarted', 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('restartModal')).hide();
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to restart: ${error.message}`, 'error');
    }
}

// Показать модальное окно для удаления
function showDeleteModal(namespace, name) {
    currentDeployment = { namespace, name };
    
    document.getElementById('delete-deployment-name').textContent = name;
    const modal = new bootstrap.Modal(document.getElementById('deleteModal'));
    modal.show();
}

// Подтверждение удаления
async function confirmDelete() {
    const namespace = currentDeployment.namespace;
    const name = currentDeployment.name;
    const cascade = document.getElementById('cascade-delete').checked;
    
    try {
        const response = await fetch(`/api/deployment/${namespace}/${name}?cascade=${cascade}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || `HTTP ${response.status}`);
        }
        
        const data = await response.json();
        
        showToast(data.message || 'Deployment deleted', 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('deleteModal')).hide();
        
        // Обновляем список
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to delete: ${error.message}`, 'error');
    }
}

// Показать конфигурацию деплоймента
async function showConfig(namespace, name) {
    currentDeployment = { namespace, name };
    
    document.getElementById('config-deployment-name').textContent = name;
    const modal = new bootstrap.Modal(document.getElementById('configModal'));
    modal.show();
    
    await loadYAML();
}

// Загрузить YAML конфигурацию
async function loadYAML() {
    const name = currentDeployment.name;
    const namespace = currentDeployment.namespace;
    
    try {
        const response = await fetch(`/api/deployment/yaml/${namespace}/${name}`);
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
    const name = currentDeployment.name;
    const yaml = document.getElementById('yaml-content').textContent;
    
    const blob = new Blob([yaml], { type: 'text/yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${name}-deployment.yaml`;
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
    const name = currentDeployment.name;
    const namespace = currentDeployment.namespace;
    
    try {
        const response = await fetch(`/api/deployment/yaml/${namespace}/${name}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml })
        });
        
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        
        showToast('Deployment configuration updated successfully!', 'success');
        document.getElementById('yaml-content').textContent = yaml;
        document.getElementById('yaml-content').style.display = 'block';
        document.getElementById('yaml-editor').style.display = 'none';
        document.getElementById('save-yaml-btn').style.display = 'none';
        
        // Перезагружаем список деплойментов
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to update: ${error.message}`, 'error');
    }
}

// Создание нового деплоймента
async function createDeployment() {
    const name = document.getElementById('deployment-name').value.trim();
    const namespace = document.getElementById('deployment-namespace').value;
    const image = document.getElementById('deployment-image').value.trim();
    const replicas = parseInt(document.getElementById('deployment-replicas').value) || 2;
    const port = parseInt(document.getElementById('deployment-port').value) || 80;
    const strategy = document.getElementById('deployment-strategy').value;
    const labelsText = document.getElementById('deployment-labels').value;
    
    // Валидация
    if (!name || !image) {
        showToast('Please fill in all required fields', 'error');
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
    
    // Создаем YAML деплоймента
    const deploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    app: ${name}
${Object.entries(labels).map(([k, v]) => `    ${k}: ${v}`).join('\n')}
spec:
  replicas: ${replicas}
  selector:
    matchLabels:
      app: ${name}
  strategy:
    type: ${strategy}
  template:
    metadata:
      labels:
        app: ${name}
    spec:
      containers:
      - name: ${name}
        image: ${image}
        ports:
        - containerPort: ${port}`;
    
    try {
        // В реальном приложении здесь будет вызов API для создания деплоймента
        console.log('Creating deployment:', { name, namespace, image, replicas, port, strategy });
        
        // Показываем успешное сообщение
        showToast(`Deployment "${name}" created successfully!`, 'success');
        
        // Закрываем модальное окно
        bootstrap.Modal.getInstance(document.getElementById('createModal')).hide();
        
        // Сбрасываем форму
        document.getElementById('create-deployment-form').reset();
        
        // Обновляем список деплойментов
        setTimeout(() => loadDeployments(), 1000);
        
    } catch (error) {
        showToast(`Failed to create deployment: ${error.message}`, 'error');
    }
}

// Вспомогательные функции
function getStatusIcon(status) {
    switch(status) {
        case 'ready': return 'fa-check-circle';
        case 'not-ready': return 'fa-times-circle';
        case 'progressing': return 'fa-sync-alt';
        default: return 'fa-question-circle';
    }
}

function getPodStatusClass(status) {
    switch(status) {
        case 'Running': return 'running';
        case 'Pending': return 'pending';
        case 'Failed': return 'failed';
        case 'Succeeded': return 'succeeded';
        default: return '';
    }
}

function getPodStatusIcon(status) {
    switch(status) {
        case 'Running': return 'fa-play-circle';
        case 'Pending': return 'fa-clock';
        case 'Failed': return 'fa-exclamation-circle';
        case 'Succeeded': return 'fa-check-circle';
        default: return 'fa-question-circle';
    }
}

function filterDeployments() {
    renderDeploymentsTable(allDeployments);
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

// Экспорт в CSV
function exportToCSV() {
    const rows = allDeployments.map(deployment => ({
        Name: deployment.name,
        Namespace: deployment.namespace,
        Status: deployment.ready_count === deployment.total_count ? 'Ready' : 'Not Ready',
        Ready: deployment.ready,
        Replicas: deployment.replicas || 0,
        Strategy: deployment.strategy || 'RollingUpdate',
        Age: deployment.age
    }));
    
    const csvContent = [
        Object.keys(rows[0]).join(','),
        ...rows.map(row => Object.values(row).join(','))
    ].join('\n');
    
    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `deployments-${currentNamespace}-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// Тест подключения
async function testConnection() {
    try {
        const response = await fetch('/api/test');
        const data = await response.json();
        
        if (data.connected) {
            showToast('Connected to Kubernetes API!', 'success');
            loadDeployments();
        } else {
            showToast(`Not connected: ${data.error}`, 'error');
        }
    } catch (error) {
        showToast('Connection test failed: ' + error.message, 'error');
    }
}

function toggleViewMode() {
    showToast('Grid view would be implemented here', 'info');
}