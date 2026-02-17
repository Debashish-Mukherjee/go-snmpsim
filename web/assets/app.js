// API Base URL
const API_BASE = '/api';

// Application State
const appState = {
    isRunning: false,
    currentTab: 'dashboard',
    lastStatus: null,
    testResults: null,
    statusRefreshInterval: null,
};

// Initialize app on page load
document.addEventListener('DOMContentLoaded', () => {
    initializeTabs();
    initializeEventListeners();
    refreshStatus();
    startStatusRefresh();
    loadWorkloads();
});

// ===== TAB MANAGEMENT =====
function initializeTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            const tabName = e.target.dataset.tab;
            switchTab(tabName);
        });
    });
}

function switchTab(tabName) {
    // Hide all tabs
    document.querySelectorAll('.tab-panel').forEach(panel => {
        panel.classList.remove('active');
    });
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('active');
    });

    // Show selected tab
    document.getElementById(tabName).classList.add('active');
    document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');
    appState.currentTab = tabName;
}

// ===== EVENT LISTENERS =====
function initializeEventListeners() {
    // Dashboard Controls
    document.getElementById('btn-start').addEventListener('click', startSimulator);
    document.getElementById('btn-stop').addEventListener('click', stopSimulator);

    // Test Controls
    document.getElementById('btn-run-test').addEventListener('click', runTest);
    document.getElementById('btn-clear-results').addEventListener('click', clearResults);

    // Workload Controls
    document.getElementById('btn-save-workload').addEventListener('click', saveWorkload);

    // Quick workload loading
    document.addEventListener('click', (e) => {
        if (e.target.classList.contains('workload-quick-btn')) {
            loadWorkloadAndSwitch(e.target.dataset.name);
        }
        if (e.target.classList.contains('workload-load-btn')) {
            loadWorkloadAndSwitch(e.target.dataset.name);
        }
        if (e.target.classList.contains('workload-delete-btn')) {
            deleteWorkload(e.target.dataset.name);
        }
    });
}

// ===== STATUS & MONITORING =====
function startStatusRefresh() {
    appState.statusRefreshInterval = setInterval(refreshStatus, 2000);
}

function stopStatusRefresh() {
    if (appState.statusRefreshInterval) {
        clearInterval(appState.statusRefreshInterval);
    }
}

