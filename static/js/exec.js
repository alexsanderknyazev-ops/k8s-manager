(function() {
    let term = null;
    let fitAddon = null;
    let ws = null;
    let connected = false;

    function getEl(id) {
        return document.getElementById(id);
    }

    function setStatus(msg, isError) {
        const el = getEl('exec-status');
        el.textContent = msg;
        el.className = 'small mt-2 ' + (isError ? 'text-danger' : 'text-muted');
    }

    function parseQuery() {
        const params = new URLSearchParams(window.location.search);
        return {
            namespace: params.get('namespace') || '',
            pod: params.get('pod') || '',
            container: params.get('container') || ''
        };
    }

    function showTerminal() {
        getEl('exec-placeholder').style.display = 'none';
        getEl('terminal-container').style.display = 'block';
        getEl('exec-disconnect-btn').style.display = 'inline-block';
        getEl('exec-connect-btn').disabled = true;
        if (term && fitAddon) {
            setTimeout(function() { fitAddon.fit(); }, 50);
        }
    }

    function hideTerminal() {
        getEl('terminal-container').style.display = 'none';
        getEl('exec-placeholder').style.display = 'block';
        getEl('exec-disconnect-btn').style.display = 'none';
        getEl('exec-connect-btn').disabled = false;
        connected = false;
    }

    function disconnect() {
        if (ws) {
            try { ws.close(); } catch (_) {}
            ws = null;
        }
        hideTerminal();
        setStatus('Disconnected.');
    }

    function sendResize() {
        if (ws && ws.readyState === WebSocket.OPEN && term && fitAddon) {
            fitAddon.fit();
            ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
        }
    }

    function connect() {
        const namespace = getEl('exec-namespace').value.trim();
        const pod = getEl('exec-pod').value.trim();
        const container = getEl('exec-container').value.trim();
        if (!namespace || !pod) {
            setStatus('Namespace and Pod are required.', true);
            return;
        }
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        let wsUrl = `${protocol}//${window.location.host}/api/pod/exec/ws?namespace=${encodeURIComponent(namespace)}&pod=${encodeURIComponent(pod)}`;
        if (container) {
            wsUrl += '&container=' + encodeURIComponent(container);
        }
        setStatus('Connecting...');
        getEl('exec-pod-label').textContent = `${namespace}/${pod}`;

        ws = new WebSocket(wsUrl);
        ws.binaryType = 'arraybuffer';

        ws.onopen = function() {
            connected = true;
            if (!term) {
                term = new Terminal({
                    cursorBlink: true,
                    theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
                    fontSize: 14,
                    fontFamily: 'Menlo, Monaco, "Courier New", monospace'
                });
                fitAddon = new FitAddon.FitAddon();
                term.loadAddon(fitAddon);
                term.open(getEl('terminal'));
                term.onData(function(data) {
                    if (ws && ws.readyState === WebSocket.OPEN) {
                        ws.send(data);
                    }
                });
            }
            term.clear();
            setStatus('Connected. Type in the terminal below.');
            showTerminal();
            setTimeout(sendResize, 100);
        };

        ws.onmessage = function(ev) {
            if (!term) return;
            const data = ev.data;
            if (data instanceof ArrayBuffer) {
                term.write(new Uint8Array(data));
            } else if (typeof data === 'string') {
                term.write(data);
            }
        };

        ws.onerror = function() {
            setStatus('WebSocket error.', true);
        };

        ws.onclose = function() {
            if (connected) {
                setStatus('Connection closed.', true);
            }
            ws = null;
            hideTerminal();
        };
    }

    document.addEventListener('DOMContentLoaded', function() {
        const q = parseQuery();
        if (q.namespace) getEl('exec-namespace').value = q.namespace;
        if (q.pod) getEl('exec-pod').value = q.pod;
        if (q.container) getEl('exec-container').value = q.container;

        getEl('exec-connect-btn').addEventListener('click', connect);
        getEl('exec-disconnect-btn').addEventListener('click', disconnect);

        window.addEventListener('resize', function() {
            if (fitAddon && connected) sendResize();
        });
    });
})();
