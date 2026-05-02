/**
 * Shared helpers for UI scripts (loaded as ES module; exposed on window from templates).
 * Covered by Node tests: static/js/utils.test.mjs
 */

/**
 * @param {unknown} text
 * @returns {string}
 */
export function escapeHtml(text) {
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

/**
 * Reads k8s_manager_csrf value from a document.cookie-style string (no DOM).
 * @param {string} cookieString
 * @returns {string}
 */
export function csrfTokenFromCookies(cookieString) {
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

/**
 * @param {string} [type]
 * @returns {'info'|'success'|'warning'|'error'}
 */
export function normalizeToastType(type) {
	let t = type || 'info';
	if (t === 'danger') {
		t = 'error';
	}
	return t;
}
