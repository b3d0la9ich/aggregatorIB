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

const incidentsState = {
    page: 1,
    limit: 6,
    search: '',
    status: '',
    severity: '',
    sort: 'created_desc',
};

let currentUser = null;
let analyticsChart = null;

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

function toDateTimeLocal(value) {
    if (!value) return '';
    const dt = new Date(value);
    return new Date(dt.getTime() - dt.getTimezoneOffset() * 60000)
        .toISOString()
        .slice(0, 16);
}

function getStatusLabel(status) {
    switch (status) {
        case 'in_progress':
            return 'В работе';
        case 'closed':
            return 'Закрыт';
        default:
            return status || '—';
    }
}

function getStatusClass(status) {
    if (status === 'closed') return 'badge-closed';
    return 'badge-open';
}

function incidentCard(incident, canClose = false) {
    const severityClass = incident.severity === 'Критическое' ? 'badge-critical' : 'badge-normal';
    const statusClass = getStatusClass(incident.status);
    const statusLabel = getStatusLabel(incident.status);
    const isAdmin = currentUser?.role === 'admin';

    return `
        <article class="incident-card">
            <h3>${incident.title}</h3>
            <div>
                <span class="badge ${severityClass}">${incident.severity}</span>
                <span class="badge ${statusClass}">${statusLabel}</span>
            </div>
            <p>${incident.description}</p>
            <p><strong>Произошел:</strong> ${formatDate(incident.occurred_at)}</p>
            <p><strong>Назначен:</strong> ${incident.assigned_to?.login || '—'}</p>
            <p><strong>Создал:</strong> ${incident.created_by?.login || '—'}</p>
            <p><strong>Закрыт:</strong> ${formatDate(incident.closed_at)}</p>

            <div style="display:flex; gap:8px; flex-wrap:wrap; margin-top:12px;">
                ${canClose && incident.status !== 'closed' ? `<button onclick="changeStatus(${incident.id}, 'closed')">Закрыть инцидент</button>` : ''}
                <button onclick="toggleIncidentDetails(${incident.id})">История и комментарии</button>
                ${isAdmin ? `<button onclick="window.location.href='/incident?id=${incident.id}'">Редактировать</button>` : ''}
                ${isAdmin ? `<button onclick="deleteIncident(${incident.id})">Удалить</button>` : ''}
            </div>

            <div id="details-${incident.id}" class="details-block" style="display:none;">
                <div>
                    <strong>История действий</strong>
                    <div id="history-${incident.id}" class="history-list"></div>
                </div>
                <div style="margin-top:16px;">
                    <strong>Комментарии</strong>
                    <div id="comments-${incident.id}" class="comments-list"></div>
                    <div class="comment-box">
                        <input type="text" id="comment-input-${incident.id}" placeholder="Введите комментарий">
                        <button onclick="addComment(${incident.id})">Отправить</button>
                    </div>
                </div>
            </div>
        </article>
    `;
}

async function closeIncident(id) {
    try {
        await api.request(`/api/incidents/close?id=${id}`, { method: 'POST' });
        await Promise.all([loadIncidents(), loadTasks(), loadAnalytics(), loadNotifications(), loadChart()]);
    } catch (error) {
        alert(error.message);
    }
}

async function changeStatus(id, status) {
    try {
        await api.request(`/api/incidents/status?id=${id}&status=${status}`, { method: 'POST' });
        await Promise.all([loadIncidents(), loadTasks(), loadAnalytics(), loadNotifications(), loadChart()]);
    } catch (error) {
        alert(error.message);
    }
}

async function deleteIncident(id) {
    const confirmed = confirm('Удалить инцидент? Это действие нельзя отменить.');
    if (!confirmed) return;

    try {
        await api.request(`/api/incidents/delete?id=${id}`, { method: 'POST' });
        await Promise.all([loadIncidents(), loadTasks(), loadAnalytics(), loadNotifications(), loadChart()]);
    } catch (error) {
        alert(error.message);
    }
}

async function toggleIncidentDetails(id) {
    const details = document.getElementById(`details-${id}`);
    if (!details) return;

    const isHidden = details.style.display === 'none' || details.style.display === '';
    details.style.display = isHidden ? 'block' : 'none';

    if (isHidden) {
        await Promise.all([
            loadIncidentHistory(id),
            loadIncidentComments(id),
        ]);
    }
}

