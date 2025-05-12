// fileListService.js

const FileListService = (function() {
    'use strict';

    let templateListElement = null;
    let addTemplateFormElement = null;
    let currentModuleId = null;
    let csrfToken = null;
    let editorLayoutElementRef = null; // To get CSRF token if not passed directly

    // Callback for when a file is selected from the list
    let onFileSelectCallback = function(filename) { console.warn("onFileSelectCallback not implemented in FileListService for", filename); };
    // Callback to display dynamic messages
    let displayDynamicMessageCallback = function(message, type) { console.warn(`Dynamic message: ${type} - ${message}`); alert(`${type}: ${message}`); };


    function renderTemplateList(templates) {
        if (!templateListElement) {
            console.error("templateListElement not found for rendering in FileListService.");
            return;
        }
        templateListElement.innerHTML = ''; // Clear existing list

        if (!templates || templates.length === 0) {
            const li = document.createElement('li');
            li.textContent = 'No templates found for this module.';
            templateListElement.appendChild(li);
            return;
        }

        templates.forEach(tmpl => {
            const li = document.createElement('li');
            li.dataset.filename = tmpl.name;
            
            const nameSpan = document.createElement('span');
            nameSpan.textContent = tmpl.name;
            
            if (tmpl.isBase) {
                const badge = document.createElement('span');
                badge.className = 'gws-base-badge';
                badge.textContent = 'Base';
                nameSpan.appendChild(badge); // Append badge to nameSpan
            }
            li.appendChild(nameSpan); // Append nameSpan (which might contain badge) to li
            
            const removeForm = document.createElement('form');
            // Action and method are still useful for non-JS fallback, though HTMX will override
            removeForm.action = `/admin/modules/edit/${currentModuleId}/remove-template/${tmpl.name}`;
            removeForm.method = 'POST';
            removeForm.className = 'gws-inline-form remove-template-form';
            removeForm.style.margin = '0';

            // Add HTMX attributes for AJAX submission
            removeForm.setAttribute('hx-post', `/admin/modules/edit/${currentModuleId}/remove-template/${tmpl.name}`);
            removeForm.setAttribute('hx-target', '#template-file-list');
            removeForm.setAttribute('hx-swap', 'innerHTML');
            removeForm.setAttribute('hx-confirm', `Are you sure you want to remove the template '${tmpl.name}'? This action cannot be undone.`);
            
            // The JS submit handler (handleRemoveTemplateFormSubmit) will be modified/removed later
            // as HTMX will now handle the submission. For now, let's keep it to see if HTMX intercepts.
            // If HTMX intercepts, this JS listener might not be fully executed or its fetch part will be problematic.
            // removeForm.addEventListener('submit', function(e) { // HTMX now handles this
            //     handleRemoveTemplateFormSubmit(e, tmpl.name);
            // });

            const csrfInput = document.createElement('input');
            csrfInput.type = 'hidden';
            csrfInput.name = 'csrf_token';
            csrfInput.value = csrfToken || (editorLayoutElementRef ? editorLayoutElementRef.dataset.csrfToken : '');
            removeForm.appendChild(csrfInput);

            const removeButton = document.createElement('button');
            removeButton.type = 'submit';
            removeButton.className = 'btn-danger'; // Use existing class for styling
            removeButton.style.fontSize = '0.7rem';
            removeButton.style.padding = '0.2rem 0.5rem';
            removeButton.style.lineHeight = '1.2';
            removeButton.textContent = 'Remove';
            removeForm.appendChild(removeButton);
            
            li.appendChild(removeForm);
            li.style.display = 'flex';
            li.style.justifyContent = 'space-between';
            li.style.alignItems = 'center';

            // Event listener for file selection will be delegated from templateListElement
            
            templateListElement.appendChild(li);
        });

        // After rendering the list, tell HTMX to process the new content
        if (typeof htmx !== 'undefined' && templateListElement) {
            htmx.process(templateListElement);
        }
    }

    // handleAddTemplateFormSubmit and handleRemoveTemplateFormSubmit functions
    // are no longer needed as HTMX handles these form submissions directly.
    // Their event listeners were previously commented out.

    // Public API for FileListService
    return {
        init: function(options) {
            templateListElement = options.listElement;
            addTemplateFormElement = options.addFormElement;
            currentModuleId = options.moduleId;
            csrfToken = options.csrfToken; // General CSRF for new forms
            editorLayoutElementRef = options.editorLayoutElement; // For CSRF if needed by dynamic forms
            
            if (typeof options.onFileSelect === 'function') {
                onFileSelectCallback = options.onFileSelect;
            }
            if (typeof options.displayMessage === 'function') {
                displayDynamicMessageCallback = options.displayMessage;
            }

            if (addTemplateFormElement) {
                // Event listener for addTemplateFormElement was removed as HTMX handles submission.
            } else {
                console.warn("Add template form element not provided to FileListService.");
            }
            
            // Initial render if templates are provided
            if (options.initialTemplates && Array.isArray(options.initialTemplates)) {
                renderTemplateList(options.initialTemplates);
            }

            // Event delegation for file selection
            if (templateListElement && typeof onFileSelectCallback === 'function') {
                templateListElement.addEventListener('click', function(event) {
                    const listItem = event.target.closest('li[data-filename]');
                    if (listItem && !event.target.closest('form.remove-template-form')) {
                        const filename = listItem.dataset.filename;
                        onFileSelectCallback(filename, listItem);
                    }
                });
            }
        },
        renderList: renderTemplateList // Expose for external updates if needed
    };
})();