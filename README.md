# GoWebSmith Project

GoWebSmith is a comprehensive Go-based ecosystem designed for modular web development. It empowers developers to build dynamic websites by creating, managing, and composing reusable web components ("modules"). The project features a powerful command-line interface (CLI) for module lifecycle management, a user-friendly Admin UI for visual editing and administration, and a dynamic web server that intelligently discovers and renders these modules.

## Core Components

1.  **Admin UI (`cmd/admin/`)**: A web-based interface running on `http://localhost:8081` for managing modules. It provides:
    *   A dashboard to view active, inactive, and soft-deleted modules.
    *   Forms for creating new modules, specifying names and custom URL slugs.
    *   An integrated code editor (CodeMirror) for editing module template files (HTML, CSS, JS, TMPL) with live preview.
    *   Functionality to add new template files to modules and remove existing ones.
    *   Options for soft-deleting (archiving) and force-deleting modules.

2.  **Builder CLI (`cmd/builder-cli/`)**: A command-line tool for scaffolding, managing, and maintaining modules. Key features include:
    *   Module creation with a standardized directory structure (`modules/{moduleID}/templates/`) and boilerplate files (e.g., `base.html`, `content.html`, `style.css`).
    *   Automatic generation and management of module metadata in JSON format (stored in `.module_metadata/`).
    *   Adding new template files to existing modules.
    *   Updating module metadata (name, slug, group, layout, description).
    *   Soft deletion (moves module files to `modules_removed/` and marks as inactive) and hard deletion (permanent removal).
    *   Purging all soft-deleted modules.
    *   Previewing module output in a browser.

3.  **Main Web Server (`cmd/server/`)**: A dynamic Go web server that serves the composed web pages. It:
    *   Runs on `https://localhost:8443` by default (configurable via `config.yaml`), using HTTPS with self-signed certificates generated if not present.
    *   Discovers active modules by reading their JSON metadata.
    *   Loads and parses module templates (HTML, CSS, TMPL).
    *   Serves module pages dynamically based on their URL slugs (e.g., `/my-module-slug`).
    *   Renders module content by processing templates in the order specified in their metadata, injecting the combined output into a base layout.
    *   Supports HTMX for enhanced client-side interactions and partial page updates.
    *   Serves global static assets and module-specific static assets.

## Features

*   **Modular Architecture**: Build websites by combining independent, reusable components.
*   **CLI for Developers**: Efficiently manage module lifecycle through a comprehensive set of commands.
*   **Visual Admin UI**: Intuitive web interface for module management and template editing with live previews.
*   **Dynamic Rendering**: Server intelligently assembles pages from modules based on metadata.
*   **Templating**: Uses Go's `html/template` package for server-side rendering.
*   **Configuration**: Server behavior (port, certificates) managed via `config.yaml`.
*   **HTMX Integration**: Enables modern, dynamic user experiences with partial page updates.
*   **Static Asset Serving**: Handles both global and module-specific static files.
*   **Metadata Driven**: Module behavior and rendering controlled by JSON metadata files.

## Project Structure

*   `cmd/admin/`: Source code for the Admin UI.
*   `cmd/builder-cli/`: Source code for the Builder CLI tool.
*   `cmd/server/`: Source code for the Main Web Server.
*   `internal/`: Shared packages:
    *   `generator/`: Module boilerplate generation.
    *   `model/`: Data structures (`Module`, `Template`).
    *   `modulemanager/`: Core logic for module management operations.
    *   `storage/`: Metadata persistence (JSON-based).
    *   `templating/`: Server-side template processing.
*   `modules/`: Root directory for active module files. Each module resides in a subdirectory named by its ID (e.g., `modules/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx/`).
*   `modules_removed/`: Directory for soft-deleted module files.
*   `.module_metadata/`: Stores JSON metadata files for each module.
*   `web/`:
    *   `admin/`: Static assets (CSS, JS) and HTML templates for the Admin UI.
    *   `static/`: Global static assets for the Main Web Server.
    *   `templates/`: Global layout templates for the Main Web Server.
*   `pkg/fsutils/`: Filesystem utility functions.
*   `config.yaml`: Configuration file for the Main Web Server.
*   `cert.pem`, `key.pem`: TLS certificate and key files (auto-generated if not present).

## Prerequisites

*   Go (version 1.24.2 or later, as specified in `go.mod`)

## Building the Project

1.  **Build the Builder CLI**:
    ```bash
    # For Windows
    go build -o builder-cli.exe ./cmd/builder-cli
    # For Linux/macOS
    go build -o builder-cli ./cmd/builder-cli
    ```

2.  **Build the Admin UI**:
    ```bash
    # For Windows
    go build -o admin.exe ./cmd/admin
    # For Linux/macOS
    go build -o admin ./cmd/admin
    ```

3.  **Build the Main Web Server**:
    ```bash
    # For Windows
    go build -o server.exe ./cmd/server
    # For Linux/macOS
    go build -o server ./cmd/server
    ```

## Running the Project

### 1. Running the Admin UI

*   Execute the built Admin UI application:
    ```bash
    # For Windows
    .\admin.exe
    # For Linux/macOS
    ./admin
    ```
