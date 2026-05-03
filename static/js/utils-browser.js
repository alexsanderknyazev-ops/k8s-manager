/**
 * Browser entry: задаёт глобали до загрузки common.js.
 * Логика совпадает с static/js/utils.mjs (тесты запускаются по .mjs).
 */
(function (w) {
	'use strict';

	function escapeHtml(text) {
		if (text == null || text === '') {
			return '';
		}
		return String(text)
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
	}

	function csrfTokenFromCookies(cookieString) {
		if (!cookieString || typeof cookieString !== 'string') {
			return '';
		}
		const prefix = 'k8s_manager_csrf=';
		const parts = cookieString.split(';');
		for (let i = 0; i < parts.length; i++) {
			const c = parts[i].trim();
			if (c.indexOf(prefix) === 0) {
				return c.substring(prefix.length);
			}
		}
		return '';
	}

	function normalizeToastType(type) {
		let t = type || 'info';
		if (t === 'danger') {
			t = 'error';
		}
		return t;
	}

	w.escapeHtml = escapeHtml;
	w.csrfTokenFromCookies = csrfTokenFromCookies;
	w.normalizeToastType = normalizeToastType;
})(window);
