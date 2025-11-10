// API 配置
const API_BASE = '/api';

// 认证相关 - 使用 cookie，浏览器自动发送
function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop().split(';').shift();
    return null;
}

function isAuthenticated() {
    return !!getCookie('session_token');
}

function clearSession() {
    // 清除 cookie
    document.cookie = 'session_token=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT;';
}

// 带认证的 fetch - cookie 会自动发送，不需要手动添加
async function authFetch(url, options = {}) {
    const response = await fetch(url, {
        ...options,
        credentials: 'same-origin', // 确保发送 cookie
    });

    if (response.status === 401) {
        clearSession();
        window.location.href = '/login';
        throw new Error('Unauthorized');
    }

    return response;
}

// Toast 通知系统
let toastContainer = null;

function initToast() {
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.className = 'toast-container';
        document.body.appendChild(toastContainer);
    }
}

function showToast(message, type = 'info', title = '', duration = 3000) {
    initToast();

    const toast = document.createElement('div');
    toast.className = `toast ${type}`;

    const icons = {
        success: '✓',
        error: '✕',
        warning: '⚠',
        info: 'ℹ'
    };

    toast.innerHTML = `
        <div class="toast-icon">${icons[type] || icons.info}</div>
        <div class="toast-content">${message}</div>
        <button class="toast-close" onclick="this.parentElement.remove()">×</button>
    `;

    toastContainer.appendChild(toast);

    // 如果duration为0，不自动关闭
    if (duration > 0) {
        setTimeout(() => {
            toast.remove();
        }, duration);
    }

    return toast;
}

// 模态框管理
function openModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.add('active');
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.remove('active');
    }
}

// 点击模态框外部关闭
document.addEventListener('click', function(event) {
    if (event.target.classList.contains('modal')) {
        event.target.classList.remove('active');
    }
});

// ESC 键关闭模态框
document.addEventListener('keydown', function(event) {
    if (event.key === 'Escape') {
        const modals = document.querySelectorAll('.modal.active');
        modals.forEach(modal => modal.classList.remove('active'));
    }
});

// 导航栏高亮当前页面
function highlightCurrentNav() {
    const currentPath = window.location.pathname;
    const navLinks = document.querySelectorAll('nav .nav-menu a');

    navLinks.forEach(link => {
        if (link.getAttribute('href') === currentPath ||
            (currentPath.includes(link.getAttribute('href')) && link.getAttribute('href') !== '/static/')) {
            link.classList.add('active');
        } else {
            link.classList.remove('active');
        }
    });
}

// 登出功能
function logout() {
    if (confirm('确定要退出登录吗？')) {
        clearSession();
        window.location.href = '/login';
    }
}

// 格式化时间
function formatTime(timeStr) {
    const date = new Date(timeStr);
    return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    });
}

// 格式化文件大小
function formatSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', async function() {
    // 检查认证状态（登录页面除外）
    if (window.location.pathname !== '/login') {
        // 先检查认证功能是否开启
        try {
            const response = await fetch('/api/auth/config');
            const data = await response.json();

            if (data.code === 0 && data.data) {
                // 如果认证功能开启，检查是否已登录
                if (data.data.auth_enabled && !isAuthenticated()) {
                    window.location.href = '/login';
                    return;
                }
            }
        } catch (error) {
            console.error('Failed to check auth config:', error);
            // 如果无法获取配置，回退到原有逻辑
            if (!isAuthenticated()) {
                window.location.href = '/login';
                return;
            }
        }
    }

    // 高亮当前导航
    highlightCurrentNav();
});
