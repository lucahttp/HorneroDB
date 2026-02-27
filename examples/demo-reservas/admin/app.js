// HorneroDB Admin Demo App
let token = localStorage.getItem('demo_admin_token');
let user = null;

document.addEventListener('DOMContentLoaded', () => {
    checkAuth();
    setupTabs();
});

function checkAuth() {
    // Check if we are in the callback from OIDC
    const urlParams = new URLSearchParams(window.location.search);
    const tokenParam = urlParams.get('token');

    if (tokenParam) {
        token = tokenParam;
        localStorage.setItem('demo_admin_token', token);
        // Clean URL
        window.history.replaceState({}, document.title, window.location.pathname);
    }

    if (token) {
        fetchUser();
    } else {
        showView('login-view');
    }
}

async function fetchUser() {
    try {
        const response = await fetch(`${CONFIG.API_URL}/auth/me`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        
        if (response.ok) {
            user = result.data;
            document.getElementById('user-welcome').innerText = `Bienvenido, ${user.email}`;
            showView('dashboard-view');
            loadTab('turnos');
        } else {
            logout();
        }
    } catch (err) {
        console.error('Auth error:', err);
        logout();
    }
}

function logout() {
    localStorage.removeItem('demo_admin_token');
    token = null;
    showView('login-view');
}

function showView(viewId) {
    document.querySelectorAll('.view').forEach(v => v.classList.add('hidden'));
    document.getElementById(viewId).classList.remove('hidden');
}

// Tab Logic
function setupTabs() {
    document.querySelectorAll('.tab').forEach(tab => {
        tab.onclick = () => {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            const target = tab.getAttribute('data-tab');
            loadTab(target);
        };
    });
}

function loadTab(tabName) {
    document.querySelectorAll('.tab-content').forEach(c => c.classList.add('hidden'));
    document.getElementById(`tab-${tabName}`).classList.remove('hidden');

    if (tabName === 'turnos') fetchTurnos();
    if (tabName === 'servicios') fetchServicios();
    if (tabName === 'usuarios') fetchUsuarios();
}

// API Fetches
async function fetchTurnos() {
    const data = await apiGet(`/workspaces/${CONFIG.WORKSPACE_ID}/data/turnos`);
    const tbody = document.querySelector('#turnos-table tbody');
    tbody.innerHTML = '';
    
    (data || []).forEach(t => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${new Date(t.from).toLocaleString()}</td>
            <td>${t.client_name || 'N/A'}</td>
            <td>${t.client_email || 'N/A'}</td>
            <td>${t.servicio_id || 'N/A'}</td>
            <td><button class="btn btn-sm btn-danger" onclick="deleteRecord('turnos', '${t.id}')">Borrar</button></td>
        `;
        tbody.appendChild(row);
    });
}

async function fetchServicios() {
    const data = await apiGet(`/workspaces/${CONFIG.WORKSPACE_ID}/data/servicios`);
    const tbody = document.querySelector('#servicios-table tbody');
    tbody.innerHTML = '';
    
    (data || []).forEach(s => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${s.nombre}</td>
            <td>${s.descripcion}</td>
            <td>$${s.precio}</td>
            <td><button class="btn btn-sm btn-danger" onclick="deleteRecord('servicios', '${s.id}')">Borrar</button></td>
        `;
        tbody.appendChild(row);
    });
}

async function fetchUsuarios() {
    const data = await apiGet(`/workspaces/${CONFIG.WORKSPACE_ID}/users`);
    const tbody = document.querySelector('#usuarios-table tbody');
    tbody.innerHTML = '';
    
    (data || []).forEach(u => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>${u.name || 'N/A'}</td>
            <td>${u.email}</td>
            <td>${u.role_name}</td>
            <td><button class="btn btn-sm btn-danger" onclick="removeUser('${u.id}')">Quitar</button></td>
        `;
        tbody.appendChild(row);
    });
}

// Helpers
async function apiGet(path) {
    try {
        const response = await fetch(`${CONFIG.API_URL}${path}`, {
            headers: { 'Authorization': `Bearer ${token}` }
        });
        const result = await response.json();
        return result.data;
    } catch (err) {
        console.error('API Get Error:', err);
    }
}

async function deleteRecord(table, id) {
    if (!confirm('¿Seguro que deseas borrar este registro?')) return;
    try {
        const response = await fetch(`${CONFIG.API_URL}/workspaces/${CONFIG.WORKSPACE_ID}/data/${table}/${id}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${token}` }
        });
        if (response.ok) loadTab(table);
    } catch (err) { alert('Error al borrar'); }
}

// UI Triggers
document.getElementById('login-btn').onclick = () => {
    const callbackUrl = encodeURIComponent(window.location.origin + window.location.pathname);
    window.location.href = `${CONFIG.API_URL}/auth/oidc/login?redirect=${callbackUrl}`;
};

document.getElementById('logout-btn').onclick = logout;

function showAddService() {
    openModal('Nuevo Servicio', `
        <div class="form-group">
            <label>Nombre</label>
            <input type="text" id="srv-name" required>
        </div>
        <div class="form-group">
            <label>Descripción</label>
            <input type="text" id="srv-desc">
        </div>
        <div class="form-group">
            <label>Precio</label>
            <input type="number" id="srv-price" required>
        </div>
    `, async () => {
        const data = {
            nombre: document.getElementById('srv-name').value,
            descripcion: document.getElementById('srv-desc').value,
            precio: parseFloat(document.getElementById('srv-price').value)
        };
        await apiPost(`/workspaces/${CONFIG.WORKSPACE_ID}/data/servicios`, data);
        loadTab('servicios');
    });
}

async function apiPost(path, body) {
    await fetch(`${CONFIG.API_URL}${path}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(body)
    });
}

// Modal Engine
let modalCallback = null;
function openModal(title, html, onSave) {
    document.getElementById('modal-title').innerText = title;
    document.getElementById('modal-fields').innerHTML = html;
    document.getElementById('modal-overlay').classList.remove('hidden');
    modalCallback = onSave;
}

function closeModal() {
    document.getElementById('modal-overlay').classList.add('hidden');
}

document.getElementById('modal-form').onsubmit = async (e) => {
    e.preventDefault();
    if (modalCallback) await modalCallback();
    closeModal();
};
