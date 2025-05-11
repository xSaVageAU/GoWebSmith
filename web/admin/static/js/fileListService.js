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
            removeForm.action = `/admin/modules/edit/${currentModuleId}/remove-template/${tmpl.name}`;
            removeForm.method = 'POST';
            removeForm.className = 'gws-inline-form remove-template-form'; // Keep class for potential styling
            removeForm.style.margin = '0';
            
            removeForm.addEventListener('submit', function(e) {
                // Pass the event and filename to the handler now part of this service
                handleRemoveTemplateFormSubmit(e, tmpl.name); 
            });

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

            li.addEventListener('click', function(event) {
                if (event.target.closest('form.remove-template-form')) {
                    return; // Don't trigger file select if clicking on the remove form/button
                }
                // Call the provided callback when a file is selected
                if (typeof onFileSelectCallback === 'function') {
                    onFileSelectCallback(tmpl.name, event.currentTarget); // Pass the LI element too
                }
            });
            
            templateListElement.appendChild(li);
        });
    }

    async function handleAddTemplateFormSubmit(event) {
        event.preventDefault();
        if (!addTemplateFormElement || !currentModuleId) {
            console.error("Add template form or module ID not found in FileListService.");
            return;
        }

        const formData = new FormData(addTemplateFormElement);
        const newTemplateName = formData.get('new_template_name');
        
        if (!newTemplateName) {
            displayDynamicMessageCallback('New template name cannot be empty.', 'error');
            return;
        }
        
        const submitButton = addTemplateFormElement.querySelector('button[type="submit"]');
        if(submitButton) submitButton.disabled = true;

        try {
            const response = await fetch(`/admin/modules/edit/${currentModuleId}/add-template`, {
                method: 'POST',
                body: new URLSearchParams(formData) 
            });

            const result = await response.json();

            if (response.ok && result.status === 'success') {
                displayDynamicMessageCallback(result.message || 'Template added successfully!', 'success');
                if (result.data && Array.isArray(result.data)) {
                    renderTemplateList(result.data); 
                }
                addTemplateFormElement.reset();
            } else {
                displayDynamicMessageCallback(result.message || `Failed to add template (HTTP ${response.status})`, 'error');
            }
        } catch (error) {
            console.error('Error adding template:', error);
            displayDynamicMessageCallback('An unexpected error occurred while adding the template.', 'error');
        } finally {
            if(submitButton) submitButton.disabled = false;
        }
    }

    async function handleRemoveTemplateFormSubmit(event, templateFilename) {
        event.preventDefault(); 
        if (!currentModuleId || !templateFilename) {
            displayDynamicMessageCallback('Module ID or template filename is missing.', 'error');
            return;
        }

        if (!confirm(`Are you sure you want to remove the template '${templateFilename}'? This action cannot be undone.`)) {
            return; 
        }

        const form = event.target;
        const currentCsrfToken = form.querySelector('input[name="csrf_token"]').value;

        if (!currentCsrfToken) {
            displayDynamicMessageCallback('CSRF token missing. Cannot remove template.', 'error');
            return;
        }
        
        const submitButton = form.querySelector('button[type="submit"]');
        if(submitButton) submitButton.disabled = true;

        try {
            const response = await fetch(`/admin/modules/edit/${currentModuleId}/remove-template/${templateFilename}`, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': currentCsrfToken,
                    'Content-Type': 'application/x-www-form-urlencoded'
                },
            });

            const result = await response.json();

            if (response.ok && result.status === 'success') {
                displayDynamicMessageCallback(result.message || `Template '${templateFilename}' removed successfully.`, 'success');
                if (result.data && Array.isArray(result.data)) {
                    renderTemplateList(result.data);
                } else {
                    console.warn("Updated template list not received in remove response.");
                }
            } else {
                displayDynamicMessageCallback(result.message || `Failed to remove template '${templateFilename}'.`, 'error');
            }
        } catch (error) {
            console.error('Error removing template:', error);
            displayDynamicMessageCallback(`An unexpected error occurred while removing '${templateFilename}'.`, 'error');
        } finally {
             if(submitButton) submitButton.disabled = false;
        }
    }

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
                addTemplateFormElement.addEventListener('submit', handleAddTemplateFormSubmit);
            } else {
                console.warn("Add template form element not provided to FileListService.");
            }
            
            // Initial render if templates are provided
            if (options.initialTemplates && Array.isArray(options.initialTemplates)) {
                renderTemplateList(options.initialTemplates);
            }
        },
        renderList: renderTemplateList // Expose for external updates if needed
    };
})();