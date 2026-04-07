const api = {
    async request(url, options = {}) {
        const response = await fetch(url, {
            headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
            credentials: 'include',
            ...options,
        });

        let data = {};
        try {
            data = await response.json();
        } catch (_) {}

        if (!response.ok) {
            throw new Error(data.error || 'Ошибка запроса');
        }
        return data;
    },
};

function showMessage(node, text, type = 'error') {
    if (!node) return;
    node.textContent = text;
    node.className = `message show ${type}`;
}

function clearMessage(node) {
    if (!node) return;
    node.textContent = '';
    node.className = 'message';
}

async function ensureAuth() {
    try {
        return await api.request('/api/me', { method: 'GET' });
    } catch (_) {
        window.location.href = '/login';
        return null;
    }
}

function setActiveNav() {
    const pathname = window.location.pathname;
    document.querySelectorAll('.nav-link').forEach((link) => {
        const href = link.getAttribute('href');
        if (href === pathname) {
            link.classList.add('active');
        }
    });
}

function formatDate(value) {
    if (!value) return '—';
    return new Date(value).toLocaleString('ru-RU');
}

function incidentCard(incident, canClose = false) {
    const severityClass = incident.severity === 'Критическое' ? 'badge-critical' : 'badge-normal';
    const statusClass = incident.status === 'closed' ? 'badge-closed' : 'badge-open';

    return `
        <article class="incident-card">
            <h3>${incident.title}</h3>
            <div>
                <span class="badge ${severityClass}">${incident.severity}</span>
                <span class="badge ${statusClass}">${incident.status === 'closed' ? 'Закрыт' : 'Открыт'}</span>
            </div>
            <p>${incident.description}</p>
            <p><strong>Произошел:</strong> ${formatDate(incident.occurred_at)}</p>
            <p><strong>Назначен:</strong> ${incident.assigned_to?.login || '—'}</p>
            <p><strong>Создал:</strong> ${incident.created_by?.login || '—'}</p>
            <p><strong>Закрыт:</strong> ${formatDate(incident.closed_at)}</p>
            ${canClose && incident.status !== 'closed' ? `<button onclick="closeIncident(${incident.id})">Закрыть инцидент</button>` : ''}
        </article>
    `;
}

async function closeIncident(id) {
    try {
        await api.request(`/api/incidents/close?id=${id}`, { method: 'POST' });
        await Promise.all([loadIncidents(), loadTasks(), loadAnalytics()]);
    } catch (error) {
        alert(error.message);
    }
}

async function handleRegisterForm() {
    const form = document.getElementById('register-form');
    if (!form) return;
    const message = document.getElementById('register-message');

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        clearMessage(message);

        const payload = {
            login: form.login.value.trim(),
            password: form.password.value,
            password_confirm: form.password_confirm.value,
        };

        try {
            const result = await api.request('/api/register', {
                method: 'POST',
                body: JSON.stringify(payload),
            });
            showMessage(message, result.message, 'success');
            setTimeout(() => { window.location.href = '/login'; }, 1000);
        } catch (error) {
            showMessage(message, error.message, 'error');
        }
    });
}

async function handleLoginForm() {
    const form = document.getElementById('login-form');
    if (!form) return;
    const message = document.getElementById('login-message');

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        clearMessage(message);

        const payload = {
            login: form.login.value.trim(),
            password: form.password.value,
        };

        try {
            await api.request('/api/login', {
                method: 'POST',
                body: JSON.stringify(payload),
            });
            window.location.href = '/dashboard';
        } catch (error) {
            showMessage(message, error.message, 'error');
        }
    });
}

