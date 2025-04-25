# Go Module Builder & Server (Phase 2 Complete)

This document describes the `go-module-builder` project at the completion of Phase 2.

## Overview

`go-module-builder` consists of two main parts:

1.  **Builder CLI (`builder-cli.exe`):** A tool to scaffold and manage reusable web components or "modules". It creates a consistent structure, including boilerplate Go handlers, template files (`base.html`, `style.css`, etc.), and JSON metadata (`.module_metadata/`).
2.  **Web Server (`server.exe`):** A dynamic Go web server that discovers modules based on their metadata, loads their templates, and renders them according to the `Order` specified in the JSON. It supports HTMX requests for partial page updates.

This project allows for building dynamic web pages by composing independent modules managed by the CLI and rendered by the server.

## Features (End of Phase 2)

*   **Builder CLI:**
    *   Module scaffolding with default templates (`base.html`, `style.css`) using the correct server-side rendering syntax (`{{ .RenderedContent }}`).
    *   Metadata management using JSON files in `.module_metadata/`.
    *   Adding new template files (`.html`, `.css`, etc.) to existing modules and updating the module's JSON metadata with the correct `Order` for rendering.
    *   Soft and hard deletion of modules.
    *   Purging of soft-deleted modules.
*   **Web Server:**
    *   Dynamically discovers active modules by reading JSON metadata.
    *   Loads and parses module templates (`.html`, `.tmpl`, `.css`).
    *   Serves a main page listing available modules.
    *   Renders individual module pages (`/view/module/{id}`).
    *   Pre-renders sub-templates (like `card.html`) in the order specified by the `Order` field in the module's JSON metadata.
    *   Injects the ordered, pre-rendered content into the module's `base.html` via `{{ .RenderedContent }}`.
    *   Supports basic HTMX requests for loading module content into the main page.
    *   Serves static CSS files.

## Building

You need Go installed.

1.  **Build the CLI:**
    ```bash
    go build -o builder-cli.exe ./cmd/builder-cli
    ```
2.  **Build the Server:**
    ```bash
    go build -o server.exe ./cmd/server
    ```

## Running the Server

1.  Ensure you have built `server.exe`.
2.  Run the executable from the project root:
    ```bash
    .\server.exe
    ```
3.  Open your web browser and navigate to `http://localhost:8080`.

## CLI Commands

The main executable is `builder-cli.exe`.

**1. `create`**

   Creates a new module with a unique ID, default directory structure (`modules/{id}`), boilerplate files (`base.html`, `style.css`), and metadata (`.module_metadata/{id}.json`).

   ```bash
   .\builder-cli.exe create -name <module-name>
   ```
   *   `-name`: (Required) The user-friendly name for the new module.

**2. `list`**

   Lists all known modules from the metadata, showing their ID, Name, Status, Path, and the number of additional (non-base) templates.

   ```bash
   .\builder-cli.exe list
   ```

**3. `add-template`**

   Adds a new template file (e.g., `card.html`) to an existing module's `templates` directory. It creates the file with basic `{{ define "..." }}` content and updates the module's JSON metadata, adding the new template to the `templates` list with the next available `Order` number. **It no longer modifies `base.html` directly.**

   ```bash
   .\builder-cli.exe add-template -name <template-filename> -moduleId <module-id>
   ```
   *   `-name`: (Required) The filename for the new template (e.g., `card.html`).
   *   `-moduleId`: (Required) The ID of the module to add the template to.

**4. `delete`**

   Deletes a module. By default, performs a "soft delete" (moves files to `modules_removed/`, updates status in metadata). Use `--force` for immediate permanent deletion.

   ```bash
   # Soft delete (move files, mark as removed)
   .\builder-cli.exe delete -id <module-id>

   # Hard delete (permanently remove files and metadata)
   .\builder-cli.exe delete -id <module-id> --force

   # DANGER: Delete ALL modules and metadata (for development)
   .\builder-cli.exe delete --nuke-all
   ```
   *   `-id`: (Required unless using `--nuke-all`) The ID(s) of the module(s) to delete (comma-separated).
   *   `--force`: (Optional) If true, permanently deletes files and metadata immediately.
   *   `--nuke-all`: (Optional) DANGEROUS. Deletes all module files and metadata.

**5. `purge-removed`**

   Finds all modules currently marked with status "removed" (via soft delete) and permanently deletes their files (from `modules_removed/`) and metadata.

   ```bash
   .\builder-cli.exe purge-removed
   ```

**6. `update`**

   Updates a module's metadata (currently only supports changing the name).

   ```bash
   .\builder-cli.exe update -id <module-id> -name <new-name>
   ```
   *   `-id`: (Required) The ID of the module to update.
   *   `-name`: (Required) The new name for the module.

## Project Structure

*   `cmd/builder-cli`: Source code for the CLI tool.
*   `cmd/server`: Source code for the web server.
*   `internal/`: Shared packages used by CLI and server.
    *   `generator`: Code for generating module files.
    *   `model`: Data structures (e.g., `Module`, `Template`).
    *   `storage`: Handles reading/writing module metadata (JSON).
    *   `templating`: Template parsing/execution logic (used by server).
*   `modules/`: Contains the actual directories and files for active modules.
*   `modules_removed/`: Contains files for soft-deleted modules.
*   `.module_metadata/`: Stores JSON files, one for each module's metadata.
*   `web/`: Contains files used by the web server.
    *   `static/`: Static assets (e.g., CSS).
    *   `templates/`: Global templates (e.g., `layout.html`).
*   `pkg/`: Utility packages.
    *   `fsutils`: Filesystem utility functions.