async function refreshStatus() {
    try {
        const response = await fetch(`${API_BASE}/status`);
        const status = await response.json();
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

    if (status.is_running) {
        appState.isRunning = true;
        statusBadge.textContent = 'Running';
        statusBadge.className = 'badge badge-running';
        runButton.disabled = true;
        stopButton.disabled = false;
    } else {
        appState.isRunning = false;
        statusBadge.textContent = 'Stopped';
        statusBadge.className = 'badge badge-stopped';
        runButton.disabled = false;
        stopButton.disabled = true;
    }

    document.getElementById('status-devices').textContent = status.total_devices || 0;
    document.getElementById('status-ports').textContent = status.port_start && status.port_end 
        ? `${status.port_start}-${status.port_end}` 
        : '-';
    document.getElementById('status-address').textContent = status.listen_addr || '-';
    document.getElementById('status-uptime').textContent = status.uptime || '-';
    document.getElementById('status-polls').textContent = status.total_polls || 0;
    document.getElementById('status-latency').textContent = status.avg_latency || '-';
}

// ===== SIMULATOR CONTROL =====
async function startSimulator() {
    const portStart = parseInt(document.getElementById('config-port-start').value);
    const portEnd = parseInt(document.getElementById('config-port-end').value);
    const devices = parseInt(document.getElementById('config-devices').value);
    const listenAddr = document.getElementById('config-listen').value;

    if (portStart >= portEnd) {
        alert('Port start must be less than port end');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/start`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                port_start: portStart,
                port_end: portEnd,
                devices: devices,
                listen_addr: listenAddr,
            }),
        });

        if (!response.ok) {
            throw new Error('Failed to start simulator');
        }

        const result = await response.json();
        showNotification(result.message, 'success');
        refreshStatus();
    } catch (error) {
        showNotification(`Error: ${error.message}`, 'error');
    }
}

async function stopSimulator() {
    try {
        const response = await fetch(`${API_BASE}/stop`, { method: 'POST' });
        if (!response.ok) {
            throw new Error('Failed to stop simulator');
        }

        const result = await response.json();
        showNotification(result.message, 'success');
        refreshStatus();
    } catch (error) {
        showNotification(`Error: ${error.message}`, 'error');
    }
}

// ===== SNMP TESTING =====
async function runTest() {
    const testType = document.getElementById('test-type').value;
    const oidsText = document.getElementById('test-oids').value;
    const portStart = parseInt(document.getElementById('test-port-start').value);
    const portEnd = parseInt(document.getElementById('test-port-end').value);
    const community = document.getElementById('test-community').value;
    const timeout = parseInt(document.getElementById('test-timeout').value);
    const maxRepeaters = parseInt(document.getElementById('test-repeaters').value);

    if (!oidsText.trim()) {
        alert('Please enter at least one OID');
        return;
    }

    const oids = oidsText
        .split('\n')
        .map(oid => oid.trim())
        .filter(oid => oid.length > 0);

    const statusDiv = document.getElementById('test-status');
    statusDiv.textContent = '⏳ Running tests...';
    statusDiv.className = 'test-status running';

    try {
        const response = await fetch(`${API_BASE}/test/snmp`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                test_type: testType,
                oids: oids,
                port_start: portStart,
                port_end: portEnd,
                community: community,
                timeout: timeout,
                max_repeaters: maxRepeaters,
            }),
        });

        if (!response.ok) {
            throw new Error('Test execution failed');
        }

        const results = await response.json();
        appState.testResults = results;
        displayTestResults(results);

        statusDiv.textContent = `✅ Tests completed: ${results.success_count}/${results.total_tests} successful`;
        statusDiv.className = 'test-status success';
    } catch (error) {
        statusDiv.textContent = `❌ Error: ${error.message}`;
        statusDiv.className = 'test-status error';
        showNotification(`Test error: ${error.message}`, 'error');
    }
}

function displayTestResults(results) {
    // Show results summary
    const summaryDiv = document.getElementById('results-summary');
    summaryDiv.style.display = 'block';

    document.getElementById('result-total').textContent = results.total_tests;
    document.getElementById('result-success').textContent = results.success_count;
    document.getElementById('result-failed').textContent = results.failure_count;
    document.getElementById('result-rate').textContent = results.success_rate.toFixed(1) + '%';
    document.getElementById('result-avg').textContent = results.avg_latency_ms.toFixed(2) + 'ms';
    document.getElementById('result-min').textContent = results.min_latency_ms.toFixed(2) + 'ms';
    document.getElementById('result-max').textContent = results.max_latency_ms.toFixed(2) + 'ms';

    // Show results table
    const tableDiv = document.getElementById('results-table-container');
    const tbody = document.getElementById('results-tbody');
    tableDiv.style.display = 'block';
    document.getElementById('results-empty').style.display = 'none';

    tbody.innerHTML = '';
    results.results.forEach(result => {
        const row = tbody.insertRow();
        row.innerHTML = `
            <td>${result.port}</td>
            <td>${result.oid}</td>
            <td class="${result.success ? 'success' : 'failure'}">
                ${result.success ? '✅ Success' : '❌ Failed'}
            </td>
            <td>${result.value}</td>
            <td>${result.type}</td>
            <td>${result.latency_ms.toFixed(2)}</td>
        `;
    });
}

function clearResults() {
    document.getElementById('results-summary').style.display = 'none';
    document.getElementById('results-table-container').style.display = 'none';
    document.getElementById('results-empty').style.display = 'block';
    document.getElementById('test-status').textContent = '';
    appState.testResults = null;
}

// ===== WORKLOAD MANAGEMENT =====
async function loadWorkloads() {
    try {
        const response = await fetch(`${API_BASE}/workloads`);
        const workloads = await response.json();

        // Update quick workload list
        const quickList = document.getElementById('workload-list');
        quickList.innerHTML = '';

        if (workloads && workloads.length > 0) {
            workloads.forEach(w => {
                const btn = document.createElement('button');
                btn.className = 'workload-quick-btn';
                btn.dataset.name = w.name;
                btn.innerHTML = `
                    <div>
                        <strong>${w.name}</strong>
                        <div style="font-size: 0.8rem; color: #6b7280;">${w.description}</div>
                    </div>
                `;
                quickList.appendChild(btn);
            });
        } else {
            quickList.innerHTML = '<p style="color: #6b7280; text-align: center;">No saved workloads</p>';
        }

        // Update workload list in workloads tab
        const workloadList = document.getElementById('saved-workloads');
        workloadList.innerHTML = '';

        if (workloads && workloads.length > 0) {
            workloads.forEach(w => {
                const item = document.createElement('div');
                item.className = 'workload-item';
                item.innerHTML = `
                    <div>
                        <strong>${w.name}</strong>
                        <div style="font-size: 0.8rem; color: #6b7280;">${w.description}</div>
                    </div>
                    <div class="workload-actions">
                        <button class="workload-load-btn" data-name="${w.name}">📂 Load</button>
                        <button class="workload-delete-btn" data-name="${w.name}">🗑️ Delete</button>
                    </div>
                `;
                workloadList.appendChild(item);
            });
        } else {
            workloadList.innerHTML = '<p style="color: #6b7280; text-align: center;">No saved workloads</p>';
        }
    } catch (error) {
        console.error('Error loading workloads:', error);
    }
}

async function saveWorkload() {
    const name = document.getElementById('workload-name').value.trim();
    const description = document.getElementById('workload-description').value.trim();

    if (!name) {
        alert('Please enter a workload name');
        return;
    }

    const testType = document.getElementById('test-type').value;
    const oidsText = document.getElementById('test-oids').value;
    const portStart = parseInt(document.getElementById('test-port-start').value);
    const portEnd = parseInt(document.getElementById('test-port-end').value);
    const community = document.getElementById('test-community').value;
    const timeout = parseInt(document.getElementById('test-timeout').value);

    const oids = oidsText
        .split('\n')
        .map(oid => oid.trim())
        .filter(oid => oid.length > 0);

    if (oids.length === 0) {
        alert('Please configure at least one OID');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/workloads/save`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name: name,
                description: description,
                test_type: testType,
                oids: oids,
                port_start: portStart,
                port_end: portEnd,
                community: community,
                timeout: timeout,
            }),
        });

        if (!response.ok) {
            throw new Error('Failed to save workload');
        }

        showNotification('Workload saved successfully!', 'success');
        document.getElementById('workload-name').value = '';
        document.getElementById('workload-description').value = '';
        loadWorkloads();
    } catch (error) {
        showNotification(`Error saving workload: ${error.message}`, 'error');
    }
}

