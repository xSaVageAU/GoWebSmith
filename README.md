# Go Module Builder (Phase 1)

This document describes the `go-module-builder` project at the completion of Phase 1.

## Overview

`go-module-builder` is a Command-Line Interface (CLI) tool designed to scaffold and manage reusable web components or "modules" for a dynamic Go web application, potentially using HTMX. It helps create a consistent structure for modules, including boilerplate Go handlers, template files, and metadata.

This tool generates the individual, pluggable pieces (modules) that a main Go web server application (planned for Phase 2) can discover and use to dynamically render content.

## Features (End of Phase 1)

*   Module scaffolding with default templates (`base.html`, `style.css`) and a Go handler (`handler.go`).
*   Metadata management using JSON files.
*   Adding new template files to existing modules.
*   Soft and hard deletion of modules.
*   Purging of soft-deleted modules.
*   Basic preview generation by rendering templates to an HTML file.

## CLI Commands

The main executable is `builder-cli.exe`.

**1. `create`**

   Creates a new module with a unique ID, default directory structure, boilerplate files, and metadata.

   ```bash
   .\builder-cli.exe create -name <module-name>
   ```
   *   `-name`: (Required) The user-friendly name for the new module.

**2. `list`**

   Lists all known modules, showing their ID, Name, Status, Path, and the number of additional (non-base) templates.

   ```bash
   .\builder-cli.exe list
   ```

**3. `add-template`**

   Adds a new template file (e.g., `.html`, `.css`, `.js`) to an existing module's `templates` directory, adds basic boilerplate content, updates the module's metadata, and inserts the necessary `{{ template "..." . }}` call into the module's `base.html` if the new file is an HTML template.

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
   ```
   *   `-id`: (Required) The ID of the module to delete.
   *   `--force`: (Optional) If true, permanently deletes files and metadata immediately.

**5. `purge-removed`**

   Finds all modules currently marked with status "removed" (via soft delete) and permanently deletes their files (from `modules_removed/`) and metadata.

   ```bash
   .\builder-cli.exe purge-removed
   ```

**6. `preview`**

   Renders the specified module's templates (starting with the `page` template) using Go's `html/template` engine, saves the output to a temporary HTML file, and attempts to open it in the default web browser.

   ```bash
   .\builder-cli.exe preview -id <module-id>
   ```
   *   `-id`: (Required) The ID of the module to preview.

## Building

To build the CLI tool:

```bash
go build -o builder-cli.exe ./cmd/builder-cli
```

## Next Steps (Phase 2)

*   Develop the web server (`cmd/server`) to dynamically discover, load, and render these modules.
*   Implement the dynamic rendering based on the `Order` field in `module.json`.
*   Define a strategy for integrating and executing the `handler.go` logic from each module.
