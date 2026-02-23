const API_BASE = '/api';
const RESULTS_PAGE_SIZE = 100;

const appState = {
    isRunning: false,
    currentTab: 'dashboard',
    lastStatus: null,
    testResults: null,
    statusRefreshInterval: null,
    testProgressInterval: null,
    activeTest: null,
    currentResultsPage: 1,
};

document.addEventListener('DOMContentLoaded', () => {
    initializeTabs();
    initializeEventListeners();
    refreshStatus();
    startStatusRefresh();
    refreshTestResults();
    loadWorkloads();
});

function initializeTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => switchTab(e.currentTarget.dataset.tab));
    });
}

function switchTab(tabName) {
    document.querySelectorAll('.tab-panel').forEach(panel => panel.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));

    const panel = document.getElementById(tabName);
    const tab = document.querySelector(`[data-tab="${tabName}"]`);
    if (panel) panel.classList.add('active');
    if (tab) tab.classList.add('active');
    appState.currentTab = tabName;
}

function initializeEventListeners() {
    document.getElementById('btn-start').addEventListener('click', startSimulator);
    document.getElementById('btn-stop').addEventListener('click', stopSimulator);
    document.getElementById('btn-run-test').addEventListener('click', runTest);
    document.getElementById('btn-cancel-test').addEventListener('click', cancelTest);
    document.getElementById('btn-clear-results').addEventListener('click', clearResults);
    document.getElementById('btn-save-workload').addEventListener('click', saveWorkload);

    document.addEventListener('click', (e) => {
        const target = e.target;
        if (!(target instanceof HTMLElement)) return;
        if (target.classList.contains('workload-quick-btn') || target.classList.contains('workload-load-btn')) {
            loadWorkloadAndSwitch(target.dataset.name);
        }
        if (target.classList.contains('workload-delete-btn')) {
            deleteWorkload(target.dataset.name);
        }
        if (target.classList.contains('pagination-prev')) {
            changeResultsPage(-1);
        }
        if (target.classList.contains('pagination-next')) {
            changeResultsPage(1);
        }
    });
}

function startStatusRefresh() {
    appState.statusRefreshInterval = setInterval(refreshStatus, 2000);
}

function stopStatusRefresh() {
    if (appState.statusRefreshInterval) {
        clearInterval(appState.statusRefreshInterval);
    }
}

async function apiFetch(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (!headers.has('Content-Type') && options.body) {
        headers.set('Content-Type', 'application/json');
    }
    const token = localStorage.getItem('snmpsim_api_token');
    if (token) {
        headers.set('X-API-Token', token);
    }

    const response = await fetch(`${API_BASE}${path}`, { ...options, headers });
    if (!response.ok) {
        let message = `${response.status} ${response.statusText}`;
        const body = await response.text();
        if (body) message = body;
        throw new Error(message.trim());
    }

    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
        return response.json();
    }
    return response.text();
}

async function refreshStatus() {
    try {
        const status = await apiFetch('/status');
        appState.lastStatus = status;
        updateStatusDisplay(status);
    } catch (error) {
        console.error('Error fetching status:', error);
    }
}

function updateStatusDisplay(status) {
    const statusBadge = document.getElementById('status-indicator');
    const runButton = document.getElementById('btn-start');
    const stopButton = document.getElementById('btn-stop');

    appState.isRunning = Boolean(status.is_running);
    statusBadge.textContent = appState.isRunning ? 'Running' : 'Stopped';
    statusBadge.className = appState.isRunning ? 'badge badge-running' : 'badge badge-stopped';
    runButton.disabled = appState.isRunning;
    stopButton.disabled = !appState.isRunning;

    setText('status-devices', status.total_devices || 0);
    setText('status-ports', status.port_start !== 0 || status.port_end !== 0 ? `${status.port_start}-${status.port_end}` : '-');
    setText('status-address', status.listen_addr || '-');
    setText('status-uptime', status.uptime || '-');
    setText('status-polls', status.total_polls || 0);
    setText('status-latency', status.avg_latency_ms ? `${status.avg_latency_ms} ms` : '-');

    updateDashboardMetrics(appState.testResults);
}

