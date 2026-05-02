import test from 'node:test';
import assert from 'node:assert/strict';
import { escapeHtml, csrfTokenFromCookies, normalizeToastType } from './utils.mjs';

test('escapeHtml escapes HTML special chars', () => {
	assert.equal(escapeHtml('<a>&"\'</a>'), '&lt;a&gt;&amp;&quot;&#39;&lt;/a&gt;');
});

test('escapeHtml handles nullish', () => {
	assert.equal(escapeHtml(null), '');
	assert.equal(escapeHtml(undefined), '');
});

test('csrfTokenFromCookies parses token', () => {
	const c = 'foo=bar; k8s_manager_csrf=abc123; other=1';
	assert.equal(csrfTokenFromCookies(c), 'abc123');
});

test('csrfTokenFromCookies returns empty when missing', () => {
	assert.equal(csrfTokenFromCookies(''), '');
	assert.equal(csrfTokenFromCookies('foo=bar;'), '');
});

test('normalizeToastType maps danger to error', () => {
	assert.equal(normalizeToastType('danger'), 'error');
	assert.equal(normalizeToastType('success'), 'success');
	assert.equal(normalizeToastType(undefined), 'info');
});
