let currentUser = null;
let token = null;
let servicios = [];
let turnos = [];

const API_BASE = `${CONFIG.API_BASE}/workspaces/${CONFIG.WORKSPACE_ID}/data`;
const SERVER_URL = CONFIG.API_BASE.replace('/api/v1', '');

// Check for existing session
function checkSession() {
  const savedToken = localStorage.getItem('hornero_token');
  const savedUser = localStorage.getItem('hornero_user');
  
  if (savedToken && savedUser) {
    token = savedToken;
    currentUser = JSON.parse(savedUser);
    showDashboard();
    loadData();
  }
}

function startOIDCLogin() {
  // Redirect to HorneroDB OIDC login
  const redirectUri = encodeURIComponent(window.location.origin + window.location.pathname);
  window.location.href = `${SERVER_URL}/auth/oidc/login?redirect_uri=${redirectUri}`;
}

function handleOIDCCallback() {
  // Check if there's a token in the URL hash (PocketID returns token in hash)
  const hash = window.location.hash;
  if (hash && hash.includes('token=')) {
    const tokenMatch = hash.match(/token=([^&]+)/);
    if (tokenMatch) {
      token = decodeURIComponent(tokenMatch[1]);
      localStorage.setItem('hornero_token', token);
      
      // Clean URL
      window.history.replaceState(null, '', window.location.pathname);
      
      // Fetch user info
      fetchUserAndShowDashboard();
    }
  }
}

async function fetchUserAndShowDashboard() {
  try {
    const response = await fetch(`${CONFIG.API_BASE}/auth/me`, {
      headers: { 'Authorization': `Bearer ${token}` }
    });
    
    if (response.ok) {
      const userData = await response.json();
      currentUser = { email: userData.email };
      localStorage.setItem('hornero_user', JSON.stringify(currentUser));
      showDashboard();
      loadData();
    }
  } catch (error) {
    console.error('Error fetching user:', error);
    showLogin();
  }
}

function showTokenInput() {
  document.getElementById('token-input').style.display = 'block';
}

async function handleTokenLogin() {
  const tokenInput = document.getElementById('token').value.trim();
  const errorEl = document.getElementById('login-error');
  
  errorEl.style.display = 'none';
  
  if (!tokenInput) {
    errorEl.textContent = 'Por favor ingresa un token';
    errorEl.style.display = 'block';
    return;
  }
  
  try {
    const response = await fetch(`${CONFIG.API_BASE}/auth/me`, {
      headers: { 'Authorization': `Bearer ${tokenInput}` }
    });
    
    if (!response.ok) {
      throw new Error('Token inválido');
    }
    
    const userData = await response.json();
    token = tokenInput;
    currentUser = { email: userData.email };
    
    localStorage.setItem('hornero_token', token);
    localStorage.setItem('hornero_user', JSON.stringify(currentUser));
    
    document.getElementById('token').value = '';
    document.getElementById('token-input').style.display = 'none';
    
    showDashboard();
    loadData();
  } catch (error) {
    errorEl.textContent = 'Token inválido: ' + error.message;
    errorEl.style.display = 'block';
  }
}

function logout() {
  localStorage.removeItem('hornero_token');
  localStorage.removeItem('hornero_user');
  token = null;
  currentUser = null;
  showLogin();
}

function showLogin() {
  document.getElementById('login-view').style.display = 'block';
  document.getElementById('dashboard-view').style.display = 'none';
}

function showDashboard() {
  document.getElementById('login-view').style.display = 'none';
  document.getElementById('dashboard-view').style.display = 'block';
  document.getElementById('user-info').textContent = currentUser?.email || 'Admin';
}