async function refreshTestResults() {
    try {
        const results = await apiFetch('/test/results');
        if (results && results.total_tests) {
            appState.testResults = results;
            displayTestResults(results);
            updateDashboardMetrics(results);
        }
    } catch (error) {
        console.error('Error fetching test results:', error);
    }
}

function updateDashboardMetrics(results) {
    if (!results || !results.total_tests) {
        setText('metric-success', '0');
        setText('metric-failed', '0');
        setText('metric-avg', '0ms');
        setText('metric-rate', '0%');
        return;
    }

    setText('metric-success', results.success_count || 0);
    setText('metric-failed', results.failure_count || 0);
    setText('metric-avg', `${Number(results.avg_latency_ms || 0).toFixed(2)}ms`);
    setText('metric-rate', `${Number(results.success_rate || 0).toFixed(1)}%`);
}

async function startSimulator() {
    const portStart = parseInt(document.getElementById('config-port-start').value, 10);
    const portEnd = parseInt(document.getElementById('config-port-end').value, 10);
    const devices = parseInt(document.getElementById('config-devices').value, 10);
    const listenAddr = document.getElementById('config-listen').value.trim();
    const snmprecFile = document.getElementById('config-snmprec').value.trim();

    if (!Number.isInteger(portStart) || !Number.isInteger(portEnd) || portStart <= 0 || portEnd <= 0 || portStart >= portEnd) {
        showNotification('Port range is invalid. Ensure port start < port end.', 'error');
        return;
    }
    if (!Number.isInteger(devices) || devices < 1 || devices > 100000) {
        showNotification('Device count must be between 1 and 100000.', 'error');
        return;
    }

    try {
        const result = await apiFetch('/start', {
            method: 'POST',
            body: JSON.stringify({
                port_start: portStart,
                port_end: portEnd,
                devices,
                listen_addr: listenAddr,
                snmprec_file: snmprecFile,
            }),
        });
        showNotification(result.message || 'Simulator started', 'success');
        await refreshStatus();
    } catch (error) {
        showNotification(`Start failed: ${error.message}`, 'error');
    }
}

async function stopSimulator() {
    try {
        const result = await apiFetch('/stop', { method: 'POST' });
        showNotification(result.message || 'Simulator stopped', 'success');
        await refreshStatus();
    } catch (error) {
        showNotification(`Stop failed: ${error.message}`, 'error');
    }
}

async function runTest() {
    const testType = document.getElementById('test-type').value;
    const oids = parseOIDList(document.getElementById('test-oids').value);
    const portStart = parseInt(document.getElementById('test-port-start').value, 10);
    const portEnd = parseInt(document.getElementById('test-port-end').value, 10);
    const community = document.getElementById('test-community').value.trim();
    const timeout = parseInt(document.getElementById('test-timeout').value, 10);
    const concurrency = parseInt(document.getElementById('test-concurrency').value, 10);
    const intervalSeconds = parseInt(document.getElementById('test-interval').value, 10);
    const durationSeconds = parseInt(document.getElementById('test-duration').value, 10);
    const maxRepeaters = parseInt(document.getElementById('test-repeaters').value, 10);

    if (!oids.length) {
        showNotification('Enter at least one OID.', 'error');
        return;
    }
    if (portEnd < portStart) {
        showNotification('Test port range is invalid.', 'error');
        return;
    }

    const statusDiv = document.getElementById('test-status');
    const runButton = document.getElementById('btn-run-test');
    const cancelButton = document.getElementById('btn-cancel-test');
    const progressDiv = document.getElementById('live-progress');
    const liveResultsDiv = document.getElementById('last-results-window');

    statusDiv.className = 'test-status running';
    statusDiv.textContent = 'Starting test job...';
    runButton.disabled = true;
    cancelButton.disabled = false;
    if (progressDiv) progressDiv.style.display = 'block';
    if (liveResultsDiv) liveResultsDiv.style.display = 'block';

    try {
        const payload = {
            test_type: testType,
            oids,
            port_start: portStart,
            port_end: portEnd,
            community,
            timeout,
            concurrency,
            interval_seconds: intervalSeconds,
            duration_seconds: durationSeconds,
            max_repeaters: maxRepeaters,
        };
        const jobResp = await apiFetch('/test/snmp', { method: 'POST', body: JSON.stringify(payload) });
        if (!jobResp.job_id) {
            throw new Error('No job id returned by server');
        }

        appState.activeTest = { jobId: jobResp.job_id };
        startTestProgressPolling(jobResp.job_id);
        showNotification(`Test job started: ${jobResp.job_id}`, 'info');
    } catch (error) {
        statusDiv.textContent = `Error: ${error.message}`;
        statusDiv.className = 'test-status error';
        runButton.disabled = false;
        cancelButton.disabled = true;
        showNotification(`Test start failed: ${error.message}`, 'error');
    }
}