async function initDashboardShell() {
    if (!document.body.classList.contains('private-page')) return null;

    const user = await ensureAuth();
    if (!user) return null;

    const avatar = document.getElementById('user-avatar');
    const login = document.getElementById('user-login');
    const role = document.getElementById('user-role');
    const mobileLogin = document.getElementById('mobile-login');

    if (avatar) avatar.textContent = user.avatar;
    if (login) login.textContent = user.login;
    if (role) role.textContent = user.role === 'admin' ? 'Администратор' : 'Пользователь';
    if (mobileLogin) mobileLogin.textContent = user.login;

    const adminOnlyBlocks = document.querySelectorAll('[data-admin-only="true"]');
    adminOnlyBlocks.forEach((block) => {
        block.style.display = user.role === 'admin' ? '' : 'none';
    });

    const logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
        logoutBtn.addEventListener('click', async () => {
            await api.request('/api/logout', { method: 'POST' });
            window.location.href = '/login';
        });
    }

    const toggle = document.getElementById('sidebar-toggle');
    const sidebar = document.getElementById('sidebar');
    if (toggle && sidebar) {
        toggle.addEventListener('click', () => sidebar.classList.toggle('open'));
    }

    setActiveNav();
    return user;
}

async function loadAnalytics() {
    const target = document.getElementById('analytics-stats');
    if (!target) return;

    try {
        const stats = await api.request('/api/analytics', { method: 'GET' });
        target.innerHTML = `
            <div class="card"><div class="muted">Всего инцидентов</div><div class="stat-number">${stats.total}</div></div>
            <div class="card"><div class="muted">Открытых</div><div class="stat-number">${stats.open}</div></div>
            <div class="card"><div class="muted">Критических</div><div class="stat-number">${stats.critical}</div></div>
            <div class="card"><div class="muted">Закрытых</div><div class="stat-number">${stats.closed}</div></div>
        `;
    } catch (error) {
        target.innerHTML = `<div class="card">${error.message}</div>`;
    }
}

async function loadTasks() {
    const target = document.getElementById('tasks-list');
    if (!target) return;

    try {
        const tasks = await api.request('/api/tasks', { method: 'GET' });
        if (!tasks.length) {
            target.innerHTML = '<div class="empty">Открытых задач нет.</div>';
            return;
        }
        target.innerHTML = tasks.map((task) => incidentCard(task, true)).join('');
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function loadIncidents() {
    const target = document.getElementById('incidents-list');
    if (!target) return;

    try {
        const user = await api.request('/api/me', { method: 'GET' });
        const incidents = await api.request('/api/incidents', { method: 'GET' });
        if (!incidents.length) {
            target.innerHTML = '<div class="empty">Инциденты пока отсутствуют.</div>';
            return;
        }
        target.innerHTML = incidents.map((item) => incidentCard(item, user.role === 'admin' || item.assigned_to_id === user.id)).join('');
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function initIncidentForm(user) {
    const form = document.getElementById('incident-form');
    if (!form) return;
    const message = document.getElementById('incident-message');

    if (user.role !== 'admin') {
        form.parentElement.innerHTML = '<div class="card">Создание инцидента доступно только администратору.</div>';
        return;
    }

    try {
        const users = await api.request('/api/users', { method: 'GET' });
        const select = form.assigned_to_id;
        users.filter((item) => item.role === 'user' || item.role === 'admin').forEach((item) => {
            const option = document.createElement('option');
            option.value = item.id;
            option.textContent = `${item.login} (${item.role})`;
            select.appendChild(option);
        });
    } catch (error) {
        showMessage(message, error.message, 'error');
    }

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        clearMessage(message);

        const payload = {
            title: form.title.value.trim(),
            description: form.description.value.trim(),
            severity: form.severity.value,
            occurred_at: new Date(form.occurred_at.value).toISOString(),
            assigned_to_id: Number(form.assigned_to_id.value),
        };

        try {
            const result = await api.request('/api/incidents', {
                method: 'POST',
                body: JSON.stringify(payload),
            });
            form.reset();
            showMessage(message, result.message, 'success');
            await Promise.all([loadIncidents(), loadTasks(), loadAnalytics()]);
        } catch (error) {
            showMessage(message, error.message, 'error');
        }
    });
}

async function boot() {
    await handleRegisterForm();
    await handleLoginForm();
    const user = await initDashboardShell();
    if (user) {
        await Promise.all([
            loadAnalytics(),
            loadTasks(),
            loadIncidents(),
            initIncidentForm(user),
        ]);
    }
}

document.addEventListener('DOMContentLoaded', boot);