async function apiRequest(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const headers = {
    'Content-Type': 'application/json',
    ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
    ...options.headers
  };

  const response = await fetch(url, {
    ...options,
    headers
  });

  if (!response.ok) {
    if (response.status === 401) {
      logout();
      throw new Error('Sesión expirada');
    }
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

async function loadData() {
  try {
    const [serviciosResult, turnosResult] = await Promise.all([
      apiRequest(`/${CONFIG.TABLES.SERVICIOS}?limit=100`),
      apiRequest(`/${CONFIG.TABLES.TURNOS}?limit=100`)
    ]);
    
    servicios = serviciosResult.data || [];
    turnos = turnosResult.data || [];
    
    renderTurnos();
    renderServicios();
  } catch (error) {
    console.error('Error loading data:', error);
  }
}

function renderTurnos() {
  const container = document.getElementById('turnos-list');
  
  if (!turnos.length) {
    container.innerHTML = '<p class="empty">No hay turnos registrados</p>';
    return;
  }
  
  container.innerHTML = `
    <table>
      <thead>
        <tr>
          <th>Cliente</th>
          <th>Email</th>
          <th>Teléfono</th>
          <th>Servicio</th>
          <th>Fecha</th>
          <th>Horario</th>
          <th>Estado</th>
        </tr>
      </thead>
      <tbody>
        ${turnos.map(t => `
          <tr>
            <td>${t.cliente || '-'}</td>
            <td>${t.email || '-'}</td>
            <td>${t.telefono || '-'}</td>
            <td>${t.servicio_nombre || '-'}</td>
            <td>${t.fecha || '-'}</td>
            <td>${t.from || '-'} - ${t.to || '-'}</td>
            <td>${t.estado || 'confirmado'}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function renderServicios() {
  const container = document.getElementById('servicios-list');
  
  if (!servicios.length) {
    container.innerHTML = '<p class="empty">No hay servicios configurados</p>';
    return;
  }
  
  container.innerHTML = `
    <table>
      <thead>
        <tr>
          <th>Nombre</th>
          <th>Precio</th>
          <th>Duración</th>
          <th>Acciones</th>
        </tr>
      </thead>
      <tbody>
        ${servicios.map(s => `
          <tr>
            <td>${s.nombre || '-'}</td>
            <td>$${s.precio || 0}</td>
            <td>${s.duracion_minutos || 30} min</td>
            <td>
              <button class="btn btn-sm btn-secondary" onclick="editServicio('${s.id}')">Editar</button>
              <button class="btn btn-sm btn-danger" onclick="deleteServicio('${s.id}')">Eliminar</button>
            </td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function showAddServicio() {
  document.getElementById('modal-title').textContent = 'Nuevo Servicio';
  document.getElementById('servicio-id').value = '';
  document.getElementById('servicio-form').reset();
  document.getElementById('servicio-modal').style.display = 'flex';
}

function editServicio(id) {
  const servicio = servicios.find(s => s.id === id);
  if (!servicio) return;
  
  document.getElementById('modal-title').textContent = 'Editar Servicio';
  document.getElementById('servicio-id').value = id;
  document.getElementById('servicio-nombre').value = servicio.nombre || '';
  document.getElementById('servicio-precio').value = servicio.precio || 0;
  document.getElementById('servicio-duracion').value = servicio.duracion_minutos || 30;
  document.getElementById('servicio-modal').style.display = 'flex';
}

function closeServicioModal() {
  document.getElementById('servicio-modal').style.display = 'none';
}

async function handleServicioSubmit(e) {
  e.preventDefault();
  
  const id = document.getElementById('servicio-id').value;
  const data = {
    nombre: document.getElementById('servicio-nombre').value,
    precio: parseInt(document.getElementById('servicio-precio').value),
    duracion_minutos: parseInt(document.getElementById('servicio-duracion').value)
  };
  
  try {
    if (id) {
      await apiRequest(`/${CONFIG.TABLES.SERVICIOS}/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data)
      });
    } else {
      await apiRequest(`/${CONFIG.TABLES.SERVICIOS}`, {
        method: 'POST',
        body: JSON.stringify(data)
      });
    }
    
    closeServicioModal();
    loadData();
  } catch (error) {
    alert('Error: ' + error.message);
  }
}

async function deleteServicio(id) {
  if (!confirm('¿Eliminar este servicio?')) return;
  
  try {
    await apiRequest(`/${CONFIG.TABLES.SERVICIOS}/${id}`, {
      method: 'DELETE'
    });
    loadData();
  } catch (error) {
    alert('Error: ' + error.message);
  }
}

function setupTabs() {
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');
      
      tab.classList.add('active');
      document.getElementById(`tab-${tab.dataset.tab}`).style.display = 'block';
    });
  });
}

function init() {
  document.getElementById('logout-btn').addEventListener('click', logout);
  document.getElementById('servicio-form').addEventListener('submit', handleServicioSubmit);
  
  setupTabs();
  
  // Check for OIDC callback
  if (window.location.hash && window.location.hash.includes('token=')) {
    handleOIDCCallback();
  } else {
    checkSession();
  }
}

document.addEventListener('DOMContentLoaded', init);
