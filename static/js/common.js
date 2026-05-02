(function () {
    var theme = localStorage.getItem('k8s_manager_theme') || 'light';
    if (theme === 'dark') document.documentElement.classList.add('theme-dark');
    document.addEventListener('DOMContentLoaded', function () {
        var btn = document.getElementById('theme-toggle');
        var icon = document.getElementById('theme-icon');
        var label = document.getElementById('theme-label');
        if (btn) {
            function updateThemeUI() {
                var isDark = document.documentElement.classList.contains('theme-dark');
                if (icon) icon.className = isDark ? 'fas fa-sun' : 'fas fa-moon';
                if (label) label.textContent = isDark ? 'Light' : 'Dark';
            }
            updateThemeUI();
            btn.addEventListener('click', function () {
                document.documentElement.classList.toggle('theme-dark');
                localStorage.setItem('k8s_manager_theme', document.documentElement.classList.contains('theme-dark') ? 'dark' : 'light');
                updateThemeUI();
            });
        }
        var searchInput = document.getElementById('global-search');
        var searchResults = document.getElementById('global-search-results');
        if (searchInput && searchResults) {
            var debounce = null;
            searchInput.addEventListener('input', function () {
                clearTimeout(debounce);
                var q = searchInput.value.trim();
                if (q.length < 2) { searchResults.classList.add('d-none'); return; }
                debounce = setTimeout(function () {
                    var ns = (document.getElementById('namespace-selector') || {}).value || 'default';
                    fetch('/api/search?q=' + encodeURIComponent(q) + '&namespace=' + encodeURIComponent(ns))
                        .then(function (r) { return r.json(); })
                        .then(function (data) {
                            var items = [];
                            (data.pods || []).forEach(function (p) { items.push({ kind: 'Pod', name: p.name, ns: p.namespace, url: '/ui/pods?ns=' + p.namespace }); });
                            (data.deployments || []).forEach(function (d) { items.push({ kind: 'Deployment', name: d.name, ns: d.namespace, url: '/ui/deployments?ns=' + d.namespace }); });
                            (data.services || []).forEach(function (s) { items.push({ kind: 'Service', name: s.name, ns: s.namespace, url: '/ui/config' }); });
                            if (items.length === 0) { searchResults.innerHTML = '<div class="p-2 small text-muted">No results</div>'; }
                            else { searchResults.innerHTML = items.slice(0, 20).map(function (it) { return '<a class="d-block px-2 py-1 small text-decoration-none text-dark" href="' + it.url + '">' + escapeHtml(it.kind) + ': ' + escapeHtml(it.name) + '</a>'; }).join(''); }
                            searchResults.classList.remove('d-none');
                        })
                        .catch(function () { searchResults.classList.add('d-none'); });
                }, 200);
            });
            searchInput.addEventListener('blur', function () { setTimeout(function () { searchResults.classList.add('d-none'); }, 150); });
        }
        var favWrap = document.getElementById('sidebar-favorites-wrap');
        var favList = document.getElementById('sidebar-favorites-list');
        var favAdd = document.getElementById('sidebar-fav-add');
        var favKey = 'k8s_manager_fav_ns';
        function getFav() { try { return JSON.parse(localStorage.getItem(favKey) || '[]'); } catch (e) { return []; } }
        function setFav(arr) { localStorage.setItem(favKey, JSON.stringify(arr)); }
        function renderFav() {
            var arr = getFav();
            if (!favList) return;
            favList.innerHTML = arr.map(function (ns) {
                return '<a href="#" class="d-block small text-decoration-none sidebar-fav-link" data-ns="' + escapeHtml(ns) + '">' + escapeHtml(ns) + '</a>';
            }).join('');
            if (favWrap) favWrap.style.display = arr.length ? 'block' : 'none';
            favList.querySelectorAll('.sidebar-fav-link').forEach(function (a) {
                a.addEventListener('click', function (e) {
                    e.preventDefault();
                    var sel = document.getElementById('namespace-selector');
                    if (sel) sel.value = a.getAttribute('data-ns');
                });
            });
        }
        if (favAdd) {
            favAdd.addEventListener('click', function () {
                var sel = document.getElementById('namespace-selector');
                var ns = sel ? sel.value : '';
                if (!ns || ns === 'all') return;
                var arr = getFav();
                if (arr.indexOf(ns) >= 0) return;
                arr.push(ns);
                setFav(arr);
                renderFav();
            });
        }
        renderFav();
        var ctxEl = document.getElementById('sidebar-context');
        if (ctxEl) {
            fetch('/api/contexts').then(function (r) { return r.json(); }).then(function (d) {
                if (d.current) ctxEl.textContent = 'Context: ' + d.current;
            }).catch(function () {});
        }
    });
})();

/**
 * Показывает toast-уведомление (создаёт контейнер при первом вызове).
 * @param {string} message
 * @param {string} type - 'info' | 'success' | 'warning' | 'error' | 'danger'
 */
