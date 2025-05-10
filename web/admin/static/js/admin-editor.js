document.addEventListener('DOMContentLoaded', function() {
    // --- DOM Elements ---
    const editorTextarea = document.getElementById('editor-content'); // Still needed for CM init
    const currentFilenameSpan = document.getElementById('current-filename');
    const previewPane = document.getElementById('preview-pane');
    const saveChangesButton = document.getElementById('save-changes-button');
    const saveStatusSpan = document.getElementById('save-status');
    const editorOverlay = document.getElementById('gws-editor-overlay-container');
    const editorLayoutElement = document.querySelector('.gws-editor-layout');
    const editorResizer = document.getElementById('gws-editor-resizer');
    const templateList = document.getElementById('template-file-list');
    const addTemplateForm = document.getElementById('add-template-form'); // Get the new form
    const dynamicMessageContainer = document.getElementById('dynamic-message-container'); // For AJAX messages

    // --- State Variables ---
    let currentEditingFile = null;
    let activeListItem = null;
    let currentModuleID = editorLayoutElement ? editorLayoutElement.dataset.moduleId : null;
    let codeMirrorInstance = null; // To hold the CodeMirror instance
    let previewTimeout;

    // --- Initialization ---
    function initializeApp() {
        if (!checkModuleID()) return;
        codeMirrorInstance = initializeCodeMirror();
        if (!codeMirrorInstance) return; // Stop if CM failed
        setupEventListeners();
        setupResizer();
    }

    function checkModuleID() {
        if (!currentModuleID) {
            console.error("Module ID not found. Ensure it's set as a data-module-id attribute on an element like .gws-editor-layout");
            updateEditorState("Error: Configuration problem (Module ID missing).", true, 'Error');
            if(saveChangesButton) saveChangesButton.disabled = true;
            if(editorOverlay) editorOverlay.classList.remove('visible');
            return false;
        }
        return true;
    }

    function initializeCodeMirror() {
        if (!editorTextarea) {
            console.error("Textarea element #editor-content not found!");
            return null;
        }
        // Ensure CodeMirror library is loaded
        if (typeof CodeMirror === 'undefined') {
             console.error("CodeMirror library not loaded. Check script includes.");
             if(editorTextarea) editorTextarea.value = "Error: Code editor library failed to load.";
             return null;
        }
        try {
            const cm = CodeMirror.fromTextArea(editorTextarea, {
                lineNumbers: true,
                theme: 'dracula',
                mode: 'htmlmixed', // Default mode
                // Add other options like autoCloseBrackets, matchBrackets etc. if desired
                // autoCloseBrackets: true,
                // matchBrackets: true,
            });
            // Set initial state
            cm.setValue('Select a file from the left to edit...');
            cm.setOption('readOnly', true);
            console.log("CodeMirror initialized successfully.");
            return cm;
        } catch (error) {
             console.error("Failed to initialize CodeMirror:", error);
             // Fallback or error display? For now, just log.
             if(editorTextarea) editorTextarea.value = "Error initializing code editor.";
             return null;
        }
    }

    // --- UI State Management ---
    function showEditorOverlay() {
        if (editorOverlay) {
             editorOverlay.classList.add('visible');
             // Refresh CodeMirror when it becomes visible
             if (codeMirrorInstance) {
                 // Use setTimeout to ensure refresh happens after transition might start
                 setTimeout(() => {
                     try {
                         codeMirrorInstance.refresh();
                         console.log("CodeMirror refreshed after show.");
                     } catch(e) { console.error("Error refreshing CodeMirror:", e); }
                 }, 10); 
             }
        }
    }

    function hideEditorOverlay() {
        if (editorOverlay) editorOverlay.classList.remove('visible');
        if (activeListItem) {
            activeListItem.classList.remove('gws-active-file');
            activeListItem = null;
        }
        currentEditingFile = null;
        updateEditorState('Select a file from the left to edit...', true, 'No file selected');
        if(saveChangesButton) saveChangesButton.disabled = true;
    }

    // Updates CodeMirror content, readOnly state, filename display, and language mode
    function updateEditorState(content, isReadOnly, filename) {
        if(currentFilenameSpan) currentFilenameSpan.textContent = filename || 'No file selected';
        
        if (codeMirrorInstance) {
            try {
                codeMirrorInstance.setValue(content || '');
                codeMirrorInstance.setOption('readOnly', isReadOnly);
                
                // Set CodeMirror mode based on filename extension
                let mode = 'htmlmixed'; // Default
                if (filename) {
                    if (filename.endsWith('.css')) {
                        mode = 'css';
                    } else if (filename.endsWith('.js')) {
                        mode = 'javascript';
                    } else if (filename.endsWith('.xml')) {
                        mode = 'xml';
                    } // Add more modes if needed (e.g., json)
                }
                codeMirrorInstance.setOption('mode', mode);
                console.log(`CodeMirror state updated: file=${filename}, readOnly=${isReadOnly}, mode=${mode}`);
            } catch(e) {
                 console.error("Error updating CodeMirror state:", e);
                 // Fallback for safety
                 if(editorTextarea) {
                     editorTextarea.value = content || '';
                     editorTextarea.readOnly = isReadOnly;
                 }
            }
        } else if (editorTextarea) { // Fallback if CodeMirror failed
             editorTextarea.value = content || '';
             editorTextarea.readOnly = isReadOnly;
        }
    }

    // --- Event Handlers ---
    async function handleFileSelect(event) {
        const listItem = event.currentTarget; // The clicked LI element
        const filename = listItem.dataset.filename;

        if (!filename) return; // Should not happen if listener is correct

        if (currentEditingFile === filename && editorOverlay && editorOverlay.classList.contains('visible')) {
            hideEditorOverlay();
            return;
        }

        // Update active list item styling
        if (activeListItem) {
            activeListItem.classList.remove('gws-active-file');
        }
        listItem.classList.add('gws-active-file');
        activeListItem = listItem;

        currentEditingFile = filename;
        updateEditorState('Loading...', true, filename); // Update state via function
        showEditorOverlay();

        await loadFileContent(filename);
    }

    async function loadFileContent(filename) {
        if (!currentModuleID) {
            updateEditorState('Error: Module ID is missing. Cannot load file.', true, filename);
            hideEditorOverlay();
            return;
        }

        try {
            const response = await fetch(`/api/admin/modules/${currentModuleID}/templates/${filename}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch template: ${response.status} ${response.statusText}`);
            }
            const content = await response.text();
            updateEditorState(content, false, filename); // Update editor
            if(saveChangesButton) saveChangesButton.disabled = false;
            triggerPreview(); // Trigger initial preview
        } catch (error) {
            updateEditorState(`Error loading file: ${error.message}`, true, filename);
            if(previewPane) previewPane.innerHTML = `<p class="gws-preview-error">Error loading file for preview.</p>`;
            console.error("Error loading template:", error);
        }
    }

    async function triggerPreview() {
        if (!currentEditingFile || !currentModuleID || !codeMirrorInstance) return;

        const content = codeMirrorInstance.getValue(); // Get content from CodeMirror
        try {
            const response = await fetch(`/api/admin/preview/${currentModuleID}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ filename: currentEditingFile, content: content }),
            });
            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`Preview failed: ${response.status} ${response.statusText} - ${errorText}`);
            }
            const previewHtml = await response.text();
            if(previewPane) previewPane.innerHTML = `<iframe srcdoc="${escapeHtml(previewHtml)}" style="width:100%; height:100%; border:none;"></iframe>`;
        } catch (error) {
            if(previewPane) previewPane.innerHTML = `<p class="gws-preview-error">Preview error: ${error.message}</p>`;
            console.error("Error triggering preview:", error);
        }
    }

    async function saveChanges() {
        if (!currentEditingFile || !codeMirrorInstance || codeMirrorInstance.getOption('readOnly') || !currentModuleID) {
            alert("No file selected, editor is read-only, or Module ID is missing.");
            return;
        }

        const content = codeMirrorInstance.getValue(); // Get content from CodeMirror
        const csrfToken = editorLayoutElement ? editorLayoutElement.dataset.csrfToken : ''; // Read token from data attribute

        if (!csrfToken) {
             console.error("CSRF token not found in data attribute.");
             alert("Security token missing. Cannot save.");
             // Re-enable button if needed, or handle differently
             if(saveChangesButton) {
                 saveChangesButton.textContent = 'Save Changes';
                 saveChangesButton.disabled = false;
             }
             return;
        }

        if(saveChangesButton) {
            saveChangesButton.textContent = 'Saving...';
            saveChangesButton.disabled = true;
        }
        if(saveStatusSpan) saveStatusSpan.textContent = '';

        try {
            const response = await fetch(`/api/admin/modules/${currentModuleID}/templates/${currentEditingFile}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'text/plain',
                    'X-CSRF-Token': csrfToken // Add CSRF token header
                },
                body: content,
            });
            const responseText = await response.text();
            if (!response.ok) {
                throw new Error(`Failed to save file: ${response.status} ${response.statusText} - ${responseText}`);
            }
            if(saveStatusSpan) {
                 saveStatusSpan.textContent = responseText || "File saved successfully!";
                 setTimeout(() => { if(saveStatusSpan) saveStatusSpan.textContent = ''; }, 3000);
            }
        } catch (error) {
            console.error("Error saving file:", error);
            if(saveStatusSpan) saveStatusSpan.textContent = `Error: ${error.message}`;
        } finally {
            if(saveChangesButton) {
                 saveChangesButton.textContent = 'Save Changes';
                 saveChangesButton.disabled = false;
            }
        }
    }

    // --- Utility Functions ---
    function escapeHtml(unsafe) {
        // Basic escape, sufficient for srcdoc attribute
        return unsafe
             .replace(/&/g, "&amp;")
             .replace(/</g, "&lt;")
             .replace(/>/g, "&gt;")
             .replace(/"/g, "&quot;") // Corrected
             .replace(/'/g, "&#039;"); // Corrected
    }

    function displayDynamicMessage(message, type = 'success') {
        if (!dynamicMessageContainer) {
            console.warn("Dynamic message container not found. Message:", message);
            alert(`${type.toUpperCase()}: ${message}`); // Fallback to alert
            return;
        }
        // Clear previous messages
        dynamicMessageContainer.innerHTML = '';
        const messageP = document.createElement('p');
        // Apply classes for styling based on admin-main.css and module_editor.html
        messageP.style.padding = "0.5em";
        messageP.style.border = `1px solid var(--${type === 'error' ? 'red' : 'green'})`;
        messageP.style.backgroundColor = `rgba(${type === 'error' ? '244,63,94,0.1' : '16,185,129,0.1'})`;
        messageP.style.color = `var(--${type === 'error' ? 'red' : 'green'})`;
        if (type === 'error') {
            messageP.innerHTML = `<strong>Error:</strong> ${escapeHtml(message)}`;
        } else {
            messageP.innerHTML = `<strong>Success:</strong> ${escapeHtml(message)}`;
        }
        
        dynamicMessageContainer.appendChild(messageP);
        dynamicMessageContainer.style.display = 'block';
        dynamicMessageContainer.style.marginBottom = '1rem'; // Match existing message style

        setTimeout(() => {
            if (dynamicMessageContainer) {
                dynamicMessageContainer.style.display = 'none';
                dynamicMessageContainer.innerHTML = '';
            }
        }, 5000); // Hide after 5 seconds
    }

    function renderTemplateList(templates) {
        if (!templateList) {
            console.error("templateList element not found for rendering.");
            return;
        }
        templateList.innerHTML = ''; // Clear existing list

        if (!templates || templates.length === 0) {
            const li = document.createElement('li');
            li.textContent = 'No templates found for this module.';
            templateList.appendChild(li);
            return;
        }

        templates.forEach(tmpl => {
            const li = document.createElement('li');
            li.dataset.filename = tmpl.name; // CORRECTED: Use tmpl.name (from JSON)
            
            const nameSpan = document.createElement('span');
            nameSpan.textContent = tmpl.name; // CORRECTED: Use tmpl.name
            
            if (tmpl.isBase) { // CORRECTED: Use tmpl.isBase (from JSON)
                const badge = document.createElement('span');
                badge.className = 'gws-base-badge';
                badge.textContent = 'Base';
                nameSpan.appendChild(badge);
            }
            li.appendChild(nameSpan);
            
            // Add Remove button/form (as per existing HTML structure for consistency)
            const removeForm = document.createElement('form');
            removeForm.action = `/admin/modules/edit/${currentModuleID}/remove-template/${tmpl.name}`; // CORRECTED: Use tmpl.name
            removeForm.method = 'POST';
            removeForm.className = 'gws-inline-form remove-template-form';
            removeForm.style.margin = '0';
            
            // Attach the AJAX handler directly
            removeForm.addEventListener('submit', function(e) {
                handleRemoveTemplateFormSubmit(e, tmpl.name);
            });

            const csrfInput = document.createElement('input');
            csrfInput.type = 'hidden';
            csrfInput.name = 'csrf_token';
            csrfInput.value = editorLayoutElement ? editorLayoutElement.dataset.csrfToken : '';
            removeForm.appendChild(csrfInput);

            const removeButton = document.createElement('button');
            removeButton.type = 'submit';
            removeButton.className = 'btn-danger';
            removeButton.style.fontSize = '0.7rem';
            removeButton.style.padding = '0.2rem 0.5rem';
            removeButton.style.lineHeight = '1.2';
            removeButton.textContent = 'Remove';
            removeForm.appendChild(removeButton);
            
            li.appendChild(removeForm);
            li.style.display = 'flex';
            li.style.justifyContent = 'space-between';
            li.style.alignItems = 'center';

            // Re-attach event listener for file selection to the LI element
            // Ensure clicking on the form/button doesn't trigger file select
            li.addEventListener('click', function(event) {
                if (event.target.closest('form.remove-template-form')) {
                    return;
                }
                handleFileSelect(event);
            });
            
            templateList.appendChild(li);
        });
    }

    async function handleAddTemplateFormSubmit(event) {
        event.preventDefault();
        if (!addTemplateForm || !currentModuleID) {
            console.error("Add template form or module ID not found.");
            return;
        }

        const formData = new FormData(addTemplateForm);
        const newTemplateName = formData.get('new_template_name');
        // CSRF token is already part of formData if the hidden input is named 'csrf_token'
        // const csrfToken = formData.get('csrf_token');

        if (!newTemplateName) {
            displayDynamicMessage('New template name cannot be empty.', 'error');
            return;
        }
        
        const submitButton = addTemplateForm.querySelector('button[type="submit"]');
        if(submitButton) submitButton.disabled = true;

        try {
            const response = await fetch(`/admin/modules/edit/${currentModuleID}/add-template`, {
                method: 'POST',
                // No need to set X-CSRF-Token header if it's in the form body and nosurf checks form body
                body: new URLSearchParams(formData) // Sends as x-www-form-urlencoded
            });

            const result = await response.json(); // Expecting JSON response

            if (response.ok && result.status === 'success') {
                displayDynamicMessage(result.message || 'Template added successfully!', 'success');
                if (result.data && Array.isArray(result.data)) {
                    renderTemplateList(result.data); // result.data should be the updated list of templates
                }
                addTemplateForm.reset(); // Clear the form input
            } else {
                displayDynamicMessage(result.message || `Failed to add template (HTTP ${response.status})`, 'error');
            }
        } catch (error) {
            console.error('Error adding template:', error);
            displayDynamicMessage('An unexpected error occurred while adding the template.', 'error');
        } finally {
            if(submitButton) submitButton.disabled = false;
        }
    }

    async function handleRemoveTemplateFormSubmit(event, templateFilename) {
        event.preventDefault();
        if (!currentModuleID || !templateFilename) {
            displayDynamicMessage('Module ID or template filename is missing.', 'error');
            return;
        }

        if (!confirm(`Are you sure you want to remove the template '${templateFilename}'? This action cannot be undone.`)) {
            return;
        }

        const form = event.target;
        const csrfToken = form.querySelector('input[name="csrf_token"]').value;

        if (!csrfToken) {
            displayDynamicMessage('CSRF token missing. Cannot remove template.', 'error');
            return;
        }
        
        const submitButton = form.querySelector('button[type="submit"]');
        if(submitButton) submitButton.disabled = true;

        try {
            const response = await fetch(`/admin/modules/edit/${currentModuleID}/remove-template/${templateFilename}`, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': csrfToken,
                    // Content-Type is not strictly needed for POST with no body if server doesn't require it,
                    // but for consistency with form submissions that might have bodies:
                    'Content-Type': 'application/x-www-form-urlencoded'
                },
                // No body needed for this specific remove operation as info is in URL & CSRF in header/form
            });

            const result = await response.json();

            if (response.ok && result.status === 'success') {
                displayDynamicMessage(result.message || `Template '${templateFilename}' removed successfully.`, 'success');
                if (result.data && Array.isArray(result.data)) {
                    renderTemplateList(result.data);
                } else {
                    // If data isn't returned, we might need to manually remove the item or re-fetch
                    // For now, assume data is returned. If not, this part needs adjustment.
                    console.warn("Updated template list not received in remove response, list may be stale until refresh.");
                }
            } else {
                displayDynamicMessage(result.message || `Failed to remove template '${templateFilename}'.`, 'error');
            }
        } catch (error) {
            console.error('Error removing template:', error);
            displayDynamicMessage(`An unexpected error occurred while removing '${templateFilename}'.`, 'error');
        } finally {
             if(submitButton) submitButton.disabled = false;
        }
    }


    // --- Event Listener Setup ---
    function setupEventListeners() {
        // File list item clicks - initial setup for LIs already in HTML
        // renderTemplateList will re-attach these to dynamically generated LIs
        if (templateList) {
            templateList.querySelectorAll('li[data-filename]').forEach(listItem => {
                listItem.addEventListener('click', function(event) {
                    if (event.target.closest('form.remove-template-form')) {
                        return;
                    }
                    handleFileSelect(event);
                });

                // Attach to existing remove forms on initial load
                const removeForm = listItem.querySelector('form.remove-template-form');
                if (removeForm) {
                    const filename = listItem.dataset.filename; // Get filename from parent LI
                    removeForm.addEventListener('submit', function(e) {
                        handleRemoveTemplateFormSubmit(e, filename);
                    });
                }
            });
        } else {
             console.error("Template list #template-file-list not found!");
        }

        // Add Template Form Submission
        if (addTemplateForm) {
            addTemplateForm.addEventListener('submit', handleAddTemplateFormSubmit);
        } else {
            console.warn("Add template form #add-template-form not found! Ensure it has id='add-template-form'.");
        }

        // CodeMirror changes for preview debounce
        if (codeMirrorInstance) {
            codeMirrorInstance.on('change', () => {
                clearTimeout(previewTimeout);
                // Only trigger preview if the editor isn't read-only (i.e., content loaded)
                if (!codeMirrorInstance.getOption('readOnly')) { 
                    previewTimeout = setTimeout(triggerPreview, 750); // Debounce
                }
            });
        } else if (editorTextarea) { // Fallback if CM failed
             // Keep original keyup listener if CM failed
             editorTextarea.addEventListener('keyup', () => {
                 clearTimeout(previewTimeout);
                 previewTimeout = setTimeout(triggerPreview, 750);
             });
        }

        // Save button click
        if (saveChangesButton) {
            saveChangesButton.addEventListener('click', saveChanges);
        }
    }

    // --- Resizer Logic Setup ---
    function setupResizer() {
        if (editorResizer && editorOverlay && codeMirrorInstance) { // Check CM instance too
            let isResizing = false;
            let lastDownY = 0;
            let initialHeight = 0;

            editorResizer.addEventListener('mousedown', function(e) {
                isResizing = true;
                lastDownY = e.clientY;
                initialHeight = editorOverlay.offsetHeight;
                document.addEventListener('mousemove', handleMouseMove);
                document.addEventListener('mouseup', handleMouseUp);
                document.body.style.userSelect = 'none';
            });

            function handleMouseMove(e) {
                if (!isResizing) return;
                const deltaY = e.clientY - lastDownY;
                let newHeight = initialHeight - deltaY;
                const minHeight = 100;
                const maxHeight = window.innerHeight * 0.8;
                newHeight = Math.max(minHeight, Math.min(newHeight, maxHeight)); // Clamp height
                editorOverlay.style.height = newHeight + 'px';
                codeMirrorInstance.refresh(); // Refresh CM during resize
            }

            function handleMouseUp() {
                if (isResizing) {
                    isResizing = false;
                    document.removeEventListener('mousemove', handleMouseMove);
                    document.removeEventListener('mouseup', handleMouseUp);
                    document.body.style.userSelect = '';
                    codeMirrorInstance.refresh(); // Final refresh after resize
                }
            }
        }
    }

    // --- Run Application ---
    initializeApp();

});