function startTestProgressPolling(jobID) {
    stopTestProgress();
    appState.testProgressInterval = setInterval(async () => {
        try {
            const job = await apiFetch(`/test/jobs/${encodeURIComponent(jobID)}`);
            updateTestProgressUI(job);

            if (job.status === 'completed' || job.status === 'failed' || job.status === 'canceled') {
                stopTestProgress();
                document.getElementById('btn-run-test').disabled = false;
                document.getElementById('btn-cancel-test').disabled = true;
                appState.activeTest = null;

                if (job.results) {
                    appState.testResults = job.results;
                    displayTestResults(job.results);
                    updateDashboardMetrics(job.results);
                } else {
                    await refreshTestResults();
                }

                const statusDiv = document.getElementById('test-status');
                if (job.status === 'completed') {
                    statusDiv.className = 'test-status success';
                    statusDiv.textContent = `Completed: ${job.progress.success_count}/${job.progress.completed_jobs} successful`; 
                    showNotification('Test job completed', 'success');
                } else if (job.status === 'canceled') {
                    statusDiv.className = 'test-status error';
                    statusDiv.textContent = 'Test job canceled';
                    showNotification('Test job canceled', 'info');
                } else {
                    statusDiv.className = 'test-status error';
                    statusDiv.textContent = `Test job failed: ${job.error || 'unknown error'}`;
                    showNotification('Test job failed', 'error');
                }
            }
        } catch (error) {
            console.error('Error polling job:', error);
        }
    }, 1000);
}

function stopTestProgress() {
    if (appState.testProgressInterval) {
        clearInterval(appState.testProgressInterval);
        appState.testProgressInterval = null;
    }
}

async function cancelTest() {
    if (!appState.activeTest || !appState.activeTest.jobId) {
        return;
    }
    try {
        await apiFetch(`/test/jobs/${encodeURIComponent(appState.activeTest.jobId)}/cancel`, { method: 'POST' });
        showNotification('Cancellation requested', 'info');
    } catch (error) {
        showNotification(`Cancel failed: ${error.message}`, 'error');
    }
}

function updateTestProgressUI(job) {
    const statusDiv = document.getElementById('test-status');
    const p = job.progress || {};
    statusDiv.className = 'test-status running';
    statusDiv.textContent = `Running... iter ${p.current_iteration || 0}/${p.total_iterations || 0} | ${p.completed_jobs || 0}/${p.total_jobs || 0} jobs`;

    const total = Math.max(1, Number(p.total_jobs || 1));
    const completed = Number(p.completed_jobs || 0);
    const percent = Math.min(100, Math.round((completed / total) * 100));

    const progressBar = document.getElementById('progress-fill');
    const progressIter = document.getElementById('progress-iter');
    const progressRate = document.getElementById('progress-rate');
    const progressSuccess = document.getElementById('progress-success');
    const progressElapsed = document.getElementById('progress-elapsed');
    const progressRemaining = document.getElementById('progress-remaining');

    if (progressBar) progressBar.style.width = `${percent}%`;
    if (progressIter) progressIter.textContent = `Iter ${p.current_iteration || 0}/${p.total_iterations || 0}`;
    if (progressRate) progressRate.textContent = `Rate: ${Number(p.rate_per_second || 0).toFixed(1)}/s`;
    if (progressSuccess) progressSuccess.textContent = `✅ ${p.success_count || 0}`;
    if (progressElapsed) progressElapsed.textContent = `Elapsed: ${formatDuration(p.elapsed_seconds || 0)}`;
    if (progressRemaining) progressRemaining.textContent = `Remaining: ${formatDuration(p.remaining_seconds || 0)}`;
}