async function loadIncidentHistory(id) {
    const target = document.getElementById(`history-${id}`);
    if (!target) return;

    try {
        const history = await api.request(`/api/incidents/history?incident_id=${id}`, { method: 'GET' });
        if (!history.length) {
            target.innerHTML = '<div class="empty">История пока пуста.</div>';
            return;
        }

        target.innerHTML = history.map((item) => `
            <div class="history-item">
                <div><strong>${item.user?.login || 'Пользователь'}</strong> — ${item.action}</div>
                <div class="muted">${formatDate(item.created_at)}</div>
            </div>
        `).join('');
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function loadIncidentComments(id) {
    const target = document.getElementById(`comments-${id}`);
    if (!target) return;

    try {
        const comments = await api.request(`/api/incidents/comments?incident_id=${id}`, { method: 'GET' });
        if (!comments.length) {
            target.innerHTML = '<div class="empty">Комментариев пока нет.</div>';
            return;
        }

        target.innerHTML = comments.map((item) => `
            <div class="comment-item">
                <div><strong>${item.user?.login || 'Пользователь'}</strong></div>
                <div>${item.text}</div>
                <div class="muted">${formatDate(item.created_at)}</div>
            </div>
        `).join('');
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function addComment(incidentId) {
    const input = document.getElementById(`comment-input-${incidentId}`);
    if (!input) return;

    const text = input.value.trim();
    if (!text) {
        alert('Введите комментарий');
        return;
    }

    try {
        await api.request('/api/incidents/comments', {
            method: 'POST',
            body: JSON.stringify({
                incident_id: incidentId,
                text,
            }),
        });

        input.value = '';
        await Promise.all([
            loadIncidentComments(incidentId),
            loadIncidentHistory(incidentId),
            loadNotifications(),
        ]);
    } catch (error) {
        alert(error.message);
    }
}

async function loadNotifications() {
    const target = document.getElementById('notifications-list');
    if (!target) return;

    try {
        const notifications = await api.request('/api/notifications', { method: 'GET' });
        if (!notifications.length) {
            target.innerHTML = '<div class="empty">Уведомлений нет.</div>';
            return;
        }

        target.innerHTML = notifications.map((item) => `
            <div class="notification-item">
                <div>${item.text}</div>
                <div class="muted">${formatDate(item.created_at)}</div>
                ${!item.is_read ? `<button onclick="markNotificationRead(${item.id})">Отметить прочитанным</button>` : '<div class="muted">Прочитано</div>'}
            </div>
        `).join('');
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function markNotificationRead(id) {
    try {
        await api.request(`/api/notifications?id=${id}`, { method: 'POST' });
        await loadNotifications();
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
    currentUser = user;

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
            <div class="card"><div class="muted">В работе</div><div class="stat-number">${stats.open}</div></div>
            <div class="card"><div class="muted">Критических</div><div class="stat-number">${stats.critical}</div></div>
            <div class="card"><div class="muted">Закрытых</div><div class="stat-number">${stats.closed}</div></div>
        `;
    } catch (error) {
        target.innerHTML = `<div class="card">${error.message}</div>`;
    }
}

async function loadChart() {
    const canvas = document.getElementById('chart');
    if (!canvas || typeof Chart === 'undefined') return;

    try {
        const data = await api.request('/api/analytics', { method: 'GET' });

        if (analyticsChart) {
            analyticsChart.destroy();
        }

        analyticsChart = new Chart(canvas, {
            type: 'bar',
            data: {
                labels: ['Всего', 'В работе', 'Критические', 'Закрытые'],
                datasets: [{
                    label: 'Количество инцидентов',
                    data: [data.total, data.open, data.critical, data.closed],
                    borderWidth: 1,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
            },
        });
    } catch (error) {
        console.error('Ошибка загрузки графика:', error.message);
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

function buildIncidentsQuery() {
    const params = new URLSearchParams();

    params.set('page', String(incidentsState.page));
    params.set('limit', String(incidentsState.limit));

    if (incidentsState.search) params.set('search', incidentsState.search);
    if (incidentsState.status) params.set('status', incidentsState.status);
    if (incidentsState.severity) params.set('severity', incidentsState.severity);
    if (incidentsState.sort) params.set('sort', incidentsState.sort);

    return `/api/incidents?${params.toString()}`;
}

function renderPagination(meta) {
    const target = document.getElementById('pagination');
    if (!target) return;

    const page = meta.page || 1;
    const totalPages = meta.total_pages || 1;

    target.innerHTML = `
        <button ${page <= 1 ? 'disabled' : ''} id="prev-page-btn">← Назад</button>
        <span class="pagination-info">Страница ${page} из ${totalPages}</span>
        <button ${page >= totalPages ? 'disabled' : ''} id="next-page-btn">Вперёд →</button>
    `;

    const prevBtn = document.getElementById('prev-page-btn');
    const nextBtn = document.getElementById('next-page-btn');

    if (prevBtn) {
        prevBtn.addEventListener('click', async () => {
            if (incidentsState.page > 1) {
                incidentsState.page -= 1;
                await loadIncidents();
            }
        });
    }

    if (nextBtn) {
        nextBtn.addEventListener('click', async () => {
            if (incidentsState.page < totalPages) {
                incidentsState.page += 1;
                await loadIncidents();
            }
        });
    }
}

function initIncidentsFilters() {
    const searchInput = document.getElementById('filter-search');
    const statusSelect = document.getElementById('filter-status');
    const severitySelect = document.getElementById('filter-severity');
    const sortSelect = document.getElementById('filter-sort');
    const applyBtn = document.getElementById('apply-filters-btn');

    if (!searchInput || !statusSelect || !severitySelect || !sortSelect || !applyBtn) return;

    applyBtn.addEventListener('click', async () => {
        incidentsState.search = searchInput.value.trim();
        incidentsState.status = statusSelect.value;
        incidentsState.severity = severitySelect.value;
        incidentsState.sort = sortSelect.value;
        incidentsState.page = 1;
        await loadIncidents();
    });

    searchInput.addEventListener('keydown', async (event) => {
        if (event.key === 'Enter') {
            incidentsState.search = searchInput.value.trim();
            incidentsState.status = statusSelect.value;
            incidentsState.severity = severitySelect.value;
            incidentsState.sort = sortSelect.value;
            incidentsState.page = 1;
            await loadIncidents();
        }
    });
}

async function loadIncidents() {
    const target = document.getElementById('incidents-list');
    const metaTarget = document.getElementById('incidents-meta');
    if (!target) return;

    try {
        const user = currentUser || await api.request('/api/me', { method: 'GET' });
        const response = await api.request(buildIncidentsQuery(), { method: 'GET' });
        const incidents = response.items || [];

        if (metaTarget) {
            metaTarget.textContent = `Найдено инцидентов: ${response.total || 0}`;
        }

        if (!incidents.length) {
            target.innerHTML = '<div class="empty">По вашему запросу ничего не найдено.</div>';
            renderPagination(response);
            return;
        }

        target.innerHTML = incidents
            .map((item) => incidentCard(item, user.role === 'admin' || item.assigned_to_id === user.id))
            .join('');

        renderPagination(response);
    } catch (error) {
        target.innerHTML = `<div class="empty">${error.message}</div>`;
    }
}

async function loadIncidentForEdit() {
    const form = document.getElementById('incident-form');
    if (!form) return;

    const params = new URLSearchParams(window.location.search);
    const id = params.get('id');
    if (!id) return;

    try {
        const response = await api.request(`/api/incidents?id=${id}&page=1&limit=1`, { method: 'GET' });
        const incident = (response.items || [])[0];
        if (!incident) return;

        document.getElementById('incident-id').value = incident.id;
        document.getElementById('title').value = incident.title || '';
        document.getElementById('description').value = incident.description || '';
        document.getElementById('severity').value = incident.severity || 'Обычное';
        document.getElementById('occurred_at').value = toDateTimeLocal(incident.occurred_at);
        document.getElementById('assigned_to_id').value = incident.assigned_to_id || '';

        const pageTitle = document.getElementById('incident-page-title');
        if (pageTitle) pageTitle.textContent = 'Редактирование инцидента';

        const submitBtn = document.getElementById('incident-submit-btn');
        if (submitBtn) submitBtn.textContent = 'Сохранить изменения';
    } catch (error) {
        console.error(error.message);
    }
}

async function initIncidentForm(user) {
    const form = document.getElementById('incident-form');
    const message = document.getElementById('incident-message');
    if (!form) return;

    if (user.role !== 'admin') {
        form.parentElement.innerHTML = '<div class="card">Создание инцидента доступно только администратору.</div>';
        return;
    }

    try {
        const users = await api.request('/api/users', { method: 'GET' });
        const select = form.assigned_to_id;
        select.innerHTML = '<option value="">Выберите пользователя</option>';

        users
            .filter((item) => item.role === 'user' || item.role === 'admin')
            .forEach((item) => {
                const option = document.createElement('option');
                option.value = item.id;
                option.textContent = `${item.login} (${item.role})`;
                select.appendChild(option);
            });

        await loadIncidentForEdit();
    } catch (error) {
        showMessage(message, error.message, 'error');
    }

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        clearMessage(message);

        const incidentId = Number(document.getElementById('incident-id').value);

        const payload = {
            title: form.title.value.trim(),
            description: form.description.value.trim(),
            severity: form.severity.value,
            occurred_at: new Date(form.occurred_at.value).toISOString(),
            assigned_to_id: Number(form.assigned_to_id.value),
        };

        try {
            let result;

            if (incidentId) {
                result = await api.request('/api/incidents/update', {
                    method: 'POST',
                    body: JSON.stringify({
                        id: incidentId,
                        ...payload,
                    }),
                });
            } else {
                result = await api.request('/api/incidents', {
                    method: 'POST',
                    body: JSON.stringify(payload),
                });
            }

            showMessage(message, result.message, 'success');

            setTimeout(() => {
                window.location.href = '/incidents';
            }, 700);
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
        initIncidentsFilters();
        await Promise.all([
            loadAnalytics(),
            loadTasks(),
            loadIncidents(),
            loadNotifications(),
            initIncidentForm(user),
            loadChart(),
        ]);
    }
}

document.addEventListener('DOMContentLoaded', boot);