async function loadWorkloadAndSwitch(name) {
    try {
        const response = await fetch(`${API_BASE}/workloads/load?name=${encodeURIComponent(name)}`);
        if (!response.ok) {
            throw new Error('Failed to load workload');
        }

        const workload = await response.json();

        // Populate test configuration
        document.getElementById('test-type').value = workload.test_type;
        document.getElementById('test-oids').value = workload.oids.join('\n');
        document.getElementById('test-port-start').value = workload.port_start;
        document.getElementById('test-port-end').value = workload.port_end;
        document.getElementById('test-community').value = workload.community;
        document.getElementById('test-timeout').value = workload.timeout;

        // Switch to test tab
        switchTab('test');
        showNotification(`Loaded workload: ${name}`, 'success');
    } catch (error) {
        showNotification(`Error loading workload: ${error.message}`, 'error');
    }
}

async function deleteWorkload(name) {
    if (!confirm(`Delete workload "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/workloads/delete?name=${encodeURIComponent(name)}`, {
            method: 'DELETE',
        });

        if (!response.ok) {
            throw new Error('Failed to delete workload');
        }

        showNotification('Workload deleted', 'success');
        loadWorkloads();
    } catch (error) {
        showNotification(`Error deleting workload: ${error.message}`, 'error');
    }
}

// ===== UTILITIES =====
function showNotification(message, type) {
    // Simple notification (could be enhanced with a toast library)
    console.log(`[${type.toUpperCase()}] ${message}`);
    
    // You could implement a toast notification here
}

// Cleanup on page unload
window.addEventListener('beforeunload', () => {
    stopStatusRefresh();
});
