// apiService.js

const ApiService = (function() {
    'use strict';

    // Common function to handle fetch responses and errors
    async function handleResponse(response) {
        if (!response.ok) {
            const errorText = await response.text().catch(() => 'Failed to get error details.');
            throw new Error(`API request failed: ${response.status} ${response.statusText} - ${errorText}`);
        }
        // For JSON responses, use: return response.json();
        // For text responses (like template content or simple success messages), use:
        return response.text();
    }

    async function loadTemplate(moduleId, filename) {
        if (!moduleId || !filename) {
            throw new Error("Module ID and filename are required to load template.");
        }
        const response = await fetch(`/api/admin/modules/${moduleId}/templates/${filename}`);
        return handleResponse(response);
    }

    async function saveTemplate(moduleId, filename, content, csrfToken) {
        if (!moduleId || !filename || content === undefined || !csrfToken) {
            throw new Error("Module ID, filename, content, and CSRF token are required to save template.");
        }
        const response = await fetch(`/api/admin/modules/${moduleId}/templates/${filename}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'text/plain',
                'X-CSRF-Token': csrfToken
            },
            body: content,
        });
        return handleResponse(response); // Expects text response
    }

    async function getPreview(moduleId, filename, content) {
        if (!moduleId || !filename || content === undefined) {
            throw new Error("Module ID, filename, and content are required for preview.");
        }
        const response = await fetch(`/api/admin/preview/${moduleId}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ filename: filename, content: content }),
        });
        return handleResponse(response); // Expects HTML string as text
    }

    // Public API
    return {
        loadTemplateContent: loadTemplate,
        saveTemplateContent: saveTemplate,
        fetchPreview: getPreview
    };
})();