// editorService.js

const EditorService = (function() {
    'use strict';

    let codeMirrorInstance = null;

    function initializeCodeMirror(textareaElement) {
        if (!textareaElement) {
            console.error("Textarea element not provided to initializeCodeMirror!");
            return null;
        }
        // Ensure CodeMirror library is loaded
        if (typeof CodeMirror === 'undefined') {
             console.error("CodeMirror library not loaded. Check script includes.");
             textareaElement.value = "Error: Code editor library failed to load.";
             return null;
        }
        try {
            const cm = CodeMirror.fromTextArea(textareaElement, {
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
            console.log("CodeMirror initialized successfully via EditorService.");
            codeMirrorInstance = cm;
            return cm; // Return the instance for direct use if needed by the caller initially
        } catch (error) {
             console.error("Failed to initialize CodeMirror via EditorService:", error);
             textareaElement.value = "Error initializing code editor.";
             return null;
        }
    }

    // Public API for EditorService
    return {
        init: function(textareaElement) {
            if (!codeMirrorInstance) {
                // Initialize and store the instance if not already done
                codeMirrorInstance = initializeCodeMirror(textareaElement);
            }
            return codeMirrorInstance; // Still return for initial setup if needed by caller
        },
        getInstance: function() { // Keep for direct access if absolutely necessary
            return codeMirrorInstance;
        },
        setValue: function(content) {
            if (codeMirrorInstance) {
                codeMirrorInstance.setValue(content || '');
            }
        },
        getValue: function() {
            if (codeMirrorInstance) {
                return codeMirrorInstance.getValue();
            }
            return '';
        },
        setReadOnly: function(isReadOnly) {
            if (codeMirrorInstance) {
                codeMirrorInstance.setOption('readOnly', isReadOnly);
            }
        },
        getReadOnly: function() {
            if (codeMirrorInstance) {
                return codeMirrorInstance.getOption('readOnly');
            }
            return true; // Default to read-only if not initialized
        },
        setMode: function(mode) {
            if (codeMirrorInstance) {
                codeMirrorInstance.setOption('mode', mode);
            }
        },
        refresh: function() {
            if (codeMirrorInstance) {
                codeMirrorInstance.refresh();
            }
        },
        focus: function() {
            if (codeMirrorInstance) {
                codeMirrorInstance.focus();
            }
        },
        onInputChange: function(callback) { // Renamed from onEditorChange
            if (codeMirrorInstance) {
                codeMirrorInstance.on('change', callback);
            }
        }
    };
})();