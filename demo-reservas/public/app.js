let servicios = [];
let horarios = [];
let selectedHorario = null;

const API_BASE = `${CONFIG.API_BASE}/workspaces/${CONFIG.WORKSPACE_ID}/data`;

async function apiRequest(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${CONFIG.API_KEY}`,
    ...options.headers
  };

  const response = await fetch(url, {
    ...options,
    headers
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }));
    throw new Error(error.error || `HTTP ${response.status}`);
  }

  return response.json();
}

async function loadServicios() {
  try {
    const result = await apiRequest(`/${CONFIG.TABLES.SERVICIOS}`);
    servicios = result.data || [];
    
    const select = document.getElementById('service');
    select.innerHTML = '<option value="">Seleccionar servicio</option>';
    
    servicios.forEach(servicio => {
      const option = document.createElement('option');
      option.value = servicio.id;
      option.textContent = `${servicio.nombre} - $${servicio.precio}`;
      select.appendChild(option);
    });
  } catch (error) {
    showError('Error al cargar servicios: ' + error.message);
  }
}

async function loadHorarios() {
  const container = document.getElementById('horarios');
  container.innerHTML = '<p class="loading">Cargando horarios...</p>';
  hideError();

  const fecha = document.getElementById('date').value;
  const servicioId = document.getElementById('service').value;

  if (!fecha || !servicioId) {
    container.innerHTML = '<p class="empty">Selecciona un servicio y una fecha</p>';
    return;
  }

  try {
    const [horariosResult, turnosResult] = await Promise.all([
      apiRequest(`/${CONFIG.TABLES.HORARIOS}?limit=100`),
      apiRequest(`/${CONFIG.TABLES.TURNOS}?limit=100`)
    ]);

    const turnos = (turnosResult.data || []).filter(t => t.fecha === fecha);
    const servicio = servicios.find(s => s.id === servicioId);

    renderHorarios(horariosResult.data || [], turnos, servicio);
  } catch (error) {
    showError('Error al cargar horarios: ' + error.message);
    container.innerHTML = '';
  }
}

function renderHorarios(horariosDisponibles, turnos, servicio) {
  const container = document.getElementById('horarios');
  
  if (!horariosDisponibles.length) {
    container.innerHTML = '<p class="empty">No hay horarios disponibles</p>';
    return;
  }

  container.innerHTML = '';

  const maxHorarios = 10;
  horariosDisponibles.slice(0, maxHorarios).forEach(horario => {
    const isOccupied = turnos.some(t => 
      t.from === horario.hora_inicio && t.to === horario.hora_fin
    );

    const card = document.createElement('div');
    card.className = 'horario-card';
    card.innerHTML = `
      <div class="horario-info">
        <span class="horario-time">${horario.hora_inicio} - ${horario.hora_fin}</span>
        ${servicio ? `<span class="horario-service">${servicio.nombre}</span>` : ''}
      </div>
      <span class="horario-status ${isOccupied ? 'occupied' : 'available'}">
        ${isOccupied ? 'Ocupado' : 'Disponible'}
      </span>
      ${!isOccupied ? `
        <button class="btn btn-primary" onclick="openReserva('${horario.hora_inicio}', '${horario.hora_fin}')">
          Reservar
        </button>
      ` : ''}
    `;
    container.appendChild(card);
  });
}

function openReserva(from, to) {
  selectedHorario = { from, to };
  
  const details = document.getElementById('reserva-details');
  const fecha = document.getElementById('date').value;
  const servicio = servicios.find(s => s.id === document.getElementById('service').value);
  
  details.innerHTML = `
    <p><strong>Fecha:</strong> ${fecha}</p>
    <p><strong>Horario:</strong> ${from} - ${to}</p>
    ${servicio ? `<p><strong>Servicio:</strong> ${servicio.nombre} - $${servicio.precio}</p>` : ''}
  `;

  document.getElementById('reserva-modal').style.display = 'flex';
}

function closeModal() {
  document.getElementById('reserva-modal').style.display = 'none';
  document.getElementById('reserva-form').reset();
  selectedHorario = null;
}

async function handleReserva(e) {
  e.preventDefault();

  const fecha = document.getElementById('date').value;
  const servicioId = document.getElementById('service').value;
  const servicio = servicios.find(s => s.id === servicioId);

  const reserva = {
    cliente: document.getElementById('cliente').value,
    email: document.getElementById('email').value,
    telefono: document.getElementById('telefono').value,
    servicio_id: servicioId,
    servicio_nombre: servicio?.nombre || '',
    from: selectedHorario.from,
    to: selectedHorario.to,
    fecha: fecha,
    estado: 'confirmado'
  };

  try {
    await apiRequest(`/${CONFIG.TABLES.TURNOS}`, {
      method: 'POST',
      body: JSON.stringify(reserva)
    });

    alert('¡Reserva confirmada! Te enviaremos un email de confirmación.');
    closeModal();
    loadHorarios();
  } catch (error) {
    alert('Error al realizar la reserva: ' + error.message);
  }
}

function showError(message) {
  const el = document.getElementById('error');
  el.textContent = message;
  el.style.display = 'block';
}

function hideError() {
  document.getElementById('error').style.display = 'none';
}

function init() {
  const dateInput = document.getElementById('date');
  const today = new Date().toISOString().split('T')[0];
  dateInput.min = today;
  dateInput.value = today;

  document.getElementById('reserva-form').addEventListener('submit', handleReserva);

  loadServicios().then(() => {
    loadHorarios();
  });
}

document.addEventListener('DOMContentLoaded', init);