function displayTestResults(results) {
    appState.currentResultsPage = 1;
    const summaryDiv = document.getElementById('results-summary');
    summaryDiv.style.display = 'block';

    setText('result-total', results.total_tests || 0);
    setText('result-success', results.success_count || 0);
    setText('result-failed', results.failure_count || 0);
    setText('result-rate', `${Number(results.success_rate || 0).toFixed(1)}%`);
    setText('result-avg', `${Number(results.avg_latency_ms || 0).toFixed(2)}ms`);
    setText('result-min', `${Number(results.min_latency_ms || 0).toFixed(2)}ms`);
    setText('result-max', `${Number(results.max_latency_ms || 0).toFixed(2)}ms`);

    const liveResultsDiv = document.getElementById('last-results-window');
    if (liveResultsDiv) {
        liveResultsDiv.style.display = 'block';
        updateLiveResultsTable(results.results || []);
    }

    document.getElementById('results-table-container').style.display = 'block';
    document.getElementById('results-empty').style.display = 'none';
    renderResultsPage();
}

function renderResultsPage() {
    const results = appState.testResults;
    const tbody = document.getElementById('results-tbody');
    const paginationDiv = document.getElementById('results-pagination');
    tbody.innerHTML = '';

    if (!results || !Array.isArray(results.results)) {
        paginationDiv.textContent = '';
        return;
    }

    const totalRows = results.results.length;
    const totalPages = Math.max(1, Math.ceil(totalRows / RESULTS_PAGE_SIZE));
    appState.currentResultsPage = Math.min(Math.max(1, appState.currentResultsPage), totalPages);

    const start = (appState.currentResultsPage - 1) * RESULTS_PAGE_SIZE;
    const pageRows = results.results.slice(start, start + RESULTS_PAGE_SIZE);

    pageRows.forEach(result => {
        const row = document.createElement('tr');
        appendCell(row, result.port);
        appendCell(row, result.oid);
        appendStatusCell(row, Boolean(result.success));
        appendCell(row, result.value || '');
        appendCell(row, result.type || '');
        appendCell(row, Number(result.latency_ms || 0).toFixed(2));
        tbody.appendChild(row);
    });

    paginationDiv.innerHTML = '';
    const summary = document.createElement('div');
    summary.textContent = `Showing ${start + 1}-${Math.min(totalRows, start + RESULTS_PAGE_SIZE)} of ${totalRows}`;

    const controls = document.createElement('div');
    controls.className = 'pagination-controls';
    const prev = document.createElement('button');
    prev.className = 'pagination-btn pagination-prev';
    prev.disabled = appState.currentResultsPage <= 1;
    prev.textContent = 'Prev';

    const page = document.createElement('span');
    page.textContent = `Page ${appState.currentResultsPage}/${totalPages}`;

    const next = document.createElement('button');
    next.className = 'pagination-btn pagination-next';
    next.disabled = appState.currentResultsPage >= totalPages;
    next.textContent = 'Next';

    controls.appendChild(prev);
    controls.appendChild(page);
    controls.appendChild(next);

    paginationDiv.appendChild(summary);
    paginationDiv.appendChild(controls);
}

