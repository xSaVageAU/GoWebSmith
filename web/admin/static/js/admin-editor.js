document.addEventListener('DOMContentLoaded', function() {
    const editorTextarea = document.getElementById('editor-content');
    const currentFilenameSpan = document.getElementById('current-filename');
    const previewPane = document.getElementById('preview-pane');
    const saveChangesButton = document.getElementById('save-changes-button');
    const saveStatusSpan = document.getElementById('save-status');
    let currentEditingFile = null;

    const editorLayoutElement = document.querySelector('.editor-layout');
    let currentModuleID = editorLayoutElement ? editorLayoutElement.dataset.moduleId : null;

    if (!currentModuleID) {
        console.error("Module ID not found. Ensure it's set as a data-module-id attribute on an element like .editor-layout");
        // Consider disabling editor functionality if moduleID is crucial
        if(editorTextarea) editorTextarea.value = "Error: Configuration problem (Module ID missing).";
        if(saveChangesButton) saveChangesButton.disabled = true;
    }

    document.querySelectorAll('.file-button').forEach(button => {
        button.addEventListener('click', async function() {
            const filename = this.dataset.filename;
            currentEditingFile = filename;
            currentFilenameSpan.textContent = filename;
            editorTextarea.value = 'Loading...';
            editorTextarea.readOnly = true;
            saveChangesButton.disabled = true;

            if (!currentModuleID) {
                editorTextarea.value = 'Error: Module ID is missing. Cannot load file.';
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
                triggerPreview();
            } catch (error) {
                editorTextarea.value = `Error loading file: ${error.message}`;
                previewPane.innerHTML = `<p class="preview-error">Error loading file for preview.</p>`;
                console.error("Error loading template:", error);
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
            previewPane.innerHTML = previewHtml;
            // For iframe: previewPane.innerHTML = `<iframe srcdoc="${escapeHtml(previewHtml)}"></iframe>`;
        } catch (error) {
            previewPane.innerHTML = `<p class="preview-error">Preview error: ${error.message}</p>`;
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
});