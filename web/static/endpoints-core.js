// Endpoints Core JavaScript - 核心功能和数据管理

let currentEndpoints = [];
let editingEndpointName = null;
let endpointModal = null;
let originalAuthValue = '';
let isAuthVisible = false;

let specialSortableInstance = null;
let generalSortableInstance = null;

document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    endpointModal = new bootstrap.Modal(document.getElementById('endpointModal'));
    loadEndpoints();
    
    // Auto-refresh every 30 seconds (only status updates)
    setInterval(refreshEndpointStatus, 30000);
});

function initializeSortable() {
    // Destroy existing sortable instances
    if (specialSortableInstance) {
        specialSortableInstance.destroy();
        specialSortableInstance = null;
    }
    if (generalSortableInstance) {
        generalSortableInstance.destroy();
        generalSortableInstance = null;
    }

    // Initialize special endpoint list drag-and-drop sorting
    const specialTbody = document.getElementById('special-endpoint-list');
    if (specialTbody && specialTbody.children.length > 0) {
        specialSortableInstance = new Sortable(specialTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'special-endpoints', // Restrict to special endpoint group
            onStart: function(evt) {
                document.body.style.cursor = 'grabbing';
            },
            onEnd: function (evt) {
                document.body.style.cursor = '';
                reorderEndpoints();
            }
        });
    }
    
    // Initialize general endpoint list drag-and-drop sorting
    const generalTbody = document.getElementById('general-endpoint-list');
    if (generalTbody && generalTbody.children.length > 0) {
        generalSortableInstance = new Sortable(generalTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'general-endpoints', // Restrict to general endpoint group
            onStart: function(evt) {
                document.body.style.cursor = 'grabbing';
            },
            onEnd: function (evt) {
                document.body.style.cursor = '';
                reorderEndpoints();
            }
        });
    }
}

function loadEndpoints() {
    fetch('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            currentEndpoints = data.endpoints;
            rebuildTable(currentEndpoints);
        })
        .catch(error => {
            console.error('Failed to load endpoints:', error);
            showAlert('Failed to load endpoints', 'danger');
        });
}

function refreshEndpointStatus() {
    fetch('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            // Only update status and statistics, not the full table
            data.endpoints.forEach(endpoint => {
                // Try to find in special endpoint list
                let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                if (!row) {
                    // If not found, search in general endpoint list
                    row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                }
                if (row) {
                    updateEndpointRowStatus(row, endpoint);
                }
            });
        })
        .catch(error => console.error('Failed to refresh endpoint status:', error));
}

function updateEndpointRowStatus(row, endpoint) {
    const statusCell = row.children[9]; // Adjust index: added proxy column, was 8, now 9
    
    // Update status
    let statusBadge = '';
    if (endpoint.status === 'active') {
        statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> 活跃</span>';
    } else if (endpoint.status === 'inactive') {
        statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> 不可用</span>';
    } else {
        statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> 检测中</span>';
    }
    statusCell.innerHTML = statusBadge;
}

function refreshTable() {
    // Reload endpoint data instead of refreshing the entire page
    loadEndpoints();
}