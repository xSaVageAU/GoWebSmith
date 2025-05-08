document.addEventListener('DOMContentLoaded', function() {
    const editorTextarea = document.getElementById('editor-content');
    const currentFilenameSpan = document.getElementById('current-filename');
    const previewPane = document.getElementById('preview-pane');
    const saveChangesButton = document.getElementById('save-changes-button');
    const saveStatusSpan = document.getElementById('save-status');
    let currentEditingFile = null;
    let activeFileButton = null; // Keep track of the active file button
    const editorOverlay = document.getElementById('gws-editor-overlay-container'); // Get the overlay

    const editorLayoutElement = document.querySelector('.gws-editor-layout');
    const editorResizer = document.getElementById('gws-editor-resizer'); // Get the resizer handle
    let currentModuleID = editorLayoutElement ? editorLayoutElement.dataset.moduleId : null;

    if (!currentModuleID) {
        console.error("Module ID not found. Ensure it's set as a data-module-id attribute on an element like .gws-editor-layout");
        if(editorTextarea) editorTextarea.value = "Error: Configuration problem (Module ID missing).";
        if(saveChangesButton) saveChangesButton.disabled = true;
        if(editorOverlay) editorOverlay.classList.remove('visible'); // Ensure overlay is hidden if error
    }
    
    function showEditorOverlay() {
        if (editorOverlay) editorOverlay.classList.add('visible');
    }

    function hideEditorOverlay() {
        if (editorOverlay) editorOverlay.classList.remove('visible');
        if (activeFileButton) {
            activeFileButton.classList.remove('gws-active-file'); // Remove active state from button
            activeFileButton = null;
        }
        currentEditingFile = null; // Clear current file
        currentFilenameSpan.textContent = 'No file selected';
        editorTextarea.value = 'Select a file from the left to edit...';
        editorTextarea.readOnly = true;
        saveChangesButton.disabled = true;
    }

    document.querySelectorAll('.file-button').forEach(button => {
        button.addEventListener('click', async function() {
            const filename = this.dataset.filename;

            if (currentEditingFile === filename && editorOverlay && editorOverlay.classList.contains('visible')) {
                // Clicked on the already active file, and editor is visible: hide it
                hideEditorOverlay();
                return;
            }

            // Remove active class from previously active button
            if (activeFileButton) {
                activeFileButton.classList.remove('gws-active-file');
            }
            // Add active class to current button and store it
            this.classList.add('gws-active-file');
            activeFileButton = this;

            currentEditingFile = filename;
            currentFilenameSpan.textContent = filename;
            editorTextarea.value = 'Loading...';
            editorTextarea.readOnly = true;
            saveChangesButton.disabled = true;
            showEditorOverlay(); // Show editor as soon as a file is clicked

            if (!currentModuleID) {
                editorTextarea.value = 'Error: Module ID is missing. Cannot load file.';
                hideEditorOverlay(); // Hide overlay on error
                return;
            }

            try {
                const response = await fetch(`/api/admin/modules/${currentModuleID}/templates/${filename}`);
                if (!response.ok) {
                    throw new Error(`Failed to fetch template: ${response.status} ${response.statusText}`);
                }
                const content = await response.text();
                editorTextarea.value = content;
                editorTextarea.readOnly = false;
                saveChangesButton.disabled = false;
                triggerPreview(); // This will also show the editor overlay if not already visible
            } catch (error) {
                editorTextarea.value = `Error loading file: ${error.message}`;
                previewPane.innerHTML = `<p class="gws-preview-error">Error loading file for preview.</p>`;
                console.error("Error loading template:", error);
                // Do not hide overlay here, user might want to see the error in editor context
            }
        });
    });

    let previewTimeout;
    editorTextarea.addEventListener('keyup', () => {
        clearTimeout(previewTimeout);
        previewTimeout = setTimeout(triggerPreview, 750); // Debounce
    });

    async function triggerPreview() {
        if (!currentEditingFile || !currentModuleID) return;

        const content = editorTextarea.value;
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
            // Use iframe with srcdoc for proper style isolation
            previewPane.innerHTML = `<iframe srcdoc="${escapeHtml(previewHtml)}" style="width:100%; height:100%; border:none;"></iframe>`;
        } catch (error) {
            // Display error inside the preview pane, but not in an iframe
            previewPane.innerHTML = `<p class="gws-preview-error">Preview error: ${error.message}</p>`;
            console.error("Error triggering preview:", error);
        }
    }
    
    function escapeHtml(unsafe) {
        return unsafe
             .replace(/&/g, "&amp;") // Ensures & is written to JS file
             .replace(/</g, "&lt;")   // Ensures < is written to JS file
             .replace(/>/g, "&gt;")   // Ensures > is written to JS file
             .replace(/"/g, "&quot;") // Ensures " is written to JS file
             .replace(/'/g, "&#039;"); // Ensures &#039; is written to JS file
    }

    saveChangesButton.addEventListener('click', async () => {
        if (!currentEditingFile || editorTextarea.readOnly || !currentModuleID) {
            alert("No file selected, file is read-only, or Module ID is missing.");
            return;
        }

        const content = editorTextarea.value;
        saveChangesButton.textContent = 'Saving...';
        saveChangesButton.disabled = true;
        saveStatusSpan.textContent = '';

        try {
            const response = await fetch(`/api/admin/modules/${currentModuleID}/templates/${currentEditingFile}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'text/plain' },
                body: content,
            });
            const responseText = await response.text();
            if (!response.ok) {
                throw new Error(`Failed to save file: ${response.status} ${response.statusText} - ${responseText}`);
            }
            saveStatusSpan.textContent = responseText || "File saved successfully!";
            setTimeout(() => { saveStatusSpan.textContent = ''; }, 3000);
        } catch (error) {
            console.error("Error saving file:", error);
            saveStatusSpan.textContent = `Error: ${error.message}`;
        } finally {
            saveChangesButton.textContent = 'Save Changes';
            saveChangesButton.disabled = false;
        }
    });

    // --- Editor Resizing Logic ---
    if (editorResizer && editorOverlay) {
        let isResizing = false;
        let lastDownY = 0;
        let initialHeight = 0;

        editorResizer.addEventListener('mousedown', function(e) {
            isResizing = true;
            lastDownY = e.clientY;
            initialHeight = editorOverlay.offsetHeight;
            document.addEventListener('mousemove', handleMouseMove);
            document.addEventListener('mouseup', handleMouseUp);
            // Optional: Add class to body to prevent text selection during resize
            document.body.style.userSelect = 'none';
        });

        function handleMouseMove(e) {
            if (!isResizing) return;

            const deltaY = e.clientY - lastDownY;
            let newHeight = initialHeight - deltaY; // Dragging up increases height

            // Enforce min/max height (e.g., min 100px, max 80% of viewport)
            const minHeight = 100; // px
            const maxHeight = window.innerHeight * 0.8;

            if (newHeight < minHeight) newHeight = minHeight;
            if (newHeight > maxHeight) newHeight = maxHeight;
            
            editorOverlay.style.height = newHeight + 'px';
        }

        function handleMouseUp() {
            if (isResizing) {
                isResizing = false;
                document.removeEventListener('mousemove', handleMouseMove);
                document.removeEventListener('mouseup', handleMouseUp);
                // Optional: Remove class from body
                document.body.style.userSelect = '';
            }
        }
    }
    // --- End Editor Resizing Logic ---
});