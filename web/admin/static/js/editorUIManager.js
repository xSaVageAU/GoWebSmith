// editorUIManager.js

const EditorUIManager = (function() {
    'use strict';

    let editorOverlayElement = null;
    let editorResizerElement = null;
    // let activeListItemRef = null; // To be managed by admin-editor.js or passed in
    // let currentEditingFileRef = null; // To be managed by admin-editor.js or passed in
    // let saveChangesButtonRef = null; // To be managed by admin-editor.js or passed in
    // let updateEditorStateCallback = function() {}; // Callback to update editor state from admin-editor.js

    // For resizer
    let isResizing = false;
    let lastDownY = 0;
    let initialHeight = 0;

    function show() {
        if (editorOverlayElement) {
            editorOverlayElement.classList.add('visible');
            // EditorService.refresh() should be called by the orchestrator (admin-editor.js)
            // after calling show, as UIManager shouldn't directly depend on EditorService.
            // However, for simplicity in this step, we might call it here if passed as a callback.
            // For now, let's assume admin-editor.js handles the refresh.
            console.log("EditorUIManager: Overlay shown.");
        }
    }

    function hide(updateEditorStateFn, saveChangesButtonEl, activeListItemElHolder) { // Pass necessary refs/callbacks
        if (editorOverlayElement) {
            editorOverlayElement.classList.remove('visible');
        }
        // Logic to reset active list item and editor state, formerly in admin-editor.js's hideEditorOverlay
        if (activeListItemElHolder && activeListItemElHolder.item) {
            activeListItemElHolder.item.classList.remove('gws-active-file');
            activeListItemElHolder.item = null;
        }
        // currentEditingFileRef should be reset by the caller (admin-editor.js)
        if (typeof updateEditorStateFn === 'function') {
            updateEditorStateFn('Select a file from the left to edit...', true, 'No file selected');
        }
        if (saveChangesButtonEl) {
            saveChangesButtonEl.disabled = true;
        }
        console.log("EditorUIManager: Overlay hidden.");
    }

    function handleMouseMove(e) {
        if (!isResizing || !editorOverlayElement) return;
        const deltaY = e.clientY - lastDownY;
        let newHeight = initialHeight - deltaY;
        const minHeight = 100; // Minimum height for the overlay
        const maxHeight = window.innerHeight * 0.8; // Max 80% of viewport height
        newHeight = Math.max(minHeight, Math.min(newHeight, maxHeight));
        editorOverlayElement.style.height = newHeight + 'px';
        
        // It's better if EditorService.refresh() is called by the main script
        // after a resize event, rather than UIManager knowing about EditorService.
        // We can dispatch a custom event here.
        editorOverlayElement.dispatchEvent(new CustomEvent('editorOverlayResized'));
    }

    function handleMouseUp() {
        if (isResizing) {
            isResizing = false;
            document.removeEventListener('mousemove', handleMouseMove);
            document.removeEventListener('mouseup', handleMouseUp);
            document.body.style.userSelect = '';
            if (editorOverlayElement) {
                 editorOverlayElement.dispatchEvent(new CustomEvent('editorOverlayResizeEnd'));
            }
        }
    }

    function initResizer() {
        if (editorResizerElement && editorOverlayElement) {
            editorResizerElement.addEventListener('mousedown', function(e) {
                isResizing = true;
                lastDownY = e.clientY;
                initialHeight = editorOverlayElement.offsetHeight;
                document.addEventListener('mousemove', handleMouseMove);
                document.addEventListener('mouseup', handleMouseUp);
                document.body.style.userSelect = 'none'; // Prevent text selection during drag
            });
            console.log("EditorUIManager: Resizer initialized.");
        } else {
            console.warn("EditorUIManager: Resizer or Overlay element not found for initResizer.");
        }
    }

    // Public API
    return {
        init: function(options) {
            editorOverlayElement = options.overlayElement;
            editorResizerElement = options.resizerElement;
            // updateEditorStateCallback = options.updateEditorStateCb; // Store callback

            if (!editorOverlayElement) {
                console.error("EditorUIManager: Overlay element not provided during init.");
                return;
            }
            if (editorResizerElement) {
                initResizer();
            } else {
                console.warn("EditorUIManager: Resizer element not provided, resizing disabled.");
            }
        },
        showOverlay: show,
        // hideOverlay needs to be called with context from admin-editor.js
        // or admin-editor.js needs to handle the state updates after calling a simpler hide()
        hideOverlay: function(updateEditorStateFn, saveChangesBtnEl, activeListItemHolder) {
             hide(updateEditorStateFn, saveChangesBtnEl, activeListItemHolder);
        }
    };
})();