function changeResultsPage(direction) {
    appState.currentResultsPage += direction;
    renderResultsPage();
}

function updateLiveResultsTable(allResults) {
    const tbody = document.getElementById('live-results-tbody');
    if (!tbody) return;

    tbody.innerHTML = '';
    const lastResults = allResults.slice(-20);
    lastResults.forEach(result => {
        const row = document.createElement('tr');
        appendCell(row, result.port);
        appendCell(row, result.oid);
        appendCompactStatusCell(row, Boolean(result.success));
        appendCell(row, result.value || '');
        appendCell(row, `${Number(result.latency_ms || 0).toFixed(2)}ms`);
        tbody.appendChild(row);
    });
}

function clearResults() {
    document.getElementById('results-summary').style.display = 'none';
    document.getElementById('results-table-container').style.display = 'none';
    document.getElementById('results-empty').style.display = 'block';
    document.getElementById('test-status').textContent = '';
    const progressDiv = document.getElementById('live-progress');
    if (progressDiv) progressDiv.style.display = 'none';
    const liveResultsDiv = document.getElementById('last-results-window');
    if (liveResultsDiv) liveResultsDiv.style.display = 'none';

    appState.testResults = null;
    appState.activeTest = null;
    updateDashboardMetrics(null);
}

async function loadWorkloads() {
    try {
        const workloads = await apiFetch('/workloads');
        renderWorkloadLists(Array.isArray(workloads) ? workloads : []);
    } catch (error) {
        renderWorkloadLists([]);
        showNotification(`Failed to load workloads: ${error.message}`, 'error');
    }
}

function renderWorkloadLists(workloads) {
    const quickList = document.getElementById('workload-list');
    const workloadList = document.getElementById('saved-workloads');
    quickList.innerHTML = '';
    workloadList.innerHTML = '';

    if (!workloads.length) {
        const emptyQuick = document.createElement('p');
        emptyQuick.style.color = '#6b7280';
        emptyQuick.style.textAlign = 'center';
        emptyQuick.textContent = 'No saved workloads';
        quickList.appendChild(emptyQuick);

        const empty = emptyQuick.cloneNode(true);
        workloadList.appendChild(empty);
        return;
    }

    workloads.forEach(w => {
        const quick = document.createElement('button');
        quick.className = 'workload-quick-btn';
        quick.dataset.name = w.name;
        const quickTitle = document.createElement('strong');
        quickTitle.textContent = w.name;
        const quickDesc = document.createElement('div');
        quickDesc.style.fontSize = '0.8rem';
        quickDesc.style.color = '#6b7280';
        quickDesc.textContent = w.description || '';
        quick.appendChild(quickTitle);
        quick.appendChild(quickDesc);
        quickList.appendChild(quick);

        const item = document.createElement('div');
        item.className = 'workload-item';
        const left = document.createElement('div');
        const title = document.createElement('strong');
        title.textContent = w.name;
        const desc = document.createElement('div');
        desc.style.fontSize = '0.8rem';
        desc.style.color = '#6b7280';
        desc.textContent = w.description || '';
        left.appendChild(title);
        left.appendChild(desc);

        const actions = document.createElement('div');
        actions.className = 'workload-actions';
        const load = document.createElement('button');
        load.className = 'workload-load-btn';
        load.dataset.name = w.name;
        load.textContent = 'Load';
        const del = document.createElement('button');
        del.className = 'workload-delete-btn';
        del.dataset.name = w.name;
        del.textContent = 'Delete';

        actions.appendChild(load);
        actions.appendChild(del);
        item.appendChild(left);
        item.appendChild(actions);
        workloadList.appendChild(item);
    });
}

