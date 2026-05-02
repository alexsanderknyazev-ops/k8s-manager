// Функции для работы с метриками

// Получение детальных метрик для пода
async function getPodMetrics(namespace, podName) {
    try {
        const response = await fetch(`/api/metrics/pod/${namespace}/${podName}`);
        const data = await response.json();
        return data;
    } catch (error) {
        console.error('Error fetching pod metrics:', error);
        return null;
    }
}

// Получение метрик для namespace
async function getNamespaceMetrics(namespace) {
    try {
        const response = await fetch(`/api/metrics/pods/${namespace}`);
        const data = await response.json();
        return data;
    } catch (error) {
        console.error('Error fetching namespace metrics:', error);
        return null;
    }
}

// Отображение деталей метрик пода
async function showPodMetricsDetails(namespace, podName) {
    const metrics = await getPodMetrics(namespace, podName);
    if (!metrics) {
        showToast('Failed to load pod metrics', 'error');
        return;
    }
    
    let details = `
        <h5>Pod Metrics: ${podName}</h5>
        <p><strong>Namespace:</strong> ${namespace}</p>
        <p><strong>Timestamp:</strong> ${metrics.timestamp || 'N/A'}</p>
        
        <h6>Total Usage:</h6>
        <ul>
            <li>CPU: ${metrics.total_cpu || 'N/A'}</li>
            <li>Memory: ${metrics.total_memory || 'N/A'}</li>
        </ul>
    `;
    
    if (metrics.containers && metrics.containers.length > 0) {
        details += '<h6>Container Metrics:</h6><ul>';
        metrics.containers.forEach(container => {
            details += `
                <li>
                    <strong>${container.name}:</strong><br>
                    CPU: ${container.cpu_usage} (Limit: ${container.cpu_limit})<br>
                    Memory: ${container.memory_usage} (Limit: ${container.memory_limit})<br>
                    CPU: ${container.cpu_percent}%, Memory: ${container.memory_percent}%
                </li>
            `;
        });
        details += '</ul>';
    }
    
    // Создаем модальное окно для отображения деталей
    const modal = document.createElement('div');
    modal.className = 'modal fade';
    modal.id = 'metricsModal';
    modal.innerHTML = `
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">
                        <i class="fas fa-chart-line me-2"></i>Pod Metrics Details
                    </h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    ${details}
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>
                </div>
            </div>
        </div>
    `;
    
    document.body.appendChild(modal);
    const bsModal = new bootstrap.Modal(modal);
    bsModal.show();
    
    // Удаляем модальное окно после закрытия
    modal.addEventListener('hidden.bs.modal', () => {
        modal.remove();
    });
}

// Экспорт метрик в CSV
function exportMetricsToCSV() {
    if (!metricsData.pods || metricsData.pods.length === 0) {
        showToast('No metrics data to export', 'warning');
        return;
    }
    
    // Создаем CSV заголовок
    let csv = 'Pod,Namespace,CPU Usage,Memory Usage,CPU Limit,Memory Limit,CPU %,Memory %\n';
    
    // Добавляем данные
    metricsData.pods.forEach(pod => {
        csv += `"${pod.pod}","${pod.namespace}","${pod.cpu_usage || ''}","${pod.memory_usage || ''}",`;
        csv += `"${pod.cpu_limit || ''}","${pod.memory_limit || ''}",`;
        csv += `"${pod.cpu_percent || ''}","${pod.memory_percent || ''}"\n`;
    });
    
    // Создаем blob и скачиваем
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `kubernetes-metrics-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    
    showToast('Metrics exported to CSV', 'success');
}

// Функция для сравнения метрик
function compareMetrics(metric1, metric2) {
    const comparisons = [];
    
    // Сравнение использования CPU
    if (metric1.cpu_raw && metric2.cpu_raw) {
        const diff = metric1.cpu_raw - metric2.cpu_raw;
        comparisons.push({
            metric: 'CPU Usage',
            diff: diff,
            unit: 'millicores'
        });
    }
    
    // Сравнение использования памяти
    if (metric1.memory_raw && metric2.memory_raw) {
        const diff = metric1.memory_raw - metric2.memory_raw;
        comparisons.push({
            metric: 'Memory Usage',
            diff: diff,
            unit: 'bytes'
        });
    }
    
    return comparisons;
}

// Мониторинг изменений метрик
class MetricsMonitor {
    constructor() {
        this.previousMetrics = null;
        this.changes = [];
    }
    
    updateMetrics(newMetrics) {
        if (!this.previousMetrics) {
            this.previousMetrics = newMetrics;
            return;
        }
        
        // Находим изменения
        const changes = this.findChanges(this.previousMetrics, newMetrics);
        if (changes.length > 0) {
            this.changes.push({
                timestamp: new Date(),
                changes: changes
            });
            
            // Ограничиваем историю изменений
            if (this.changes.length > 100) {
                this.changes.shift();
            }
        }
        
        this.previousMetrics = newMetrics;
    }
    
    findChanges(oldMetrics, newMetrics) {
        const changes = [];
        
        // Сравниваем общее использование CPU
        if (oldMetrics.clusterUsage?.cpu_percent !== newMetrics.clusterUsage?.cpu_percent) {
            changes.push({
                type: 'cpu_usage_change',
                old: oldMetrics.clusterUsage?.cpu_percent,
                new: newMetrics.clusterUsage?.cpu_percent,
                diff: newMetrics.clusterUsage?.cpu_percent - (oldMetrics.clusterUsage?.cpu_percent || 0)
            });
        }
        
        // Сравниваем количество подов
        if (oldMetrics.pods?.length !== newMetrics.pods?.length) {
            changes.push({
                type: 'pods_count_change',
                old: oldMetrics.pods?.length || 0,
                new: newMetrics.pods?.length || 0,
                diff: (newMetrics.pods?.length || 0) - (oldMetrics.pods?.length || 0)
            });
        }
        
        return changes;
    }
    
    getRecentChanges(limit = 10) {
        return this.changes.slice(-limit).reverse();
    }
}

// Глобальный экземпляр монитора метрик
const metricsMonitor = new MetricsMonitor();

// Обновляем монитор при загрузке новых метрик
function updateMetricsMonitor() {
    if (metricsData) {
        metricsMonitor.updateMetrics(metricsData);
    }
}

// Функция для отображения истории изменений метрик
function showMetricsHistory() {
    const changes = metricsMonitor.getRecentChanges(5);
    if (changes.length === 0) {
        showToast('No recent metric changes detected', 'info');
        return;
    }
    
    let history = '<h5>Recent Metric Changes</h5>';
    changes.forEach((record, index) => {
        history += `<h6>${record.timestamp.toLocaleTimeString()}</h6>`;
        history += '<ul>';
        record.changes.forEach(change => {
            const sign = change.diff > 0 ? '+' : '';
            history += `<li>${change.type}: ${sign}${change.diff}</li>`;
        });
        history += '</ul>';
    });
    
    // Показываем в alert для простоты
    alert(history);
}