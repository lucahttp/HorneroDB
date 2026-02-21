// HorneroDB Public Demo App
document.addEventListener('DOMContentLoaded', () => {
    init();
});

let services = [];
let bookings = [];
let selectedService = null;

async function init() {
    await fetchServices();
    renderServices();
}

async function fetchServices() {
    try {
        const response = await fetch(`${CONFIG.API_URL}/workspaces/${CONFIG.WORKSPACE_ID}/data/servicios`, {
            headers: {
                'Authorization': CONFIG.API_KEY,
                'X-Workspace-ID': CONFIG.WORKSPACE_ID
            }
        });
        const result = await response.json();
        services = result.data || [];
    } catch (error) {
        console.error('Error fetching services:', error);
        document.getElementById('services-list').innerHTML = '<p class="error">Error al cargar servicios. Verifica la configuración.</p>';
    }
}

async function fetchBookings() {
    try {
        const response = await fetch(`${CONFIG.API_URL}/workspaces/${CONFIG.WORKSPACE_ID}/data/turnos`, {
            headers: {
                'Authorization': CONFIG.API_KEY,
                'X-Workspace-ID': CONFIG.WORKSPACE_ID
            }
        });
        const result = await response.json();
        bookings = result.data || [];
    } catch (error) {
        console.error('Error fetching bookings:', error);
    }
}

function renderServices() {
    const list = document.getElementById('services-list');
    list.innerHTML = '';

    if (services.length === 0) {
        list.innerHTML = '<p>No hay servicios disponibles.</p>';
        return;
    }

    services.forEach(service => {
        const card = document.createElement('div');
        card.className = 'card';
        card.innerHTML = `
            <h3>${service.nombre || service.name}</h3>
            <p>${service.descripcion || service.description || ''}</p>
            <span class="price">$${service.precio || service.price}</span>
        `;
        card.onclick = () => selectService(service);
        list.appendChild(card);
    });
}

async function selectService(service) {
    selectedService = service;
    document.getElementById('services-section').classList.add('hidden');
    document.getElementById('booking-section').classList.remove('hidden');
    document.getElementById('selected-service-info').innerText = `Servicio: ${service.nombre || service.name}`;
    
    document.getElementById('slots-grid').innerHTML = '<p class="loading">Cargando disponibilidad...</p>';
    
    await fetchBookings();
    renderSlots();
}

function renderSlots() {
    const grid = document.getElementById('slots-grid');
    grid.innerHTML = '';

    // Generar slots de ejemplo para hoy (9am a 18pm)
    const today = new Date();
    today.setHours(9, 0, 0, 0);

    for (let i = 0; i < 9; i++) {
        const from = new Date(today);
        from.setHours(today.getHours() + i);
        
        const to = new Date(from);
        to.setHours(from.getHours() + 1);

        const slotFromStr = from.toISOString();
        const slotToStr = to.toISOString();

        // Verificar si el slot está ocupado
        // OJO: Solo vemos 'from' y 'to' en los turnos por seguridad
        const isOccupied = bookings.some(b => b.from === slotFromStr);

        const slotEl = document.createElement('div');
        slotEl.className = `slot ${isOccupied ? 'occupied' : ''}`;
        slotEl.innerText = `${from.getHours()}:00`;
        
        if (!isOccupied) {
            slotEl.onclick = () => selectSlot(slotFromStr, slotToStr, slotEl);
        }
        
        grid.appendChild(slotEl);
    }
}

function selectSlot(from, to, element) {
    // Deseleccionar otros
    document.querySelectorAll('.slot').forEach(el => el.classList.remove('selected'));
    element.classList.add('selected');

    document.getElementById('slot-from').value = from;
    document.getElementById('slot-to').value = to;
    document.getElementById('booking-form-container').classList.remove('hidden');
    
    // Scroll to form
    document.getElementById('booking-form-container').scrollIntoView({ behavior: 'smooth' });
}

document.getElementById('cancel-btn').onclick = () => {
    document.getElementById('booking-section').classList.add('hidden');
    document.getElementById('services-section').classList.remove('hidden');
    document.getElementById('booking-form-container').classList.add('hidden');
    selectedService = null;
};

document.getElementById('booking-form').onsubmit = async (e) => {
    e.preventDefault();
    
    const formData = {
        from: document.getElementById('slot-from').value,
        to: document.getElementById('slot-to').value,
        client_name: document.getElementById('client_name').value,
        client_email: document.getElementById('client_email').value,
        client_phone: document.getElementById('client_phone').value,
        servicio_id: selectedService.id
    };

    try {
        const response = await fetch(`${CONFIG.API_URL}/workspaces/${CONFIG.WORKSPACE_ID}/data/turnos`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': CONFIG.API_KEY,
                'X-Workspace-ID': CONFIG.WORKSPACE_ID
            },
            body: JSON.stringify(formData)
        });

        if (response.ok) {
            alert('¡Turno reservado con éxito!');
            location.reload();
        } else {
            const err = await response.json();
            alert('Error al reservar: ' + (err.error || 'Intenta de nuevo'));
        }
    } catch (error) {
        console.error('Booking error:', error);
        alert('Ocurrió un error en la conexión.');
    }
};