async function saveWorkload() {
    const name = document.getElementById('workload-name').value.trim();
    const description = document.getElementById('workload-description').value.trim();
    if (!/^[a-zA-Z0-9._-]+$/.test(name)) {
        showNotification('Workload name must use letters, numbers, dot, underscore, or dash.', 'error');
        return;
    }

    const oids = parseOIDList(document.getElementById('test-oids').value);
    if (!oids.length) {
        showNotification('Configure at least one OID before saving.', 'error');
        return;
    }

    try {
        await apiFetch('/workloads/save', {
            method: 'POST',
            body: JSON.stringify({
                name,
                description,
                test_type: document.getElementById('test-type').value,
                oids,
                port_start: parseInt(document.getElementById('test-port-start').value, 10),
                port_end: parseInt(document.getElementById('test-port-end').value, 10),
                community: document.getElementById('test-community').value.trim(),
                timeout: parseInt(document.getElementById('test-timeout').value, 10),
                concurrency: parseInt(document.getElementById('test-concurrency').value, 10),
                interval_seconds: parseInt(document.getElementById('test-interval').value, 10),
                duration_seconds: parseInt(document.getElementById('test-duration').value, 10),
                max_repeaters: parseInt(document.getElementById('test-repeaters').value, 10),
                snmprec_file: document.getElementById('test-snmprec-file').value.trim(),
            }),
        });

        showNotification('Workload saved successfully.', 'success');
        document.getElementById('workload-name').value = '';
        document.getElementById('workload-description').value = '';
        await loadWorkloads();
    } catch (error) {
        showNotification(`Save workload failed: ${error.message}`, 'error');
    }
}

async function loadWorkloadAndSwitch(name) {
    try {
        const workload = await apiFetch(`/workloads/load?name=${encodeURIComponent(name)}`);
        document.getElementById('test-type').value = workload.test_type || 'get';
        document.getElementById('test-oids').value = (workload.oids || []).join('\n');
        document.getElementById('test-port-start').value = workload.port_start || 20000;
        document.getElementById('test-port-end').value = workload.port_end || 20009;
        document.getElementById('test-community').value = workload.community || 'public';
        document.getElementById('test-timeout').value = workload.timeout || 5;
        document.getElementById('test-concurrency').value = workload.concurrency || 20;
        document.getElementById('test-interval').value = workload.interval_seconds || 5;
        document.getElementById('test-duration').value = workload.duration_seconds || 60;
        document.getElementById('test-repeaters').value = workload.max_repeaters || 10;
        document.getElementById('test-snmprec-file').value = workload.snmprec_file || '';

        switchTab('test');
        showNotification(`Loaded workload: ${name}`, 'success');
    } catch (error) {
        showNotification(`Load workload failed: ${error.message}`, 'error');
    }
}

async function deleteWorkload(name) {
    if (!confirm(`Delete workload "${name}"?`)) {
        return;
    }

    try {
        await apiFetch(`/workloads/delete?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
        showNotification('Workload deleted', 'success');
        await loadWorkloads();
    } catch (error) {
        showNotification(`Delete workload failed: ${error.message}`, 'error');
    }
}

function appendCell(row, text) {
    const td = document.createElement('td');
    td.textContent = text == null ? '' : String(text);
    row.appendChild(td);
}

function appendStatusCell(row, success) {
    const td = document.createElement('td');
    td.className = success ? 'success' : 'failure';
    td.textContent = success ? 'Success' : 'Failed';
    row.appendChild(td);
}

function appendCompactStatusCell(row, success) {
    const td = document.createElement('td');
    td.className = success ? 'success' : 'failure';
    td.textContent = success ? 'OK' : 'ERR';
    row.appendChild(td);
}

function parseOIDList(text) {
    return text
        .split('\n')
        .map(line => line.trim())
        .filter(Boolean);
}

function formatDuration(totalSeconds) {
    const sec = Math.max(0, Number(totalSeconds || 0));
    const minutes = Math.floor(sec / 60);
    const seconds = sec % 60;
    return `${minutes}m ${seconds}s`;
}

function setText(id, value) {
    const node = document.getElementById(id);
    if (node) node.textContent = String(value);
}

function showNotification(message, type) {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 4000);
}

window.addEventListener('beforeunload', () => {
    stopStatusRefresh();
    stopTestProgress();
});
