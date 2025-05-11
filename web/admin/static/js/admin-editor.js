document.addEventListener('DOMContentLoaded', function() {
    // --- DOM Elements ---
    const editorTextarea = document.getElementById('editor-content'); // Still needed for CM init
    const currentFilenameSpan = document.getElementById('current-filename');
    const previewPane = document.getElementById('preview-pane');
    const saveChangesButton = document.getElementById('save-changes-button');
    // const saveStatusSpan = document.getElementById('save-status'); // No longer used
    const editorOverlay = document.getElementById('gws-editor-overlay-container');
    const editorLayoutElement = document.querySelector('.gws-editor-layout');
    const editorResizer = document.getElementById('gws-editor-resizer');
    const templateList = document.getElementById('template-file-list');
    const addTemplateForm = document.getElementById('add-template-form');
    const dynamicMessageContainer = document.getElementById('dynamic-message-container');

    // --- State Variables ---
    let currentEditingFile = null;
    let activeListItem = null;
    let currentModuleID = editorLayoutElement ? editorLayoutElement.dataset.moduleId : null;
    // let codeMirrorInstance = null; // No longer directly managed here
    let previewTimeout;

    // --- Initialization ---
    function initializeApp() {
        if (!checkModuleID()) return;
        
        const cmInstance = EditorService.init(editorTextarea);
        if (!cmInstance) {
            console.error("CodeMirror initialization failed via EditorService.");
            if(saveChangesButton) saveChangesButton.disabled = true;
            return; 
        }

        let serverRenderedTemplates = [];
        if (templateList) {
            templateList.querySelectorAll('li[data-filename]').forEach(li => {
                const name = li.dataset.filename;
                const isBase = li.querySelector('.gws-base-badge') !== null;
                serverRenderedTemplates.push({ 
                    name: name, 
                    isBase: isBase
                });
            });
        }
        
        const initialTemplatesForService = window.initialModuleTemplates || serverRenderedTemplates;
        if (window.initialModuleTemplates) {
            console.log("Using initialModuleTemplates from window object for FileListService.");
        } else if (serverRenderedTemplates.length > 0) {
            console.log("Using serverRenderedTemplates parsed from DOM for FileListService.");
        } else {
            console.log("No initial templates found for FileListService.");
        }

        FileListService.init({
            listElement: templateList,
            addFormElement: addTemplateForm,
            moduleId: currentModuleID,
            csrfToken: editorLayoutElement ? editorLayoutElement.dataset.csrfToken : '',
            editorLayoutElement: editorLayoutElement, 
            initialTemplates: initialTemplatesForService, 
            onFileSelect: handleFileSelect, 
            displayMessage: displayDynamicMessage 
        });

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

    // --- UI State Management ---
    function showEditorOverlay() {
        if (editorOverlay) {
             editorOverlay.classList.add('visible');
             setTimeout(() => { 
                 EditorService.refresh(); 
                 console.log("CodeMirror refreshed after showEditorOverlay.");
             }, 10); 
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

    function updateEditorState(content, isReadOnly, filename) {
        if(currentFilenameSpan) currentFilenameSpan.textContent = filename || 'No file selected';
        
        try {
            EditorService.setValue(content || '');
            EditorService.setReadOnly(isReadOnly);
            
            let mode = 'htmlmixed'; 
            if (filename) {
                if (filename.endsWith('.css')) mode = 'css';
                else if (filename.endsWith('.js')) mode = 'javascript';
                else if (filename.endsWith('.xml')) mode = 'xml';
            }
            EditorService.setMode(mode);
            console.log(`Editor state updated: file=${filename}, readOnly=${isReadOnly}, mode=${mode}`);
        } catch(e) { 
             console.error("Error updating editor state:", e);
             if(editorTextarea) { 
                 editorTextarea.value = content || '';
                 editorTextarea.readOnly = isReadOnly;
             }
        }
    }

    // --- Event Handlers (and callbacks for services) ---
    async function handleFileSelect(filename, listItemElement) { 
        if (!filename) return;

        if (currentEditingFile === filename && editorOverlay && editorOverlay.classList.contains('visible')) {
            hideEditorOverlay(); 
            return;
        }

        if (activeListItem) {
            activeListItem.classList.remove('gws-active-file');
        }
        if (listItemElement) { 
            listItemElement.classList.add('gws-active-file');
            activeListItem = listItemElement;
        } else {
            const items = templateList ? templateList.querySelectorAll(`li[data-filename="${filename}"]`) : [];
            if (items.length > 0) {
                 items[0].classList.add('gws-active-file');
                 activeListItem = items[0];
            }
        }

        currentEditingFile = filename;
        updateEditorState('Loading...', true, filename); 
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
            updateEditorState(content, false, filename); 
            if(saveChangesButton) saveChangesButton.disabled = false;
            triggerPreview(); 
        } catch (error) {
            updateEditorState(`Error loading file: ${error.message}`, true, filename);
            if(previewPane) previewPane.innerHTML = `<p class="gws-preview-error">Error loading file for preview.</p>`;
            console.error("Error loading template:", error);
        }
    }

    async function triggerPreview() {
        if (!currentEditingFile || !currentModuleID || !EditorService.getInstance()) return;

        const content = EditorService.getValue(); 
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
        if (!currentEditingFile || !EditorService.getInstance() || EditorService.getReadOnly() || !currentModuleID) {
            displayDynamicMessage("No file selected, editor is read-only, or Module ID is missing.", "error");
            return;
        }

        const content = EditorService.getValue(); 
        const csrfToken = editorLayoutElement ? editorLayoutElement.dataset.csrfToken : ''; 

        if (!csrfToken) {
             console.error("CSRF token not found in data attribute.");
             displayDynamicMessage("Security token missing. Cannot save.", "error");
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
        
        try {
            const response = await fetch(`/api/admin/modules/${currentModuleID}/templates/${currentEditingFile}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'text/plain',
                    'X-CSRF-Token': csrfToken 
                },
                body: content,
            });
            const responseText = await response.text();
            if (!response.ok) {
                throw new Error(`Failed to save file: ${response.status} ${response.statusText} - ${responseText}`);
            }
            displayDynamicMessage(responseText || "File saved successfully!", 'success');
        } catch (error) {
            console.error("Error saving file:", error);
            displayDynamicMessage(`Error: ${error.message}`, 'error');
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
             .replace(/"/g, "&quot;") // Ensures double quotes are properly escaped
             .replace(/'/g, "&#039;"); 
    }


    function displayDynamicMessage(message, type = 'success') {
        if (!dynamicMessageContainer) {
            console.warn("Dynamic message container not found. Message:", message);
            alert(`${type.toUpperCase()}: ${message}`); 
            return;
        }
        dynamicMessageContainer.innerHTML = '';
        const messageP = document.createElement('p');
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
        dynamicMessageContainer.style.marginBottom = '1rem'; 

        setTimeout(() => {
            if (dynamicMessageContainer) {
                dynamicMessageContainer.style.display = 'none';
                dynamicMessageContainer.innerHTML = '';
            }
        }, 5000); 
    }

    // renderTemplateList, handleAddTemplateFormSubmit, handleRemoveTemplateFormSubmit are now in FileListService

    // --- Event Listener Setup ---
    function setupEventListeners() {
        // File list item click listeners and add/remove form event listeners 
        // are now primarily handled by FileListService.init()

        // CodeMirror changes for preview debounce
        if (EditorService.getInstance()) { 
            EditorService.onInputChange(() => { 
                clearTimeout(previewTimeout);
                if (!EditorService.getReadOnly()) { 
                    previewTimeout = setTimeout(triggerPreview, 750); 
                }
            });
        } else if (editorTextarea) { 
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
        if (editorResizer && editorOverlay && EditorService.getInstance()) { 
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
                newHeight = Math.max(minHeight, Math.min(newHeight, maxHeight)); 
                editorOverlay.style.height = newHeight + 'px';
                EditorService.refresh(); 
            }

            function handleMouseUp() {
                if (isResizing) {
                    isResizing = false;
                    document.removeEventListener('mousemove', handleMouseMove);
                    document.removeEventListener('mouseup', handleMouseUp);
                    document.body.style.userSelect = '';
                    EditorService.refresh(); 
                }
            }
        }
    }

    // --- Run Application ---
    initializeApp();

});