*   The Admin UI runs on port `8081` by default. This can be configured in `config.yaml` under the `admin_server.port` key.
*   Access the Admin UI in your browser at: `http://localhost:{ADMIN_PORT}` (e.g., `http://localhost:8081`)

### 2. Running the Main Web Server

*   Execute the built Main Web Server application:
    ```bash
    # For Windows
    .\server.exe
    # For Linux/macOS
    ./server
    ```
*   The server will start on `https://localhost:8443` by default.
*   If `cert.pem` and `key.pem` are not found in the project root, they will be automatically generated (self-signed). Your browser will likely show a security warning for self-signed certificates; you'll need to proceed to access the site.
*   Server port and certificate paths can be configured in `config.yaml`.
*   Optionally, enable the module list page:
    ```bash
    .\server.exe -toggle-module-list
    ```

### 3. Using the Builder CLI

The Builder CLI (`builder-cli.exe` or `./builder-cli`) provides the following commands:

*   **`list`**: Lists all known modules.
    ```bash
    .\builder-cli list
    ```

*   **`create`**: Creates a new module.
    ```bash
    .\builder-cli create -name "My New Module" [-slug "my-custom-slug"]
    ```
    *   `-name`: (Required) The user-friendly name for the module.
    *   `-slug`: (Optional) A custom URL-friendly slug. If omitted, the module's UUID will be used.

*   **`update`**: Updates module metadata.
    ```bash
    .\builder-cli update -id <module-id> [-name <new-name>] [-slug <new-slug>] [-group <group>] [-layout <layout>] [-desc <description>]
    ```
    *   `-id`: (Required) The ID of the module to update.
    *   Provide at least one of the optional flags to update corresponding metadata.

*   **`delete`**: Deletes modules.
    ```bash
    # Soft delete (moves files to modules_removed/, marks inactive)
    .\builder-cli delete -id <module-id>

    # Hard delete (permanently removes files and metadata)
    .\builder-cli delete -id <module-id> --force

    # DANGER: Delete ALL modules and metadata (for development)
    .\builder-cli delete --nuke-all
    ```
    *   `-id`: (Required unless using `--nuke-all`) Comma-separated IDs of modules to delete.
    *   `--force`: (Optional) Permanently deletes files and metadata.
    *   `--nuke-all`: (Optional, DANGEROUS) Deletes all module files and metadata.

*   **`add-template`**: Adds a new template file to a module.
    ```bash
    .\builder-cli add-template -moduleId <module-id> -name <template-filename.ext>
    ```
    *   `-moduleId`: (Required) The ID of the module.
    *   `-name`: (Required) The filename for the new template (e.g., `card.html`, `custom-styles.css`).

*   **`preview`**: Generates a preview of a module and attempts to open it in the browser.
    ```bash
    .\builder-cli preview -id <module-id>
    ```

*   **`purge-removed`**: Permanently deletes all modules that have been soft-deleted.
    ```bash
    .\builder-cli purge-removed
    ```

## Configuration

The project uses a `config.yaml` file in the project root for configuration:

```yaml
# Main Web Server Configuration
server:
  port: "8443"      # Port for the HTTPS Main Web Server
  certFile: "cert.pem" # Path to the TLS certificate file for the Main Web Server
  keyFile: "key.pem"   # Path to the TLS key file for the Main Web Server

# Admin Server Configuration
admin_server:
  port: "8081"      # Port for the Admin UI HTTP server

# Add other configuration sections as needed
```
*   **Main Web Server**: Settings for the primary content-serving server. Command-line flags for `server.exe` can override these.
*   **Admin Server**: Settings for the Admin UI.

## How It Works

1.  **Module Creation (CLI/Admin UI)**:
    *   A unique ID is generated for the module.
    *   A directory `modules/{moduleID}/templates/` is created.
    *   Boilerplate template files (e.g., `base.html`, `content.html`, `style.css`) are generated within the module's `templates` directory.
    *   A JSON metadata file (`.module_metadata/{moduleID}.json`) is created, storing information like name, slug, creation/update times, active status, and a list of its template files with their rendering order.

2.  **Template Editing (Admin UI)**:
    *   Users can select a module and then a template file to edit.
    *   The content is loaded into a CodeMirror editor.
    *   Changes can be saved back to the file.
    *   A live preview updates as the user types (debounced).

3.  **Server Rendering (Main Server)**:
    *   On startup, the server scans `.module_metadata/` to discover all modules.
    *   For active modules, it parses their template files.
    *   When a request comes for a module page (e.g., `/my-module-slug`):
        *   The server identifies the module by its slug.
        *   It retrieves the module's `base.html` and its other associated templates (e.g., `content.html`, `style.css`).
        *   Sub-templates are rendered in the order specified in the module's metadata.
        *   The combined output of these sub-templates is injected into the `{{ .RenderedContent }}` placeholder within the module's `base.html`.
        *   The final HTML is sent to the client.
    *   HTMX requests can trigger partial updates, where only specific template blocks are rendered and swapped on the client side.

This modular approach allows for flexible and maintainable web development, where different parts of a page can be developed and managed independently.
