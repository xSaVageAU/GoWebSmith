# Admin Interface Development Plan

This document outlines the phased development plan for the Go Module Builder admin interface. The admin interface will be built as a separate Go web application (`admin.exe`) for better separation of concerns.

## Overall Goal

Create a separate admin web application to manage Go modules, edit their content, and potentially integrate a live editor (like GrapesJS).

## Phase 1: Foundation & Module Viewing

**Objective:** Establish the basic admin server structure and display existing modules.

1.  **Project Structure Setup:**
    *   Create `cmd/admin` directory for the admin server's `main` package.
    *   Create `web/admin/templates` and `web/admin/static` directories for admin-specific assets.
    *   **Verification:** Directory structure exists.
2.  **Basic Admin Server:**
    *   Create `cmd/admin/main.go`.
    *   Implement basic server setup (`net/http`, `chi` router).
    *   Configure admin server port (e.g., `8081`, configurable).
    *   Add basic `slog` logging.
    *   Define `adminApplication` struct for dependencies.
    *   **Verification:** `go build ./cmd/admin` succeeds. `./admin.exe` runs and shows startup logs.
3.  **Admin Layout & Root Route:**
    *   Create `web/admin/templates/layout.html` (basic layout).
    *   Create `web/admin/templates/dashboard.html` (simple dashboard).
    *   Implement root handler (`/admin/` or `/`) in `cmd/admin/routes.go` to render `dashboard.html` within `layout.html`.
    *   **Verification:** Accessing the admin server URL shows the basic dashboard page.
4.  **Module Listing:**
    *   Integrate module storage access (`internal/storage.JSONStore`) into `adminApplication`.
    *   Update dashboard handler to read all modules and pass them to the template.
    *   Update `dashboard.html` to display a list/table of modules (ID, Name, Slug, Status, Template Count).
    *   **Verification:** Admin dashboard displays the list of modules from `.module_metadata/`.

## Phase 2: Core Module Actions (Refactoring & UI Stubs)

**Objective:** Enable basic module management (create, delete) via the UI, requiring refactoring of CLI logic.

1.  **(Prerequisite - Major Task):** Refactor module creation, deletion, and update logic from `cmd/builder-cli/main.go` into shared functions in `internal/` (e.g., `internal/modulemanager`).
    *   **Verification:** CLI commands still function correctly using the refactored logic.
2.  **Create Module UI:**
    *   Add "Create New Module" button/link on dashboard (`/admin/modules/new`).
    *   Create GET handler/template for the creation form.
    *   Create POST handler that calls the refactored creation logic.
    *   **Verification:** New modules can be created via the admin UI, and they appear in the list and the `modules/` directory.
3.  **Delete Module UI:**
    *   Add "Delete" buttons next to modules in the list.
    *   Implement POST/DELETE handlers calling the refactored deletion logic (consider soft/hard delete options).
    *   Add confirmation dialogs (client-side).
    *   **Verification:** Modules can be deleted via the admin UI, and their status/files update accordingly.
4.  **Edit Module UI (Stub):**
    *   Add "Edit Code" buttons linking to `/admin/modules/edit/{moduleId}`.
    *   Create a placeholder handler and template for this page.
    *   **Verification:** Clicking the "Edit Code" button navigates to a placeholder page.

## Phase 3: Code Editing

**Objective:** Implement the ability to edit module template files directly in the admin UI.

1.  **Integrate Code Editor:**
    *   Choose and integrate a client-side code editor (e.g., CodeMirror, Monaco Editor) into the `/admin/modules/edit/{moduleId}` page.
2.  **File Loading/Saving:**
    *   Implement backend handlers (GET) to load the content of specific module template files (`.html`, `.css`) into the editor.
    *   Implement backend handlers (POST/PUT) to save the editor content back to the corresponding module files.
    *   **Verification:** Module template content can be viewed and modified through the admin code editor. Changes are reflected when viewing the module via the main server.

## Phase 4 & Beyond (Future Considerations)

*   **Live Editor (GrapesJS):** Integrate GrapesJS for visual editing. This involves significant frontend development and backend APIs for managing component structure and content, likely interacting with module templates in a more complex way.
*   **Global Assets Management:** Design and implement storage, UI, and logic for managing global templates/assets and assigning them to modules.
*   **Server Log Viewing:** Implement a mechanism to stream or display logs from the main `server.exe` within the admin interface.
*   **Authentication/Authorization:** Secure the admin interface with user login and permissions.
*   **UI/UX Improvements:** Refine the user interface based on usage and feedback.