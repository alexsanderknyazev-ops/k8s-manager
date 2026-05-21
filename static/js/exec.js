(function() {
    function getEl(id) {
        return document.getElementById(id);
    }

    function isEmbed() {
        return document.body.dataset.embed === '1' ||
            new URLSearchParams(window.location.search).get('embed') === '1';
    }

    function parseQuery() {
        const params = new URLSearchParams(window.location.search);
        return {
            namespace: params.get('namespace') || '',
            pod: params.get('pod') || '',
            container: params.get('container') || ''
        };
    }

    document.addEventListener('DOMContentLoaded', function() {
        const q = parseQuery();
        if (q.namespace) getEl('exec-namespace').value = q.namespace;
        if (q.pod) getEl('exec-pod').value = q.pod;
        if (q.container) getEl('exec-container').value = q.container;
        if (q.namespace && q.pod) {
            getEl('exec-pod-label').textContent = `${q.namespace}/${q.pod}`;
        }

        const podExec = PodExec.create({
            statusClass: document.body.classList.contains('exec-embed')
                ? 'small text-muted'
                : 'small mt-2 text-muted'
        });

        if (isEmbed() && q.namespace && q.pod) {
            podExec.connect();
        }
    });
})();
