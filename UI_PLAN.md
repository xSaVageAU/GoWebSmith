# Admin Interface Development Plan

This document outlines the phased development plan for the Go Module Builder admin interface. The admin interface will be built as a separate Go web application (`admin.exe`) for better separation of concerns.

## Overall Goal

Create a separate admin web application to manage Go modules, edit their content, and potentially integrate a live editor (like GrapesJS).

## Phase 1: Foundation & Module Viewing

**Objective:** Establish the basic admin server structure and display existing modules. **(COMPLETED)**

1.  **Project Structure Setup:** (COMPLETED)
    *   Create `cmd/admin` directory for the admin server's `main` package.
    *   Create `web/admin/templates` and `web/admin/static` directories for admin-specific assets.
    *   **Verification:** Directory structure exists.
2.  **Basic Admin Server:** (COMPLETED)
    *   Create `cmd/admin/main.go`.
    *   Implement basic server setup (`net/http`, `chi` router).
    *   Configure admin server port (e.g., `8081`, configurable).
    *   Add basic `slog` logging.
    *   Define `adminApplication` struct for dependencies.
    *   **Verification:** `go build ./cmd/admin` succeeds. `./admin.exe` runs and shows startup logs.
3.  **Admin Layout & Root Route:** (COMPLETED)
    *   Create `web/admin/templates/layout.html` (basic layout).
    *   Create `web/admin/templates/dashboard.html` (simple dashboard).
    *   Implement root handler (`/admin/` or `/`) in `cmd/admin/routes.go` to render `dashboard.html` within `layout.html`.
    *   **Verification:** Accessing the admin server URL shows the basic dashboard page.
4.  **Module Listing:** (COMPLETED)
    *   Integrate module storage access (`internal/storage.JSONStore`) into `adminApplication`.
    *   Update dashboard handler to read all modules and pass them to the template.
    *   Update `dashboard.html` to display a list/table of modules (ID, Name, Slug, Status, Template Count).
    *   **Verification:** Admin dashboard displays the list of modules from `.module_metadata/`.

## Phase 2: Core Module Actions (Refactoring & UI Stubs)

**Objective:** Enable basic module management (create, delete) via the UI, requiring refactoring of CLI logic. **(COMPLETED)**

1.  **(Prerequisite - Major Task):** Refactor module creation, deletion, and update logic from `cmd/builder-cli/main.go` into shared functions in `internal/` (e.g., `internal/modulemanager`). (COMPLETED)
    *   **Verification:** CLI commands still function correctly using the refactored logic.
2.  **Create Module UI:** (COMPLETED)
    *   Add "Create New Module" button/link on dashboard (`/admin/modules/new`).
    *   Create GET handler/template for the creation form.
    *   Create POST handler that calls the refactored creation logic.
    *   **Verification:** New modules can be created via the admin UI, and they appear in the list and the `modules/` directory.
3.  **Delete Module UI:** (COMPLETED)
    *   Add "Delete" buttons next to modules in the list.
    *   Implement POST/DELETE handlers calling the refactored deletion logic (consider soft/hard delete options).
    *   Add confirmation dialogs (client-side).
    *   **Verification:** Modules can be deleted via the admin UI, and their status/files update accordingly.
4.  **Edit Module UI (Stub):** (COMPLETED)
    *   Add "Edit Code" buttons linking to `/admin/modules/edit/{moduleId}`.
    *   Create a placeholder handler and template for this page.
    *   **Verification:** Clicking the "Edit Code" button navigates to a placeholder page.

## Phase 3: Admin Code Editor & Live Preview (Admin-Focused Rendering)

**Objective:** Implement the ability to edit module template files directly in the admin UI with a live preview. Preview rendering logic will be self-contained within the admin application.

1.  **Create Admin Editor Page (`module_editor.html`):**
    *   Design `web/admin/templates/module_editor.html` with:
        *   A section to list template files of the module.
        *   A `<textarea>` for editing file content.
        *   A "preview pane" `<div>`.
    *   **Verification:** Basic structure is in place.
2.  **Update Edit Page Handler (`moduleEditFormHandler`):**
    *   Modify `cmd/admin/routes.go` for `GET /admin/modules/edit/{moduleID}`.
    *   Handler loads module metadata (template file list).
    *   Renders `module_editor.html`, passing module ID and file list.
    *   **Verification:** Editor page loads, showing the correct module ID and its template files.
3.  **API: Fetch Template Content:**
    *   Create `GET /api/admin/modules/{moduleID}/templates/{filename}` in `cmd/admin/routes.go`.
    *   Handler reads the specified template file from disk (e.g., `modules/{moduleID}/templates/{filename}`).
    *   Returns content as plain text.
    *   **Verification:** Calling this API endpoint returns the correct file content.
4.  **API: Live Preview (Admin-Side Rendering):**
    *   Create `POST /api/admin/preview/{moduleID}` in `cmd/admin/routes.go`.
    *   Handler receives `filename` and `modifiedContent`.
    *   Loads module metadata.
    *   Reads all module template files (using `modifiedContent` for the edited file, others from disk).
    *   Reads the main server's base layout (`web/templates/layout.html`).
    *   Parses all these into a *new, temporary `html/template.Template` set* on each request.
    *   Executes the `layout.html` from this temporary set with module data.
    *   Returns the rendered HTML.
    *   **Verification:** Sending data to this API returns rendered HTML reflecting changes.
5.  **API: Save Template Content:**
    *   Create `PUT /api/admin/modules/{moduleID}/templates/{filename}` in `cmd/admin/routes.go`.
    *   Handler receives new content and writes it to the specified template file on disk.
    *   **Verification:** Changes made in the editor can be saved back to the file.
6.  **Frontend Integration (HTMX/JavaScript):**
    *   Ensure HTMX is included in `web/admin/templates/layout.html`.
    *   In `module_editor.html`:
        *   Use HTMX/JS to call the "Fetch Template Content" API when a filename is clicked and populate the textarea.
        *   Use HTMX/JS to call the "Live Preview" API on textarea changes and update the preview pane.
        *   Implement "Save" button to call the "Save Template Content" API.
    *   **Verification:** Full editor workflow: select file, content loads, edit content, preview updates, save content. Changes are reflected on the main server.
7.  **(Optional Stretch) Draft System:**
    *   Implement functionality to save/load drafts of module edits, perhaps to a `.module_drafts/` directory.
    *   Adapt preview and loading logic to consider drafts.

## Phase 4 & Beyond (Future Considerations)

*   **Live Editor (GrapesJS):** Integrate GrapesJS for visual editing. This involves significant frontend development and backend APIs for managing component structure and content, likely interacting with module templates in a more complex way.
*   **Global Assets Management:** Design and implement storage, UI, and logic for managing global templates/assets and assigning them to modules.
*   **Server Log Viewing:** Implement a mechanism to stream or display logs from the main `server.exe` within the admin interface.
*   **Authentication/Authorization:** Secure the admin interface with user login and permissions.
*   **UI/UX Improvements:** Refine the user interface based on usage and feedback.