function showToast(message, type) {
    type = normalizeToastType(type);
    let container = document.getElementById('toast-container');
    if (!container) {
        container = document.createElement('div');
        container.id = 'toast-container';
        container.className = 'toast-container position-fixed bottom-0 end-0 p-3';
        container.style.zIndex = '1100';
        document.body.appendChild(container);
    }
    const alertClass = type === 'error' ? 'danger' : type;
    const icon = { success: 'fa-check-circle', error: 'fa-exclamation-circle', warning: 'fa-exclamation-triangle', info: 'fa-info-circle' }[type] || 'fa-info-circle';
    const el = document.createElement('div');
    el.className = `toast-message alert alert-${alertClass} alert-dismissible fade show shadow-sm`;
    el.setAttribute('role', 'alert');
    el.innerHTML = `<i class="fas ${icon} me-2"></i>${escapeHtml(message)}<button type="button" class="btn-close" data-bs-dismiss="alert"></button>`;
    container.appendChild(el);
    setTimeout(function () {
        el.classList.remove('show');
        setTimeout(function () { el.remove(); }, 150);
    }, 4000);
}

/**
 * Показывает модальное подтверждение. Возвращает Promise<boolean> (true = подтверждено).
 * @param {string} title
 * @param {string} body - HTML или текст
 * @param {string} [confirmLabel='Удалить']
 * @param {string} [confirmClass='btn-danger']
 */
function confirmDialog(title, body, confirmLabel, confirmClass) {
    confirmLabel = confirmLabel || 'Удалить';
    confirmClass = confirmClass || 'btn-danger';
    return new Promise(function (resolve) {
        var id = 'confirm-modal-' + Date.now();
        var modalHtml =
            '<div class="modal fade" id="' + id + '" tabindex="-1">' +
            '  <div class="modal-dialog">' +
            '    <div class="modal-content">' +
            '      <div class="modal-header">' +
            '        <h5 class="modal-title">' + escapeHtml(title) + '</h5>' +
            '        <button type="button" class="btn-close" data-bs-dismiss="modal"></button>' +
            '      </div>' +
            '      <div class="modal-body">' + (body || '') + '</div>' +
            '      <div class="modal-footer">' +
            '        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Отмена</button>' +
            '        <button type="button" class="btn ' + confirmClass + ' btn-confirm-ok">' + escapeHtml(confirmLabel) + '</button>' +
            '      </div>' +
            '    </div>' +
            '  </div>' +
            '</div>';
        var wrap = document.createElement('div');
        wrap.innerHTML = modalHtml;
        var el = wrap.firstElementChild;
        document.body.appendChild(el);
        var modal = new bootstrap.Modal(el);
        el.addEventListener('hidden.bs.modal', function () {
            el.remove();
            resolve(false);
        }, { once: true });
        el.querySelector('.btn-confirm-ok').addEventListener('click', function () {
            modal.hide();
            resolve(true);
        });
        modal.show();
    });
}

function getCSRFToken() {
    if (typeof csrfTokenFromCookies === 'function') {
        return csrfTokenFromCookies(document.cookie);
    }
    const name = 'k8s_manager_csrf=';
    const ca = document.cookie.split(';');
    for (let i = 0; i < ca.length; i++) {
        let c = ca[i].trim();
        if (c.indexOf(name) === 0) return c.substring(name.length);
    }
    return '';
}

/**
 * Обёртка над fetch: проверяет response.ok, возвращает JSON или бросает ошибку.
 * Для POST/PUT/PATCH/DELETE добавляет заголовок X-CSRF-Token из cookie.
 * @param {string} url
 * @param {RequestInit} [options]
 * @returns {Promise<object>}
 */
async function apiFetch(url, options = {}) {
    const method = (options.method || 'GET').toUpperCase();
    if (['POST', 'PUT', 'PATCH', 'DELETE'].indexOf(method) >= 0) {
        options.headers = options.headers || {};
        var headers = new Headers(options.headers);
        if (!headers.has('X-CSRF-Token')) {
            var token = getCSRFToken();
            if (token) headers.set('X-CSRF-Token', token);
        }
        options.headers = headers;
    }
    const res = await fetch(url, options);
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        const msg = data.error || data.message || res.statusText || `HTTP ${res.status}`;
        if (typeof showToast === 'function') showToast(msg, 'error');
        throw new Error(msg);
    }
    return data;
}

/**
 * Загружает список namespace из API и заполняет селект.
 * @param {string} selectId - id элемента <select>
 * @param {boolean} includeAll - добавить опцию "All Namespaces"
 */
async function loadNamespacesIntoSelect(selectId, includeAll = true) {
    const sel = document.getElementById(selectId);
    if (!sel) return;
    try {
        const data = await apiFetch('/api/namespaces');
        const namespaces = data.namespaces || [];
        const current = sel.value;
        sel.innerHTML = '';
        const add = (val, label) => {
            const o = document.createElement('option');
            o.value = val;
            o.textContent = label || val;
            sel.appendChild(o);
        };
        add('default', 'default');
        add('market', 'market');
        add('kube-system', 'kube-system');
        const known = new Set(['default', 'market', 'kube-system']);
        namespaces.forEach(ns => {
            const name = ns.name || ns;
            if (!known.has(name)) {
                add(name, name);
                known.add(name);
            }
        });
        if (includeAll) add('all', 'All Namespaces');
        if (current && Array.from(sel.options).some(o => o.value === current)) {
            sel.value = current;
        }
    } catch (e) {
        console.warn('Could not load namespaces:', e);
    }
}
