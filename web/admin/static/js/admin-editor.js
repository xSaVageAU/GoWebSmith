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

    // --- Event Listener Setup ---
    function setupEventListeners() {
        // File list item clicks
        if (templateList) {
            templateList.querySelectorAll('li').forEach(listItem => {
                if (listItem.dataset.filename) {
                    listItem.addEventListener('click', handleFileSelect);
                }
            });
        } else {
             console.error("Template list #template-file-list not found!");
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