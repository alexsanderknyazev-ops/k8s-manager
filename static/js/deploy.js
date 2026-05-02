(function() {
    let templates = [];

    document.addEventListener('DOMContentLoaded', function() {
        loadTemplates();
        loadNamespaces();
        document.getElementById('template-select').addEventListener('change', onTemplateChange);
        var btn = document.getElementById('btn-deploy');
        if (btn) btn.addEventListener('click', doDeploy);
    });

    async function loadTemplates() {
        const sel = document.getElementById('template-select');
        try {
            const data = await apiFetch('/api/deploy/templates');
            templates = data.templates || [];
            sel.innerHTML = '<option value="">— Свой образ —</option>' +
                templates.map(function(t) {
                    return '<option value="' + (t.id || '') + '">' + (t.name || t.id) + '</option>';
                }).join('');
        } catch (e) {
            console.warn('Templates load failed:', e);
            sel.innerHTML = '<option value="">— Свой образ —</option>';
        }
    }

    function loadNamespaces() {
        const sel = document.getElementById('deploy-namespace');
        if (!sel) return;
        fetch('/api/namespaces').then(function(r) { return r.json(); }).then(function(data) {
            const list = data.namespaces || [];
            const names = list.map(function(n) { return n.name || n; });
            if (!names.includes('default')) names.unshift('default');
            if (!names.includes('market')) names.push('market');
            sel.innerHTML = names.map(function(n) { return '<option value="' + n + '">' + n + '</option>'; }).join('');
            if (names.includes('market')) sel.value = 'market';
            else if (names.length) sel.selectedIndex = 0;
        }).catch(function() {
            sel.innerHTML = '<option value="default">default</option><option value="market">market</option>';
        });
    }

    function onTemplateChange() {
        var id = document.getElementById('template-select').value;
        var t = templates.find(function(x) { return x.id === id; });
        if (!t) return;
        document.getElementById('deploy-name').value = (t.id === 'custom' ? '' : (t.name && t.name !== 'Свой образ' ? t.name.toLowerCase().replace(/\s+/, '-') : t.id));
        document.getElementById('deploy-image').value = t.image || '';
        document.getElementById('deploy-container-port').value = t.port || 80;
        document.getElementById('deploy-service-port').value = t.port || 80;
        document.getElementById('template-desc').textContent = t.description || '';
    }

    async function doDeploy() {
        var name = (document.getElementById('deploy-name').value || '').trim();
        var image = (document.getElementById('deploy-image').value || '').trim();
        if (!name || !image) {
            showToast('Укажите имя приложения и образ контейнера', 'error');
            return;
        }
        var namespace = document.getElementById('deploy-namespace').value || 'default';
        var replicas = parseInt(document.getElementById('deploy-replicas').value, 10) || 1;
        var containerPort = parseInt(document.getElementById('deploy-container-port').value, 10) || 80;
        var servicePort = parseInt(document.getElementById('deploy-service-port').value, 10) || containerPort;
        var createService = document.getElementById('deploy-create-service').checked;

        var btn = document.getElementById('btn-deploy');
        if (btn) btn.disabled = true;
        try {
            showToast('Создание Deployment...', 'info');
            var data = await apiFetch('/api/deploy/simple', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    name: name,
                    namespace: namespace,
                    image: image,
                    replicas: replicas,
                    container_port: containerPort,
                    create_service: createService,
                    service_port: servicePort
                })
            });
            showToast((data.message || 'Готово') + ': ' + (data.created && data.created.length ? data.created.join(', ') : ''), 'success');
            document.getElementById('deploy-name').value = '';
            setTimeout(function() { window.location.href = '/ui/deployments?namespace=' + encodeURIComponent(namespace); }, 1500);
        } catch (e) {
            showToast('Ошибка: ' + (e.message || 'неизвестная'), 'error');
        } finally {
            if (btn) btn.disabled = false;
        }
    }
})();
