/**
 * Reusable pod exec terminal (xterm + WebSocket).
 * Used by /ui/exec and pod details modal.
 */
(function(global) {
    function createPodExec(cfg) {
        const ids = cfg.ids || {};
        const id = (key, fallback) => ids[key] || fallback;

        let term = null;
        let fitAddon = null;
        let ws = null;
        let connected = false;
        let resizeHandler = null;

        function el(key, fallback) {
            return document.getElementById(id(key, fallback));
        }

        function setStatus(msg, isError) {
            const statusEl = el('status', 'exec-status');
            if (!statusEl) return;
            statusEl.textContent = msg;
            const extra = statusEl.classList.contains('text-muted') ? '' : '';
            statusEl.className = (cfg.statusClass || 'small text-muted') + (isError ? ' text-danger' : '');
            if (!isError && !cfg.statusClass) {
                statusEl.classList.add('text-muted');
            }
        }

        function showTerminal() {
            const placeholder = el('placeholder', 'exec-placeholder');
            const container = el('terminalContainer', 'terminal-container');
            const disconnectBtn = el('disconnectBtn', 'exec-disconnect-btn');
            const connectBtn = el('connectBtn', 'exec-connect-btn');
            if (placeholder) placeholder.style.display = 'none';
            if (container) container.style.display = 'block';
            if (disconnectBtn) disconnectBtn.style.display = 'inline-block';
            if (connectBtn) connectBtn.disabled = true;
            if (term && fitAddon) {
                setTimeout(function() { fitAddon.fit(); }, 50);
            }
        }

        function hideTerminal() {
            const placeholder = el('placeholder', 'exec-placeholder');
            const container = el('terminalContainer', 'terminal-container');
            const disconnectBtn = el('disconnectBtn', 'exec-disconnect-btn');
            const connectBtn = el('connectBtn', 'exec-connect-btn');
            if (container) container.style.display = 'none';
            if (placeholder) placeholder.style.display = 'block';
            if (disconnectBtn) disconnectBtn.style.display = 'none';
            if (connectBtn) connectBtn.disabled = false;
            connected = false;
        }

        function sendResize() {
            if (ws && ws.readyState === WebSocket.OPEN && term && fitAddon) {
                fitAddon.fit();
                ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
            }
        }

        function getConnectionParams() {
            if (typeof cfg.getParams === 'function') {
                return cfg.getParams();
            }
            const nsEl = el('namespace', 'exec-namespace');
            const podEl = el('pod', 'exec-pod');
            const ctrEl = el('container', 'exec-container');
            return {
                namespace: (nsEl && nsEl.value ? nsEl.value : '').trim(),
                pod: (podEl && podEl.value ? podEl.value : '').trim(),
                container: (ctrEl && ctrEl.value ? ctrEl.value : '').trim()
            };
        }

        function disconnect() {
            if (ws) {
                try { ws.close(); } catch (_) {}
                ws = null;
            }
            hideTerminal();
            setStatus('Disconnected.');
        }

        function connect() {
            const { namespace, pod, container } = getConnectionParams();
            if (!namespace || !pod) {
                setStatus('Namespace and Pod are required.', true);
                return;
            }
            if (ws) {
                try { ws.close(); } catch (_) {}
                ws = null;
            }
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            let wsUrl = `${protocol}//${window.location.host}/api/pod/exec/ws?namespace=${encodeURIComponent(namespace)}&pod=${encodeURIComponent(pod)}`;
            if (container) {
                wsUrl += '&container=' + encodeURIComponent(container);
            }
            setStatus('Connecting...');
            const labelEl = el('podLabel', 'exec-pod-label');
            if (labelEl) labelEl.textContent = `${namespace}/${pod}`;

            ws = new WebSocket(wsUrl);
            ws.binaryType = 'arraybuffer';

            ws.onopen = function() {
                connected = true;
                const terminalEl = el('terminal', 'terminal');
                if (!term && terminalEl) {
                    term = new Terminal({
                        cursorBlink: true,
                        theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
                        fontSize: 14,
                        fontFamily: 'Menlo, Monaco, "Courier New", monospace'
                    });
                    fitAddon = new FitAddon.FitAddon();
                    term.loadAddon(fitAddon);
                    term.open(terminalEl);
                    term.onData(function(data) {
                        if (ws && ws.readyState === WebSocket.OPEN) {
                            ws.send(data);
                        }
                    });
                    if (!resizeHandler) {
                        resizeHandler = function() {
                            if (fitAddon && connected) sendResize();
                        };
                        window.addEventListener('resize', resizeHandler);
                    }
                }
                if (term) term.clear();
                setStatus('Connected.');
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

        function bindButtons() {
            const connectBtn = el('connectBtn', 'exec-connect-btn');
            const disconnectBtn = el('disconnectBtn', 'exec-disconnect-btn');
            if (connectBtn && !connectBtn._podExecBound) {
                connectBtn._podExecBound = true;
                connectBtn.addEventListener('click', connect);
            }
            if (disconnectBtn && !disconnectBtn._podExecBound) {
                disconnectBtn._podExecBound = true;
                disconnectBtn.addEventListener('click', disconnect);
            }
        }

        bindButtons();

        return {
            connect: connect,
            disconnect: disconnect,
            destroy: function() {
                disconnect();
                if (resizeHandler) {
                    window.removeEventListener('resize', resizeHandler);
                    resizeHandler = null;
                }
                if (term) {
                    try { term.dispose(); } catch (_) {}
                    term = null;
                    fitAddon = null;
                }
            }
        };
    }

    global.PodExec = { create: createPodExec };
})(window);
